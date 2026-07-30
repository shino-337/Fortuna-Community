package watcher

import (
	"context"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// ServiceAccountWatcher watches for ServiceAccount changes
type ServiceAccountWatcher struct {
	client    kubernetes.Interface
	namespace string
	handler   func(*corev1.ServiceAccount, watch.EventType) error
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewServiceAccountWatcher creates a new ServiceAccount watcher
func NewServiceAccountWatcher(client kubernetes.Interface, namespace string, handler func(*corev1.ServiceAccount, watch.EventType) error) *ServiceAccountWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &ServiceAccountWatcher{
		client:    client,
		namespace: namespace,
		handler:   handler,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start starts watching for ServiceAccount changes
func (w *ServiceAccountWatcher) Start() error {
	log.Printf("[ServiceAccountWatcher] Starting watcher for namespace: %s", w.namespace)

	watcher, err := w.client.CoreV1().ServiceAccounts(w.namespace).Watch(w.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to create serviceaccount watcher: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-w.ctx.Done():
			log.Printf("[ServiceAccountWatcher] Watcher stopped")
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				log.Printf("[ServiceAccountWatcher] Watcher channel closed, reconnecting...")
				time.Sleep(5 * time.Second)
				watcher, err = w.client.CoreV1().ServiceAccounts(w.namespace).Watch(w.ctx, metav1.ListOptions{})
				if err != nil {
					return fmt.Errorf("failed to reconnect serviceaccount watcher: %w", err)
				}
				continue
			}

			sa, ok := event.Object.(*corev1.ServiceAccount)
			if !ok {
				log.Printf("[ServiceAccountWatcher] Unexpected object type: %T", event.Object)
				continue
			}

			if err := w.handler(sa, event.Type); err != nil {
				log.Printf("[ServiceAccountWatcher] Handler error: %v", err)
			}
		}
	}
}

// Stop stops the watcher
func (w *ServiceAccountWatcher) Stop() {
	w.cancel()
}


