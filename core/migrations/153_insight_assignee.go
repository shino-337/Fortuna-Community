package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration153_InsightAssignee adds the person a finding is assigned to. The user id is
// authoritative; the username is kept beside it so lists render without a users join.
func Migration153_InsightAssignee(db *gorm.DB) error {
	log.Println("[Migration 153] Adding finding assignee")
	stmts := []string{
		`ALTER TABLE insights ADD COLUMN IF NOT EXISTS assignee_user_id INTEGER`,
		`ALTER TABLE insights ADD COLUMN IF NOT EXISTS assignee_username VARCHAR(255) NOT NULL DEFAULT ''`,
		`ALTER TABLE insights ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMP WITH TIME ZONE`,
		`CREATE INDEX IF NOT EXISTS idx_insights_assignee_user_id ON insights(assignee_user_id)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("[Migration 153] %w", err)
		}
	}
	return nil
}
