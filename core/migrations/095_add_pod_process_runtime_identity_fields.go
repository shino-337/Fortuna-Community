package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration095_AddPodProcessRuntimeIdentityFields enriches pod_processes with runtime identity fields
// collected from host /proc so we can detect privilege/capability abuse patterns.
func Migration095_AddPodProcessRuntimeIdentityFields(db *gorm.DB) error {
	log.Println("Running migration 095: Add runtime identity fields to pod_processes (user_id, group_id, working_dir, cap_eff)")

	if err := db.Exec(`
		ALTER TABLE pod_processes ADD COLUMN IF NOT EXISTS user_id INTEGER DEFAULT 0;
		ALTER TABLE pod_processes ADD COLUMN IF NOT EXISTS group_id INTEGER DEFAULT 0;
		ALTER TABLE pod_processes ADD COLUMN IF NOT EXISTS working_dir VARCHAR(1024) DEFAULT '';
		ALTER TABLE pod_processes ADD COLUMN IF NOT EXISTS cap_eff VARCHAR(128) DEFAULT '';
		CREATE INDEX IF NOT EXISTS idx_pod_processes_observed_at_pid ON pod_processes(observed_at DESC, p_id);
	`).Error; err != nil {
		return err
	}

	log.Println("[Migration 095] Completed successfully")
	return nil
}
