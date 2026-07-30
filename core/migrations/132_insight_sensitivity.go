package migrations

import (
	"log"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Migration132_InsightSensitivity adds findings.sensitivity classification hook (default internal).
// Uses explicit DDL when needed: AutoMigrate can skip adding a new column if the table already
// exists with a legacy shape (e.g. after SQL-only resets), which then breaks insight INSERTs.
func Migration132_InsightSensitivity(db *gorm.DB) error {
	log.Println("Running migration 132: insights.sensitivity")
	if !db.Migrator().HasTable(&models.Insight{}) {
		log.Println("Migration 132: insights table missing, skipping")
		return nil
	}
	if db.Migrator().HasColumn(&models.Insight{}, "Sensitivity") {
		log.Println("Migration 132: insights.sensitivity already present")
		return nil
	}
	switch db.Dialector.Name() {
	case "sqlite":
		if err := db.Exec("ALTER TABLE insights ADD COLUMN sensitivity TEXT NOT NULL DEFAULT 'internal'").Error; err != nil {
			return err
		}
	default:
		if err := db.Exec("ALTER TABLE insights ADD COLUMN sensitivity VARCHAR(32) NOT NULL DEFAULT 'internal'").Error; err != nil {
			return err
		}
	}
	return db.AutoMigrate(&models.Insight{})
}
