package collector

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/ksam/agent/internal/client"
	"github.com/ksam/agent/internal/config"
)

// New creates a new collector using the new implementation
func New(cfg *config.Config) (*Collector, error) {
	// Create Kubernetes client
	k8sClient, err := createK8sClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	// Create gRPC client
	grpcClient, err := client.NewNewGRPCClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	// Get cluster ID and name
	clusterID := cfg.ClusterID
	if clusterID == "" {
		clusterID = "default"
	}
	clusterName := clusterID // Use clusterID as name for now

	return NewCollector(k8sClient, grpcClient, clusterID, clusterName)
}

// createK8sClient creates a Kubernetes client from config
func createK8sClient(cfg *config.Config) (kubernetes.Interface, error) {
	// Use in-cluster config if kubeconfig is empty
	if cfg.Kubeconfig == "" {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
		}
		return kubernetes.NewForConfig(config)
	}

	// Use kubeconfig file
	config, err := clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}
	return kubernetes.NewForConfig(config)
}

