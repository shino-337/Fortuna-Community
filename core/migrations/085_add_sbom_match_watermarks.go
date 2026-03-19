package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration085_AddSBOMMatchWatermarks creates a small watermark table used as an atomic replay guard.
// It ensures we don't process stale sbom.created events (older SBOM versions / timestamps) after a newer one was processed.
func Migration085_AddSBOMMatchWatermarks(db *gorm.DB) error {
	log.Println("[Migration 085] Creating sbom_match_watermarks table")

	if err := db.Exec(`
CREATE TABLE IF NOT EXISTS sbom_match_watermarks (
  sbom_id BIGINT PRIMARY KEY,
  latest_version INTEGER NOT NULL,
  latest_event_ts BIGINT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`).Error; err != nil {
		return fmt.Errorf("[Migration 085] failed to create sbom_match_watermarks table: %w", err)
	}

	log.Println("[Migration 085] Completed creating sbom_match_watermarks table")
	return nil
}

