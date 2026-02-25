package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration063_AddPodsLastSeenCleanupIndex adds an index used by stale pod cleanup
// and dashboard pod-count queries.
func Migration063_AddPodsLastSeenCleanupIndex(db *gorm.DB) error {
	log.Println("Running migration 063: Add pods cleanup index")

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_pods_cluster_deleted_updated
		ON pods(cluster_id, deleted_at, updated_at);
	`).Error; err != nil {
		return err
	}

	log.Println("Migration 063 completed successfully")
	return nil
}
