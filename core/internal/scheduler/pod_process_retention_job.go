package scheduler

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// PodProcessRetentionJob deletes pod_processes rows older than retention days (e.g. 30).
type PodProcessRetentionJob struct {
	db             *gorm.DB
	retentionDays  int
	interval       time.Duration
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewPodProcessRetentionJob creates a retention job. retentionDays must be > 0 (default 30).
func NewPodProcessRetentionJob(db *gorm.DB, retentionDays int, interval time.Duration) *PodProcessRetentionJob {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &PodProcessRetentionJob{
		db:            db,
		retentionDays: retentionDays,
		interval:      interval,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start runs the job periodically. Call in a goroutine (blocks).
func (j *PodProcessRetentionJob) Start() {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	log.Printf("[PodProcessRetentionJob] Started - retention=%d days, interval=%s", j.retentionDays, j.interval)
	j.run()
	for {
		select {
		case <-j.ctx.Done():
			log.Printf("[PodProcessRetentionJob] Stopped")
			return
		case <-ticker.C:
			j.run()
		}
	}
}

// Stop stops the job.
func (j *PodProcessRetentionJob) Stop() {
	j.cancel()
}

func (j *PodProcessRetentionJob) run() {
	cutoff := time.Now().UTC().Add(-time.Duration(j.retentionDays) * 24 * time.Hour)
	result := j.db.Where("observed_at < ?", cutoff).Delete(&models.PodProcess{})
	if result.Error != nil {
		log.Printf("[PodProcessRetentionJob] Error deleting old process data: %v", result.Error)
		return
	}
	if result.RowsAffected > 0 {
		log.Printf("[PodProcessRetentionJob] Deleted %d process rows (observed_at < %s)", result.RowsAffected, cutoff.Format(time.RFC3339))
	}
}
