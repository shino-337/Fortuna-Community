package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration042_AddPodFactColumns adds pod security fact columns to pods table
func Migration042_AddPodFactColumns(db *gorm.DB) error {
	log.Println("Running migration 042: Add pod fact columns to pods")

	if err := db.Exec(`
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS pod_security_context JSONB;
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS container_security_contexts JSONB;
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS volume_mounts JSONB;
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS volumes JSONB;
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS tolerations JSONB;
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS affinity JSONB;
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS host_network BOOLEAN DEFAULT FALSE;
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS host_pid BOOLEAN DEFAULT FALSE;
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS host_ipc BOOLEAN DEFAULT FALSE;
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS automount_service_account_token BOOLEAN;
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS node_name VARCHAR(255);
		CREATE INDEX IF NOT EXISTS idx_pods_node_name ON pods(node_name);
	`).Error; err != nil {
		return err
	}

	log.Println("[Migration 042] ✅ Completed successfully")
	return nil
}
