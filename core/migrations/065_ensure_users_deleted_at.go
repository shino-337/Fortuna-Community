package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration065_EnsureUsersDeletedAt guarantees users.deleted_at exists.
// This prevents auth/login queries from failing on legacy schemas.
func Migration065_EnsureUsersDeletedAt(db *gorm.DB) error {
	log.Println("Running migration 065: Ensure users.deleted_at exists")

	// If users table is absent, let other migrations/bootstrap create it.
	if !db.Migrator().HasTable("users") {
		log.Println("Migration 065: users table not found, skipping")
		return nil
	}

	var columnExists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = CURRENT_SCHEMA()
			  AND table_name = 'users'
			  AND column_name = 'deleted_at'
		)
	`).Scan(&columnExists).Error; err != nil {
		return err
	}

	if !columnExists {
		log.Println("Migration 065: adding users.deleted_at column")
		if err := db.Exec("ALTER TABLE users ADD COLUMN deleted_at TIMESTAMP WITH TIME ZONE").Error; err != nil {
			return err
		}
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at)").Error; err != nil {
		return err
	}

	log.Println("Migration 065 completed successfully")
	return nil
}
