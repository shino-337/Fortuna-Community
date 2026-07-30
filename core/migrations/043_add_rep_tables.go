package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration043_AddREPTables creates runtime escape probe tables.
func Migration043_AddREPTables(db *gorm.DB) error {
	log.Println("Running migration 043: Add REP tables")

	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS pod_risk_profiles (
			id SERIAL PRIMARY KEY,
			pod_uid VARCHAR(255) NOT NULL,
			namespace VARCHAR(255) NOT NULL,
			static_risk INTEGER NOT NULL DEFAULT 0,
			runtime_score INTEGER NOT NULL DEFAULT 0,
			capabilities TEXT[],
			last_event_at TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_pod_risk_profiles_unique ON pod_risk_profiles(pod_uid);
		CREATE INDEX IF NOT EXISTS idx_pod_risk_profiles_pod_uid ON pod_risk_profiles(pod_uid);

		CREATE TABLE IF NOT EXISTS runtime_events (
			id SERIAL PRIMARY KEY,
			pod_uid VARCHAR(255) NOT NULL,
			namespace VARCHAR(255) NOT NULL,
			syscall VARCHAR(100) NOT NULL,
			target_path VARCHAR(500),
			capability VARCHAR(100),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_runtime_events_pod_uid ON runtime_events(pod_uid);
		CREATE INDEX IF NOT EXISTS idx_runtime_events_syscall ON runtime_events(syscall);
	`).Error; err != nil {
		return err
	}

	log.Println("[Migration 043] ✅ Completed successfully")
	return nil
}
