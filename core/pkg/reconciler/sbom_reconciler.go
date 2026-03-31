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

// cleanupOrphanedSBOMs soft-deletes SBOMs for pods that have been deleted (or missing for too long)
// Only treats SBOM as orphaned when: (1) pod exists in DB and is soft-deleted, or
// (2) pod is not in DB at all AND SBOM is older than orphanGracePeriod (avoids deleting SBOM when pod sync simply hasn't arrived yet)
func (r *SBOMReconciler) cleanupOrphanedSBOMs(ctx context.Context, stats *ReconciliationStats) error {
	// Find all active SBOMs (non-deleted), need created_at for grace period
	var activeSBOMs []models.SBOM
	if err := r.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Select("id, pod_uid, pod_name, namespace, created_at").
		Find(&activeSBOMs).Error; err != nil {
		return fmt.Errorf("query active SBOMs: %w", err)
	}

	stats.TotalSBOMs = len(activeSBOMs)

	if len(activeSBOMs) == 0 {
		return nil
	}

	// Collect pod_uid -> sbom (id + created_at)
	type sbomInfo struct {
		id        uint
		createdAt time.Time
	}
	sbomByPodUID := make(map[string]sbomInfo)
	for _, sbom := range activeSBOMs {
		if sbom.PodUID != "" {
			sbomByPodUID[sbom.PodUID] = sbomInfo{id: sbom.ID, createdAt: sbom.CreatedAt}
		}
	}
	podUIDs := make([]string, 0, len(sbomByPodUID))
	for uid := range sbomByPodUID {
		podUIDs = append(podUIDs, uid)
	}

	// Check if pods table exists first
	var tableExists bool
	if err := r.db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema='public' AND table_name='pods')").Scan(&tableExists).Error; err != nil {
		return fmt.Errorf("check pods table: %w", err)
	}

	var orphanedSBOMIDs []uint
	if !tableExists {
		r.logger.Printf("⚠️  Pods table does not exist, skipping orphan SBOM cleanup")
		stats.OrphanedSBOMs = 0
		return nil
	}

	// Active pod UIDs
	var existingPods []models.Pod
	if err := r.db.WithContext(ctx).
		Where("uid IN ? AND deleted_at IS NULL", podUIDs).
		Select("uid").
		Find(&existingPods).Error; err != nil {
		return fmt.Errorf("query existing pods: %w", err)
	}
	existingPodUIDs := make(map[string]bool)
	for _, pod := range existingPods {
		existingPodUIDs[pod.UID] = true
	}

	// Pods that exist but are soft-deleted (explicitly deleted → SBOM can be removed)
	var deletedPods []struct{ UID string }
	if err := r.db.WithContext(ctx).Unscoped().
		Where("uid IN ? AND deleted_at IS NOT NULL", podUIDs).
		Model(&models.Pod{}).
		Select("uid").
		Find(&deletedPods).Error; err != nil {
		return fmt.Errorf("query deleted pods: %w", err)
	}
	deletedPodUIDs := make(map[string]bool)
	for _, p := range deletedPods {
		deletedPodUIDs[p.UID] = true
	}

	now := time.Now()
	orphanedSBOMIDs = make([]uint, 0)
	for podUID, info := range sbomByPodUID {
		if existingPodUIDs[podUID] {
			continue // pod is active, keep SBOM
		}
		if deletedPodUIDs[podUID] {
			// Pod was explicitly soft-deleted → SBOM is orphaned
			orphanedSBOMIDs = append(orphanedSBOMIDs, info.id)
			continue
		}
		// Pod not in DB at all: avoid race with pod sync — only treat as orphan if SBOM is old enough
		if now.Sub(info.createdAt) > orphanGracePeriod() {
			// Keep SBOM if runtime security evidence still references this pod UID.
			if keep, reason, err := r.hasRuntimeSecurityEvidence(ctx, podUID); err == nil && keep {
				r.logger.Printf("⏭️  Keep SBOM id=%d pod_uid=%s due to %s evidence", info.id, podUID, reason)
				continue
			}
			orphanedSBOMIDs = append(orphanedSBOMIDs, info.id)
		}
	}

	stats.OrphanedSBOMs = len(orphanedSBOMIDs)

	if len(orphanedSBOMIDs) == 0 {
		r.logger.Printf("No orphaned SBOMs found")
		return nil
	}

	// Soft delete orphaned SBOMs
	result := r.db.WithContext(ctx).
		Where("id IN ?", orphanedSBOMIDs).
		Delete(&models.SBOM{})

	if result.Error != nil {
		return fmt.Errorf("soft delete orphaned SBOMs: %w", result.Error)
	}

	r.logger.Printf("✅ Soft-deleted %d orphaned SBOMs", result.RowsAffected)
	stats.DeletedSBOMs = int(result.RowsAffected)

	return nil
}

func (r *SBOMReconciler) hasRuntimeSecurityEvidence(ctx context.Context, podUID string) (bool, string, error) {
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
		if err := r.db.WithContext(ctx).Table(check.table).Where("resource_uid = ?", podUID).Count(&count).Error; err != nil {
			return false, "", fmt.Errorf("query %s for pod_uid=%s: %w", check.table, podUID, err)
		}
		if count > 0 {
			return true, check.name, nil
		}
	}
	return false, "", nil
}

// identifyMissingSBOMs identifies pods that don't have SBOMs (informational only)
func (r *SBOMReconciler) identifyMissingSBOMs(ctx context.Context, stats *ReconciliationStats) error {
	// Check if pods table exists first
	var tableExists bool
	if err := r.db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema='public' AND table_name='pods')").Scan(&tableExists).Error; err != nil {
		return fmt.Errorf("check pods table: %w", err)
	}

	if !tableExists {
		r.logger.Printf("⚠️  Pods table does not exist, skipping missing SBOM identification")
		return nil
	}

	// Find all running pods
	var runningPods []models.Pod
	if err := r.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Select("uid, name, namespace").
		Find(&runningPods).Error; err != nil {
		return fmt.Errorf("query running pods: %w", err)
	}

	stats.TotalPods = len(runningPods)

	if len(runningPods) == 0 {
		return nil
	}

	// Collect all pod UIDs
	podUIDs := make([]string, 0, len(runningPods))
	for _, pod := range runningPods {
		if pod.UID != "" {
			podUIDs = append(podUIDs, pod.UID)
		}
	}

	// Query for SBOMs for these pods
	var existingSBOMs []models.SBOM
	if err := r.db.WithContext(ctx).
		Where("pod_uid IN ? AND deleted_at IS NULL", podUIDs).
		Select("pod_uid").
		Find(&existingSBOMs).Error; err != nil {
		return fmt.Errorf("query existing SBOMs: %w", err)
	}

	// Build set of pod UIDs that have SBOMs
	podsWithSBOMs := make(map[string]bool)
	for _, sbom := range existingSBOMs {
		podsWithSBOMs[sbom.PodUID] = true
	}

	// Identify pods without SBOMs
	podsWithoutSBOMs := make([]string, 0)
	for _, pod := range runningPods {
		if !podsWithSBOMs[pod.UID] {
			podsWithoutSBOMs = append(podsWithoutSBOMs, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
		}
	}

	stats.MissingSBOMs = len(podsWithoutSBOMs)

	if len(podsWithoutSBOMs) > 0 {
		r.logger.Printf("⚠️  Found %d pods without SBOMs (agents should create these):", len(podsWithoutSBOMs))
		// Log first 10 for visibility
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
	// Check if pods table exists first
	var tableExists bool
	if err := r.db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema='public' AND table_name='pods')").Scan(&tableExists).Error; err != nil {
		return fmt.Errorf("check pods table: %w", err)
	}

	if !tableExists {
		r.logger.Printf("⚠️  Pods table does not exist, skipping SBOM timestamp update")
		return nil
	}

	// Find all running pods
	var runningPodUIDs []string
	if err := r.db.WithContext(ctx).
		Model(&models.Pod{}).
		Where("deleted_at IS NULL").
		Pluck("uid", &runningPodUIDs).Error; err != nil {
		return fmt.Errorf("query running pod UIDs: %w", err)
	}

	if len(runningPodUIDs) == 0 {
		return nil
	}

	// Update last_used_at for SBOMs of running pods
	result := r.db.WithContext(ctx).
		Model(&models.SBOM{}).
		Where("pod_uid IN ? AND deleted_at IS NULL", runningPodUIDs).
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
