package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration038_AddAgentsTable creates the agents table for dashboard metrics.
func Migration038_AddAgentsTable(db *gorm.DB) error {
	log.Println("Running migration 038: Add agents table")
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS agents (
			id SERIAL PRIMARY KEY,
			agent_id VARCHAR(255) UNIQUE NOT NULL,
			node_name VARCHAR(255),
			version VARCHAR(100),
			status VARCHAR(50),
			capabilities JSONB,
			last_seen_at TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP WITH TIME ZONE
		);
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_agents_deleted_at ON agents(deleted_at)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status)`).Error; err != nil {
		return err
	}

	log.Println("Migration 038 completed successfully")
	return nil
}
