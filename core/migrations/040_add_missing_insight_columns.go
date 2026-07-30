package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration040_AddMissingInsightColumns adds missing columns to insights table
func Migration040_AddMissingInsightColumns(db *gorm.DB) error {
	log.Println("Running migration 040: Add missing insights columns")

	var tableExists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'insights')").Scan(&tableExists).Error; err != nil {
		return err
	}
	if !tableExists {
		log.Println("[Migration 040] insights table does not exist, skipping")
		return nil
	}

	if err := db.Exec("ALTER TABLE insights ADD COLUMN IF NOT EXISTS fixed_version VARCHAR(100)").Error; err != nil {
		return err
	}
	if err := db.Exec("ALTER TABLE insights ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMP WITH TIME ZONE").Error; err != nil {
		return err
	}

	log.Println("[Migration 040] ✅ Completed successfully")
	return nil
}
