package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/supporttools/push-to-k8s/pkg/config"
	"github.com/supporttools/push-to-k8s/pkg/k8s"
	"github.com/supporttools/push-to-k8s/pkg/logging"
	"github.com/supporttools/push-to-k8s/pkg/metrics"
	"k8s.io/client-go/kubernetes"
)

func main() {
	// Load configuration from environment
	cfg := config.LoadConfigFromEnv()

	// Setup logging with debug level from config
	log := logging.SetupLogging(cfg.Debug)

	logConfigStatus(cfg, log)

	// Initialize Kubernetes client
	clientset := initializeK8sClient(log)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// WaitGroup to track all goroutines
	var wg sync.WaitGroup

	// Start Prometheus metrics server
	startMetricsServer(cfg, log)

	// Start periodic secret sync and namespace watcher
	startPeriodicSync(ctx, &wg, clientset, cfg, log)
	startNamespaceWatcher(ctx, &wg, clientset, cfg, log)

	// Start periodic metrics updates
	startMetricsUpdater(ctx, &wg, clientset, cfg, log)

	// Wait for shutdown signal
	sig := <-sigChan
	log.Infof("Received signal %v, initiating graceful shutdown...", sig)

	// Cancel context to stop all goroutines
	cancel()

	// Wait for all goroutines to complete with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info("All goroutines completed successfully")
	case <-time.After(30 * time.Second):
		log.Warn("Shutdown timeout reached, forcing exit")
	}

	log.Info("Shutdown complete")
}

func logConfigStatus(cfg config.Config, log *logrus.Logger) {
	if cfg.Debug {
		log.Debug("Debug mode enabled")
	} else {
		log.Info("Debug mode disabled")
	}

	if cfg.Namespace == "" {
		log.Fatalf("Source namespace is not specified. Set the NAMESPACE environment variable.")
	}
}

func initializeK8sClient(log *logrus.Logger) *kubernetes.Clientset {
	clientset, err := k8s.CreateClusterConnection(log)
	if err != nil {
		log.Fatalf("Failed to connect to Kubernetes cluster: %v", err)
	}
	return clientset
}

func startMetricsServer(cfg config.Config, log *logrus.Logger) {
	metricsPort := fmt.Sprintf(":%d", cfg.MetricsPort)
	go metrics.StartMetricsServer(metricsPort, log)
}

func startPeriodicSync(ctx context.Context, wg *sync.WaitGroup, clientset *kubernetes.Clientset, cfg config.Config, log *logrus.Logger) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer log.Info("Periodic sync goroutine stopped")

		// Perform initial sync immediately on startup
		log.Info("Performing initial secret sync on startup")
		if err := k8s.SyncSecrets(clientset, cfg.Namespace, cfg.ExcludeNamespaceLabel, log); err != nil {
			log.Errorf("Error during initial sync: %v", err)
		}

		// Start periodic sync
		ticker := time.NewTicker(time.Duration(cfg.SyncInterval) * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Info("Periodic sync shutting down...")
				return
			case <-ticker.C:
				if err := k8s.SyncSecrets(clientset, cfg.Namespace, cfg.ExcludeNamespaceLabel, log); err != nil {
					log.Errorf("Error syncing secrets: %v", err)
				}
			}
		}
	}()
}

func startNamespaceWatcher(ctx context.Context, wg *sync.WaitGroup, clientset *kubernetes.Clientset, cfg config.Config, log *logrus.Logger) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer log.Info("Namespace watcher goroutine stopped")
		k8s.WatchNamespaces(ctx, clientset, cfg.Namespace, cfg.ExcludeNamespaceLabel, log)
	}()
}

func startMetricsUpdater(ctx context.Context, wg *sync.WaitGroup, clientset *kubernetes.Clientset, cfg config.Config, log *logrus.Logger) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer log.Info("Metrics updater goroutine stopped")

		// Update metrics every 60 seconds
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Info("Metrics updater shutting down...")
				return
			case <-ticker.C:
				metrics.SyncMetrics(clientset, cfg.Namespace, log)
			}
		}
	}()
}
