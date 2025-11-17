package k8s

import (
	"context"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

// InformerFactory wraps Kubernetes informer factory with shared cache
type InformerFactory struct {
	factory informers.SharedInformerFactory
	stopCh  chan struct{}
}

// NewInformerFactory creates a new shared informer factory
func NewInformerFactory(clientset kubernetes.Interface, namespace string, resyncPeriod time.Duration) *InformerFactory {
	var factory informers.SharedInformerFactory

	if namespace != "" {
		factory = informers.NewSharedInformerFactoryWithOptions(
			clientset,
			resyncPeriod,
			informers.WithNamespace(namespace),
		)
	} else {
		factory = informers.NewSharedInformerFactory(clientset, resyncPeriod)
	}

	return &InformerFactory{
		factory: factory,
		stopCh:  make(chan struct{}),
	}
}

// Start starts all informers
func (f *InformerFactory) Start(ctx context.Context) {
	f.factory.Start(f.stopCh)

	// Wait for context cancellation
	go func() {
		<-ctx.Done()
		close(f.stopCh)
	}()
}

// GetFactory returns the underlying informer factory
func (f *InformerFactory) GetFactory() informers.SharedInformerFactory {
	return f.factory
}

// Stop stops all informers
func (f *InformerFactory) Stop() {
	close(f.stopCh)
}

