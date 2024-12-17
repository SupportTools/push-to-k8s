package main

import (
	"fmt"
	"time"

	"github.com/supporttools/push-to-k8s/pkg/config"
	"github.com/supporttools/push-to-k8s/pkg/k8s"
	"github.com/supporttools/push-to-k8s/pkg/logging"
	"github.com/supporttools/push-to-k8s/pkg/metrics"
	"k8s.io/client-go/kubernetes"
)

var log = logging.SetupLogging()

func main() {
	// Load configuration from environment
	cfg := config.LoadConfigFromEnv()
	logConfigStatus(cfg)

	// Initialize Kubernetes client
	clientset := initializeK8sClient()

	// Start Prometheus metrics server
	startMetricsServer(cfg)

	// Start periodic secret sync and namespace watcher
	startPeriodicSync(clientset, cfg)
	startNamespaceWatcher(clientset, cfg)

	// Block forever
	select {}
}

func logConfigStatus(cfg config.Config) {
	if cfg.Debug {
		log.Debug("Debug mode enabled")
	} else {
		log.Info("Debug mode disabled")
	}

	if cfg.Namespace == "" {
		log.Fatalf("Source namespace is not specified. Set the NAMESPACE environment variable.")
	}
}

func initializeK8sClient() *kubernetes.Clientset {
	clientset, err := k8s.CreateClusterConnection(log)
	if err != nil {
		log.Fatalf("Failed to connect to Kubernetes cluster: %v", err)
	}
	return clientset
}

func startMetricsServer(cfg config.Config) {
	metricsPort := fmt.Sprintf(":%d", cfg.MetricsPort)
	go metrics.StartMetricsServer(metricsPort, log)
}

func startPeriodicSync(clientset *kubernetes.Clientset, cfg config.Config) {
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.SyncInterval) * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			if err := k8s.SyncSecrets(clientset, cfg.Namespace, cfg.ExcludeNamespaceLabel, log); err != nil {
				log.Errorf("Error syncing secrets: %v", err)
			}
		}
	}()
}

func startNamespaceWatcher(clientset *kubernetes.Clientset, cfg config.Config) {
	go k8s.WatchNamespaces(clientset, cfg.Namespace, cfg.ExcludeNamespaceLabel, log)
}
