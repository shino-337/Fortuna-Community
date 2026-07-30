package risk

import (
	"context"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// DistinctResourceUIDsWithActiveInsights returns distinct non-empty resource_uid
// values from insights in active or acknowledged state.
func DistinctResourceUIDsWithActiveInsights(db *gorm.DB) ([]string, error) {
	var uids []string
	err := db.Model(&models.Insight{}).
		Where("status IN ? AND deleted_at IS NULL", []string{"active", "acknowledged"}).
		Distinct("resource_uid").
		Pluck("resource_uid", &uids).Error
	if err != nil {
		return nil, err
	}
	return filterNonEmptyUIDs(uids), nil
}

// DistinctPodUIDs returns distinct non-empty pod UIDs from non-deleted pods.
func DistinctPodUIDs(db *gorm.DB) ([]string, error) {
	var uids []string
	err := db.Model(&models.Pod{}).
		Where("deleted_at IS NULL").
		Distinct("uid").
		Pluck("uid", &uids).Error
	if err != nil {
		return nil, err
	}
	return filterNonEmptyUIDs(uids), nil
}

// UIDsForV3ClusterBackfill merges insight-bearing resources with all known pod UIDs
// so periodic jobs can refresh V3 scores across the cluster, not only where insights exist.
func UIDsForV3ClusterBackfill(db *gorm.DB) ([]string, error) {
	a, err := DistinctResourceUIDsWithActiveInsights(db)
	if err != nil {
		return nil, err
	}
	b, err := DistinctPodUIDs(db)
	if err != nil {
		return nil, err
	}
	return mergeDedupeUIDs(a, b), nil
}

// BackfillV3RiskScores runs CalculateScoreV3 + SaveScoreV3 for each UID.
// itemDelay is applied between resources when > 0 (throttle large clusters).
func BackfillV3RiskScores(ctx context.Context, db *gorm.DB, uids []string, itemDelay time.Duration) (ok, fail int) {
	if len(uids) == 0 {
		return 0, 0
	}
	scorerV3 := NewUnifiedScorerV3(db)
	for i, uid := range uids {
		select {
		case <-ctx.Done():
			return ok, fail
		default:
		}
		scoreV3, err := scorerV3.CalculateScoreV3(ctx, uid)
		if err != nil {
			log.Printf("[BackfillV3RiskScores] calculate failed resource_uid=%s: %v", uid, err)
			fail++
		} else if err := scorerV3.SaveScoreV3(ctx, scoreV3); err != nil {
			log.Printf("[BackfillV3RiskScores] save failed resource_uid=%s: %v", uid, err)
			fail++
		} else {
			ok++
		}
		if itemDelay > 0 && i < len(uids)-1 {
			select {
			case <-ctx.Done():
				return ok, fail
			case <-time.After(itemDelay):
			}
		}
	}
	return ok, fail
}

// BackfillV3RiskScoresFromActiveInsights is used by POST /risk/scores/sync (insights only).
func BackfillV3RiskScoresFromActiveInsights(ctx context.Context, db *gorm.DB) (ok, fail, total int, err error) {
	uids, err := DistinctResourceUIDsWithActiveInsights(db)
	if err != nil {
		return 0, 0, 0, err
	}
	total = len(uids)
	ok, fail = BackfillV3RiskScores(ctx, db, uids, 0)
	return ok, fail, total, nil
}

func filterNonEmptyUIDs(uids []string) []string {
	out := make([]string, 0, len(uids))
	for _, u := range uids {
		if strings.TrimSpace(u) != "" {
			out = append(out, u)
		}
	}
	return out
}

func mergeDedupeUIDs(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	for _, u := range a {
		seen[u] = struct{}{}
	}
	for _, u := range b {
		seen[u] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for u := range seen {
		out = append(out, u)
	}
	return out
}
