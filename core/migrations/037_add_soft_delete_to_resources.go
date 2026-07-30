package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration037_AddSoftDeleteToResources adds deleted_at columns to resource tables
// to match GORM models and avoid missing column errors.
func Migration037_AddSoftDeleteToResources(db *gorm.DB) error {
	log.Println("Running migration 037: Add soft delete columns to resource tables")

	tables := []string{
		"pods",
		"service_accounts",
		"roles",
		"role_bindings",
		"cluster_roles",
		"cluster_role_bindings",
	}

	for _, table := range tables {
		if err := db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE`).Error; err != nil {
			return err
		}
		if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_` + table + `_deleted_at ON ` + table + `(deleted_at)`).Error; err != nil {
			return err
		}
	}

	log.Println("Migration 037 completed successfully")
	return nil
}
