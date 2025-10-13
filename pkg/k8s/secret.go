package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// getSourceSecrets fetches secrets from the source namespace with the label push-to-k8s=source.
// Returns an empty slice if no secrets are found (which is a valid state).
func getSourceSecrets(clientset kubernetes.Interface, sourceNamespace string, log *logrus.Logger) ([]v1.Secret, error) {
	labelSelector := "push-to-k8s=source"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	secretList, err := clientset.CoreV1().Secrets(sourceNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets in namespace %s with label %s: %w", sourceNamespace, labelSelector, err)
	}

	if len(secretList.Items) == 0 {
		log.Infof("No secrets found in namespace %s with label %s", sourceNamespace, labelSelector)
		return []v1.Secret{}, nil
	}

	return secretList.Items, nil
}

// syncSecretToNamespace ensures the given secret is synced to the specified namespace.
func syncSecretToNamespace(clientset kubernetes.Interface, sourceSecret *v1.Secret, namespace, excludeNamespaceLabel string, log *logrus.Logger) error {
	// Skip namespaces with the exclude label
	if excludeNamespaceLabel != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		ns, err := clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if err == nil && ns.Labels != nil {
			if _, exists := ns.Labels[excludeNamespaceLabel]; exists {
				log.Infof("Skipping namespace %s due to exclude label %s", namespace, excludeNamespaceLabel)
				return nil
			}
		}
	}

	// Check if the secret already exists in the target namespace
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	existingSecret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, sourceSecret.Name, metav1.GetOptions{})
	if err == nil {
		// Compare existing secret with source secret
		if compareSecrets(existingSecret, sourceSecret) {
			log.Infof("Secret %s in namespace %s is up-to-date. Skipping update.", sourceSecret.Name, namespace)
			return nil
		}

		// Secret exists but is different, update it
		sourceSecretCopy := sourceSecret.DeepCopy()
		sourceSecretCopy.Namespace = namespace
		sourceSecretCopy.ResourceVersion = existingSecret.ResourceVersion // Preserve ResourceVersion for updates
		// Remove source label to avoid confusion (target secrets should not have the source label)
		if sourceSecretCopy.Labels != nil {
			delete(sourceSecretCopy.Labels, "push-to-k8s")
		}
		updateCtx, updateCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer updateCancel()
		_, err = clientset.CoreV1().Secrets(namespace).Update(updateCtx, sourceSecretCopy, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update secret %s in namespace %s: %w", sourceSecret.Name, namespace, err)
		}

		log.Infof("Updated secret %s in namespace %s", sourceSecret.Name, namespace)
		return nil
	}

	// Secret does not exist, create it
	sourceSecretCopy := sourceSecret.DeepCopy()
	sourceSecretCopy.Namespace = namespace
	sourceSecretCopy.ResourceVersion = ""
	// Remove source label to avoid confusion (target secrets should not have the source label)
	if sourceSecretCopy.Labels != nil {
		delete(sourceSecretCopy.Labels, "push-to-k8s")
	}
	createCtx, createCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer createCancel()
	_, err = clientset.CoreV1().Secrets(namespace).Create(createCtx, sourceSecretCopy, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create secret %s in namespace %s: %w", sourceSecret.Name, namespace, err)
	}

	log.Infof("Created secret %s in namespace %s", sourceSecret.Name, namespace)
	return nil
}

// compareSecrets compares two secrets and returns true if they are identical.
func compareSecrets(existingSecret, sourceSecret *v1.Secret) bool {
	// Compare Data field
	if !equalByteMaps(existingSecret.Data, sourceSecret.Data) {
		return false
	}

	// Compare StringData field (if set)
	if !equalStringMaps(existingSecret.StringData, sourceSecret.StringData) {
		return false
	}

	return true
}

// equalByteMaps compares two maps[string][]byte for equality.
func equalByteMaps(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for key, valA := range a {
		valB, exists := b[key]
		if !exists || string(valA) != string(valB) {
			return false
		}
	}
	return true
}

// equalStringMaps compares two maps[string]string for equality.
func equalStringMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, valA := range a {
		valB, exists := b[key]
		if !exists || valA != valB {
			return false
		}
	}
	return true
}

// syncSecretsToSingleNamespace syncs all labeled secrets from the source namespace to a single target namespace.
// This is more efficient than SyncSecrets when you only need to sync to one namespace (e.g., when a new namespace is created).
func syncSecretsToSingleNamespace(clientset kubernetes.Interface, sourceNamespace, targetNamespace, excludeNamespaceLabel string, log *logrus.Logger) error {
	// Get source secrets
	sourceSecrets, err := getSourceSecrets(clientset, sourceNamespace, log)
	if err != nil {
		return err
	}

	// Sync each secret to the target namespace
	for _, secret := range sourceSecrets {
		if err := syncSecretToNamespace(clientset, &secret, targetNamespace, excludeNamespaceLabel, log); err != nil {
			log.Warnf("Failed to sync secret %s to namespace %s: %v", secret.Name, targetNamespace, err)
		} else {
			log.Infof("Secret %s synced to namespace %s", secret.Name, targetNamespace)
		}
	}
	return nil
}

// SyncSecrets syncs all labeled secrets from the source namespace to all other namespaces,
// skipping the source namespace itself and any namespaces with the exclude label.
func SyncSecrets(clientset kubernetes.Interface, sourceNamespace, excludeNamespaceLabel string, log *logrus.Logger) error {
	// Get source secrets
	sourceSecrets, err := getSourceSecrets(clientset, sourceNamespace, log)
	if err != nil {
		return err
	}

	// List all namespaces
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	// Sync each secret to all namespaces (excluding the source namespace and excluded namespaces)
	for _, secret := range sourceSecrets {
		for _, ns := range namespaces.Items {
			if ns.Name == sourceNamespace {
				continue // Skip the source namespace
			}

			if excludeNamespaceLabel != "" && ns.Labels != nil {
				if _, exists := ns.Labels[excludeNamespaceLabel]; exists {
					log.Infof("Skipping namespace %s due to exclude label %s", ns.Name, excludeNamespaceLabel)
					continue
				}
			}

			if err := syncSecretToNamespace(clientset, &secret, ns.Name, excludeNamespaceLabel, log); err != nil {
				log.Warnf("Failed to sync secret %s to namespace %s: %v", secret.Name, ns.Name, err)
			} else {
				log.Infof("Secret %s synced to namespace %s", secret.Name, ns.Name)
			}
		}
	}
	return nil
}

// WatchNamespaces starts a namespace informer to watch for new namespaces and sync secrets,
// skipping namespaces with the exclude label or matching the source namespace.
// It respects context cancellation for graceful shutdown.
func WatchNamespaces(ctx context.Context, clientset kubernetes.Interface, sourceNamespace, excludeNamespaceLabel string, log *logrus.Logger) {
	factory := informers.NewSharedInformerFactory(clientset, 0)
	namespaceInformer := factory.Core().V1().Namespaces().Informer()

	// Add event handler to the namespace informer
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Recovered from panic while adding event handler: %v", r)
		}
	}()

	_, err := namespaceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ns, ok := obj.(*v1.Namespace)
			if !ok {
				log.Errorf("Failed to cast object to Namespace")
				return
			}
			log.Infof("New namespace created: %s", ns.Name)

			// Skip the source namespace
			if ns.Name == sourceNamespace {
				log.Infof("Skipping sync for the source namespace: %s", sourceNamespace)
				return
			}

			// Skip namespaces with the exclude label
			if excludeNamespaceLabel != "" && ns.Labels != nil {
				if _, exists := ns.Labels[excludeNamespaceLabel]; exists {
					log.Infof("Skipping namespace %s due to exclude label %s", ns.Name, excludeNamespaceLabel)
					return
				}
			}

			// Sync secrets to the new namespace (using targeted single-namespace sync for efficiency)
			if err := syncSecretsToSingleNamespace(clientset, sourceNamespace, ns.Name, excludeNamespaceLabel, log); err != nil {
				log.Warnf("Failed to sync secrets to new namespace %s: %v", ns.Name, err)
				// Optional: retry logic could be implemented here
			} else {
				log.Infof("Successfully synced secrets to namespace: %s", ns.Name)
			}
		},
	})
	if err != nil {
		log.Errorf("Failed to add event handler for namespace informer: %v", err)
		// Continue execution despite error, as this is a background watcher
	}

	// Start the informer with a stop channel
	stopCh := make(chan struct{})
	factory.Start(stopCh)

	// Wait for the informer cache to sync
	if !cache.WaitForCacheSync(stopCh, namespaceInformer.HasSynced) {
		log.Error("Failed to sync informer cache")
		close(stopCh)
		return
	}

	log.Info("Namespace watcher started successfully")

	// Wait for context cancellation
	<-ctx.Done()
	log.Info("Namespace watcher received shutdown signal")
	close(stopCh)
}
