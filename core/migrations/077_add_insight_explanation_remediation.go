package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration077_AddInsightExplanationRemediation adds risk_explanation and remediation columns to insights table
// to support structured explanation/remediation content for Risk Detail UI.
func Migration077_AddInsightExplanationRemediation(db *gorm.DB) error {
	log.Println("Running migration 077: Add risk_explanation and remediation to insights")

	var tableExists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'insights')").Scan(&tableExists).Error; err != nil {
		return err
	}
	if !tableExists {
		log.Println("[Migration 077] insights table does not exist, skipping")
		return nil
	}

	if err := db.Exec("ALTER TABLE insights ADD COLUMN IF NOT EXISTS risk_explanation TEXT").Error; err != nil {
		return err
	}
	if err := db.Exec("ALTER TABLE insights ADD COLUMN IF NOT EXISTS remediation JSONB").Error; err != nil {
		return err
	}

	log.Println("[Migration 077] ✅ Completed successfully")
	return nil
}

