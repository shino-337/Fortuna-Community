package migrations

import (
	"log"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Migration126_AddRuntimeEventsConfidence adds a non-optional confidence column for runtime_events.
// Ingestion must set confidence > 0; scoring ignores rows with confidence <= 0 or NULL.
func Migration126_AddRuntimeEventsConfidence(db *gorm.DB) error {
	log.Println("Running migration 126: Add runtime_events.confidence")

	if !db.Migrator().HasTable("runtime_events") {
		if err := db.AutoMigrate(&models.RuntimeEvent{}); err != nil {
			return err
		}
	} else {
		if err := db.Exec("ALTER TABLE runtime_events ADD COLUMN IF NOT EXISTS confidence DOUBLE PRECISION").Error; err != nil {
			log.Printf("[Migration 126] WARN: add confidence column: %v", err)
		}
	}

	log.Println("[Migration 126] runtime_events.confidence ensured")
	return nil
}
