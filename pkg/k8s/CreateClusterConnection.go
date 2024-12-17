package k8s

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/supporttools/push-to-k8s/pkg/metrics"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// CreateClusterConnection creates a Kubernetes clientset.
// It uses the KUBECONFIG environment variable if set, or falls back to in-cluster config.
func CreateClusterConnection(logger *logrus.Logger) (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error
	source := "in-cluster"

	// Check for KUBECONFIG environment variable
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig != "" {
		logger.Infof("Using KUBECONFIG from environment: %s", kubeconfig)
		source = "kubeconfig"
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			metrics.K8sConnectionFailures.WithLabelValues(source, err.Error()).Inc()
			logger.Fatalf("Failed to create config from KUBECONFIG: %v", err)
			return nil, err
		}
	} else {
		logger.Info("KUBECONFIG not set, using in-cluster config")
		config, err = rest.InClusterConfig()
		if err != nil {
			metrics.K8sConnectionFailures.WithLabelValues(source, err.Error()).Inc()
			logger.Fatalf("Failed to create in-cluster config: %v", err)
			return nil, err
		}
	}

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		metrics.K8sConnectionFailures.WithLabelValues(source, err.Error()).Inc()
		logger.Fatalf("Failed to create clientset: %v", err)
		return nil, err
	}

	metrics.K8sConnectionSuccess.WithLabelValues(source).Inc()
	logger.Infof("Successfully connected to Kubernetes cluster using %s configuration", source)

	return clientset, nil
}
