package migrations

import (
	"fmt"
	"log"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Migration139_AddSBOMMatchRunCatalogGeneration links matcher runs to catalog generations.
func Migration139_AddSBOMMatchRunCatalogGeneration(db *gorm.DB) error {
	log.Println("[Migration 139] Adding catalog_generation_id to sbom_match_runs...")

	if !db.Migrator().HasTable("sbom_match_runs") {
		log.Println("[Migration 139] sbom_match_runs table missing, skipping")
		return nil
	}

	if !db.Migrator().HasColumn(&models.SBOMMatchRun{}, "catalog_generation_id") {
		if err := db.Exec(`ALTER TABLE sbom_match_runs ADD COLUMN catalog_generation_id BIGINT NOT NULL DEFAULT 0`).Error; err != nil {
			return fmt.Errorf("[Migration 139] add sbom_match_runs.catalog_generation_id: %w", err)
		}
	}

	if err := db.Exec(`
CREATE INDEX IF NOT EXISTS idx_sbom_match_runs_catalog_generation
ON sbom_match_runs(catalog_generation_id)
`).Error; err != nil {
		return fmt.Errorf("[Migration 139] create catalog generation index: %w", err)
	}
	if err := db.Exec(`
CREATE INDEX IF NOT EXISTS idx_sbom_match_runs_mirror_generation_status
ON sbom_match_runs(mirror_version, catalog_generation_id, status)
`).Error; err != nil {
		return fmt.Errorf("[Migration 139] create mirror generation status index: %w", err)
	}

	log.Println("[Migration 139] sbom_match_runs catalog_generation_id added")
	return nil
}
