package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration046_AddCapabilityStateMachine adds state machine fields to pod_capabilities
func Migration046_AddCapabilityStateMachine(db *gorm.DB) error {
	log.Println("Running migration 046: Add capability state machine")

	// Add state column
	if err := db.Exec(`
		ALTER TABLE pod_capabilities 
		ADD COLUMN IF NOT EXISTS state VARCHAR(20) CHECK (state IN ('detected', 'confirmed', 'exploited', 'chained')) DEFAULT 'detected';
	`).Error; err != nil {
		return err
	}

	// Add confidence column
	if err := db.Exec(`
		ALTER TABLE pod_capabilities 
		ADD COLUMN IF NOT EXISTS confidence FLOAT DEFAULT 0.5 CHECK (confidence >= 0 AND confidence <= 1);
	`).Error; err != nil {
		return err
	}

	// Add timestamps
	if err := db.Exec(`
		ALTER TABLE pod_capabilities 
		ADD COLUMN IF NOT EXISTS first_seen_at TIMESTAMP WITH TIME ZONE;
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		ALTER TABLE pod_capabilities 
		ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMP WITH TIME ZONE;
	`).Error; err != nil {
		return err
	}

	// Initialize timestamps for existing records
	if err := db.Exec(`
		UPDATE pod_capabilities 
		SET first_seen_at = created_at, last_seen_at = updated_at
		WHERE first_seen_at IS NULL;
	`).Error; err != nil {
		log.Printf("[Migration 046] Warning: Failed to initialize timestamps: %v", err)
	}

	// Create indexes
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_pod_capabilities_state ON pod_capabilities(state);
		CREATE INDEX IF NOT EXISTS idx_pod_capabilities_confidence ON pod_capabilities(confidence);
	`).Error; err != nil {
		return err
	}

	log.Println("[Migration 046] ✅ Completed successfully")
	return nil
}
