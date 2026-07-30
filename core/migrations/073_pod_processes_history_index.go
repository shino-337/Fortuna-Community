package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration073_PodProcessesHistoryIndex adds composite index for process history:
// - (pod_uid, observed_at) for efficient "latest snapshot" query and retention delete.
func Migration073_PodProcessesHistoryIndex(db *gorm.DB) error {
	log.Println("Running migration 073: Add pod_processes composite index (pod_uid, observed_at) for history and retention")
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_pod_processes_pod_uid_observed_at ON pod_processes(pod_uid, observed_at DESC)`).Error; err != nil {
		return err
	}
	log.Println("[Migration 073] Completed successfully")
	return nil
}
