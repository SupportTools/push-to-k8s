package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/supporttools/push-to-k8s/pkg/version"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Define Prometheus metrics
var (
	// K8sConnectionSuccess counts successful Kubernetes client connections
	K8sConnectionSuccess = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "k8s_connection_success_total",
			Help: "Total number of successful Kubernetes client connections",
		},
		[]string{"source"},
	)
	// K8sConnectionFailures counts failed Kubernetes client connections
	K8sConnectionFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "k8s_connection_failures_total",
			Help: "Total number of failed Kubernetes client connections",
		},
		[]string{"source", "error"},
	)
	// NamespaceTotal counts the total number of namespaces in the cluster
	NamespaceTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "k8s_namespace_total",
			Help: "Total number of namespaces in the cluster",
		},
	)
	// NamespaceSyncedTotal counts the number of namespaces successfully synced
	NamespaceSyncedTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "k8s_namespace_synced_total",
			Help: "Number of namespaces successfully synced",
		},
	)
	// NamespaceNotSyncedTotal counts the number of namespaces that failed to sync
	NamespaceNotSyncedTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "k8s_namespace_not_synced_total",
			Help: "Number of namespaces that failed to sync",
		},
	)
	// SourceSecretsTotal counts the total number of source secrets
	SourceSecretsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "k8s_source_secrets_total",
			Help: "Total number of secrets in the source namespace with the label push-to-k8s=source",
		},
	)
	// ManagedSecretsTotal counts the total number of secrets managed by the application
	ManagedSecretsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "k8s_managed_secrets_total",
			Help: "Total number of secrets managed by the application",
		},
	)
)

func init() {
	// Register Prometheus metrics
	prometheus.MustRegister(K8sConnectionSuccess)
	prometheus.MustRegister(K8sConnectionFailures)
	prometheus.MustRegister(NamespaceTotal)
	prometheus.MustRegister(NamespaceSyncedTotal)
	prometheus.MustRegister(NamespaceNotSyncedTotal)
	prometheus.MustRegister(SourceSecretsTotal)
	prometheus.MustRegister(ManagedSecretsTotal)
}

// StartMetricsServer starts an HTTP server to expose Prometheus metrics.
func StartMetricsServer(addr string, logger *logrus.Logger) {
	// HTTP multiplexer
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`<html>
			<head><title>Kubernetes Metrics Server</title></head>
			<body>
			<h1>Kubernetes Metrics Server</h1>
			<p><a href="/metrics">Metrics</a></p>
			<p><a href="/healthz">Health</a></p>
			<p><a href="/version">Version</a></p>
			</body>
			</html>`))
		if err != nil {
			logger.Errorf("Failed to write response for / endpoint: %v", err)
		}
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("OK"))
		if err != nil {
			logger.Errorf("Failed to write response for /healthz endpoint: %v", err)
		}
	})

	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(version.Info()))
		if err != nil {
			logger.Errorf("Failed to write response for /version endpoint: %v", err)
		}
	})

	// HTTP server with timeouts
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
		// Set timeouts to prevent abuse
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       15 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}

	logger.Infof("Starting Prometheus metrics server at %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Failed to start metrics server: %v", err)
	}
}

// SyncMetrics updates Prometheus metrics for namespaces and secrets.
func SyncMetrics(clientset *kubernetes.Clientset, sourceNamespace string, logger *logrus.Logger) {
	// Fetch all namespaces
	namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Errorf("Failed to list namespaces: %v", err)
		return
	}
	NamespaceTotal.Set(float64(len(namespaces.Items)))

	// Fetch source secrets
	secrets, err := clientset.CoreV1().Secrets(sourceNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "push-to-k8s=source",
	})
	if err != nil {
		logger.Errorf("Failed to list source secrets: %v", err)
		return
	}
	SourceSecretsTotal.Set(float64(len(secrets.Items)))

	// Update synced/unsynced namespace metrics
	var synced, notSynced int
	for _, ns := range namespaces.Items {
		if isNamespaceSynced(clientset, ns.Name, secrets.Items) {
			synced++
		} else {
			notSynced++
		}
	}
	NamespaceSyncedTotal.Set(float64(synced))
	NamespaceNotSyncedTotal.Set(float64(notSynced))
	ManagedSecretsTotal.Set(float64(synced + notSynced))

	logger.Infof("Metrics updated: Total namespaces=%d, Synced=%d, Not Synced=%d, Source Secrets=%d, Managed Secrets=%d",
		len(namespaces.Items), synced, notSynced, len(secrets.Items), synced+notSynced)
}

// isNamespaceSynced simulates checking if a namespace has been synced.
// You can replace this with your actual sync check logic.
func isNamespaceSynced(clientset *kubernetes.Clientset, namespace string, sourceSecrets []v1.Secret) bool {
	// Example: Check if the namespace has all source secrets
	for _, secret := range sourceSecrets {
		_, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secret.Name, metav1.GetOptions{})
		if err != nil {
			return false
		}
	}
	return true
}
