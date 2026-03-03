package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration069_AddPodSpecHash adds spec_hash column for hash-based change detection (POD_SYNC_ARCHITECTURE §4.3).
// PCE is triggered only when spec_hash changes.
func Migration069_AddPodSpecHash(db *gorm.DB) error {
	log.Println("Running migration 069: Add pods.spec_hash column")

	if err := db.Exec(`
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS spec_hash VARCHAR(64) DEFAULT '';
	`).Error; err != nil {
		return err
	}

	log.Println("Migration 069 completed successfully")
	return nil
}
