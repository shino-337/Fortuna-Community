package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration094_EnsureAgentsTable creates agents if missing (repair after reset-db or partial migrations).
// Same DDL as Migration038; idempotent.
func Migration094_EnsureAgentsTable(db *gorm.DB) error {
	log.Println("[Migration 094] Ensure agents table (Agent Ping / Register)")

	var exists bool
	if err := db.Raw(`
SELECT EXISTS (
  SELECT 1 FROM information_schema.tables
  WHERE table_schema = current_schema() AND table_name = 'agents'
)`).Scan(&exists).Error; err != nil {
		return fmt.Errorf("[Migration 094] check agents: %w", err)
	}
	if exists {
		log.Println("[Migration 094] agents already exists, skipping DDL")
		return nil
	}

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
		CREATE INDEX IF NOT EXISTS idx_agents_deleted_at ON agents(deleted_at);
		CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
	`).Error; err != nil {
		return fmt.Errorf("[Migration 094] create agents: %w", err)
	}
	log.Println("[Migration 094] ✅ Created agents")
	return nil
}
