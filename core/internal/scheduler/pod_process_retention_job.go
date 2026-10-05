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

// PodProcessRetentionJob deletes pod_processes snapshots older than the retention window.
// Every Pod Detail report appends a snapshot, so without this job the table grows for
// as long as a Pod runs. Only the latest snapshot is served, and the previous one is
// read to derive process-diff events.
type PodProcessRetentionJob struct {
	db             *gorm.DB
	retentionHours int
	interval       time.Duration
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewPodProcessRetentionJob creates a retention job tuned via POD_PROCESS_RETENTION_HOURS
// (default 24) and POD_PROCESS_CLEANUP_INTERVAL (default 1h).
func NewPodProcessRetentionJob(db *gorm.DB) *PodProcessRetentionJob {
	retentionHours := 24
	if v := os.Getenv("POD_PROCESS_RETENTION_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			retentionHours = n
		}
	}
	interval := time.Hour
	if v := os.Getenv("POD_PROCESS_CLEANUP_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			interval = d
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &PodProcessRetentionJob{
		db:             db,
		retentionHours: retentionHours,
		interval:       interval,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start runs the job periodically. Call in a goroutine (blocks).
func (j *PodProcessRetentionJob) Start() {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	log.Printf("[PodProcessRetentionJob] Started - retention=%dh, interval=%s", j.retentionHours, j.interval)
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
	cutoff := time.Now().UTC().Add(-time.Duration(j.retentionHours) * time.Hour)
	result := j.db.WithContext(j.ctx).Where("observed_at < ?", cutoff).Delete(&models.PodProcess{})
	if result.Error != nil {
		log.Printf("[PodProcessRetentionJob] Error deleting old process data: %v", result.Error)
		return
	}
	if result.RowsAffected > 0 {
		log.Printf("[PodProcessRetentionJob] Deleted %d process rows (observed_at < %s)", result.RowsAffected, cutoff.Format(time.RFC3339))
	}
}
