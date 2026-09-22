package migrations

import (
	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

func EnsureInventoryCollection(db *gorm.DB) error {
	// Generic AutoMigrate introspection is not reliable on PostgreSQL reruns.
	// Create once, then validate the required shape without rewriting evidence.
	if !db.Migrator().HasTable(&models.InventoryCollection{}) {
		if err := db.Migrator().CreateTable(&models.InventoryCollection{}); err != nil {
			return err
		}
	}
	if !db.Migrator().HasColumn(&models.InventoryCollection{}, "kind_observed_at") {
		if err := db.Migrator().AddColumn(&models.InventoryCollection{}, "KindObservedAt"); err != nil {
			return err
		}
	}
	if err := db.Exec("SELECT cluster_id,agent_id,collection_id,namespace,status,started_at,observed_at,received_at,payload_sha256,counts,kind_observed_at,role_digests,failure_stage FROM inventory_collections LIMIT 0").Error; err != nil {
		return err
	}
	return ensureIndex(db, "idx_inventory_collection_cluster", "inventory_collections", "cluster_id", true)
}
