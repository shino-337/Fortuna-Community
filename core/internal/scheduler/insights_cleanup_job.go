package scheduler

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// InsightsCleanupJob periodically cleans up old or resolved insights
type InsightsCleanupJob struct {
	db     *gorm.DB
	ctx    context.Context
	cancel context.CancelFunc
}

// NewInsightsCleanupJob creates a new insights cleanup job
func NewInsightsCleanupJob(db *gorm.DB) *InsightsCleanupJob {
	ctx, cancel := context.WithCancel(context.Background())
	return &InsightsCleanupJob{
		db:     db,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start starts the cleanup job (runs every 24 hours)
// CRITICAL: Must run in goroutine - Start() has infinite loop that blocks!
func (j *InsightsCleanupJob) Start() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	log.Printf("[InsightsCleanupJob] Started - will run every 24 hours")

	// Run immediately on start
	j.run()

	for {
		select {
		case <-j.ctx.Done():
			log.Printf("[InsightsCleanupJob] Stopped")
			return
		case <-ticker.C:
			j.run()
		}
	}
}

// Stop stops the cleanup job
func (j *InsightsCleanupJob) Stop() {
	j.cancel()
}

// run executes the cleanup
func (j *InsightsCleanupJob) run() {
	log.Printf("[InsightsCleanupJob] Running cleanup...")

	// 1. Soft delete resolved insights older than 30 days
	result := j.db.Model(&models.Insight{}).
		Where("status = ? AND updated_at < ?", "resolved", time.Now().Add(-30*24*time.Hour)).
		Update("deleted_at", time.Now())

	if result.Error != nil {
		log.Printf("[InsightsCleanupJob] Error soft-deleting resolved insights: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("[InsightsCleanupJob] Soft-deleted %d resolved insights (older than 30 days)", result.RowsAffected)
	}

	// 2. Soft delete old active insights that haven't been updated in 90 days
	// (likely no longer relevant)
	result2 := j.db.Model(&models.Insight{}).
		Where("(status = ? OR status IS NULL) AND updated_at < ? AND deleted_at IS NULL", 
			"active", time.Now().Add(-90*24*time.Hour)).
		Update("deleted_at", time.Now())

	if result2.Error != nil {
		log.Printf("[InsightsCleanupJob] Error soft-deleting old active insights: %v", result2.Error)
	} else if result2.RowsAffected > 0 {
		log.Printf("[InsightsCleanupJob] Soft-deleted %d old active insights (not updated in 90 days)", result2.RowsAffected)
	}

	// 3. Clean up duplicate insights (same description, type, severity)
	// Keep only the latest one, soft delete older duplicates
	var duplicates []struct {
		Description string
		Type        string
		Severity    string
		Count       int64
	}
	j.db.Raw(`
		SELECT description, type, severity, COUNT(*) as count
		FROM insights
		WHERE deleted_at IS NULL AND (status = 'active' OR status IS NULL)
		GROUP BY description, type, severity
		HAVING COUNT(*) > 1
	`).Scan(&duplicates)

	if len(duplicates) > 0 {
		log.Printf("[InsightsCleanupJob] Found %d groups of duplicate insights", len(duplicates))
		for _, dup := range duplicates {
			// Keep the latest insight, soft delete older ones
			result := j.db.Exec(`
				UPDATE insights
				SET deleted_at = NOW()
				WHERE deleted_at IS NULL 
				  AND (status = 'active' OR status IS NULL)
				  AND description = ?
				  AND type = ?
				  AND severity = ?
				  AND id NOT IN (
					SELECT id FROM insights
					WHERE deleted_at IS NULL
					  AND (status = 'active' OR status IS NULL)
					  AND description = ?
					  AND type = ?
					  AND severity = ?
					ORDER BY created_at DESC
					LIMIT 1
				  )
			`, dup.Description, dup.Type, dup.Severity,
				dup.Description, dup.Type, dup.Severity)
			if result.Error != nil {
				log.Printf("[InsightsCleanupJob] Error cleaning up duplicates for %s/%s: %v", dup.Type, dup.Severity, result.Error)
			} else if result.RowsAffected > 0 {
				log.Printf("[InsightsCleanupJob] Soft-deleted %d duplicate insights (description: %s)", result.RowsAffected, dup.Description[:50])
			}
		}
	}

	// 4. Count remaining active insights
	var activeCount int64
	j.db.Model(&models.Insight{}).
		Where("deleted_at IS NULL AND (status = ? OR status IS NULL)", "active").
		Count(&activeCount)

	log.Printf("[InsightsCleanupJob] Cleanup completed. Active insights: %d", activeCount)
}

