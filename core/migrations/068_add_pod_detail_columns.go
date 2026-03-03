package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration068_AddPodDetailColumns adds Pod Detail (POD_DETAIL_SPEC) columns to pods table:
// pod_ip, start_time, restart_count, owner_kind, owner_name, replica_set_name, qos_class.
func Migration068_AddPodDetailColumns(db *gorm.DB) error {
	log.Println("Running migration 068: Add pod detail columns (POD_DETAIL_SPEC)")

	if err := db.Exec(`
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS pod_ip VARCHAR(45) DEFAULT '';
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS start_time TIMESTAMP WITH TIME ZONE;
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS restart_count INTEGER DEFAULT 0;
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS owner_kind VARCHAR(64) DEFAULT '';
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS owner_name VARCHAR(255) DEFAULT '';
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS replica_set_name VARCHAR(255) DEFAULT '';
		ALTER TABLE pods ADD COLUMN IF NOT EXISTS qos_class VARCHAR(32) DEFAULT '';
	`).Error; err != nil {
		return err
	}

	log.Println("Migration 068 completed successfully")
	return nil
}
