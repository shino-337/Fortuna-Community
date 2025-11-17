package collector

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	grpcClient "github.com/ksam/agent/internal/client"
	"github.com/ksam/agent/internal/config"
	"github.com/ksam/agent/internal/k8s"
	"github.com/ksam/agent/pkg/types"
)

// Collector collects ServiceAccount and RBAC data from Kubernetes
type Collector struct {
	config        *config.Config
	k8s           *k8s.Client
	grpcClient    *grpcClient.Client
	lastSyncState map[string]string // Track last sync state: resourceType:UID -> hash
	lastFullSync  time.Time
	fullSyncCount int
}

// New creates a new Collector
func New(cfg *config.Config) (*Collector, error) {
	k8sClient, err := k8s.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	// Initialize gRPC client to Core Controller
	grpcClient, err := grpcClient.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	return &Collector{
		config:        cfg,
		k8s:           k8sClient,
		grpcClient:    grpcClient,
		lastSyncState: make(map[string]string),
		fullSyncCount: 0,
	}, nil
}

func canonicalizeLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(labels))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, labels[k]))
	}
	return strings.Join(pairs, ",")
}

// hashResource creates a simple hash for resource to detect changes
func (c *Collector) hashResource(resourceType, uid string, data interface{}) string {
	switch v := data.(type) {
	case types.ServiceAccountData:
		return fmt.Sprintf("%s:%s:%s:%s:%s:%s",
			resourceType,
			uid,
			v.Name,
			v.Namespace,
			canonicalizeLabels(v.Labels),
			strings.Join(v.Secrets, ","),
		)
	default:
		return fmt.Sprintf("%s:%s:%v", resourceType, uid, data)
	}
}

// Collect collects all ServiceAccount and RBAC data
// Uses delta sync: first sync is full, subsequent syncs only send changes
func (c *Collector) Collect(ctx context.Context) error {
	// Determine if this should be a full sync
	// Full sync: first sync, or every 10th sync (every ~5 minutes with 30s interval)
	shouldFullSync := c.fullSyncCount == 0 || (c.fullSyncCount%10 == 0)

	if shouldFullSync {
		log.Println("Starting FULL sync collection...")
		c.lastSyncState = make(map[string]string) // Reset state for full sync
	} else {
		log.Println("Starting DELTA sync collection (changes only)...")
	}

	c.fullSyncCount++

	data := &types.CollectedData{
		ClusterID:           c.config.ClusterID,
		ServiceAccounts:     []types.ServiceAccountData{},
		RoleBindings:        []types.RoleBindingData{},
		ClusterRoleBindings: []types.ClusterRoleBindingData{},
		Roles:               []types.RoleData{},
		ClusterRoles:        []types.ClusterRoleData{},
		Pods:                []types.PodData{},
		CollectedAt:         metav1.NewTime(time.Now()),
		IsFullSync:          shouldFullSync,
		IsDeltaSync:         !shouldFullSync,
	}

	// Collect ServiceAccounts (with delta sync support)
	if err := c.collectServiceAccounts(ctx, data, shouldFullSync); err != nil {
		log.Printf("Error collecting ServiceAccounts: %v", err)
	}

	if shouldFullSync {
		if err := c.collectRoleBindings(ctx, data); err != nil {
			log.Printf("Error collecting RoleBindings: %v", err)
		}
		if err := c.collectClusterRoleBindings(ctx, data); err != nil {
			log.Printf("Error collecting ClusterRoleBindings: %v", err)
		}
		if err := c.collectRoles(ctx, data); err != nil {
			log.Printf("Error collecting Roles: %v", err)
		}
		if err := c.collectClusterRoles(ctx, data); err != nil {
			log.Printf("Error collecting ClusterRoles: %v", err)
		}
		if err := c.collectPods(ctx, data); err != nil {
			log.Printf("Error collecting Pods: %v", err)
		}
	} else {
		log.Println("Skipping RBAC/Pod collections for DELTA sync (handled via full snapshots)")
	}

	syncType := "FULL"
	if !shouldFullSync {
		syncType = "DELTA"
	}
	log.Printf("Collection complete (%s): %d SAs, %d RBs, %d CRBs, %d Roles, %d CRoles, %d Pods",
		syncType,
		len(data.ServiceAccounts),
		len(data.RoleBindings),
		len(data.ClusterRoleBindings),
		len(data.Roles),
		len(data.ClusterRoles),
		len(data.Pods))

	// Send data to Core Controller via gRPC with retry
	if err := c.grpcClient.SendData(ctx, data); err != nil {
		return fmt.Errorf("failed to send data to core: %w", err)
	}

	// Update last sync time
	c.lastFullSync = time.Now()

	return nil
}

// collectServiceAccounts collects ServiceAccounts (with delta sync support)
func (c *Collector) collectServiceAccounts(ctx context.Context, data *types.CollectedData, fullSync bool) error {
	var listOptions metav1.ListOptions

	// If WatchNamespace is set, only collect from that namespace
	// Otherwise, collect from all namespaces
	if c.config.WatchNamespace != "" {
		saList, err := c.k8s.Clientset.CoreV1().ServiceAccounts(c.config.WatchNamespace).List(ctx, listOptions)
		if err != nil {
			return err
		}
		for i := range saList.Items {
			sa := &saList.Items[i]
			saData := types.ConvertServiceAccount(sa)

			// Delta sync: only include if changed or new
			if !fullSync {
				key := fmt.Sprintf("sa:%s", saData.UID)
				currentHash := c.hashResource("sa", saData.UID, saData)
				lastHash, exists := c.lastSyncState[key]

				if !exists || currentHash != lastHash {
					// Changed or new - include in delta
					data.ServiceAccounts = append(data.ServiceAccounts, saData)
					c.lastSyncState[key] = currentHash
				}
			} else {
				// Full sync: include all
				data.ServiceAccounts = append(data.ServiceAccounts, saData)
				key := fmt.Sprintf("sa:%s", saData.UID)
				c.lastSyncState[key] = c.hashResource("sa", saData.UID, saData)
			}
		}
	} else {
		// Collect from all namespaces
		saList, err := c.k8s.Clientset.CoreV1().ServiceAccounts("").List(ctx, listOptions)
		if err != nil {
			return err
		}
		for i := range saList.Items {
			sa := &saList.Items[i]
			saData := types.ConvertServiceAccount(sa)

			// Delta sync: only include if changed or new
			if !fullSync {
				key := fmt.Sprintf("sa:%s", saData.UID)
				currentHash := c.hashResource("sa", saData.UID, saData)
				lastHash, exists := c.lastSyncState[key]

				if !exists || currentHash != lastHash {
					// Changed or new - include in delta
					data.ServiceAccounts = append(data.ServiceAccounts, saData)
					c.lastSyncState[key] = currentHash
				}
			} else {
				// Full sync: include all
				data.ServiceAccounts = append(data.ServiceAccounts, saData)
				key := fmt.Sprintf("sa:%s", saData.UID)
				c.lastSyncState[key] = c.hashResource("sa", saData.UID, saData)
			}
		}
	}

	return nil
}

// collectRoleBindings collects all RoleBindings
func (c *Collector) collectRoleBindings(ctx context.Context, data *types.CollectedData) error {
	var listOptions metav1.ListOptions

	if c.config.WatchNamespace != "" {
		rbList, err := c.k8s.Clientset.RbacV1().RoleBindings(c.config.WatchNamespace).List(ctx, listOptions)
		if err != nil {
			return err
		}
		for i := range rbList.Items {
			rb := &rbList.Items[i]
			data.RoleBindings = append(data.RoleBindings, types.ConvertRoleBinding(rb))
		}
	} else {
		rbList, err := c.k8s.Clientset.RbacV1().RoleBindings("").List(ctx, listOptions)
		if err != nil {
			return err
		}
		for i := range rbList.Items {
			rb := &rbList.Items[i]
			data.RoleBindings = append(data.RoleBindings, types.ConvertRoleBinding(rb))
		}
	}

	return nil
}

// collectClusterRoleBindings collects all ClusterRoleBindings
func (c *Collector) collectClusterRoleBindings(ctx context.Context, data *types.CollectedData) error {
	var listOptions metav1.ListOptions

	crbList, err := c.k8s.Clientset.RbacV1().ClusterRoleBindings().List(ctx, listOptions)
	if err != nil {
		return err
	}

	for i := range crbList.Items {
		crb := &crbList.Items[i]
		data.ClusterRoleBindings = append(data.ClusterRoleBindings, types.ConvertClusterRoleBinding(crb))
	}

	return nil
}

// collectRoles collects all Roles
func (c *Collector) collectRoles(ctx context.Context, data *types.CollectedData) error {
	var listOptions metav1.ListOptions

	if c.config.WatchNamespace != "" {
		roleList, err := c.k8s.Clientset.RbacV1().Roles(c.config.WatchNamespace).List(ctx, listOptions)
		if err != nil {
			return err
		}
		for i := range roleList.Items {
			role := &roleList.Items[i]
			data.Roles = append(data.Roles, types.ConvertRole(role))
		}
	} else {
		roleList, err := c.k8s.Clientset.RbacV1().Roles("").List(ctx, listOptions)
		if err != nil {
			return err
		}
		for i := range roleList.Items {
			role := &roleList.Items[i]
			data.Roles = append(data.Roles, types.ConvertRole(role))
		}
	}

	return nil
}

// collectClusterRoles collects all ClusterRoles
func (c *Collector) collectClusterRoles(ctx context.Context, data *types.CollectedData) error {
	var listOptions metav1.ListOptions

	crList, err := c.k8s.Clientset.RbacV1().ClusterRoles().List(ctx, listOptions)
	if err != nil {
		return err
	}

	for i := range crList.Items {
		cr := &crList.Items[i]
		data.ClusterRoles = append(data.ClusterRoles, types.ConvertClusterRole(cr))
	}

	return nil
}

// collectPods collects Pods to see which ServiceAccounts are in use
func (c *Collector) collectPods(ctx context.Context, data *types.CollectedData) error {
	var listOptions metav1.ListOptions

	if c.config.WatchNamespace != "" {
		podList, err := c.k8s.Clientset.CoreV1().Pods(c.config.WatchNamespace).List(ctx, listOptions)
		if err != nil {
			return err
		}
		for i := range podList.Items {
			pod := &podList.Items[i]
			data.Pods = append(data.Pods, types.ConvertPod(pod))
		}
	} else {
		podList, err := c.k8s.Clientset.CoreV1().Pods("").List(ctx, listOptions)
		if err != nil {
			return err
		}
		for i := range podList.Items {
			pod := &podList.Items[i]
			data.Pods = append(data.Pods, types.ConvertPod(pod))
		}
	}

	return nil
}
