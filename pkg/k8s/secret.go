package k8s

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// getSourceSecrets fetches secrets from the source namespace with the label push-to-k8s=source.
func getSourceSecrets(clientset *kubernetes.Clientset, sourceNamespace string) ([]v1.Secret, error) {
	labelSelector := "push-to-k8s=source"
	secretList, err := clientset.CoreV1().Secrets(sourceNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets in namespace %s with label %s: %w", sourceNamespace, labelSelector, err)
	}

	if len(secretList.Items) == 0 {
		return nil, fmt.Errorf("no secrets found in namespace %s with label %s", sourceNamespace, labelSelector)
	}

	return secretList.Items, nil
}

// syncSecretToNamespace ensures the given secret is synced to the specified namespace.
func syncSecretToNamespace(clientset *kubernetes.Clientset, sourceSecret *v1.Secret, namespace, excludeNamespaceLabel string, log *logrus.Logger) error {
	// Skip namespaces with the exclude label
	if excludeNamespaceLabel != "" {
		ns, err := clientset.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
		if err == nil && ns.Labels != nil {
			if _, exists := ns.Labels[excludeNamespaceLabel]; exists {
				log.Infof("Skipping namespace %s due to exclude label %s", namespace, excludeNamespaceLabel)
				return nil
			}
		}
	}

	// Check if the secret already exists in the target namespace
	existingSecret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), sourceSecret.Name, metav1.GetOptions{})
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
		_, err = clientset.CoreV1().Secrets(namespace).Update(context.TODO(), sourceSecretCopy, metav1.UpdateOptions{})
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
	_, err = clientset.CoreV1().Secrets(namespace).Create(context.TODO(), sourceSecretCopy, metav1.CreateOptions{})
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

// SyncSecrets syncs all labeled secrets from the source namespace to all other namespaces,
// skipping the source namespace itself and any namespaces with the exclude label.
func SyncSecrets(clientset *kubernetes.Clientset, sourceNamespace, excludeNamespaceLabel string, log *logrus.Logger) error {
	// Get source secrets
	sourceSecrets, err := getSourceSecrets(clientset, sourceNamespace)
	if err != nil {
		return err
	}

	// List all namespaces
	namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	// Sync each secret to all namespaces (excluding the source namespace and excluded namespaces)
	for _, secret := range sourceSecrets {
		for _, ns := range namespaces.Items {
			if ns.Name == sourceNamespace {
				continue // Skip the source namespace
			}

			if excludeNamespaceLabel != "" && ns.Labels[excludeNamespaceLabel] != "" {
				log.Infof("Skipping namespace %s due to exclude label %s", ns.Name, excludeNamespaceLabel)
				continue
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
func WatchNamespaces(clientset *kubernetes.Clientset, sourceNamespace, excludeNamespaceLabel string, log *logrus.Logger) {
	factory := informers.NewSharedInformerFactory(clientset, 0)
	namespaceInformer := factory.Core().V1().Namespaces().Informer()

	// Add event handler to the namespace informer
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Recovered from panic while adding event handler: %v", r)
		}
	}()

	namespaceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
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

			// Sync secrets to the new namespace
			if err := SyncSecrets(clientset, sourceNamespace, excludeNamespaceLabel, log); err != nil {
				log.Warnf("Failed to sync secrets to new namespace %s: %v", ns.Name, err)
				// Optional: retry logic could be implemented here
			} else {
				log.Infof("Successfully synced secrets to namespace: %s", ns.Name)
			}
		},
	})

	// Start the informer
	stopCh := make(chan struct{})
	defer close(stopCh)
	factory.Start(stopCh)
	if ok := factory.WaitForCacheSync(stopCh); !ok {
		log.Error("Failed to sync informer cache")
		return
	}
	<-stopCh
}
