package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration056_AddNotificationsTable creates the notifications table for real dashboard data.
func Migration056_AddNotificationsTable(db *gorm.DB) error {
	log.Println("Running migration 056: Add notifications table")
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS notifications (
			id SERIAL PRIMARY KEY,
			title VARCHAR(255) NOT NULL,
			message TEXT,
			severity VARCHAR(50),
			read_at TIMESTAMP WITH TIME ZONE,
			source VARCHAR(100),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP WITH TIME ZONE
		);
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_notifications_deleted_at ON notifications(deleted_at)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at DESC)`).Error; err != nil {
		return err
	}
	log.Println("Migration 056 completed successfully")
	return nil
}
