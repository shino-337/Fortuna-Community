package migrations

import (
	"log"
	"strings"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Migration065_EnsureUsersDeletedAt guarantees users.deleted_at exists.
// This prevents auth/login queries from failing on legacy schemas.
// Uses GORM Migrator (not information_schema) so SQLite and Postgres both work.
func Migration065_EnsureUsersDeletedAt(db *gorm.DB) error {
	log.Println("Running migration 065: Ensure users.deleted_at exists")

	// If users table is absent, let other migrations/bootstrap create it.
	if !db.Migrator().HasTable("users") {
		log.Println("Migration 065: users table not found, skipping")
		return nil
	}

	if !db.Migrator().HasColumn(&models.User{}, "DeletedAt") {
		log.Println("Migration 065: adding users.deleted_at column")
		switch db.Dialector.Name() {
		case "sqlite":
			if err := db.Exec("ALTER TABLE users ADD COLUMN deleted_at datetime").Error; err != nil {
				le := strings.ToLower(err.Error())
				if strings.Contains(le, "duplicate") || strings.Contains(le, "already exists") {
					break
				}
				return err
			}
		default:
			if err := db.Exec("ALTER TABLE users ADD COLUMN deleted_at TIMESTAMP WITH TIME ZONE").Error; err != nil {
				le := strings.ToLower(err.Error())
				if strings.Contains(le, "duplicate") || strings.Contains(le, "already exists") {
					break
				}
				return err
			}
		}
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at)").Error; err != nil {
		return err
	}

	log.Println("Migration 065 completed successfully")
	return nil
}
