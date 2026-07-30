package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration149_NotificationsResourceName stores a human-readable resource name
// next to resource_uid so notification UI does not need to expose opaque IDs.
func Migration149_NotificationsResourceName(db *gorm.DB) error {
	log.Println("[Migration 149] Adding notifications.resource_name")
	stmts := []string{
		`ALTER TABLE notifications ADD COLUMN IF NOT EXISTS resource_name VARCHAR(255) DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_resource_name ON notifications(resource_name)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("[Migration 149] %s: %w", stmt, err)
		}
	}
	return nil
}
