package watcher

import (
	"context"
	"fmt"
	"log"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// RoleWatcher watches for Role changes
type RoleWatcher struct {
	client    kubernetes.Interface
	namespace string
	handler   func(*rbacv1.Role, watch.EventType) error
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewRoleWatcher creates a new Role watcher
func NewRoleWatcher(client kubernetes.Interface, namespace string, handler func(*rbacv1.Role, watch.EventType) error) *RoleWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &RoleWatcher{
		client:    client,
		namespace: namespace,
		handler:   handler,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start starts watching for Role changes
func (w *RoleWatcher) Start() error {
	log.Printf("[RoleWatcher] Starting watcher for namespace: %s", w.namespace)

	watcher, err := w.client.RbacV1().Roles(w.namespace).Watch(w.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to create role watcher: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-w.ctx.Done():
			log.Printf("[RoleWatcher] Watcher stopped")
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				log.Printf("[RoleWatcher] Watcher channel closed, reconnecting...")
				time.Sleep(5 * time.Second)
				watcher, err = w.client.RbacV1().Roles(w.namespace).Watch(w.ctx, metav1.ListOptions{})
				if err != nil {
					return fmt.Errorf("failed to reconnect role watcher: %w", err)
				}
				continue
			}

			role, ok := event.Object.(*rbacv1.Role)
			if !ok {
				log.Printf("[RoleWatcher] Unexpected object type: %T", event.Object)
				continue
			}

			if err := w.handler(role, event.Type); err != nil {
				log.Printf("[RoleWatcher] Handler error: %v", err)
			}
		}
	}
}

// Stop stops the watcher
func (w *RoleWatcher) Stop() {
	w.cancel()
}

// ClusterRoleWatcher watches for ClusterRole changes
type ClusterRoleWatcher struct {
	client  kubernetes.Interface
	handler func(*rbacv1.ClusterRole, watch.EventType) error
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewClusterRoleWatcher creates a new ClusterRole watcher
func NewClusterRoleWatcher(client kubernetes.Interface, handler func(*rbacv1.ClusterRole, watch.EventType) error) *ClusterRoleWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &ClusterRoleWatcher{
		client:  client,
		handler: handler,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start starts watching for ClusterRole changes
func (w *ClusterRoleWatcher) Start() error {
	log.Printf("[ClusterRoleWatcher] Starting watcher")

	watcher, err := w.client.RbacV1().ClusterRoles().Watch(w.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to create clusterrole watcher: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-w.ctx.Done():
			log.Printf("[ClusterRoleWatcher] Watcher stopped")
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				log.Printf("[ClusterRoleWatcher] Watcher channel closed, reconnecting...")
				time.Sleep(5 * time.Second)
				watcher, err = w.client.RbacV1().ClusterRoles().Watch(w.ctx, metav1.ListOptions{})
				if err != nil {
					return fmt.Errorf("failed to reconnect clusterrole watcher: %w", err)
				}
				continue
			}

			role, ok := event.Object.(*rbacv1.ClusterRole)
			if !ok {
				log.Printf("[ClusterRoleWatcher] Unexpected object type: %T", event.Object)
				continue
			}

			if err := w.handler(role, event.Type); err != nil {
				log.Printf("[ClusterRoleWatcher] Handler error: %v", err)
			}
		}
	}
}

// Stop stops the watcher
func (w *ClusterRoleWatcher) Stop() {
	w.cancel()
}

// RoleBindingWatcher watches for RoleBinding changes
type RoleBindingWatcher struct {
	client    kubernetes.Interface
	namespace string
	handler   func(*rbacv1.RoleBinding, watch.EventType) error
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewRoleBindingWatcher creates a new RoleBinding watcher
func NewRoleBindingWatcher(client kubernetes.Interface, namespace string, handler func(*rbacv1.RoleBinding, watch.EventType) error) *RoleBindingWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &RoleBindingWatcher{
		client:    client,
		namespace: namespace,
		handler:   handler,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start starts watching for RoleBinding changes
func (w *RoleBindingWatcher) Start() error {
	log.Printf("[RoleBindingWatcher] Starting watcher for namespace: %s", w.namespace)

	watcher, err := w.client.RbacV1().RoleBindings(w.namespace).Watch(w.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to create rolebinding watcher: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-w.ctx.Done():
			log.Printf("[RoleBindingWatcher] Watcher stopped")
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				log.Printf("[RoleBindingWatcher] Watcher channel closed, reconnecting...")
				time.Sleep(5 * time.Second)
				watcher, err = w.client.RbacV1().RoleBindings(w.namespace).Watch(w.ctx, metav1.ListOptions{})
				if err != nil {
					return fmt.Errorf("failed to reconnect rolebinding watcher: %w", err)
				}
				continue
			}

			rb, ok := event.Object.(*rbacv1.RoleBinding)
			if !ok {
				log.Printf("[RoleBindingWatcher] Unexpected object type: %T", event.Object)
				continue
			}

			if err := w.handler(rb, event.Type); err != nil {
				log.Printf("[RoleBindingWatcher] Handler error: %v", err)
			}
		}
	}
}

// Stop stops the watcher
func (w *RoleBindingWatcher) Stop() {
	w.cancel()
}

// ClusterRoleBindingWatcher watches for ClusterRoleBinding changes
type ClusterRoleBindingWatcher struct {
	client  kubernetes.Interface
	handler func(*rbacv1.ClusterRoleBinding, watch.EventType) error
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewClusterRoleBindingWatcher creates a new ClusterRoleBinding watcher
func NewClusterRoleBindingWatcher(client kubernetes.Interface, handler func(*rbacv1.ClusterRoleBinding, watch.EventType) error) *ClusterRoleBindingWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &ClusterRoleBindingWatcher{
		client:  client,
		handler: handler,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start starts watching for ClusterRoleBinding changes
func (w *ClusterRoleBindingWatcher) Start() error {
	log.Printf("[ClusterRoleBindingWatcher] Starting watcher")

	watcher, err := w.client.RbacV1().ClusterRoleBindings().Watch(w.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to create clusterrolebinding watcher: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-w.ctx.Done():
			log.Printf("[ClusterRoleBindingWatcher] Watcher stopped")
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				log.Printf("[ClusterRoleBindingWatcher] Watcher channel closed, reconnecting...")
				time.Sleep(5 * time.Second)
				watcher, err = w.client.RbacV1().ClusterRoleBindings().Watch(w.ctx, metav1.ListOptions{})
				if err != nil {
					return fmt.Errorf("failed to reconnect clusterrolebinding watcher: %w", err)
				}
				continue
			}

			crb, ok := event.Object.(*rbacv1.ClusterRoleBinding)
			if !ok {
				log.Printf("[ClusterRoleBindingWatcher] Unexpected object type: %T", event.Object)
				continue
			}

			if err := w.handler(crb, event.Type); err != nil {
				log.Printf("[ClusterRoleBindingWatcher] Handler error: %v", err)
			}
		}
	}
}

// Stop stops the watcher
func (w *ClusterRoleBindingWatcher) Stop() {
	w.cancel()
}


