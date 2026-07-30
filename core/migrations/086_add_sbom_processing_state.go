package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration086_AddSBOMProcessingState creates sbom_processing_state used as an atomic replay guard.
// It prevents older sbom.created events (by timestamp) from overwriting newer processing outcomes.
func Migration086_AddSBOMProcessingState(db *gorm.DB) error {
	log.Println("[Migration 086] Creating sbom_processing_state table")

	if err := db.Exec(`
CREATE TABLE IF NOT EXISTS sbom_processing_state (
  sbom_id BIGINT PRIMARY KEY,
  latest_event_ts BIGINT NOT NULL DEFAULT 0,
  latest_event_id VARCHAR(64) NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);`).Error; err != nil {
		return fmt.Errorf("[Migration 086] failed to create sbom_processing_state table: %w", err)
	}

	log.Println("[Migration 086] Completed creating sbom_processing_state table")
	return nil
}

