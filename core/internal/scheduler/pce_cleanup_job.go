package scheduler

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

const defaultPCECleanupRetentionDays = 30

// PCECleanupJob periodically deletes pod_capabilities not seen for longer than retention (Phase 3).
type PCECleanupJob struct {
	db     *gorm.DB
	ctx    context.Context
	cancel context.CancelFunc
}

// NewPCECleanupJob creates a new PCE cleanup job
func NewPCECleanupJob(db *gorm.DB) *PCECleanupJob {
	ctx, cancel := context.WithCancel(context.Background())
	return &PCECleanupJob{
		db:     db,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start starts the job (runs every 24 hours). Must run in goroutine.
func (j *PCECleanupJob) Start() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	log.Printf("[PCECleanupJob] Started - will run every 24 hours (retention=%dd)", getPCECleanupRetentionDays())

	j.run()
	for {
		select {
		case <-j.ctx.Done():
			log.Printf("[PCECleanupJob] Stopped")
			return
		case <-ticker.C:
			j.run()
		}
	}
}

// Stop stops the job
func (j *PCECleanupJob) Stop() {
	j.cancel()
}

func getPCECleanupRetentionDays() int {
	if v := os.Getenv("PCE_CLEANUP_RETENTION_DAYS"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			return d
		}
	}
	return defaultPCECleanupRetentionDays
}

func (j *PCECleanupJob) run() {
	if !j.db.Migrator().HasTable(&models.PodCapability{}) {
		return
	}
	days := getPCECleanupRetentionDays()
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)

	// Delete rows where last_seen_at is older than cutoff; fallback to updated_at if last_seen_at is null
	result := j.db.Exec(`
		DELETE FROM pod_capabilities
		WHERE (last_seen_at IS NOT NULL AND last_seen_at < ?)
		   OR (last_seen_at IS NULL AND updated_at < ?)
	`, cutoff, cutoff)

	if result.Error != nil {
		log.Printf("[PCECleanupJob] Error deleting stale pod_capabilities: %v", result.Error)
		return
	}
	if result.RowsAffected > 0 {
		log.Printf("[PCECleanupJob] Deleted %d stale pod_capabilities (older than %d days)", result.RowsAffected, days)
	}
}
