package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration092_AddSBOMGoVersion adds sboms.go_version for Go toolchain / stdlib CVE matching (model field existed without DDL).
func Migration092_AddSBOMGoVersion(db *gorm.DB) error {
	log.Println("[Migration 092] Add go_version to sboms (Go stdlib / matcher context)")

	if !db.Migrator().HasTable("sboms") {
		log.Println("[Migration 092] sboms table does not exist, skipping")
		return nil
	}

	var n int64
	if err := db.Raw(`
SELECT COUNT(*) FROM information_schema.columns
WHERE table_schema = current_schema()
  AND table_name = 'sboms'
  AND column_name = 'go_version'
`).Scan(&n).Error; err != nil {
		return fmt.Errorf("[Migration 092] check go_version: %w", err)
	}
	if n > 0 {
		log.Println("[Migration 092] go_version column already exists, skipping")
		return nil
	}

	if err := db.Exec(`ALTER TABLE sboms ADD COLUMN go_version VARCHAR(50) NOT NULL DEFAULT ''`).Error; err != nil {
		return fmt.Errorf("[Migration 092] add go_version: %w", err)
	}
	log.Println("[Migration 092] ✅ Added go_version to sboms")
	return nil
}
