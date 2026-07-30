package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration147_UserPasswordBootstrapState tracks bootstrap/default credentials and first-login password changes.
func Migration147_UserPasswordBootstrapState(db *gorm.DB) error {
	log.Println("Running migration 147: user password bootstrap state")
	if !db.Migrator().HasTable("users") {
		return nil
	}

	if !db.Migrator().HasColumn("users", "must_change_password") {
		if err := execDDL(db, "ALTER TABLE users ADD COLUMN must_change_password BOOLEAN NOT NULL DEFAULT FALSE"); err != nil {
			return err
		}
	}
	if !db.Migrator().HasColumn("users", "bootstrap_credential") {
		if err := execDDL(db, "ALTER TABLE users ADD COLUMN bootstrap_credential BOOLEAN NOT NULL DEFAULT FALSE"); err != nil {
			return err
		}
	}
	if !db.Migrator().HasColumn("users", "password_changed_at") {
		colType := "TIMESTAMP WITH TIME ZONE"
		if db.Dialector.Name() == "sqlite" {
			colType = "datetime"
		}
		if err := execDDL(db, "ALTER TABLE users ADD COLUMN password_changed_at "+colType); err != nil {
			return err
		}
	}
	return nil
}
