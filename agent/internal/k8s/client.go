package k8s

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/fortuna/agent/internal/config"
)

// Client wraps Kubernetes clientset
type Client struct {
	Clientset kubernetes.Interface
	Config    *rest.Config
}

// NewClient creates a new Kubernetes client
func NewClient(cfg *config.Config) (*Client, error) {
	var restConfig *rest.Config
	var err error

	if cfg.Kubeconfig != "" {
		// Use kubeconfig file
		restConfig, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
		if err != nil {
			return nil, err
		}
	} else {
		// Use in-cluster config
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &Client{
		Clientset: clientset,
		Config:    restConfig,
	}, nil
}

// GetCurrentContext returns the current kubeconfig context
func GetCurrentContext(kubeconfigPath string) (string, error) {
	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return "", err
	}
	return config.CurrentContext, nil
}

// GetClusterName extracts the cluster name (context.cluster) from kubeconfig.
// This matches the name shown by `kubectl config get-clusters`.
func GetClusterName(kubeconfigPath string) (string, error) {
	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return "", err
	}

	ctx := config.CurrentContext
	if ctx == "" {
		return "unknown", nil
	}

	kctx, ok := config.Contexts[ctx]
	if !ok {
		return "unknown", nil
	}

	// Cluster name in kubeconfig (context.cluster); matches kubectl config get-clusters
	if kctx.Cluster == "" {
		return "unknown", nil
	}
	return kctx.Cluster, nil
}

