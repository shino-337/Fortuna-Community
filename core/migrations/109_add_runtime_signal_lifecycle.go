package migrations

import "gorm.io/gorm"

// Migration109_AddRuntimeSignalLifecycle adds lifecycle + explainability refs to runtime_signals.
func Migration109_AddRuntimeSignalLifecycle(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	_ = db.Exec("ALTER TABLE runtime_signals ADD COLUMN IF NOT EXISTS first_seen_at TIMESTAMP WITH TIME ZONE").Error
	_ = db.Exec("ALTER TABLE runtime_signals ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMP WITH TIME ZONE").Error
	_ = db.Exec("ALTER TABLE runtime_signals ADD COLUMN IF NOT EXISTS evidence_refs JSONB").Error
	_ = db.Exec("UPDATE runtime_signals SET first_seen_at = COALESCE(first_seen_at, created_at), last_seen_at = COALESCE(last_seen_at, created_at)").Error
	_ = db.Exec("CREATE INDEX IF NOT EXISTS idx_runtime_signals_first_seen_at ON runtime_signals(first_seen_at DESC)").Error
	_ = db.Exec("CREATE INDEX IF NOT EXISTS idx_runtime_signals_last_seen_at ON runtime_signals(last_seen_at DESC)").Error
	return nil
}
