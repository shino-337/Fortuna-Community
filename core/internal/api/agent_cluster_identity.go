package api

import (
	"context"
	"errors"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errAgentClusterConflict = errors.New("agent ID already belongs to another cluster")

// upsertAgentClusterIdentity preserves the legacy global AgentID uniqueness until
// every Agent control path is migrated to trusted cluster-qualified identity.
// Cross-cluster reassignment is rejected instead of silently moving ownership.
func upsertAgentClusterIdentity(ctx context.Context, db *gorm.DB, clusterID string, agent *AgentPayload) error {
	if agent == nil || agent.AgentID == "" {
		return nil
	}
	if clusterID == "" {
		return errors.New("cluster ID required")
	}
	if db == nil {
		return errors.New("database required")
	}
	now := time.Now().UTC()
	row := models.Agent{
		ClusterID:  clusterID,
		AgentID:    agent.AgentID,
		NodeName:   agent.NodeName,
		Version:    agent.Version,
		Status:     "ready",
		LastSeenAt: &now,
	}
	result := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "agent_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"cluster_id":   clusterID,
			"node_name":    agent.NodeName,
			"version":      agent.Version,
			"status":       "ready",
			"last_seen_at": now,
			"updated_at":   now,
			"deleted_at":   nil,
		}),
		Where: clause.Where{Exprs: []clause.Expression{clause.Expr{
			SQL:  "agents.cluster_id = ? OR agents.cluster_id = '' OR agents.cluster_id IS NULL",
			Vars: []any{clusterID},
		}}},
	}).Create(&row)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errAgentClusterConflict
	}
	return nil
}
