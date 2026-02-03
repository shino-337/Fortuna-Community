package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration059_AddNodeMetadataColumns adds role, os, runtime columns to nodes table
// for Node Detail page (metadata: role, OS, runtime). Agent can populate on sync when available.
func Migration059_AddNodeMetadataColumns(db *gorm.DB) error {
	log.Println("Running migration 059: Add role, os, runtime to nodes")

	var tableExists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'nodes')").Scan(&tableExists).Error; err != nil {
		return err
	}
	if !tableExists {
		log.Println("[Migration 059] nodes table does not exist, skipping")
		return nil
	}

	for _, col := range []struct{ name, def string }{
		{"role", "VARCHAR(64)"},
		{"os", "VARCHAR(128)"},
		{"runtime", "VARCHAR(128)"},
	} {
		if err := db.Exec("ALTER TABLE nodes ADD COLUMN IF NOT EXISTS " + col.name + " " + col.def).Error; err != nil {
			return err
		}
	}

	log.Println("[Migration 059] ✅ Completed successfully")
	return nil
}
