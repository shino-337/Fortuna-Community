package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration072_PodProcessesGormColumns renames pod_processes columns to match GORM default (PID->p_id, PPID->pp_id).
// Idempotent: only renames if the old column name exists (e.g. table was created by migration 071 with pid/ppid).
// If columns are already p_id/pp_id (e.g. from GORM AutoMigrate), skip.
func Migration072_PodProcessesGormColumns(db *gorm.DB) error {
	log.Println("Running migration 072: Rename pod_processes pid/ppid to p_id/pp_id for GORM")

	// PostgreSQL: only RENAME if column "pid" exists
	if err := db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'pod_processes' AND column_name = 'pid') THEN
				ALTER TABLE pod_processes RENAME COLUMN pid TO p_id;
				RAISE NOTICE '[Migration 072] Renamed pid -> p_id';
			END IF;
		END $$;
	`).Error; err != nil {
		log.Printf("[Migration 072] pid->p_id: %v", err)
		return err
	}
	if err := db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'pod_processes' AND column_name = 'ppid') THEN
				ALTER TABLE pod_processes RENAME COLUMN ppid TO pp_id;
				RAISE NOTICE '[Migration 072] Renamed ppid -> pp_id';
			END IF;
		END $$;
	`).Error; err != nil {
		log.Printf("[Migration 072] ppid->pp_id: %v", err)
		return err
	}

	log.Println("[Migration 072] Completed successfully")
	return nil
}
