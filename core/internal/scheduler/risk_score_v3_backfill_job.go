package scheduler

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/risk"
)

const defaultRiskScoreV3BackfillInterval = 6 * time.Hour

// RiskScoreV3BackfillIntervalFromEnv reads RISK_SCORE_V3_BACKFILL_INTERVAL (Go duration, e.g. "6h", "24h").
func RiskScoreV3BackfillIntervalFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("RISK_SCORE_V3_BACKFILL_INTERVAL"))
	if raw == "" {
		return defaultRiskScoreV3BackfillInterval
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		log.Printf("[RiskScoreV3BackfillJob] Invalid RISK_SCORE_V3_BACKFILL_INTERVAL=%q, using %s", raw, defaultRiskScoreV3BackfillInterval)
		return defaultRiskScoreV3BackfillInterval
	}
	return d
}

// RiskScoreV3BackfillEnabled returns false when RISK_SCORE_V3_BACKFILL_ENABLED is 0/false/no (default: enabled).
func RiskScoreV3BackfillEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("RISK_SCORE_V3_BACKFILL_ENABLED")))
	switch v {
	case "", "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// RiskScoreV3BackfillItemDelay returns pause between each resource scoring (default 0).
func RiskScoreV3BackfillItemDelay() time.Duration {
	raw := strings.TrimSpace(os.Getenv("RISK_SCORE_V3_BACKFILL_ITEM_DELAY"))
	if raw == "" {
		return 0
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		log.Printf("[RiskScoreV3BackfillJob] Invalid RISK_SCORE_V3_BACKFILL_ITEM_DELAY=%q, using 0", raw)
		return 0
	}
	return d
}

// RiskScoreV3BackfillJob periodically recomputes and persists V3 risk_scores for
// all pods plus any resource with active/acknowledged insights (cluster-wide sweep).
type RiskScoreV3BackfillJob struct {
	db         *gorm.DB
	interval   time.Duration
	itemDelay  time.Duration
	firstDelay time.Duration
	stop       chan struct{}
	stopOnce   sync.Once
}

func NewRiskScoreV3BackfillJob(db *gorm.DB, interval, itemDelay, firstDelay time.Duration) *RiskScoreV3BackfillJob {
	if interval <= 0 {
		interval = defaultRiskScoreV3BackfillInterval
	}
	if firstDelay < 0 {
		firstDelay = 0
	}
	return &RiskScoreV3BackfillJob{
		db:         db,
		interval:   interval,
		itemDelay:  itemDelay,
		firstDelay: firstDelay,
		stop:       make(chan struct{}),
	}
}

// Start runs until Stop is called. The first run waits firstDelay to reduce startup contention.
func (j *RiskScoreV3BackfillJob) Start() {
	if j.firstDelay > 0 {
		log.Printf("[RiskScoreV3BackfillJob] Deferring first backfill by %s", j.firstDelay)
		select {
		case <-time.After(j.firstDelay):
		case <-j.stop:
			return
		}
	}
	j.run()

	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			j.run()
		case <-j.stop:
			return
		}
	}
}

func (j *RiskScoreV3BackfillJob) Stop() {
	j.stopOnce.Do(func() { close(j.stop) })
}

func (j *RiskScoreV3BackfillJob) run() {
	if j.db == nil {
		return
	}
	ctx := context.Background()

	uids, err := risk.UIDsForV3ClusterBackfill(j.db)
	if err != nil {
		log.Printf("[RiskScoreV3BackfillJob] list UIDs failed: %v", err)
		return
	}
	if len(uids) == 0 {
		log.Printf("[RiskScoreV3BackfillJob] no pod/insight UIDs to backfill")
		return
	}
	log.Printf("[RiskScoreV3BackfillJob] starting V3 backfill for %d resources", len(uids))
	ok, fail := risk.BackfillV3RiskScores(ctx, j.db, uids, j.itemDelay)
	log.Printf("[RiskScoreV3BackfillJob] completed ok=%d fail=%d total=%d", ok, fail, len(uids))
}
