package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration045_AddRuntimeSignalsTable creates runtime_signals table for semantic runtime events
func Migration045_AddRuntimeSignalsTable(db *gorm.DB) error {
	log.Println("Running migration 045: Add runtime_signals table")

	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS runtime_signals (
			id BIGSERIAL PRIMARY KEY,
			pod_uid VARCHAR(255) NOT NULL,
			signal_type VARCHAR(100) NOT NULL,
			category VARCHAR(50) NOT NULL,
			confidence FLOAT DEFAULT 0.5 CHECK (confidence >= 0 AND confidence <= 1),
			evidence JSONB NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_runtime_signals_pod_uid ON runtime_signals(pod_uid);
		CREATE INDEX IF NOT EXISTS idx_runtime_signals_signal_type ON runtime_signals(signal_type);
		CREATE INDEX IF NOT EXISTS idx_runtime_signals_category ON runtime_signals(category);
		CREATE INDEX IF NOT EXISTS idx_runtime_signals_created_at ON runtime_signals(created_at);
	`).Error; err != nil {
		return err
	}

	log.Println("[Migration 045] ✅ Completed successfully")
	return nil
}
