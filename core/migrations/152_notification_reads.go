package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration152_NotificationReads moves notification read state from the shared
// notifications.read_at column to one row per (notification, user). Rows that
// were already marked read stay read for every existing user, so the upgrade
// does not bring old notifications back to everyone's bell.
func Migration152_NotificationReads(db *gorm.DB) error {
	log.Println("[Migration 152] Adding per-user notification read state")
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS notification_reads (
			notification_id INTEGER NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
			user_id INTEGER NOT NULL,
			read_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (notification_id, user_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_reads_user_id ON notification_reads(user_id)`,
		`INSERT INTO notification_reads (notification_id, user_id, read_at)
			SELECT n.id, u.id, n.read_at
			FROM notifications n CROSS JOIN users u
			WHERE n.read_at IS NOT NULL AND n.deleted_at IS NULL AND u.deleted_at IS NULL
			ON CONFLICT DO NOTHING`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("[Migration 152] %w", err)
		}
	}
	return nil
}
