package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration103_AddRuntimeSignalsCount adds an occurrence counter to runtime_signals.
// This is required for R6 multi-signal chains: SignalAdapter de-dupes by day, so we
// need a persisted counter to reflect repeated occurrences.
func Migration103_AddRuntimeSignalsCount(db *gorm.DB) error {
	log.Println("Running migration 103: Add count column to runtime_signals")

	var exists int
	if err := db.Raw(`
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_name = 'runtime_signals' AND column_name = 'count'
	`).Scan(&exists).Error; err != nil {
		return err
	}
	if exists == 0 {
		if err := db.Exec("ALTER TABLE runtime_signals ADD COLUMN count INTEGER DEFAULT 1").Error; err != nil {
			return err
		}
	}
	_ = db.Exec("CREATE INDEX IF NOT EXISTS idx_runtime_signals_created_at ON runtime_signals(created_at)").Error
	return nil
}

