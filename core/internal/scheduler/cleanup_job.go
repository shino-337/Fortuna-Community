package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// PodCleanupJob periodically cleans up pods that no longer exist in Kubernetes
// Layer 3: Background Cleanup - Eventually consistent
type PodCleanupJob struct {
	db        *gorm.DB
	k8sClient kubernetes.Interface
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewPodCleanupJob creates a new pod cleanup job
func NewPodCleanupJob(db *gorm.DB) *PodCleanupJob {
	// Initialize K8s client for validation
	var k8sClient kubernetes.Interface
	config, err := rest.InClusterConfig()
	if err == nil {
		client, err := kubernetes.NewForConfig(config)
		if err == nil {
			k8sClient = client
			log.Printf("[PodCleanupJob] K8s client initialized")
		} else {
			log.Printf("[PodCleanupJob] Warning: Failed to create K8s client: %v", err)
		}
	} else {
		log.Printf("[PodCleanupJob] Warning: Not running in cluster: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &PodCleanupJob{
		db:        db,
		k8sClient: k8sClient,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start starts the cleanup job (runs every 5 minutes)
func (j *PodCleanupJob) Start() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	log.Printf("[PodCleanupJob] Started - will run every 5 minutes (Layer 3: Background Cleanup)")

	// Run immediately on start
	j.run()

	for {
		select {
		case <-j.ctx.Done():
			log.Printf("[PodCleanupJob] Stopped")
			return
		case <-ticker.C:
			j.run()
		}
	}
}

// Stop stops the cleanup job
func (j *PodCleanupJob) Stop() {
	j.cancel()
}

// run executes the cleanup - Layer 3: Compare DB vs K8s and soft delete ghost pods
func (j *PodCleanupJob) run() {
	log.Printf("[PodCleanupJob] Running cleanup...")

	if j.k8sClient == nil {
		// Fallback: Use time-based cleanup if K8s client not available
		log.Printf("[PodCleanupJob] K8s client not available, using time-based cleanup")

		if !j.db.Migrator().HasTable("pods") {
			log.Printf("[PodCleanupJob] Pods table does not exist, skipping cleanup")
			return
		}

		result := j.db.Exec(`
			UPDATE pods 
			SET deleted_at = NOW()
			WHERE deleted_at IS NULL
			  AND created_at < NOW() - INTERVAL '1 hour'
			  AND updated_at < NOW() - INTERVAL '30 minutes'
		`)

		if result.Error != nil {
			log.Printf("[PodCleanupJob] Error cleaning up pods: %v", result.Error)
			return
		}

		if result.RowsAffected > 0 {
			log.Printf("[PodCleanupJob] Soft deleted %d stale pods (time-based)", result.RowsAffected)
		} else {
			log.Printf("[PodCleanupJob] No stale pods to clean up")
		}
		return
	}

	// Layer 3: Get all current K8s pod UIDs
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	allPods, err := j.k8sClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("[PodCleanupJob] Error listing K8s pods: %v", err)
		return
	}

	// Build map of current K8s pod UIDs
	k8sUIDs := make(map[string]bool)
	for _, pod := range allPods.Items {
		k8sUIDs[string(pod.UID)] = true
	}

	log.Printf("[PodCleanupJob] Found %d pods in Kubernetes", len(k8sUIDs))

	if !j.db.Migrator().HasTable("pods") {
		log.Printf("[PodCleanupJob] Pods table does not exist, skipping cleanup")
		return
	}

	// A Core instance can receive inventory from multiple clusters, but its in-cluster
	// Kubernetes client can only verify the local cluster. Scope cleanup to DB cluster_ids
	// that have at least one UID observed in the local API, otherwise remote-cluster pods
	// are incorrectly treated as ghosts.
	var localClusterIDs []string
	if err := j.db.Table("pods").
		Distinct("cluster_id").
		Where("deleted_at IS NULL AND uid IN ?", mapKeys(k8sUIDs)).
		Pluck("cluster_id", &localClusterIDs).Error; err != nil {
		log.Printf("[PodCleanupJob] Error resolving local cluster scope: %v", err)
		return
	}
	if len(localClusterIDs) == 0 {
		log.Printf("[PodCleanupJob] No DB cluster matched local Kubernetes pods; skipping ghost cleanup to protect remote clusters")
		return
	}

	// Get active pods from database for the local cluster scope only.
	var dbPods []struct {
		UID       string
		Namespace string
		Name      string
		ClusterID string
	}

	if err := j.db.Model(&struct {
		UID       string
		Namespace string
		Name      string
		ClusterID string
	}{}).Table("pods").
		Select("uid, namespace, name, cluster_id").
		Where("deleted_at IS NULL AND cluster_id IN ?", localClusterIDs).
		Scan(&dbPods).Error; err != nil {
		log.Printf("[PodCleanupJob] Error querying database pods: %v", err)
		return
	}

	log.Printf("[PodCleanupJob] Found %d active pods in database for local cluster scope %v", len(dbPods), localClusterIDs)

	// Find ghost pods (in DB but not in K8s)
	ghostPods := make([]string, 0)
	for _, pod := range dbPods {
		if !k8sUIDs[pod.UID] {
			ghostPods = append(ghostPods, pod.UID)
		}
	}

	if len(ghostPods) == 0 {
		log.Printf("[PodCleanupJob] No ghost pods found - database is in sync")
		return
	}

	// Soft delete ghost pods in the verified local cluster scope only.
	result := j.db.Model(&models.Pod{}).
		Where("deleted_at IS NULL AND cluster_id IN ? AND uid IN ?", localClusterIDs, ghostPods).
		Update("deleted_at", time.Now())

	if result.Error != nil {
		log.Printf("[PodCleanupJob] Error soft deleting ghost pods: %v", result.Error)
		return
	}

	log.Printf("[PodCleanupJob] Soft deleted %d ghost pods (Layer 3: Background Cleanup)", result.RowsAffected)
}

func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
