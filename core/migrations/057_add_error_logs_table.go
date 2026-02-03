package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration057_AddErrorLogsTable creates the error_logs table for real dashboard data.
func Migration057_AddErrorLogsTable(db *gorm.DB) error {
	log.Println("Running migration 057: Add error_logs table")
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS error_logs (
			id SERIAL PRIMARY KEY,
			source VARCHAR(100) NOT NULL,
			level VARCHAR(20) NOT NULL,
			message TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP WITH TIME ZONE
		);
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_error_logs_deleted_at ON error_logs(deleted_at)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_error_logs_created_at ON error_logs(created_at DESC)`).Error; err != nil {
		return err
	}
	log.Println("Migration 057 completed successfully")
	return nil
}
