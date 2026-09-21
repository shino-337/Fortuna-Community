package reconciler

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// SBOMReconciler periodically reconciles SBOM state with actual pod state
type SBOMReconciler struct {
	db                *gorm.DB
	reconcileInterval time.Duration
	logger            *log.Logger
	stopChan          chan struct{}
}

// NewSBOMReconciler creates a new SBOM reconciler
func NewSBOMReconciler(db *gorm.DB, reconcileInterval time.Duration) *SBOMReconciler {
	if reconcileInterval == 0 {
		reconcileInterval = 1 * time.Hour // Default: reconcile every hour
	}

	return &SBOMReconciler{
		db:                db,
		reconcileInterval: reconcileInterval,
		logger:            log.New(log.Writer(), "[SBOMReconciler] ", log.LstdFlags),
		stopChan:          make(chan struct{}),
	}
}

// Start starts the reconciliation loop
func (r *SBOMReconciler) Start(ctx context.Context) {
	r.logger.Printf("Starting SBOM reconciliation loop (interval: %v)", r.reconcileInterval)

	ticker := time.NewTicker(r.reconcileInterval)
	defer ticker.Stop()

	// Run initial reconciliation immediately
	if err := r.Reconcile(ctx); err != nil {
		r.logger.Printf("⚠️  Initial reconciliation failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			r.logger.Printf("Reconciliation loop stopped")
			return
		case <-r.stopChan:
			r.logger.Printf("Reconciliation loop stopped via stop channel")
			return
		case <-ticker.C:
			if err := r.Reconcile(ctx); err != nil {
				r.logger.Printf("⚠️  Reconciliation failed: %v", err)
			}
		}
	}
}

// Stop stops the reconciliation loop
func (r *SBOMReconciler) Stop() {
	close(r.stopChan)
}

// Reconcile performs the actual reconciliation logic
func (r *SBOMReconciler) Reconcile(ctx context.Context) error {
	r.logger.Printf("Starting reconciliation cycle...")
	start := time.Now()

	stats := &ReconciliationStats{}

	// 1. Identify and soft-delete orphaned SBOMs (SBOMs for pods that no longer exist)
	if err := r.cleanupOrphanedSBOMs(ctx, stats); err != nil {
		return fmt.Errorf("cleanup orphaned SBOMs: %w", err)
	}

	// 2. Identify missing SBOMs (pods without SBOMs)
	// NOTE: This is informational only - agents are responsible for creating SBOMs
	// We can't create SBOMs from Core because we don't have access to the container filesystem
	if err := r.identifyMissingSBOMs(ctx, stats); err != nil {
		r.logger.Printf("⚠️  Failed to identify missing SBOMs: %v", err)
		// Don't fail the reconciliation, just log
	}

	// 3. Update last_used_at timestamps for active SBOMs
	if err := r.updateActiveSBOMTimestamps(ctx, stats); err != nil {
		r.logger.Printf("⚠️  Failed to update SBOM timestamps: %v", err)
		// Don't fail the reconciliation, just log
	}

	duration := time.Since(start)
	r.logger.Printf("✅ Reconciliation completed in %v - Stats: %s", duration, stats.String())

	return nil
}

// orphanGracePeriod: do not delete SBOM when pod is missing from DB if SBOM is newer than this
// (avoids race where SBOM arrives before pod sync)
const defaultOrphanGracePeriod = 30 * time.Minute

func orphanGracePeriod() time.Duration {
	raw := strings.TrimSpace(os.Getenv("FORTUNA_SBOM_ORPHAN_GRACE_PERIOD"))
	if raw == "" {
		return defaultOrphanGracePeriod
	}
	if d, err := time.ParseDuration(raw); err == nil && d > 0 {
		return d
	}
	if mins, err := strconv.Atoi(raw); err == nil && mins > 0 {
		return time.Duration(mins) * time.Minute
	}
	return defaultOrphanGracePeriod
}

func podIdentityKey(clusterID, podUID string) string {
	return strings.TrimSpace(clusterID) + "\x00" + strings.TrimSpace(podUID)
}

// cleanupOrphanedSBOMs soft-deletes SBOMs for pods that have been deleted (or missing for too long)
// Only treats SBOM as orphaned when: (1) pod exists in DB and is soft-deleted, or
// (2) pod is not in DB at all AND SBOM is older than orphanGracePeriod (avoids deleting SBOM when pod sync simply hasn't arrived yet)
func (r *SBOMReconciler) cleanupOrphanedSBOMs(ctx context.Context, stats *ReconciliationStats) error {
	var activeSBOMs []models.SBOM
	if err := r.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Select("id, cluster_id, pod_uid, pod_name, namespace, image_name, image_tag, created_at").
		Find(&activeSBOMs).Error; err != nil {
		return fmt.Errorf("query active SBOMs: %w", err)
	}
	stats.TotalSBOMs = len(activeSBOMs)
	if len(activeSBOMs) == 0 {
		return nil
	}

	var tableExists bool
	if err := r.db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema='public' AND table_name='pods')").Scan(&tableExists).Error; err != nil {
		return fmt.Errorf("check pods table: %w", err)
	}
	if !tableExists {
		r.logger.Printf("⚠️  Pods table does not exist, skipping orphan SBOM cleanup")
		return nil
	}

	var activePods []models.Pod
	if err := r.db.WithContext(ctx).
		Select("cluster_id, uid").
		Where("deleted_at IS NULL").
		Find(&activePods).Error; err != nil {
		return fmt.Errorf("query active pods: %w", err)
	}
	activeKeys := make(map[string]struct{}, len(activePods))
	for _, pod := range activePods {
		activeKeys[podIdentityKey(pod.ClusterID, pod.UID)] = struct{}{}
	}

	var deletedPods []models.Pod
	if err := r.db.WithContext(ctx).Unscoped().
		Select("cluster_id, uid").
		Where("deleted_at IS NOT NULL").
		Find(&deletedPods).Error; err != nil {
		return fmt.Errorf("query deleted pods: %w", err)
	}
	deletedKeys := make(map[string]struct{}, len(deletedPods))
	for _, pod := range deletedPods {
		deletedKeys[podIdentityKey(pod.ClusterID, pod.UID)] = struct{}{}
	}

	now := time.Now()
	orphanedSBOMIDs := make([]uint, 0)
	reasonByID := make(map[uint]string)
	for _, sbom := range activeSBOMs {
		if strings.TrimSpace(sbom.ClusterID) == "" || strings.TrimSpace(sbom.PodUID) == "" {
			if now.Sub(sbom.CreatedAt) > orphanGracePeriod() {
				orphanedSBOMIDs = append(orphanedSBOMIDs, sbom.ID)
				reasonByID[sbom.ID] = "unowned_legacy_sbom"
			}
			continue
		}
		key := podIdentityKey(sbom.ClusterID, sbom.PodUID)
		if _, ok := activeKeys[key]; ok {
			continue
		}
		if _, ok := deletedKeys[key]; ok {
			orphanedSBOMIDs = append(orphanedSBOMIDs, sbom.ID)
			reasonByID[sbom.ID] = "pod_deleted"
			continue
		}
		if now.Sub(sbom.CreatedAt) <= orphanGracePeriod() {
			continue
		}
		if keep, reason, err := r.hasRuntimeSecurityEvidence(ctx, sbom.ClusterID, sbom.PodUID); err == nil && keep {
			r.logger.Printf("⏭️  Keep SBOM id=%d cluster=%s pod_uid=%s due to %s evidence", sbom.ID, sbom.ClusterID, sbom.PodUID, reason)
			continue
		}
		orphanedSBOMIDs = append(orphanedSBOMIDs, sbom.ID)
		reasonByID[sbom.ID] = "pod_missing_grace_expired"
	}

	stats.OrphanedSBOMs = len(orphanedSBOMIDs)
	if len(orphanedSBOMIDs) == 0 {
		r.logger.Printf("No orphaned SBOMs found")
		return nil
	}

	auditSBOMs := make([]models.SBOM, 0, len(orphanedSBOMIDs))
	if err := r.db.WithContext(ctx).
		Where("id IN ?", orphanedSBOMIDs).
		Select("id, cluster_id, pod_uid, pod_name, namespace, image_name, image_tag").
		Find(&auditSBOMs).Error; err != nil {
		r.logger.Printf("⚠️  Failed to pre-load SBOM audit data: %v (proceeding with deletion)", err)
	}

	result := r.db.WithContext(ctx).Where("id IN ?", orphanedSBOMIDs).Delete(&models.SBOM{})
	if result.Error != nil {
		return fmt.Errorf("soft delete orphaned SBOMs: %w", result.Error)
	}
	r.logger.Printf("✅ Soft-deleted %d orphaned SBOMs", result.RowsAffected)
	stats.DeletedSBOMs = int(result.RowsAffected)

	for _, s := range auditSBOMs {
		reason := reasonByID[s.ID]
		details := fmt.Sprintf(`{"sbom_id":%d,"cluster_id":%q,"pod_uid":%q,"pod_name":%q,"namespace":%q,"image":"%s:%s","reason":%q}`,
			s.ID, s.ClusterID, s.PodUID, s.PodName, s.Namespace, s.ImageName, s.ImageTag, reason)
		audit := models.AuditLog{
			Action: "delete", Resource: "sbom", ResourceID: fmt.Sprintf("%d", s.ID),
			User: "system/sbom-reconciler", Details: details,
		}
		if err := r.db.WithContext(ctx).Create(&audit).Error; err != nil {
			r.logger.Printf("⚠️  Failed to write audit log for SBOM id=%d: %v", s.ID, err)
		}
	}
	return nil
}

func (r *SBOMReconciler) hasRuntimeSecurityEvidence(ctx context.Context, clusterID, podUID string) (bool, string, error) {
	checks := []struct {
		table string
		name  string
	}{
		{table: "runtime_events", name: "runtime_events"},
		{table: "runtime_incidents", name: "runtime_incidents"},
		{table: "asset_security_state", name: "asset_security_state"},
	}
	for _, check := range checks {
		if !r.db.Migrator().HasTable(check.table) {
			continue
		}
		var count int64
		if err := r.db.WithContext(ctx).Table(check.table).
			Where("cluster_id = ? AND pod_uid = ?", clusterID, podUID).
			Count(&count).Error; err != nil {
			return false, "", fmt.Errorf("query %s for cluster=%s pod_uid=%s: %w", check.table, clusterID, podUID, err)
		}
		if count > 0 {
			return true, check.name, nil
		}
	}
	return false, "", nil
}

// identifyMissingSBOMs identifies pods that don't have SBOMs (informational only)
func (r *SBOMReconciler) identifyMissingSBOMs(ctx context.Context, stats *ReconciliationStats) error {
	var tableExists bool
	if err := r.db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema='public' AND table_name='pods')").Scan(&tableExists).Error; err != nil {
		return fmt.Errorf("check pods table: %w", err)
	}
	if !tableExists {
		r.logger.Printf("⚠️  Pods table does not exist, skipping missing SBOM identification")
		return nil
	}

	var runningPods []models.Pod
	if err := r.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Select("cluster_id, uid, name, namespace").
		Find(&runningPods).Error; err != nil {
		return fmt.Errorf("query running pods: %w", err)
	}
	stats.TotalPods = len(runningPods)
	if len(runningPods) == 0 {
		return nil
	}

	var existingSBOMs []models.SBOM
	if err := r.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Select("cluster_id, pod_uid").
		Find(&existingSBOMs).Error; err != nil {
		return fmt.Errorf("query existing SBOMs: %w", err)
	}
	podsWithSBOMs := make(map[string]struct{}, len(existingSBOMs))
	for _, sbom := range existingSBOMs {
		podsWithSBOMs[podIdentityKey(sbom.ClusterID, sbom.PodUID)] = struct{}{}
	}

	podsWithoutSBOMs := make([]string, 0)
	for _, pod := range runningPods {
		if _, ok := podsWithSBOMs[podIdentityKey(pod.ClusterID, pod.UID)]; !ok {
			podsWithoutSBOMs = append(podsWithoutSBOMs, fmt.Sprintf("%s:%s/%s", pod.ClusterID, pod.Namespace, pod.Name))
		}
	}
	stats.MissingSBOMs = len(podsWithoutSBOMs)
	if len(podsWithoutSBOMs) > 0 {
		r.logger.Printf("⚠️  Found %d pods without SBOMs (agents should create these):", len(podsWithoutSBOMs))
		for i, podName := range podsWithoutSBOMs {
			if i >= 10 {
				r.logger.Printf("   ... and %d more", len(podsWithoutSBOMs)-10)
				break
			}
			r.logger.Printf("   - %s", podName)
		}
	}
	return nil
}

// updateActiveSBOMTimestamps updates last_used_at for SBOMs of running pods
func (r *SBOMReconciler) updateActiveSBOMTimestamps(ctx context.Context, stats *ReconciliationStats) error {
	var tableExists bool
	if err := r.db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema='public' AND table_name='pods')").Scan(&tableExists).Error; err != nil {
		return fmt.Errorf("check pods table: %w", err)
	}
	if !tableExists {
		r.logger.Printf("⚠️  Pods table does not exist, skipping SBOM timestamp update")
		return nil
	}

	result := r.db.WithContext(ctx).
		Model(&models.SBOM{}).
		Where(`deleted_at IS NULL AND EXISTS (
			SELECT 1 FROM pods p
			WHERE p.cluster_id = sboms.cluster_id
			  AND p.uid = sboms.pod_uid
			  AND p.deleted_at IS NULL
		)`).
		Update("last_used_at", time.Now())
	if result.Error != nil {
		return fmt.Errorf("update SBOM timestamps: %w", result.Error)
	}
	stats.UpdatedTimestamps = int(result.RowsAffected)
	r.logger.Printf("✅ Updated timestamps for %d active SBOMs", result.RowsAffected)
	return nil
}

// ReconciliationStats tracks reconciliation statistics
type ReconciliationStats struct {
	TotalSBOMs        int
	TotalPods         int
	OrphanedSBOMs     int
	DeletedSBOMs      int
	MissingSBOMs      int
	UpdatedTimestamps int
}

// String returns a string representation of the stats
func (s *ReconciliationStats) String() string {
	return fmt.Sprintf("SBOMs=%d, Pods=%d, Orphaned=%d, Deleted=%d, Missing=%d, Updated=%d",
		s.TotalSBOMs, s.TotalPods, s.OrphanedSBOMs, s.DeletedSBOMs, s.MissingSBOMs, s.UpdatedTimestamps)
}
