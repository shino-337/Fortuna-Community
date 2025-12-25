package scheduler

import (
	"context"
	"log"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"gorm.io/gorm"
)

// PodCleanupJob periodically cleans up pods that no longer exist in Kubernetes
// Layer 3: Background Cleanup - Eventually consistent
type PodCleanupJob struct {
	db       *gorm.DB
	k8sClient kubernetes.Interface
	ctx      context.Context
	cancel   context.CancelFunc
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
		
		// Check if pods table exists
		var podsTableExists bool
		if err := j.db.Raw(`
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'public' 
				AND table_name = 'pods'
			)
		`).Scan(&podsTableExists).Error; err != nil {
			log.Printf("[PodCleanupJob] Error checking pods table existence: %v", err)
			return
		}

		if !podsTableExists {
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

	// Check if pods table exists
	var podsTableExists bool
	if err := j.db.Raw(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'pods'
		)
	`).Scan(&podsTableExists).Error; err != nil {
		log.Printf("[PodCleanupJob] Error checking pods table existence: %v", err)
		return
	}

	if !podsTableExists {
		log.Printf("[PodCleanupJob] Pods table does not exist, skipping cleanup")
		return
	}

	// Get all active pods from database
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
		Where("deleted_at IS NULL").
		Scan(&dbPods).Error; err != nil {
		log.Printf("[PodCleanupJob] Error querying database pods: %v", err)
		return
	}

	log.Printf("[PodCleanupJob] Found %d active pods in database", len(dbPods))

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

	// Soft delete ghost pods
	result := j.db.Exec(`
		UPDATE pods 
		SET deleted_at = NOW()
		WHERE deleted_at IS NULL
		  AND uid IN ?
	`, ghostPods)

	if result.Error != nil {
		log.Printf("[PodCleanupJob] Error soft deleting ghost pods: %v", result.Error)
		return
	}

	log.Printf("[PodCleanupJob] Soft deleted %d ghost pods (Layer 3: Background Cleanup)", result.RowsAffected)
}

