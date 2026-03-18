package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration083_AddSBOMMatchRuns creates the sbom_match_runs table used for
// idempotency of CVE matcher runs per SBOM version and mirror version.
func Migration083_AddSBOMMatchRuns(db *gorm.DB) error {
	log.Println("[Migration 083] Creating sbom_match_runs table")

	// Create table if it doesn't exist
	if err := db.Exec(`
CREATE TABLE IF NOT EXISTS sbom_match_runs (
  sbom_id BIGINT NOT NULL,
  version INTEGER NOT NULL,
  mirror_version VARCHAR(128) NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'running',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (sbom_id, version, mirror_version)
);`).Error; err != nil {
		return fmt.Errorf("[Migration 083] failed to create sbom_match_runs table: %w", err)
	}

	// Helpful index for querying by sbom_id
	if err := db.Exec(`
CREATE INDEX IF NOT EXISTS idx_sbom_match_runs_sbom_id 
ON sbom_match_runs (sbom_id);`).Error; err != nil {
		return fmt.Errorf("[Migration 083] failed to create idx_sbom_match_runs_sbom_id: %w", err)
	}

	log.Println("[Migration 083] Completed creating sbom_match_runs table")
	return nil
}

