package migrations

import (
	"log"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Migration108_AddRuntimeEventsCanonicalColumns evolves runtime_events toward canonical contract fields.
// Uses explicit ALTER TABLE instead of AutoMigrate to avoid silent failures on PostgreSQL
// when runtime_events already exists with constraints.
func Migration108_AddRuntimeEventsCanonicalColumns(db *gorm.DB) error {
	log.Println("Running migration 108: Add runtime_events canonical columns")

	if !db.Migrator().HasTable("runtime_events") {
		if err := db.AutoMigrate(&models.RuntimeEvent{}); err != nil {
			return err
		}
	} else {
		// Explicit column adds — each is idempotent (IF NOT EXISTS on Postgres 9.6+).
		// AutoMigrate can silently fail to add columns when table has complex constraints.
		cols := []struct {
			name string
			ddl  string
		}{
			{"event_id", "ALTER TABLE runtime_events ADD COLUMN IF NOT EXISTS event_id VARCHAR(64) DEFAULT ''"},
			{"observed_at", "ALTER TABLE runtime_events ADD COLUMN IF NOT EXISTS observed_at TIMESTAMPTZ"},
			{"ingested_at", "ALTER TABLE runtime_events ADD COLUMN IF NOT EXISTS ingested_at TIMESTAMPTZ"},
			{"resolution_state", "ALTER TABLE runtime_events ADD COLUMN IF NOT EXISTS resolution_state VARCHAR(20) DEFAULT ''"},
			{"source_kind", "ALTER TABLE runtime_events ADD COLUMN IF NOT EXISTS source_kind VARCHAR(64) DEFAULT ''"},
			{"source_sensor_id", "ALTER TABLE runtime_events ADD COLUMN IF NOT EXISTS source_sensor_id VARCHAR(128) DEFAULT ''"},
			{"source_rule", "ALTER TABLE runtime_events ADD COLUMN IF NOT EXISTS source_rule VARCHAR(256) DEFAULT ''"},
			{"payload_json", "ALTER TABLE runtime_events ADD COLUMN IF NOT EXISTS payload_json JSONB DEFAULT '{}'"},
			{"payload_hash", "ALTER TABLE runtime_events ADD COLUMN IF NOT EXISTS payload_hash VARCHAR(128) DEFAULT ''"},
			{"mitre_technique", "ALTER TABLE runtime_events ADD COLUMN IF NOT EXISTS mitre_technique VARCHAR(64) DEFAULT ''"},
		}
		for _, c := range cols {
			if err := db.Exec(c.ddl).Error; err != nil {
				log.Printf("[Migration 108] WARN: add column %s: %v (may already exist)", c.name, err)
			}
		}
	}

	_ = db.Exec("CREATE INDEX IF NOT EXISTS idx_runtime_events_event_id ON runtime_events(event_id)").Error
	_ = db.Exec("CREATE INDEX IF NOT EXISTS idx_runtime_events_pod_observed ON runtime_events(pod_uid, observed_at DESC)").Error
	_ = db.Exec("CREATE INDEX IF NOT EXISTS idx_runtime_events_resolution_state ON runtime_events(resolution_state)").Error
	_ = db.Exec("CREATE INDEX IF NOT EXISTS idx_runtime_events_source_kind ON runtime_events(source_kind)").Error
	_ = db.Exec("CREATE INDEX IF NOT EXISTS idx_runtime_events_source_sensor ON runtime_events(source_sensor_id)").Error
	_ = db.Exec("CREATE INDEX IF NOT EXISTS idx_runtime_events_payload_hash ON runtime_events(payload_hash)").Error
	log.Println("[Migration 108] runtime_events canonical columns ensured")
	return nil
}
