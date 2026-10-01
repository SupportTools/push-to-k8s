package k8s

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// Per-source namespace targeting (cluster-services-752): a wildcard TLS key was copied into 156
// namespaces although only ~36 use it. A source annotated with NamespaceSelectorAnnotation is only
// copied to namespaces matching the selector; copies elsewhere are pruned when PruneUnselected is on.

func selSource(sel string) *v1.Secret {
	s := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "wild", Namespace: "push-to-k8s", Labels: map[string]string{"push-to-k8s": "source"},
			Annotations: map[string]string{}},
		Data: map[string][]byte{"tls.key": []byte("k"), "tls.crt": []byte("c")},
	}
	if sel != "" {
		s.Annotations[NamespaceSelectorAnnotation] = sel
	}
	return s
}

func ns(name string, labels map[string]string) *v1.Namespace {
	return &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
}

func get(t *testing.T, c *fake.Clientset, nsName string) *v1.Secret {
	t.Helper()
	s, err := c.CoreV1().Secrets(nsName).Get(context.TODO(), "wild", metav1.GetOptions{})
	if err != nil {
		return nil
	}
	return s
}

func withPrune(t *testing.T, on bool) {
	t.Helper()
	old := PruneUnselected
	PruneUnselected = on
	t.Cleanup(func() { PruneUnselected = old })
}

func TestSelectorMatchingNamespaceGetsCopyWithMarker(t *testing.T) {
	c := fake.NewSimpleClientset(ns("app", map[string]string{"tls.support.tools/wildcard": "true"}))
	if err := syncSecretToNamespace(c, selSource("tls.support.tools/wildcard=true"), "app", "", newTestLogger()); err != nil {
		t.Fatal(err)
	}
	got := get(t, c, "app")
	if got == nil {
		t.Fatal("matching namespace did not get a copy")
	}
	if got.Labels[CopyOfLabel] != "wild" {
		t.Errorf("copy lacks marker label %s=wild: %v", CopyOfLabel, got.Labels)
	}
	if _, ok := got.Annotations[NamespaceSelectorAnnotation]; ok {
		t.Error("controller annotation was copied onto the copy")
	}
}

func TestSelectorNonMatchingNamespaceGetsNothing(t *testing.T) {
	c := fake.NewSimpleClientset(ns("other", nil))
	if err := syncSecretToNamespace(c, selSource("tls.support.tools/wildcard=true"), "other", "", newTestLogger()); err != nil {
		t.Fatal(err)
	}
	if get(t, c, "other") != nil {
		t.Error("non-matching namespace got a copy")
	}
}

func TestPruneRemovesUnmarkedCopyWithIdenticalData(t *testing.T) {
	withPrune(t, true)
	src := selSource("tls.support.tools/wildcard=true")
	old := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "wild", Namespace: "other"}, Data: src.Data}
	c := fake.NewSimpleClientset(ns("other", nil), old)
	if err := syncSecretToNamespace(c, src, "other", "", newTestLogger()); err != nil {
		t.Fatal(err)
	}
	if get(t, c, "other") != nil {
		t.Error("stale copy (identical data, pre-marker) was not pruned")
	}
}

func TestPruneRemovesMarkedCopy(t *testing.T) {
	withPrune(t, true)
	src := selSource("tls.support.tools/wildcard=true")
	old := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "wild", Namespace: "other", Labels: map[string]string{CopyOfLabel: "wild"}},
		Data: map[string][]byte{"tls.key": []byte("OLD")}}
	c := fake.NewSimpleClientset(ns("other", nil), old)
	if err := syncSecretToNamespace(c, src, "other", "", newTestLogger()); err != nil {
		t.Fatal(err)
	}
	if get(t, c, "other") != nil {
		t.Error("marked copy was not pruned")
	}
}

func TestPruneOffKeepsCopy(t *testing.T) {
	withPrune(t, false)
	src := selSource("tls.support.tools/wildcard=true")
	old := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "wild", Namespace: "other"}, Data: src.Data}
	c := fake.NewSimpleClientset(ns("other", nil), old)
	if err := syncSecretToNamespace(c, src, "other", "", newTestLogger()); err != nil {
		t.Fatal(err)
	}
	if get(t, c, "other") == nil {
		t.Error("copy deleted although PruneUnselected is off")
	}
}

func TestPruneNeverTouchesForeignSecretOfSameName(t *testing.T) {
	withPrune(t, true)
	src := selSource("tls.support.tools/wildcard=true")
	foreign := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "wild", Namespace: "other"}, Data: map[string][]byte{"x": []byte("mine")}}
	c := fake.NewSimpleClientset(ns("other", nil), foreign)
	if err := syncSecretToNamespace(c, src, "other", "", newTestLogger()); err != nil {
		t.Fatal(err)
	}
	if get(t, c, "other") == nil {
		t.Error("a same-named Secret that is not a copy was deleted")
	}
}

func TestInvalidSelectorFailsSafe(t *testing.T) {
	withPrune(t, true)
	src := selSource("this is not a selector !!")
	keep := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "wild", Namespace: "app"}, Data: src.Data}
	c := fake.NewSimpleClientset(ns("app", map[string]string{"tls.support.tools/wildcard": "true"}), ns("other", nil), keep)
	if err := syncSecretToNamespace(c, src, "other", "", newTestLogger()); err == nil {
		t.Error("invalid selector should return an error")
	}
	if get(t, c, "other") != nil {
		t.Error("invalid selector created a copy")
	}
	if err := syncSecretToNamespace(c, src, "app", "", newTestLogger()); err == nil {
		t.Error("invalid selector should return an error")
	}
	if get(t, c, "app") == nil {
		t.Error("invalid selector pruned an existing copy")
	}
}

func TestNoSelectorUnchanged(t *testing.T) {
	withPrune(t, true)
	c := fake.NewSimpleClientset(ns("any", nil))
	if err := syncSecretToNamespace(c, selSource(""), "any", "", newTestLogger()); err != nil {
		t.Fatal(err)
	}
	if get(t, c, "any") == nil {
		t.Error("source without selector must still go to every namespace")
	}
}
