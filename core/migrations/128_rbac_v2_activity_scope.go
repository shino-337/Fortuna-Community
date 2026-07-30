package migrations

import (
	"log"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Migration128_RBACV2ActivityAndUserScope adds security_activity_logs and users.scope_json (RBAC v2).
func Migration128_RBACV2ActivityAndUserScope(db *gorm.DB) error {
	log.Println("Running migration 128: RBAC v2 — security_activity_logs + users.scope_json")
	if err := db.AutoMigrate(&models.SecurityActivityLog{}); err != nil {
		return err
	}
	if !db.Migrator().HasTable(&models.User{}) {
		log.Println("Migration 128: users table missing, skipping user column")
		return nil
	}
	if db.Migrator().HasColumn(&models.User{}, "ScopeJSON") {
		log.Println("Migration 128: users.scope_json already present")
		return nil
	}
	switch db.Dialector.Name() {
	case "sqlite":
		if err := db.Exec("ALTER TABLE users ADD COLUMN scope_json TEXT DEFAULT '{}'").Error; err != nil {
			return err
		}
	default:
		if err := db.Exec("ALTER TABLE users ADD COLUMN scope_json JSONB DEFAULT '{}'").Error; err != nil {
			return err
		}
	}
	log.Println("Migration 128 completed successfully")
	return nil
}
