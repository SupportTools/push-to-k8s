package k8s

import (
	"context"
	"sync"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"

	"k8s.io/client-go/kubernetes/fake"
)

// seedSource stores src in the fake clientset, as it exists on a real cluster: syncSecretToNamespace re-reads
// the source before every write, so a source that is not in the API server is treated as deleted.
func seedSource(t *testing.T, c *fake.Clientset, src *v1.Secret) {
	t.Helper()
	if err := c.Tracker().Add(src.DeepCopy()); err != nil {
		t.Fatalf("seed source %s/%s: %v", src.Namespace, src.Name, err)
	}
}

const midRoundSourceNS = "push-to-k8s"

func midRoundSource(data string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "star-tls", Namespace: midRoundSourceNS,
			Labels: map[string]string{"push-to-k8s": "source"}},
		Data: map[string][]byte{"tls.crt": []byte(data)},
	}
}

// writtenData records the tls.crt of every copy SyncSecrets creates or updates, in order.
type writtenData struct {
	mu     sync.Mutex
	writes []string
}

func (w *writtenData) reactor(action k8stesting.Action) (bool, runtime.Object, error) {
	var obj runtime.Object
	switch a := action.(type) {
	case k8stesting.CreateAction:
		obj = a.GetObject()
	case k8stesting.UpdateAction:
		obj = a.GetObject()
	}
	if s, ok := obj.(*v1.Secret); ok && s.Namespace != midRoundSourceNS {
		w.mu.Lock()
		w.writes = append(w.writes, s.Namespace+"="+string(s.Data["tls.crt"]))
		w.mu.Unlock()
	}
	return false, nil, nil // fall through to the tracker
}

// cluster-services-1340, seen live 2026-11-12: a periodic round lists the sources once, then takes ~12 min to
// walk every namespace. A source update that lands in that window is pushed to every copy by the event path,
// and the round then reached each copy and wrote the stale list snapshot back over it.
//
// Here the source moves OLD -> NEW, and the event path rewrites every copy to NEW, right after the round has
// listed the sources. The round must not write OLD anywhere.
func TestSyncSecretsDoesNotRevertSourceChangedMidRound(t *testing.T) {
	targets := []string{"app-a", "app-b", "app-c"}
	objs := []runtime.Object{&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: midRoundSourceNS}}}
	for _, n := range targets {
		objs = append(objs, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: n}})
		cp := midRoundSource("OLD")
		cp.Namespace = n
		cp.Labels = map[string]string{CopyOfLabel: "star-tls"}
		objs = append(objs, cp)
	}
	objs = append(objs, midRoundSource("OLD"))
	c := fake.NewSimpleClientset(objs...)

	// The round lists the sources first, then the namespaces. Land the update between the two.
	var once sync.Once
	c.PrependReactor("list", "namespaces", func(k8stesting.Action) (bool, runtime.Object, error) {
		once.Do(func() {
			if err := c.Tracker().Update(v1.SchemeGroupVersion.WithResource("secrets"), midRoundSource("NEW"), midRoundSourceNS); err != nil {
				t.Errorf("update source mid-round: %v", err)
			}
			for _, n := range targets { // what the event path does on the update
				cp := midRoundSource("NEW")
				cp.Namespace = n
				cp.Labels = map[string]string{CopyOfLabel: "star-tls"}
				if err := c.Tracker().Update(v1.SchemeGroupVersion.WithResource("secrets"), cp, n); err != nil {
					t.Errorf("event-path write to %s: %v", n, err)
				}
			}
		})
		return false, nil, nil
	})
	w := &writtenData{}
	c.PrependReactor("create", "secrets", w.reactor)
	c.PrependReactor("update", "secrets", w.reactor)

	if err := SyncSecrets(c, midRoundSourceNS, "", newTestLogger()); err != nil {
		t.Fatal(err)
	}

	for _, wr := range w.writes {
		t.Errorf("round wrote %s after the source had moved to NEW", wr)
	}
	for _, n := range targets {
		got, err := c.CoreV1().Secrets(n).Get(context.TODO(), "star-tls", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get copy in %s: %v", n, err)
		}
		if string(got.Data["tls.crt"]) != "NEW" {
			t.Errorf("copy in %s = %q, want NEW", n, got.Data["tls.crt"])
		}
	}
}

// The source changes while the round is between two namespaces, before the event path has caught up: the
// namespaces not yet visited must get the new data, not the snapshot.
func TestSyncSecretsUsesSourceChangedBetweenNamespaces(t *testing.T) {
	targets := []string{"app-a", "app-b", "app-c"}
	objs := []runtime.Object{&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: midRoundSourceNS}}, midRoundSource("OLD")}
	for _, n := range targets {
		objs = append(objs, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: n}})
	}
	c := fake.NewSimpleClientset(objs...)

	var once sync.Once
	c.PrependReactor("create", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetNamespace() == "app-a" {
			once.Do(func() { // the first copy is written, then the source changes
				if err := c.Tracker().Update(v1.SchemeGroupVersion.WithResource("secrets"), midRoundSource("NEW"), midRoundSourceNS); err != nil {
					t.Errorf("update source mid-round: %v", err)
				}
			})
		}
		return false, nil, nil
	})

	if err := SyncSecrets(c, midRoundSourceNS, "", newTestLogger()); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"app-b", "app-c"} {
		got, err := c.CoreV1().Secrets(n).Get(context.TODO(), "star-tls", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get copy in %s: %v", n, err)
		}
		if string(got.Data["tls.crt"]) != "NEW" {
			t.Errorf("copy in %s = %q, want NEW (round used its stale list snapshot)", n, got.Data["tls.crt"])
		}
	}
}

// A source deleted (or unlabelled) mid-round must not be re-created in namespaces the round has not reached;
// cleanup belongs to the delete event path.
func TestSyncSecretsSkipsSourceDeletedMidRound(t *testing.T) {
	c := fake.NewSimpleClientset(
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: midRoundSourceNS}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-a"}},
		midRoundSource("OLD"),
	)
	var once sync.Once
	c.PrependReactor("list", "namespaces", func(k8stesting.Action) (bool, runtime.Object, error) {
		once.Do(func() {
			if err := c.Tracker().Delete(v1.SchemeGroupVersion.WithResource("secrets"), midRoundSourceNS, "star-tls"); err != nil {
				t.Errorf("delete source mid-round: %v", err)
			}
		})
		return false, nil, nil
	})
	if err := SyncSecrets(c, midRoundSourceNS, "", newTestLogger()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CoreV1().Secrets("app-a").Get(context.TODO(), "star-tls", metav1.GetOptions{}); err == nil {
		t.Error("round re-created a copy of a source deleted mid-round")
	}
}
