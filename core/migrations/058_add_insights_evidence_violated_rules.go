package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration058_AddInsightsEvidenceViolatedRules adds evidence and violated_rules columns to insights table
// for Risk Detail page (Evidence / Violated Rules). Optional JSONB; risk engine can populate when available.
func Migration058_AddInsightsEvidenceViolatedRules(db *gorm.DB) error {
	log.Println("Running migration 058: Add evidence and violated_rules to insights")

	var tableExists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'insights')").Scan(&tableExists).Error; err != nil {
		return err
	}
	if !tableExists {
		log.Println("[Migration 058] insights table does not exist, skipping")
		return nil
	}

	if err := db.Exec("ALTER TABLE insights ADD COLUMN IF NOT EXISTS evidence JSONB").Error; err != nil {
		return err
	}
	if err := db.Exec("ALTER TABLE insights ADD COLUMN IF NOT EXISTS violated_rules JSONB").Error; err != nil {
		return err
	}

	log.Println("[Migration 058] ✅ Completed successfully")
	return nil
}
