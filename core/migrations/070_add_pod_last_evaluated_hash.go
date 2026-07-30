package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration070_AddPodLastEvaluatedHash adds last_evaluated_hash so we can detect unevaluated pods and avoid race (discard stale PCE result).
func Migration070_AddPodLastEvaluatedHash(db *gorm.DB) error {
	log.Println("Running migration 070: Add pods.last_evaluated_hash column")

	if err := db.Exec(`
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS last_evaluated_hash VARCHAR(64) DEFAULT '';
	`).Error; err != nil {
		return err
	}

	log.Println("Migration 070 completed successfully")
	return nil
}
