package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration089_AddSBOMStatusReasonSourceDetail adds:
// - sboms.status_reason
// - sbom_components.source_detail
func Migration089_AddSBOMStatusReasonSourceDetail(db *gorm.DB) error {
	log.Println("[Migration 089] Add sboms.status_reason and sbom_components.source_detail")

	if db.Migrator().HasTable("sboms") {
		var hasStatusReason bool
		if err := db.Raw(`
SELECT EXISTS (
  SELECT 1 FROM information_schema.columns
  WHERE table_schema = current_schema() AND table_name = 'sboms' AND column_name = 'status_reason'
)`).Scan(&hasStatusReason).Error; err != nil {
			return fmt.Errorf("[Migration 089] check sboms.status_reason: %w", err)
		}
		if !hasStatusReason {
			if err := db.Exec(`ALTER TABLE sboms ADD COLUMN status_reason VARCHAR(64) NOT NULL DEFAULT ''`).Error; err != nil {
				return fmt.Errorf("[Migration 089] add sboms.status_reason: %w", err)
			}
		}
	}

	if db.Migrator().HasTable("sbom_components") {
		var hasSourceDetail bool
		if err := db.Raw(`
SELECT EXISTS (
  SELECT 1 FROM information_schema.columns
  WHERE table_schema = current_schema() AND table_name = 'sbom_components' AND column_name = 'source_detail'
)`).Scan(&hasSourceDetail).Error; err != nil {
			return fmt.Errorf("[Migration 089] check sbom_components.source_detail: %w", err)
		}
		if !hasSourceDetail {
			if err := db.Exec(`ALTER TABLE sbom_components ADD COLUMN source_detail VARCHAR(255) NOT NULL DEFAULT ''`).Error; err != nil {
				return fmt.Errorf("[Migration 089] add sbom_components.source_detail: %w", err)
			}
		}
	}

	return nil
}

