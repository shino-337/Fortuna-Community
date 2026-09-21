package capability

import (
	"context"
	"log"

	"github.com/fortuna/core/pkg/graph"
	"github.com/fortuna/core/pkg/resourceidentity"
	"gorm.io/gorm"
)

// RebuildAttackPathsForIdentity recomputes attack paths without collapsing Pod
// identity to a UID. Risk-score refresh is deliberately handled by the scoped
// scorer cutover so this function never calls the legacy UID-only scorer.
func RebuildAttackPathsForIdentity(ctx context.Context, db *gorm.DB, id resourceidentity.Identity) error {
	if err := id.Validate(); err != nil {
		return err
	}
	builder := graph.NewRelationalPathBuilder(db)
	_, err := builder.BuildPathsForPodIdentity(ctx, id, true)
	return err
}

func ScheduleAttackPathRebuildForIdentity(db *gorm.DB, id resourceidentity.Identity) {
	if db == nil || id.Validate() != nil {
		return
	}
	go func() {
		if err := RebuildAttackPathsForIdentity(context.Background(), db, id); err != nil {
			log.Printf("[CapabilityOrchestrator] failed scoped attack-path rebuild for %s/%s: %v", id.ClusterID, id.ResourceUID, err)
			return
		}
		log.Printf("[CapabilityOrchestrator] scoped attack paths rebuilt for %s/%s", id.ClusterID, id.ResourceUID)
	}()
}
