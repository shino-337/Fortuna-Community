package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration093_EnsureK8sEventsTable creates k8s_events if missing (repair for DBs where migration 071
// did not complete or table was dropped). Idempotent: CREATE TABLE IF NOT EXISTS.
func Migration093_EnsureK8sEventsTable(db *gorm.DB) error {
	log.Println("[Migration 093] Ensure k8s_events table (Pod Detail event sync)")

	var exists bool
	if err := db.Raw(`
SELECT EXISTS (
  SELECT 1 FROM information_schema.tables
  WHERE table_schema = current_schema() AND table_name = 'k8s_events'
)`).Scan(&exists).Error; err != nil {
		return fmt.Errorf("[Migration 093] check k8s_events: %w", err)
	}
	if exists {
		log.Println("[Migration 093] k8s_events already exists, skipping DDL")
		return nil
	}

	// Same DDL as Migration071 block 4
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS k8s_events (
			id BIGSERIAL PRIMARY KEY,
			cluster_id VARCHAR(255) NOT NULL,
			event_uid VARCHAR(255) NOT NULL,
			namespace VARCHAR(255) NOT NULL,
			event_name VARCHAR(255) NOT NULL,
			involved_kind VARCHAR(64) NOT NULL,
			involved_uid VARCHAR(255) NOT NULL,
			involved_name VARCHAR(255) NOT NULL,
			reason VARCHAR(128) NOT NULL,
			message TEXT DEFAULT '',
			event_type VARCHAR(32) DEFAULT 'Normal',
			count INTEGER DEFAULT 1,
			first_timestamp TIMESTAMP WITH TIME ZONE,
			last_timestamp TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_k8s_events_uid ON k8s_events(cluster_id, event_uid);
		CREATE INDEX IF NOT EXISTS idx_k8s_events_involved_uid ON k8s_events(involved_uid);
		CREATE INDEX IF NOT EXISTS idx_k8s_events_last_timestamp ON k8s_events(last_timestamp);
	`).Error; err != nil {
		return fmt.Errorf("[Migration 093] create k8s_events: %w", err)
	}
	log.Println("[Migration 093] ✅ Created k8s_events")
	return nil
}
