package scheduler

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// PodNetworkRetentionJob deletes pod_network_connections rows older than retention (by bucket_5m).
type PodNetworkRetentionJob struct {
	db             *gorm.DB
	retentionHours int
	interval       time.Duration
	batchSize      int
	maxRounds      int
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewPodNetworkRetentionJob creates a retention job. Tuned via POD_NETWORK_* env vars.
func NewPodNetworkRetentionJob(db *gorm.DB) *PodNetworkRetentionJob {
	retentionHours := 24
	if v := os.Getenv("POD_NETWORK_RETENTION_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			retentionHours = n
		}
	}
	interval := 10 * time.Minute
	if v := os.Getenv("POD_NETWORK_CLEANUP_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			interval = d
		}
	}
	batchSize := 20000
	if v := os.Getenv("POD_NETWORK_CLEANUP_BATCH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			batchSize = n
		}
	}
	maxRounds := 3
	if v := os.Getenv("POD_NETWORK_CLEANUP_ROUNDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxRounds = n
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &PodNetworkRetentionJob{
		db:             db,
		retentionHours: retentionHours,
		interval:       interval,
		batchSize:      batchSize,
		maxRounds:      maxRounds,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start runs the job periodically. Call in a goroutine (blocks).
func (j *PodNetworkRetentionJob) Start() {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	log.Printf("[PodNetworkRetentionJob] Started retention=%dh interval=%s batch=%d rounds=%d",
		j.retentionHours, j.interval, j.batchSize, j.maxRounds)
	j.run()
	for {
		select {
		case <-j.ctx.Done():
			log.Printf("[PodNetworkRetentionJob] Stopped")
			return
		case <-ticker.C:
			j.run()
		}
	}
}

// Stop stops the job.
func (j *PodNetworkRetentionJob) Stop() {
	j.cancel()
}

func (j *PodNetworkRetentionJob) run() {
	cutoff := time.Now().UTC().Add(-time.Duration(j.retentionHours) * time.Hour)
	var total int64
	for r := 0; r < j.maxRounds; r++ {
		res := j.db.Exec(`
DELETE FROM pod_network_connections
WHERE id IN (
  SELECT id FROM pod_network_connections
  WHERE bucket_5m < ?
  ORDER BY bucket_5m ASC
  LIMIT ?
)`, cutoff, j.batchSize)
		if res.Error != nil {
			log.Printf("[PodNetworkRetentionJob] delete batch error: %v", res.Error)
			return
		}
		total += res.RowsAffected
		if res.RowsAffected == 0 {
			break
		}
	}
	if total > 0 {
		log.Printf("[PodNetworkRetentionJob] Deleted %d rows (bucket_5m < %s)", total, cutoff.Format(time.RFC3339))
	}
}
