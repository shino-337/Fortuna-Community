package collector

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"os"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"github.com/fortuna/agent/internal/client"
	"github.com/fortuna/agent/internal/converter"
	"github.com/fortuna/agent/internal/watcher"
	fortuna "github.com/fortuna/api/proto/agent"
)

// WatcherInterface for all watchers
type WatcherInterface interface {
	Start() error
	Stop()
}

// Collector collects Kubernetes resources and forwards them to the core
type Collector struct {
	client      kubernetes.Interface
	grpcClient  client.GRPCClient
	clusterID   string
	clusterName string
	watchers    []WatcherInterface
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	started     chan struct{} // Signal that watchers have started
	mu          sync.Mutex    // Protects started channel
}

// NewCollector creates a new collector
func NewCollector(k8sClient kubernetes.Interface, grpcClient client.GRPCClient, clusterID, clusterName string) (*Collector, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &Collector{
		client:      k8sClient,
		grpcClient:  grpcClient,
		clusterID:   clusterID,
		clusterName: clusterName,
		ctx:         ctx,
		cancel:      cancel,
		started:     make(chan struct{}),
	}, nil
}

// Start starts the collector
// Bug 1 & 2 Fix: Add synchronization to ensure watchers are ready before returning
func (c *Collector) Start() error {
	log.Printf("[Collector] Starting collector for cluster: %s (%s)", c.clusterName, c.clusterID)

	// Register with core
	if err := c.register(); err != nil {
		return fmt.Errorf("failed to register: %w", err)
	}

	// Start watchers for all namespaces
	if err := c.startWatchers(); err != nil {
		return fmt.Errorf("failed to start watchers: %w", err)
	}

	// Start heartbeat
	c.wg.Add(1)
	go c.heartbeat()

	// Bug 1 & 2 Fix: Wait for watchers to initialize (with timeout)
	select {
	case <-c.started:
		log.Printf("[Collector] All watchers started successfully")
	case <-time.After(30 * time.Second):
		log.Printf("[Collector] Warning: Timeout waiting for watchers to start")
	case <-c.ctx.Done():
		return fmt.Errorf("collector context cancelled during startup")
	}

	return nil
}

// register registers the agent with the core
func (c *Collector) register() error {
	log.Printf("[Collector] Registering agent with core...")

	req := &fortuna.RegisterRequest{
		ClusterId: c.clusterID,
		NodeName:  getNodeName(),
		Version:   "1.0.0",
	}

	resp, err := c.grpcClient.Register(context.Background(), req)
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	if !resp.Ok {
		return fmt.Errorf("registration failed: %s", resp.Message)
	}

	log.Printf("[Collector] Registered successfully")
	return nil
}

// startWatchers starts all resource watchers
// Bug 3 Fix: Signal when all watchers have started
func (c *Collector) startWatchers() error {
	// Get all namespaces
	namespaces, err := c.client.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	log.Printf("[Collector] Starting watchers for %d namespaces", len(namespaces.Items))

	// Start watchers for each namespace
	for _, ns := range namespaces.Items {
		c.startNamespaceWatchers(ns.Name)
	}

	// Start cluster-scoped watchers
	c.startClusterWatchers()

	// Bug 3 Fix: Signal that all watchers have been started
	c.mu.Lock()
	select {
	case <-c.started:
		// Already closed, do nothing
	default:
		close(c.started)
	}
	c.mu.Unlock()

	return nil
}

// startNamespaceWatchers starts watchers for a specific namespace
// Bug 3 Fix: Add synchronization to ensure watchers have started
func (c *Collector) startNamespaceWatchers(namespace string) {
	watcherStarted := make(chan struct{}, 4) // Buffer for 4 watchers per namespace

	// Pod watcher
	podWatcher := watcher.NewPodWatcher(c.client, namespace, c.handlePod)
	c.watchers = append(c.watchers, podWatcher)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		watcherStarted <- struct{}{}
		if err := podWatcher.Start(); err != nil {
			log.Printf("[Collector] Pod watcher error: %v", err)
		}
	}()

	// ServiceAccount watcher
	saWatcher := watcher.NewServiceAccountWatcher(c.client, namespace, c.handleServiceAccount)
	c.watchers = append(c.watchers, saWatcher)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		watcherStarted <- struct{}{}
		if err := saWatcher.Start(); err != nil {
			log.Printf("[Collector] ServiceAccount watcher error: %v", err)
		}
	}()

	// Role watcher
	roleWatcher := watcher.NewRoleWatcher(c.client, namespace, c.handleRole)
	c.watchers = append(c.watchers, roleWatcher)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		watcherStarted <- struct{}{}
		if err := roleWatcher.Start(); err != nil {
			log.Printf("[Collector] Role watcher error: %v", err)
		}
	}()

	// RoleBinding watcher
	rbWatcher := watcher.NewRoleBindingWatcher(c.client, namespace, c.handleRoleBinding)
	c.watchers = append(c.watchers, rbWatcher)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		watcherStarted <- struct{}{}
		if err := rbWatcher.Start(); err != nil {
			log.Printf("[Collector] RoleBinding watcher error: %v", err)
		}
	}()

	// Wait for all watchers in this namespace to start (with timeout)
	timeout := time.After(10 * time.Second)
	for i := 0; i < 4; i++ {
		select {
		case <-watcherStarted:
			// Watcher started
		case <-timeout:
			log.Printf("[Collector] Warning: Timeout waiting for watchers in namespace %s", namespace)
			return
		case <-c.ctx.Done():
			return
		}
	}
}

// startClusterWatchers starts cluster-scoped watchers
// Bug 3 Fix: Add synchronization for cluster watchers
func (c *Collector) startClusterWatchers() {
	watcherStarted := make(chan struct{}, 2) // Buffer for 2 cluster watchers

	// ClusterRole watcher
	crWatcher := watcher.NewClusterRoleWatcher(c.client, c.handleClusterRole)
	c.watchers = append(c.watchers, crWatcher)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		watcherStarted <- struct{}{}
		if err := crWatcher.Start(); err != nil {
			log.Printf("[Collector] ClusterRole watcher error: %v", err)
		}
	}()

	// ClusterRoleBinding watcher
	crbWatcher := watcher.NewClusterRoleBindingWatcher(c.client, c.handleClusterRoleBinding)
	c.watchers = append(c.watchers, crbWatcher)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		watcherStarted <- struct{}{}
		if err := crbWatcher.Start(); err != nil {
			log.Printf("[Collector] ClusterRoleBinding watcher error: %v", err)
		}
	}()

	// Wait for cluster watchers to start (with timeout)
	timeout := time.After(10 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case <-watcherStarted:
			// Watcher started
		case <-timeout:
			log.Printf("[Collector] Warning: Timeout waiting for cluster watchers")
			return
		case <-c.ctx.Done():
			return
		}
	}
}

// handlePod handles Pod events
func (c *Collector) handlePod(pod *corev1.Pod, eventType watch.EventType) error {
	item := converter.PodToInventoryItem(pod, c.clusterID, eventType)
	return c.sendInventoryItem(item)
}

// handleServiceAccount handles ServiceAccount events
func (c *Collector) handleServiceAccount(sa *corev1.ServiceAccount, eventType watch.EventType) error {
	item := converter.ServiceAccountToInventoryItem(sa, c.clusterID, eventType)
	return c.sendInventoryItem(item)
}

// handleRole handles Role events
func (c *Collector) handleRole(role *rbacv1.Role, eventType watch.EventType) error {
	item := converter.RoleToInventoryItem(role, c.clusterID, eventType)
	return c.sendInventoryItem(item)
}

// handleRoleBinding handles RoleBinding events
func (c *Collector) handleRoleBinding(rb *rbacv1.RoleBinding, eventType watch.EventType) error {
	item := converter.RoleBindingToInventoryItem(rb, c.clusterID, eventType)
	return c.sendInventoryItem(item)
}

// handleClusterRole handles ClusterRole events
func (c *Collector) handleClusterRole(cr *rbacv1.ClusterRole, eventType watch.EventType) error {
	item := converter.ClusterRoleToInventoryItem(cr, c.clusterID, eventType)
	return c.sendInventoryItem(item)
}

// handleClusterRoleBinding handles ClusterRoleBinding events
func (c *Collector) handleClusterRoleBinding(crb *rbacv1.ClusterRoleBinding, eventType watch.EventType) error {
	item := converter.ClusterRoleBindingToInventoryItem(crb, c.clusterID, eventType)
	return c.sendInventoryItem(item)
}

// sendInventoryItem sends an inventory item to the core
func (c *Collector) sendInventoryItem(item *fortuna.InventoryItem) error {
	items := []*fortuna.InventoryItem{item}
	return c.grpcClient.StreamInventory(context.Background(), items)
}

// heartbeat sends periodic heartbeat to the core
func (c *Collector) heartbeat() {
	defer c.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			req := &fortuna.RegisterRequest{
				ClusterId: c.clusterID,
				NodeName:  getNodeName(),
				Version:   "1.0.0",
				AgentId:   getNodeName(),
			}
			if err := c.grpcClient.Heartbeat(context.Background(), req); err != nil {
				log.Printf("[Collector] Heartbeat error: %v", err)
			}
		}
	}
}

// Stop stops the collector
func (c *Collector) Stop() {
	log.Printf("[Collector] Stopping collector...")
	c.cancel()

	// Stop all watchers
	for _, w := range c.watchers {
		w.Stop()
	}

	// Wait for all goroutines
	c.wg.Wait()
	log.Printf("[Collector] Stopped")
}

// getNodeName gets the current node name
func getNodeName() string {
	// Try to get from environment variable
	if nodeName := os.Getenv("NODE_NAME"); nodeName != "" {
		return nodeName
	}
	// Try to get from hostname
	if hostname, err := os.Hostname(); err == nil {
		return hostname
	}
	return "unknown"
}
