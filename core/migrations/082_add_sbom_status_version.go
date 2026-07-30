package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration082_AddSBOMStatusAndVersion adds status and version columns to sboms table
// to support SBOM immutability (pending → finalized snapshots with versioning).
func Migration082_AddSBOMStatusAndVersion(db *gorm.DB) error {
	log.Println("[Migration 082] Adding status and version columns to sboms table")

	if !db.Migrator().HasTable("sboms") {
		log.Println("[Migration 082] sboms table does not exist, skipping")
		return nil
	}

	// Add status column if missing
	var hasStatus bool
	if err := db.Raw(`
SELECT EXISTS (
  SELECT 1 FROM information_schema.columns 
  WHERE table_schema = current_schema() AND table_name = 'sboms' AND column_name = 'status'
)`).Scan(&hasStatus).Error; err != nil {
		return fmt.Errorf("[Migration 082] failed to check sboms.status column: %w", err)
	}
	if !hasStatus {
		log.Println("[Migration 082] Adding status column to sboms...")
		if err := db.Exec(`ALTER TABLE sboms ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'pending'`).Error; err != nil {
			return fmt.Errorf("[Migration 082] failed to add status column: %w", err)
		}
		// Backfill existing rows to finalized so legacy SBOMs are matchable
		if err := db.Exec(`UPDATE sboms SET status = 'finalized' WHERE status IS NULL OR status = ''`).Error; err != nil {
			return fmt.Errorf("[Migration 082] failed to backfill sboms.status: %w", err)
		}
		// Add CHECK constraint for allowed values if missing.
		// Use execDDL + unique dollar-quote tag to avoid GORM's "insufficient arguments" with $$.
		if db.Dialector.Name() == "postgres" {
			if err := execDDL(db, `
DO $m082$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'sboms_status_check'
      AND conrelid = 'sboms'::regclass
  ) THEN
    ALTER TABLE sboms ADD CONSTRAINT sboms_status_check
      CHECK (status IN ('pending','finalized'));
  END IF;
END$m082$;
`); err != nil {
				return fmt.Errorf("[Migration 082] failed to add status CHECK constraint: %w", err)
			}
		}
	}

	// Add version column if missing
	var hasVersion bool
	if err := db.Raw(`
SELECT EXISTS (
  SELECT 1 FROM information_schema.columns 
  WHERE table_schema = current_schema() AND table_name = 'sboms' AND column_name = 'version'
)`).Scan(&hasVersion).Error; err != nil {
		return fmt.Errorf("[Migration 082] failed to check sboms.version column: %w", err)
	}
	if !hasVersion {
		log.Println("[Migration 082] Adding version column to sboms...")
		if err := db.Exec(`ALTER TABLE sboms ADD COLUMN version INTEGER NOT NULL DEFAULT 1`).Error; err != nil {
			return fmt.Errorf("[Migration 082] failed to add version column: %w", err)
		}
	}

	log.Println("[Migration 082] Completed adding status and version columns to sboms table")
	return nil
}

