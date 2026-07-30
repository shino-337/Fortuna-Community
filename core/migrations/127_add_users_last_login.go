package migrations

import (
	"log"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Migration127_AddUsersLastLoginIfMissing aligns users with models.User (LastLogin / login audit).
// Migration 002 SQL omitted last_login; login handler updates this column.
func Migration127_AddUsersLastLoginIfMissing(db *gorm.DB) error {
	log.Println("Running migration 127: Ensure users.last_login exists")
	if !db.Migrator().HasTable("users") {
		log.Println("Migration 127: users table not found, skipping")
		return nil
	}
	if db.Migrator().HasColumn(&models.User{}, "LastLogin") {
		log.Println("Migration 127: last_login already present")
		return nil
	}
	log.Println("Migration 127: adding users.last_login")
	switch db.Dialector.Name() {
	case "sqlite":
		if err := db.Exec("ALTER TABLE users ADD COLUMN last_login datetime").Error; err != nil {
			return err
		}
	default:
		if err := db.Exec("ALTER TABLE users ADD COLUMN last_login TIMESTAMP WITH TIME ZONE").Error; err != nil {
			return err
		}
	}
	log.Println("Migration 127 completed successfully")
	return nil
}
