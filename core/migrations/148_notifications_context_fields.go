package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration148_NotificationsContextFields adds context needed for header alerts:
// stable dedupe, category, target route, cluster and resource identity.
func Migration148_NotificationsContextFields(db *gorm.DB) error {
	log.Println("[Migration 148] Extending notifications with routing and dedupe context")

	stmts := []string{
		`ALTER TABLE notifications ADD COLUMN IF NOT EXISTS category VARCHAR(64) DEFAULT ''`,
		`ALTER TABLE notifications ADD COLUMN IF NOT EXISTS route VARCHAR(255) DEFAULT ''`,
		`ALTER TABLE notifications ADD COLUMN IF NOT EXISTS dedupe_key VARCHAR(255) DEFAULT ''`,
		`ALTER TABLE notifications ADD COLUMN IF NOT EXISTS cluster_id VARCHAR(255) DEFAULT ''`,
		`ALTER TABLE notifications ADD COLUMN IF NOT EXISTS resource_uid VARCHAR(255) DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_category ON notifications(category)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_cluster_id ON notifications(cluster_id)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_resource_uid ON notifications(resource_uid)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_dedupe_key_unique ON notifications(dedupe_key) WHERE dedupe_key <> '' AND deleted_at IS NULL`,
	}

	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("[Migration 148] %s: %w", stmt, err)
		}
	}
	log.Println("[Migration 148] notifications context fields ready")
	return nil
}
