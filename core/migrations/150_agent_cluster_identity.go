package migrations

import (
	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Leave historical agents unassigned: node names do not prove cluster ownership.
func Migration150_AgentClusterIdentity(db *gorm.DB) error {
	if !db.Migrator().HasColumn(&models.Agent{}, "ClusterID") {
		if err := db.Migrator().AddColumn(&models.Agent{}, "ClusterID"); err != nil {
			return err
		}
	}
	if !db.Migrator().HasIndex(&models.Agent{}, "idx_agents_cluster_node") {
		return db.Migrator().CreateIndex(&models.Agent{}, "idx_agents_cluster_node")
	}
	return nil
}
