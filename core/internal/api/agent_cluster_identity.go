package api

import (
	"context"
	"errors"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errAgentClusterConflict = errors.New("ambiguous legacy agent identity")

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
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize rows for this logical AgentID where supported. Exact cluster rows
		// and one legacy empty-cluster row may coexist during migration, but an AgentID
		// in another cluster is an independent identity and must never block this write.
		var rows []models.Agent
		if err := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("agent_id = ?", agent.AgentID).
			Find(&rows).Error; err != nil {
			return err
		}

		var exact *models.Agent
		legacy := make([]models.Agent, 0, 1)
		for i := range rows {
			switch rows[i].ClusterID {
			case clusterID:
				exact = &rows[i]
			case "":
				legacy = append(legacy, rows[i])
			}
		}

		updates := map[string]any{
			"node_name":    agent.NodeName,
			"version":      agent.Version,
			"status":       "ready",
			"last_seen_at": now,
			"updated_at":   now,
			"deleted_at":   nil,
		}

		if exact != nil {
			return tx.Unscoped().Model(&models.Agent{}).
				Where("id = ? AND cluster_id = ?", exact.ID, clusterID).
				Updates(updates).Error
		}

		if len(legacy) > 1 {
			return errAgentClusterConflict
		}
		if len(legacy) == 1 {
			updates["cluster_id"] = clusterID
			res := tx.Unscoped().Model(&models.Agent{}).
				Where("id = ? AND cluster_id = ''", legacy[0].ID).
				Updates(updates)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 1 {
				return nil
			}
			// A concurrent claim may have moved the legacy row. Fall through to the
			// composite upsert, which is safe under the unique(cluster_id,agent_id) key.
		}

		row := models.Agent{
			ClusterID:  clusterID,
			AgentID:    agent.AgentID,
			NodeName:   agent.NodeName,
			Version:    agent.Version,
			Status:     "ready",
			LastSeenAt: &now,
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "cluster_id"}, {Name: "agent_id"}},
			DoUpdates: clause.Assignments(updates),
		}).Create(&row).Error
	})
}
