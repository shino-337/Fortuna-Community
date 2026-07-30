package migrations

import (
	"log"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Migration130_UserSessions creates user_sessions for JWT session binding and governance.
func Migration130_UserSessions(db *gorm.DB) error {
	log.Println("Running migration 130: user_sessions")
	if err := db.AutoMigrate(&models.UserSession{}); err != nil {
		return err
	}
	log.Println("Migration 130 completed successfully")
	return nil
}
