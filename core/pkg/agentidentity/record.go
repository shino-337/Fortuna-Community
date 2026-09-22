package agentidentity

import (
	"context"
	"fmt"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UpsertRecord never adopts an unowned legacy row or another cluster's AgentID.
// Empty capabilities/version on a heartbeat preserve registration metadata.
func UpsertRecord(ctx context.Context, db *gorm.DB, row *models.Agent) error {
	if db == nil || row == nil {
		return fmt.Errorf("agent database and record are required")
	}
	if _, err := resourceidentity.New(row.ClusterID, row.AgentID); err != nil {
		return err
	}
	now := time.Now().UTC()
	row.LastSeenAt = &now
	row.Status = "ready"
	updates := map[string]any{"node_name": row.NodeName, "status": row.Status, "last_seen_at": now, "updated_at": now, "deleted_at": nil}
	if row.Version != "" {
		updates["version"] = row.Version
	}
	if row.Capabilities != "" {
		updates["capabilities"] = row.Capabilities
	} else {
		row.Capabilities = "[]"
	}
	return db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "cluster_id"}, {Name: "agent_id"}}, DoUpdates: clause.Assignments(updates)}).Create(row).Error
}
