package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration111_HardenSBOMRunAndEnums:
// - hardens sbom_match_runs lifecycle columns for timeout/recovery
// - adds SBOM enum-like CHECK constraints for sbom_source/confidence
func Migration111_HardenSBOMRunAndEnums(db *gorm.DB) error {
	log.Println("[Migration 111] Harden SBOM match-run lifecycle and SBOM enum constraints")

	if db.Migrator().HasTable("sbom_match_runs") {
		type colDef struct {
			name string
			sql  string
		}
		cols := []colDef{
			{
				name: "error_code",
				sql:  `ALTER TABLE sbom_match_runs ADD COLUMN error_code VARCHAR(64) NOT NULL DEFAULT ''`,
			},
			{
				name: "started_at",
				sql:  `ALTER TABLE sbom_match_runs ADD COLUMN started_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`,
			},
			{
				name: "timeout_at",
				sql:  `ALTER TABLE sbom_match_runs ADD COLUMN timeout_at TIMESTAMPTZ`,
			},
			{
				name: "updated_at",
				sql:  `ALTER TABLE sbom_match_runs ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`,
			},
		}
		for _, col := range cols {
			var has bool
			if err := db.Raw(`
SELECT EXISTS (
  SELECT 1
  FROM information_schema.columns
  WHERE table_schema = current_schema()
    AND table_name = 'sbom_match_runs'
    AND column_name = ?
);`, col.name).Scan(&has).Error; err != nil {
				return fmt.Errorf("[Migration 111] check sbom_match_runs.%s: %w", col.name, err)
			}
			if !has {
				if err := db.Exec(col.sql).Error; err != nil {
					return fmt.Errorf("[Migration 111] add sbom_match_runs.%s: %w", col.name, err)
				}
			}
		}
		if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_sbom_match_runs_timeout_at ON sbom_match_runs(timeout_at)`).Error; err != nil {
			return fmt.Errorf("[Migration 111] create idx_sbom_match_runs_timeout_at: %w", err)
		}
		if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_sbom_match_runs_status ON sbom_match_runs(status)`).Error; err != nil {
			return fmt.Errorf("[Migration 111] create idx_sbom_match_runs_status: %w", err)
		}
	}

	if db.Migrator().HasTable("sboms") {
		// Replace constraints idempotently.
		_ = db.Exec(`ALTER TABLE sboms DROP CONSTRAINT IF EXISTS chk_sboms_sbom_source`)
		_ = db.Exec(`ALTER TABLE sboms DROP CONSTRAINT IF EXISTS chk_sboms_confidence`)
		if err := db.Exec(`
ALTER TABLE sboms
ADD CONSTRAINT chk_sboms_sbom_source
CHECK (sbom_source IN ('parsers','distroless-heuristic','label-metadata','syft','unknown'));`).Error; err != nil {
			return fmt.Errorf("[Migration 111] add chk_sboms_sbom_source: %w", err)
		}
		if err := db.Exec(`
ALTER TABLE sboms
ADD CONSTRAINT chk_sboms_confidence
CHECK (confidence IN ('low','medium','high','unknown'));`).Error; err != nil {
			return fmt.Errorf("[Migration 111] add chk_sboms_confidence: %w", err)
		}
	}

	log.Println("[Migration 111] Completed")
	return nil
}
