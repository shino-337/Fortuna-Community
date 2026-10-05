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
