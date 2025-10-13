package k8s

import (
	"context"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestFullSyncWorkflowMultipleNamespaces tests the complete sync workflow
// with multiple namespaces, source secrets, and proper synchronization
func TestFullSyncWorkflowMultipleNamespaces(t *testing.T) {
	logger := newTestLogger()
	clientset := fake.NewSimpleClientset()

	// Create source namespace
	sourceNS := "push-to-k8s"
	_, err := clientset.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: sourceNS},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create source namespace: %v", err)
	}

	// Create target namespaces
	targetNamespaces := []string{"app-1", "app-2", "app-3"}
	for _, ns := range targetNamespaces {
		_, err := clientset.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		}, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create namespace %s: %v", ns, err)
		}
	}

	// Create source secrets with label
	sourceSecrets := []v1.Secret{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "registry-credentials",
				Namespace: sourceNS,
				Labels:    map[string]string{"push-to-k8s": "source"},
			},
			Data: map[string][]byte{
				"username": []byte("admin"),
				"password": []byte("secret"),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "api-keys",
				Namespace: sourceNS,
				Labels:    map[string]string{"push-to-k8s": "source"},
			},
			Data: map[string][]byte{
				"api-key": []byte("key-12345"),
			},
		},
	}

	for _, secret := range sourceSecrets {
		_, err := clientset.CoreV1().Secrets(sourceNS).Create(context.TODO(), &secret, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create source secret %s: %v", secret.Name, err)
		}
	}

	// Run full sync
	err = SyncSecrets(clientset, sourceNS, "", logger)
	if err != nil {
		t.Fatalf("SyncSecrets failed: %v", err)
	}

	// Verify all secrets were synced to all target namespaces
	for _, ns := range targetNamespaces {
		for _, sourceSecret := range sourceSecrets {
			secret, err := clientset.CoreV1().Secrets(ns).Get(context.TODO(), sourceSecret.Name, metav1.GetOptions{})
			if err != nil {
				t.Errorf("secret %s not found in namespace %s: %v", sourceSecret.Name, ns, err)
				continue
			}

			// Verify data matches
			if !equalByteMaps(secret.Data, sourceSecret.Data) {
				t.Errorf("secret %s in namespace %s has different data than source", sourceSecret.Name, ns)
			}

			// Verify source label was removed
			if secret.Labels != nil {
				if _, exists := secret.Labels["push-to-k8s"]; exists {
					t.Errorf("secret %s in namespace %s still has source label", sourceSecret.Name, ns)
				}
			}

			// Verify namespace is correct
			if secret.Namespace != ns {
				t.Errorf("secret %s has wrong namespace: got %s, want %s", sourceSecret.Name, secret.Namespace, ns)
			}
		}
	}

	// Verify secrets were NOT synced back to source namespace
	secretList, err := clientset.CoreV1().Secrets(sourceNS).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("failed to list secrets in source namespace: %v", err)
	}
	// Should only have the 2 original source secrets
	if len(secretList.Items) != 2 {
		t.Errorf("source namespace should have exactly 2 secrets, got %d", len(secretList.Items))
	}
}

// TestNamespaceWatchTriggerSync tests that the namespace watcher
// automatically syncs secrets to newly created namespaces
func TestNamespaceWatchTriggerSync(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping namespace watch test in short mode")
	}

	logger := newTestLogger()
	clientset := fake.NewSimpleClientset()

	// Create source namespace
	sourceNS := "push-to-k8s"
	_, err := clientset.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: sourceNS},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create source namespace: %v", err)
	}

	// Create source secret
	sourceSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: sourceNS,
			Labels:    map[string]string{"push-to-k8s": "source"},
		},
		Data: map[string][]byte{"key": []byte("value")},
	}
	_, err = clientset.CoreV1().Secrets(sourceNS).Create(context.TODO(), sourceSecret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create source secret: %v", err)
	}

	// Start namespace watcher in background
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go WatchNamespaces(ctx, clientset, sourceNS, "", logger)

	// Give the watcher time to initialize
	time.Sleep(500 * time.Millisecond)

	// Create a new namespace
	newNS := "dynamic-namespace"
	_, err = clientset.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: newNS},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create new namespace: %v", err)
	}

	// Give the watcher time to process the new namespace
	time.Sleep(1 * time.Second)

	// Verify secret was synced to the new namespace
	secret, err := clientset.CoreV1().Secrets(newNS).Get(context.TODO(), sourceSecret.Name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("secret not synced to new namespace: %v", err)
		return
	}

	// Verify data matches
	if !equalByteMaps(secret.Data, sourceSecret.Data) {
		t.Error("synced secret has different data than source")
	}
}

// TestExclusionLabelBehavior tests that namespaces with the exclusion
// label are properly skipped during synchronization
func TestExclusionLabelBehavior(t *testing.T) {
	logger := newTestLogger()
	clientset := fake.NewSimpleClientset()

	excludeLabel := "push-to-k8s-exclude"
	sourceNS := "push-to-k8s"

	// Create source namespace
	_, err := clientset.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: sourceNS},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create source namespace: %v", err)
	}

	// Create namespaces with and without exclusion label
	normalNS := "normal-namespace"
	excludedNS := "excluded-namespace"

	_, err = clientset.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: normalNS,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create normal namespace: %v", err)
	}

	_, err = clientset.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   excludedNS,
			Labels: map[string]string{excludeLabel: "true"},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create excluded namespace: %v", err)
	}

	// Create source secret
	sourceSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: sourceNS,
			Labels:    map[string]string{"push-to-k8s": "source"},
		},
		Data: map[string][]byte{"key": []byte("value")},
	}
	_, err = clientset.CoreV1().Secrets(sourceNS).Create(context.TODO(), sourceSecret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create source secret: %v", err)
	}

	// Run sync with exclusion label
	err = SyncSecrets(clientset, sourceNS, excludeLabel, logger)
	if err != nil {
		t.Fatalf("SyncSecrets failed: %v", err)
	}

	// Verify secret was synced to normal namespace
	_, err = clientset.CoreV1().Secrets(normalNS).Get(context.TODO(), sourceSecret.Name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("secret should be synced to normal namespace: %v", err)
	}

	// Verify secret was NOT synced to excluded namespace
	_, err = clientset.CoreV1().Secrets(excludedNS).Get(context.TODO(), sourceSecret.Name, metav1.GetOptions{})
	if err == nil {
		t.Error("secret should NOT be synced to excluded namespace")
	}
}

// TestSecretUpdatesVsCreates tests that the sync process correctly
// handles both creating new secrets and updating existing ones
func TestSecretUpdatesVsCreates(t *testing.T) {
	logger := newTestLogger()
	clientset := fake.NewSimpleClientset()

	sourceNS := "push-to-k8s"
	targetNS := "app-namespace"

	// Create namespaces
	for _, ns := range []string{sourceNS, targetNS} {
		_, err := clientset.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		}, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create namespace %s: %v", ns, err)
		}
	}

	// Create source secret v1
	sourceSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "evolving-secret",
			Namespace: sourceNS,
			Labels:    map[string]string{"push-to-k8s": "source"},
		},
		Data: map[string][]byte{"key": []byte("version1")},
	}
	_, err := clientset.CoreV1().Secrets(sourceNS).Create(context.TODO(), sourceSecret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create source secret: %v", err)
	}

	// First sync - should CREATE secret in target namespace
	err = SyncSecrets(clientset, sourceNS, "", logger)
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Verify secret was created
	targetSecret, err := clientset.CoreV1().Secrets(targetNS).Get(context.TODO(), sourceSecret.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("secret not created in target namespace: %v", err)
	}
	if string(targetSecret.Data["key"]) != "version1" {
		t.Errorf("expected version1, got %s", string(targetSecret.Data["key"]))
	}

	// Update source secret to v2
	sourceSecret.Data["key"] = []byte("version2")
	_, err = clientset.CoreV1().Secrets(sourceNS).Update(context.TODO(), sourceSecret, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("failed to update source secret: %v", err)
	}

	// Second sync - should UPDATE existing secret in target namespace
	err = SyncSecrets(clientset, sourceNS, "", logger)
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	// Verify secret was updated
	targetSecret, err = clientset.CoreV1().Secrets(targetNS).Get(context.TODO(), sourceSecret.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("secret not found after update: %v", err)
	}
	if string(targetSecret.Data["key"]) != "version2" {
		t.Errorf("secret not updated: expected version2, got %s", string(targetSecret.Data["key"]))
	}

	// Third sync with identical data - should SKIP update
	err = SyncSecrets(clientset, sourceNS, "", logger)
	if err != nil {
		t.Fatalf("third sync failed: %v", err)
	}

	// Verify secret still has correct data (no unnecessary updates)
	targetSecret, err = clientset.CoreV1().Secrets(targetNS).Get(context.TODO(), sourceSecret.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("secret not found after third sync: %v", err)
	}
	if string(targetSecret.Data["key"]) != "version2" {
		t.Errorf("secret data changed unexpectedly: got %s", string(targetSecret.Data["key"]))
	}
}

// TestSyncSecretsToSingleNamespaceIntegration tests the targeted
// single-namespace sync function used by namespace watcher
func TestSyncSecretsToSingleNamespaceIntegration(t *testing.T) {
	logger := newTestLogger()
	clientset := fake.NewSimpleClientset()

	sourceNS := "push-to-k8s"
	targetNS := "new-app"

	// Create namespaces
	for _, ns := range []string{sourceNS, targetNS} {
		_, err := clientset.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		}, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create namespace %s: %v", ns, err)
		}
	}

	// Create multiple source secrets
	secretCount := 5
	for i := 1; i <= secretCount; i++ {
		secret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret-" + string(rune('0'+i)),
				Namespace: sourceNS,
				Labels:    map[string]string{"push-to-k8s": "source"},
			},
			Data: map[string][]byte{"data": []byte("value-" + string(rune('0'+i)))},
		}
		_, err := clientset.CoreV1().Secrets(sourceNS).Create(context.TODO(), secret, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create source secret: %v", err)
		}
	}

	// Sync to single namespace
	err := syncSecretsToSingleNamespace(clientset, sourceNS, targetNS, "", logger)
	if err != nil {
		t.Fatalf("syncSecretsToSingleNamespace failed: %v", err)
	}

	// Verify all secrets were synced
	secrets, err := clientset.CoreV1().Secrets(targetNS).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("failed to list secrets in target namespace: %v", err)
	}

	if len(secrets.Items) != secretCount {
		t.Errorf("expected %d secrets in target namespace, got %d", secretCount, len(secrets.Items))
	}

	// Verify each secret has correct data and no source label
	for i := 1; i <= secretCount; i++ {
		secretName := "secret-" + string(rune('0'+i))
		secret, err := clientset.CoreV1().Secrets(targetNS).Get(context.TODO(), secretName, metav1.GetOptions{})
		if err != nil {
			t.Errorf("secret %s not found: %v", secretName, err)
			continue
		}

		expectedData := "value-" + string(rune('0'+i))
		if string(secret.Data["data"]) != expectedData {
			t.Errorf("secret %s has wrong data: got %s, want %s", secretName, string(secret.Data["data"]), expectedData)
		}

		if secret.Labels != nil && secret.Labels["push-to-k8s"] == "source" {
			t.Errorf("secret %s still has source label", secretName)
		}
	}
}
