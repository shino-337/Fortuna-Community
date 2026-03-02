package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration066_AddPodsPhase adds pod phase (Kubernetes status: Running, Pending, etc.)
// for display in Resources and Pod Detail.
func Migration066_AddPodsPhase(db *gorm.DB) error {
	log.Println("Running migration 066: Add pods.phase column")

	if err := db.Exec(`
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS phase VARCHAR(32) DEFAULT '';
	`).Error; err != nil {
		return err
	}

	log.Println("Migration 066 completed successfully")
	return nil
}
