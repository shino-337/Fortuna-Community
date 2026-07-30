package scheduler

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/graph"
)

const defaultAttackPathReconcileInterval = 30 * time.Minute

// AttackPathReconcileIntervalFromEnv reads ATTACK_PATH_RECONCILE_INTERVAL.
// Accepted values use Go duration format (e.g. "15m", "30m", "1h").
func AttackPathReconcileIntervalFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("ATTACK_PATH_RECONCILE_INTERVAL"))
	if raw == "" {
		return defaultAttackPathReconcileInterval
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		log.Printf("[AttackPathReconcileJob] Invalid ATTACK_PATH_RECONCILE_INTERVAL=%q, fallback to %s", raw, defaultAttackPathReconcileInterval)
		return defaultAttackPathReconcileInterval
	}
	return d
}

// AttackPathReconcileJob periodically recomputes relational attack paths for all pods.
// Purpose: Layer 3 reconciliation to heal drift/stale path state.
type AttackPathReconcileJob struct {
	db       *gorm.DB
	interval time.Duration
	stop     chan struct{}
}

func NewAttackPathReconcileJob(db *gorm.DB, interval time.Duration) *AttackPathReconcileJob {
	if interval <= 0 {
		interval = defaultAttackPathReconcileInterval
	}
	return &AttackPathReconcileJob{
		db:       db,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

func (j *AttackPathReconcileJob) Start() {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	// Delay first run to avoid startup contention.
	startDelay := 2 * time.Minute
	log.Printf("[AttackPathReconcileJob] Deferring first reconcile by %s", startDelay)
	select {
	case <-time.After(startDelay):
	case <-j.stop:
		return
	}

	j.run()
	for {
		select {
		case <-ticker.C:
			j.run()
		case <-j.stop:
			return
		}
	}
}

func (j *AttackPathReconcileJob) Stop() {
	close(j.stop)
}

func (j *AttackPathReconcileJob) run() {
	if j.db == nil {
		return
	}
	log.Printf("[AttackPathReconcileJob] Running Layer 3 reconciliation...")
	builder := graph.NewRelationalPathBuilder(j.db)
	paths, err := builder.BuildAllPaths(context.Background(), "", true)
	if err != nil {
		log.Printf("[AttackPathReconcileJob] reconcile failed: %v", err)
		return
	}
	log.Printf("[AttackPathReconcileJob] reconcile completed, computed paths=%d", len(paths))
}

