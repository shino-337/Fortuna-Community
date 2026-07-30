package capability

import (
	"context"
	"log"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/graph"
	"github.com/fortuna/core/pkg/risk"
)

// RebuildAttackPathsForPod recomputes and persists relational attack paths for a pod.
// This keeps Layer 3 (Path Analysis) fresh for downstream Unified Scorer V3.
func RebuildAttackPathsForPod(ctx context.Context, db *gorm.DB, podUID string) error {
	if podUID == "" {
		return nil
	}
	builder := graph.NewRelationalPathBuilder(db)
	_, err := builder.BuildPathsForPod(ctx, podUID, true)
	return err
}

// ScheduleAttackPathRebuild runs RebuildAttackPathsForPod asynchronously and
// refreshes Unified Score V3 so Layer 3 changes are reflected in persisted risk.
func ScheduleAttackPathRebuild(db *gorm.DB, podUID string) {
	if podUID == "" {
		return
	}
	go func() {
		if err := RebuildAttackPathsForPod(context.Background(), db, podUID); err != nil {
			log.Printf("[CapabilityOrchestrator] failed to rebuild attack paths for pod %s: %v", podUID, err)
			return
		}
		risk.NewUnifiedScorerV3(db).ScheduleUnifiedScoreCalculation(podUID)
		log.Printf("[CapabilityOrchestrator] attack paths rebuilt for pod %s", podUID)
	}()
}
