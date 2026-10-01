package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// NamespaceSelectorAnnotation, set on a SOURCE Secret, limits its copies to namespaces whose labels match
// this label selector (e.g. "tls.support.tools/wildcard=true"). Without it a source goes to every namespace.
const NamespaceSelectorAnnotation = "push-to-k8s.supporttools.io/namespace-selector"

// CopyOfLabel marks every copy push-to-k8s writes with the source Secret's name, so pruning can tell a copy
// from an unrelated Secret that happens to share the name.
const CopyOfLabel = "push-to-k8s.supporttools.io/copy-of"

// PruneUnselected deletes copies from namespaces that no longer match a source's namespace selector. Off by
// default: the first rollout of a selector should only log what it would prune (env PRUNE_UNSELECTED).
var PruneUnselected bool

// isControllerAnnotation reports annotations that configure push-to-k8s on the source and must not be copied.
func isControllerAnnotation(key string) bool {
	return strings.HasPrefix(key, "push-to-k8s.supporttools.io/")
}

// namespaceSelected reports whether namespace ns is a target of source. A source without a selector targets
// every namespace. An unparsable selector is an error: the caller must neither copy nor prune.
func namespaceSelected(clientset kubernetes.Interface, source *v1.Secret, ns string) (bool, error) {
	raw, ok := source.Annotations[NamespaceSelectorAnnotation]
	if !ok {
		return true, nil
	}
	sel, err := labels.Parse(raw)
	if err != nil {
		return false, fmt.Errorf("source %s: invalid %s %q: %w", source.Name, NamespaceSelectorAnnotation, raw, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	n, err := clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("get namespace %s: %w", ns, err)
	}
	return sel.Matches(labels.Set(n.Labels)), nil
}

// isCopyOf reports whether existing is a push-to-k8s copy of source: it has no ownerReferences, and it carries
// the copy marker or (copies written before the marker existed) its data is byte-identical to the source's.
func isCopyOf(existing, source *v1.Secret) bool {
	// Owned by another controller (ESO ExternalSecret, cert-manager...): never ours, whatever its data.
	if len(existing.OwnerReferences) > 0 {
		return false
	}
	if existing.Labels[CopyOfLabel] == source.Name {
		return true
	}
	return len(source.Data) > 0 && equalByteMaps(existing.Data, source.Data)
}

// pruneCopy removes source's copy from a namespace the source no longer targets. It never deletes a Secret
// that is not a copy, and only logs when PruneUnselected is off.
func pruneCopy(clientset kubernetes.Interface, source *v1.Secret, ns string, log *logrus.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	existing, err := clientset.CoreV1().Secrets(ns).Get(ctx, source.Name, metav1.GetOptions{})
	if err != nil {
		if isNotFoundError(err) {
			return nil
		}
		return err
	}
	if !isCopyOf(existing, source) {
		log.Warnf("Secret %s in namespace %s is not a copy of the source; leaving it alone", source.Name, ns)
		return nil
	}
	if !PruneUnselected {
		log.Infof("Would prune %s from namespace %s (not selected; PRUNE_UNSELECTED is off)", source.Name, ns)
		return nil
	}
	if err := clientset.CoreV1().Secrets(ns).Delete(ctx, source.Name, metav1.DeleteOptions{}); err != nil && !isNotFoundError(err) {
		return fmt.Errorf("prune %s from %s: %w", source.Name, ns, err)
	}
	log.Infof("Pruned %s from namespace %s (not selected by %s)", source.Name, ns, NamespaceSelectorAnnotation)
	return nil
}

// isDataBearingAnnotation reports whether an annotation embeds a full copy of the applied object,
// Secret data included: kubectl's client-side-apply record and wrangler/Rancher's objectset record
// (gzip+base64 JSON). Copying either onto every target namespace spreads a decodable second copy
// of the credential through metadata, which kubectl describe and UIs show while masking .data.
func isDataBearingAnnotation(key string) bool {
	return key == "kubectl.kubernetes.io/last-applied-configuration" ||
		strings.HasPrefix(key, "objectset.rio.cattle.io/")
}

// hasDataBearingAnnotation reports whether a Secret carries any isDataBearingAnnotation key.
func hasDataBearingAnnotation(s *v1.Secret) bool {
	for k := range s.Annotations {
		if isDataBearingAnnotation(k) {
			return true
		}
	}
	return false
}

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

	// Per-source namespace targeting: not selected -> prune any copy (or log), never create one.
	selected, err := namespaceSelected(clientset, sourceSecret, namespace)
	if err != nil {
		return err
	}
	if !selected {
		return pruneCopy(clientset, sourceSecret, namespace, log)
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
		// Start with existing secret to preserve UID and other Kubernetes-managed metadata
		existingSecret.Data = sourceSecret.Data
		existingSecret.StringData = sourceSecret.StringData
		existingSecret.Type = sourceSecret.Type

		// Copy labels from source but exclude the source label
		if existingSecret.Labels == nil {
			existingSecret.Labels = make(map[string]string)
		}
		for k, v := range sourceSecret.Labels {
			if k != "push-to-k8s" { // Don't copy source label to target secrets
				existingSecret.Labels[k] = v
			}
		}

		// Copy annotations from source
		if existingSecret.Annotations == nil {
			existingSecret.Annotations = make(map[string]string)
		}
		for k, v := range sourceSecret.Annotations {
			if !isDataBearingAnnotation(k) && !isControllerAnnotation(k) {
				existingSecret.Annotations[k] = v
			}
		}
		existingSecret.Labels[CopyOfLabel] = sourceSecret.Name
		// Drop any left by an older push-to-k8s, which copied them verbatim.
		for k := range existingSecret.Annotations {
			if isDataBearingAnnotation(k) {
				delete(existingSecret.Annotations, k)
			}
		}

		updateCtx, updateCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer updateCancel()
		_, err = clientset.CoreV1().Secrets(namespace).Update(updateCtx, existingSecret, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update secret %s in namespace %s: %w", sourceSecret.Name, namespace, err)
		}

		log.Infof("Updated secret %s in namespace %s", sourceSecret.Name, namespace)
		return nil
	}

	// Secret does not exist, create it
	sourceSecretCopy := sourceSecret.DeepCopy()
	sourceSecretCopy.Namespace = namespace
	// Clear metadata fields to allow Kubernetes to generate fresh identifiers
	sourceSecretCopy.ResourceVersion = ""
	sourceSecretCopy.UID = ""
	sourceSecretCopy.CreationTimestamp = metav1.Time{}
	sourceSecretCopy.Generation = 0
	sourceSecretCopy.ManagedFields = nil
	// Owner references and finalizers belong to the source namespace (e.g. an ESO
	// ExternalSecret owning the source Secret). A copy carrying an ownerReference whose
	// UID does not exist in the target namespace is garbage-collected within seconds.
	sourceSecretCopy.OwnerReferences = nil
	sourceSecretCopy.Finalizers = nil
	for k := range sourceSecretCopy.Annotations {
		if isDataBearingAnnotation(k) || isControllerAnnotation(k) {
			delete(sourceSecretCopy.Annotations, k)
		}
	}
	if sourceSecretCopy.Labels == nil {
		sourceSecretCopy.Labels = map[string]string{}
	}
	sourceSecretCopy.Labels[CopyOfLabel] = sourceSecret.Name
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
	// A copy still carrying a data-bearing annotation is stale even when its data matches.
	if hasDataBearingAnnotation(existingSecret) {
		return false
	}
	// A copy without the copy-of marker is stale too, so every copy gets marked once and becomes prunable.
	if existingSecret.Labels[CopyOfLabel] != sourceSecret.Name {
		return false
	}

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

// SecretEvent represents a secret change event for the debounce queue.
type SecretEvent struct {
	EventType string     // "add", "update", or "delete"
	Secret    *v1.Secret // The secret object (nil for delete events that only have name)
	Name      string     // Secret name (used for delete events)
}

// syncSingleSecretToAllNamespaces syncs a specific secret to all target namespaces.
// This is more efficient than SyncSecrets() when only one secret changed.
func syncSingleSecretToAllNamespaces(clientset kubernetes.Interface, secret *v1.Secret, sourceNamespace, excludeNamespaceLabel string, log *logrus.Logger) error {
	// List all namespaces
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	// Sync secret to all namespaces (excluding the source namespace and excluded namespaces)
	for _, ns := range namespaces.Items {
		if ns.Name == sourceNamespace {
			continue // Skip the source namespace
		}

		if excludeNamespaceLabel != "" && ns.Labels != nil {
			if _, exists := ns.Labels[excludeNamespaceLabel]; exists {
				log.Debugf("Skipping namespace %s due to exclude label %s", ns.Name, excludeNamespaceLabel)
				continue
			}
		}

		if err := syncSecretToNamespace(clientset, secret, ns.Name, excludeNamespaceLabel, log); err != nil {
			log.Warnf("Failed to sync secret %s to namespace %s: %v", secret.Name, ns.Name, err)
		} else {
			log.Debugf("Secret %s synced to namespace %s", secret.Name, ns.Name)
		}
	}
	return nil
}

// deleteSingleSecretFromAllNamespaces removes a specific secret from all target namespaces.
func deleteSingleSecretFromAllNamespaces(clientset kubernetes.Interface, secretName, sourceNamespace, excludeNamespaceLabel string, log *logrus.Logger) error {
	// List all namespaces
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	// Delete secret from all namespaces (excluding the source namespace and excluded namespaces)
	for _, ns := range namespaces.Items {
		if ns.Name == sourceNamespace {
			continue // Skip the source namespace
		}

		if excludeNamespaceLabel != "" && ns.Labels != nil {
			if _, exists := ns.Labels[excludeNamespaceLabel]; exists {
				log.Debugf("Skipping namespace %s due to exclude label %s", ns.Name, excludeNamespaceLabel)
				continue
			}
		}

		deleteCtx, deleteCancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := clientset.CoreV1().Secrets(ns.Name).Delete(deleteCtx, secretName, metav1.DeleteOptions{})
		deleteCancel()

		if err != nil {
			// Ignore not found errors (secret may not exist in this namespace)
			if !isNotFoundError(err) {
				log.Warnf("Failed to delete secret %s from namespace %s: %v", secretName, ns.Name, err)
			}
		} else {
			log.Infof("Deleted secret %s from namespace %s", secretName, ns.Name)
		}
	}
	return nil
}

// isNotFoundError checks if an error is a Kubernetes "not found" error.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// Check if error message contains "not found"
	return err.Error() != "" && (err.Error() == "not found" || contains(err.Error(), "not found"))
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// processDebouncedSecretQueue processes secret events from the queue with debounce logic.
// It collects events over a debounce window and processes them in batches.
func processDebouncedSecretQueue(ctx context.Context, eventQueue <-chan SecretEvent, debounceWindow time.Duration, rateLimiter *rate.Limiter, clientset kubernetes.Interface, sourceNamespace, excludeNamespaceLabel string, log *logrus.Logger) {
	var (
		timer          *time.Timer
		pendingEvents  = make(map[string]SecretEvent) // Map of secret name -> latest event
	)

	processBatch := func() {
		if len(pendingEvents) == 0 {
			return
		}

		log.Infof("Processing batch of %d secret events", len(pendingEvents))

		for _, event := range pendingEvents {
			// Wait for rate limiter token
			if err := rateLimiter.Wait(ctx); err != nil {
				log.Warnf("Rate limiter error: %v", err)
				continue
			}

			switch event.EventType {
			case "add", "update":
				if event.Secret != nil {
					log.Infof("Syncing secret %s to all namespaces (event: %s)", event.Secret.Name, event.EventType)
					if err := syncSingleSecretToAllNamespaces(clientset, event.Secret, sourceNamespace, excludeNamespaceLabel, log); err != nil {
						log.Errorf("Failed to sync secret %s: %v", event.Secret.Name, err)
					}
				}
			case "delete":
				log.Infof("Deleting secret %s from all namespaces", event.Name)
				if err := deleteSingleSecretFromAllNamespaces(clientset, event.Name, sourceNamespace, excludeNamespaceLabel, log); err != nil {
					log.Errorf("Failed to delete secret %s: %v", event.Name, err)
				}
			}
		}

		// Clear pending events after processing
		pendingEvents = make(map[string]SecretEvent)
	}

	for {
		// Use nil-safe channel pattern to prevent panic when timer is nil
		// A nil channel in select blocks forever (never selected)
		var timerC <-chan time.Time
		if timer != nil {
			timerC = timer.C
		}

		select {
		case <-ctx.Done():
			log.Info("Secret queue processor shutting down...")
			if timer != nil {
				timer.Stop()
			}
			// Process any remaining events before shutdown
			processBatch()
			return

		case event := <-eventQueue:
			// Add/update event in pending map (overwrites older events for same secret)
			if event.EventType == "delete" {
				pendingEvents[event.Name] = event
			} else if event.Secret != nil {
				pendingEvents[event.Secret.Name] = event
			}

			// Reset or create timer
			if timer == nil {
				timer = time.NewTimer(debounceWindow)
			} else {
				if !timer.Stop() {
					// Drain the channel if timer already fired
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(debounceWindow)
			}

		case <-timerC:
			// Debounce window expired, process batch
			processBatch()
			timer = nil
		}
	}
}

// WatchSourceSecrets starts a secret informer to watch for changes to source secrets
// and triggers synchronization to all target namespaces via a debounced queue.
func WatchSourceSecrets(ctx context.Context, clientset kubernetes.Interface, sourceNamespace, excludeNamespaceLabel string, debounceSeconds int, rateLimit int, log *logrus.Logger) {
	// Create rate limiter (ops per second)
	rateLimiter := rate.NewLimiter(rate.Limit(rateLimit), rateLimit)

	// Create event queue channel
	eventQueue := make(chan SecretEvent, 100)

	// Start queue processor goroutine
	go processDebouncedSecretQueue(ctx, eventQueue, time.Duration(debounceSeconds)*time.Second, rateLimiter, clientset, sourceNamespace, excludeNamespaceLabel, log)

	// Create informer factory with namespace and label selector
	factory := informers.NewSharedInformerFactoryWithOptions(
		clientset,
		0,
		informers.WithNamespace(sourceNamespace),
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = "push-to-k8s=source"
		}),
	)

	secretInformer := factory.Core().V1().Secrets().Informer()

	// Add event handlers
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Recovered from panic while adding secret event handler: %v", r)
		}
	}()

	_, err := secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			secret, ok := obj.(*v1.Secret)
			if !ok {
				log.Errorf("Failed to cast object to Secret in AddFunc")
				return
			}
			log.Infof("Source secret added: %s", secret.Name)
			eventQueue <- SecretEvent{
				EventType: "add",
				Secret:    secret.DeepCopy(),
				Name:      secret.Name,
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldSecret, ok := oldObj.(*v1.Secret)
			if !ok {
				log.Errorf("Failed to cast old object to Secret in UpdateFunc")
				return
			}
			newSecret, ok := newObj.(*v1.Secret)
			if !ok {
				log.Errorf("Failed to cast new object to Secret in UpdateFunc")
				return
			}

			// Only trigger sync if secret data actually changed
			if !compareSecrets(oldSecret, newSecret) {
				log.Infof("Source secret updated: %s", newSecret.Name)
				eventQueue <- SecretEvent{
					EventType: "update",
					Secret:    newSecret.DeepCopy(),
					Name:      newSecret.Name,
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			secret, ok := obj.(*v1.Secret)
			if !ok {
				log.Errorf("Failed to cast object to Secret in DeleteFunc")
				return
			}
			log.Infof("Source secret deleted: %s", secret.Name)
			eventQueue <- SecretEvent{
				EventType: "delete",
				Secret:    nil,
				Name:      secret.Name,
			}
		},
	})
	if err != nil {
		log.Errorf("Failed to add event handler for secret informer: %v", err)
		return
	}

	// Start the informer
	stopCh := make(chan struct{})
	factory.Start(stopCh)

	// Wait for the informer cache to sync
	if !cache.WaitForCacheSync(stopCh, secretInformer.HasSynced) {
		log.Error("Failed to sync secret informer cache")
		close(stopCh)
		return
	}

	log.Info("Source secret watcher started successfully")

	// Wait for context cancellation
	<-ctx.Done()
	log.Info("Source secret watcher received shutdown signal")
	close(stopCh)
	close(eventQueue)
}
