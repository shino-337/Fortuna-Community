package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration088_UpdateSBOMStatusCheck extends sboms.status allowed values to include:
// complete | partial | failed (+ legacy pending/finalized).
// It also backfills legacy finalized -> complete so downstream risk decisions use a unified status model.
func Migration088_UpdateSBOMStatusCheck(db *gorm.DB) error {
	log.Println("[Migration 088] Update sboms.status check constraint (complete|partial|failed)")

	if !db.Migrator().HasTable("sboms") {
		log.Println("[Migration 088] sboms table does not exist, skipping")
		return nil
	}

	// Drop existing constraint if present.
	var hasConstraint bool
	if err := db.Raw(`
SELECT EXISTS (
  SELECT 1
  FROM pg_constraint
  WHERE conname = 'sboms_status_check'
);`).Scan(&hasConstraint).Error; err != nil {
		return fmt.Errorf("[Migration 088] failed to check sboms_status_check constraint: %w", err)
	}

	if hasConstraint {
		if err := db.Exec(`ALTER TABLE sboms DROP CONSTRAINT sboms_status_check`).Error; err != nil {
			return fmt.Errorf("[Migration 088] failed to drop sboms_status_check: %w", err)
		}
	}

	if err := db.Exec(`
ALTER TABLE sboms
ADD CONSTRAINT sboms_status_check
CHECK (status IN ('pending','finalized','complete','partial','failed'))
`).Error; err != nil {
		return fmt.Errorf("[Migration 088] failed to add sboms_status_check: %w", err)
	}

	// Backfill legacy finalized to complete (idempotent).
	if err := db.Exec(`UPDATE sboms SET status = 'complete' WHERE status = 'finalized'`).Error; err != nil {
		return fmt.Errorf("[Migration 088] failed to backfill finalized -> complete: %w", err)
	}

	log.Println("[Migration 088] Completed sboms.status check constraint update")
	return nil
}

