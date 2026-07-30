package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration047_AddCapabilityMetadataTable creates capability_metadata table for semantic layer
func Migration047_AddCapabilityMetadataTable(db *gorm.DB) error {
	log.Println("Running migration 047: Add capability_metadata table")

	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS capability_metadata (
			capability_id VARCHAR(100) PRIMARY KEY,
			domain VARCHAR(50) NOT NULL,
			category VARCHAR(100) NOT NULL,
			description TEXT NOT NULL,
			severity_base VARCHAR(20) NOT NULL,
			confidence_base FLOAT DEFAULT 0.5 CHECK (confidence_base >= 0 AND confidence_base <= 1),
			preconditions JSONB,
			produces_attack_steps JSONB,
			expires_with_instance BOOLEAN DEFAULT true,
			supports_runtime_promotion BOOLEAN DEFAULT true,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_capability_metadata_domain ON capability_metadata(domain);
		CREATE INDEX IF NOT EXISTS idx_capability_metadata_category ON capability_metadata(category);
	`).Error; err != nil {
		return err
	}

	log.Println("[Migration 047] ✅ Completed successfully")
	return nil
}
