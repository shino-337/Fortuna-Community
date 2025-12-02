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

// PodWatcher watches for Pod changes
type PodWatcher struct {
	client    kubernetes.Interface
	namespace string
	handler   func(*corev1.Pod, watch.EventType) error
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewPodWatcher creates a new Pod watcher
func NewPodWatcher(client kubernetes.Interface, namespace string, handler func(*corev1.Pod, watch.EventType) error) *PodWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &PodWatcher{
		client:    client,
		namespace: namespace,
		handler:   handler,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start starts watching for Pod changes
func (w *PodWatcher) Start() error {
	log.Printf("[PodWatcher] Starting watcher for namespace: %s", w.namespace)

	watcher, err := w.client.CoreV1().Pods(w.namespace).Watch(w.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to create pod watcher: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-w.ctx.Done():
			log.Printf("[PodWatcher] Watcher stopped")
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				log.Printf("[PodWatcher] Watcher channel closed, reconnecting...")
				time.Sleep(5 * time.Second)
				watcher, err = w.client.CoreV1().Pods(w.namespace).Watch(w.ctx, metav1.ListOptions{})
				if err != nil {
					return fmt.Errorf("failed to reconnect pod watcher: %w", err)
				}
				continue
			}

			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				log.Printf("[PodWatcher] Unexpected object type: %T", event.Object)
				continue
			}

			if err := w.handler(pod, event.Type); err != nil {
				log.Printf("[PodWatcher] Handler error: %v", err)
			}
		}
	}
}

// Stop stops the watcher
func (w *PodWatcher) Stop() {
	w.cancel()
}


