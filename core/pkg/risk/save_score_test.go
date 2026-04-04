package risk

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Mirrors production Postgres: UNIQUE(resource_type, resource_uid, cluster_id) without WHERE deleted_at IS NULL.
func TestSaveScore_UpsertUndeletesSoftDeletedRow(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.RiskScore{}))
	require.NoError(t, db.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS risk_scores_resource_type_resource_uid_cluster_id_key
ON risk_scores (resource_type, resource_uid, cluster_id)`).Error)

	row := &models.RiskScore{
		ResourceType:        "Pod",
		ResourceUID:         "85114b90-9118-44ab-886e-e7c4072dbab6",
		ClusterID:           "default",
		ResourceName:        "coredns",
		Namespace:           "kube-system",
		TotalScore:          1,
		BaseScore:           1,
		SeverityWeight:      1,
		ImpactMultiplier:    1,
		TimeDecay:           1,
		ExploitabilityScore: 0,
		BusinessImpactScore: 0,
		ScorerVersion:       "v2",
		Factors:             "{}",
		PriorityLevel:       "P4",
		HighestSeverity:     "",
		CalculatedAt:        time.Now(),
	}
	require.NoError(t, db.Create(row).Error)
	require.NoError(t, db.Delete(row).Error)

	scorer := NewScorer(db)
	v2 := &RiskScoreV2{
		ResourceType:        "Pod",
		ResourceUID:         "85114b90-9118-44ab-886e-e7c4072dbab6",
		ClusterID:           "default",
		ResourceName:        "coredns",
		Namespace:           "kube-system",
		TotalScore:          42,
		BaseScore:           10,
		ExploitabilityScore: 10,
		BusinessImpactScore: 10,
		TimeDecay:           1,
		ScorerVersion:       "v2",
		Factors:             map[string]interface{}{"k": 1},
		InsightsCount:       2,
		HighestSeverity:     "high",
		PriorityLevel:       "P2",
	}
	require.NoError(t, scorer.SaveScore(context.Background(), v2))

	var got models.RiskScore
	require.NoError(t, db.Unscoped().Where("resource_uid = ?", v2.ResourceUID).First(&got).Error)
	require.False(t, got.DeletedAt.Valid, "soft delete should be cleared by upsert")
	require.InDelta(t, 42.0, got.TotalScore, 0.01)
	require.Equal(t, "P2", got.PriorityLevel)
}

func TestSaveScore_UpsertIdempotentTwice(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.RiskScore{}))
	require.NoError(t, db.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS risk_scores_resource_type_resource_uid_cluster_id_key
ON risk_scores (resource_type, resource_uid, cluster_id)`).Error)

	scorer := NewScorer(db)
	v2 := &RiskScoreV2{
		ResourceType: "Pod",
		ResourceUID:  "e470d198-3d3f-4538-9cd5-b4771d2fb9dc",
		ClusterID:    "c1",
		TotalScore:   5,
		BaseScore:    1,
		TimeDecay:    1,
		ScorerVersion: "v2",
		Factors:      map[string]interface{}{},
		PriorityLevel: "P4",
	}
	require.NoError(t, scorer.SaveScore(context.Background(), v2))
	v2.TotalScore = 7
	require.NoError(t, scorer.SaveScore(context.Background(), v2))

	var n int64
	require.NoError(t, db.Model(&models.RiskScore{}).Where("resource_uid = ?", v2.ResourceUID).Count(&n).Error)
	require.Equal(t, int64(1), n)
	var got models.RiskScore
	require.NoError(t, db.First(&got, "resource_uid = ?", v2.ResourceUID).Error)
	require.InDelta(t, 7.0, got.TotalScore, 0.01)
}
