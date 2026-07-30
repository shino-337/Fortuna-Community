package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration102_AddRuntimeEventsMetadata adds enrichment columns to runtime_events for R9.
// These fields are already present in agent payloads and help correlation/richness in UI/REP.
func Migration102_AddRuntimeEventsMetadata(db *gorm.DB) error {
	log.Println("Running migration 102: Add metadata columns to runtime_events")
	cols := []struct {
		name string
		typ  string
	}{
		{"pod_name", "VARCHAR(255)"},
		{"node_name", "VARCHAR(255)"},
		{"runtime", "VARCHAR(64)"},
		{"event_type", "VARCHAR(128)"},
		{"signal", "VARCHAR(128)"},
		{"mitre_technique", "VARCHAR(64)"},
		{"severity", "VARCHAR(32)"},
	}
	for _, c := range cols {
		var exists int
		if err := db.Raw(`
			SELECT COUNT(*)
			FROM information_schema.columns
			WHERE table_name = 'runtime_events' AND column_name = ?
		`, c.name).Scan(&exists).Error; err != nil {
			return err
		}
		if exists == 0 {
			if err := db.Exec("ALTER TABLE runtime_events ADD COLUMN " + c.name + " " + c.typ).Error; err != nil {
				return err
			}
		}
	}
	// Helpful indexes for filtering in UI/APIs
	_ = db.Exec("CREATE INDEX IF NOT EXISTS idx_runtime_events_created_at ON runtime_events(created_at)").Error
	_ = db.Exec("CREATE INDEX IF NOT EXISTS idx_runtime_events_capability ON runtime_events(capability)").Error
	return nil
}

