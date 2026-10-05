package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps Kubernetes clientset
type Client struct {
	Clientset kubernetes.Interface
	Config    *rest.Config
}

// NewClientFromKubeconfig creates a Kubernetes client from kubeconfig content
func NewClientFromKubeconfig(kubeconfigContent string) (*Client, error) {
	if kubeconfigContent == "" {
		// Try to use default kubeconfig path
		return NewClientFromPath("")
	}

	// Create temporary file with kubeconfig content
	tmpFile, err := os.CreateTemp("", "kubeconfig-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(kubeconfigContent); err != nil {
		return nil, fmt.Errorf("failed to write kubeconfig: %w", err)
	}
	tmpFile.Close()

	return NewClientFromPath(tmpFile.Name())
}

// NewClientFromPath creates a Kubernetes client from kubeconfig file path
func NewClientFromPath(kubeconfigPath string) (*Client, error) {
	var restConfig *rest.Config
	var err error

	if kubeconfigPath == "" {
		// Try default kubeconfig locations
		homeDir, _ := os.UserHomeDir()
		defaultPaths := []string{
			filepath.Join(homeDir, ".kube", "config"),
			os.Getenv("KUBECONFIG"),
		}

		for _, path := range defaultPaths {
			if path != "" {
				if _, err := os.Stat(path); err == nil {
					kubeconfigPath = path
					break
				}
			}
		}

		// If still no kubeconfig, try in-cluster config
		if kubeconfigPath == "" {
			restConfig, err = rest.InClusterConfig()
			if err != nil {
				return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
			}
		}
	}

	if restConfig == nil {
		// Build config from kubeconfig file
		restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Client{
		Clientset: clientset,
		Config:    restConfig,
	}, nil
}

// DeleteServiceAccountUID prevents deletion of a replacement object with the same name.
func (c *Client) DeleteServiceAccountUID(ctx context.Context, namespace, name, uid string) error {
	if uid == "" {
		return fmt.Errorf("ServiceAccount UID required")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	target := types.UID(uid)
	err := c.Clientset.CoreV1().ServiceAccounts(namespace).Delete(ctx, name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &target}})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
