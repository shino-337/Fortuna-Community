package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration041_AddPodCapabilitiesTable creates pod_capabilities table
func Migration041_AddPodCapabilitiesTable(db *gorm.DB) error {
	log.Println("Running migration 041: Add pod_capabilities table")

	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS pod_capabilities (
			id SERIAL PRIMARY KEY,
			pod_uid VARCHAR(255) NOT NULL,
			namespace VARCHAR(255) NOT NULL,
			capability_id VARCHAR(100) NOT NULL,
			capability_group VARCHAR(50) NOT NULL,
			severity VARCHAR(20) NOT NULL,
			evidence JSONB,
			mitre TEXT[],
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_pod_capabilities_unique ON pod_capabilities(pod_uid, capability_id);
		CREATE INDEX IF NOT EXISTS idx_pod_capabilities_pod_uid ON pod_capabilities(pod_uid);
		CREATE INDEX IF NOT EXISTS idx_pod_capabilities_capability_id ON pod_capabilities(capability_id);
	`).Error; err != nil {
		return err
	}

	log.Println("[Migration 041] ✅ Completed successfully")
	return nil
}
