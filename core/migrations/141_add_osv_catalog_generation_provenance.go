package migrations

import (
	"fmt"
	"log"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Migration141_AddOSVCatalogGenerationProvenance links OSV mirror rows to catalog generations.
func Migration141_AddOSVCatalogGenerationProvenance(db *gorm.DB) error {
	log.Println("[Migration 141] Adding catalog_generation_id to OSV mirror tables...")

	if db.Migrator().HasTable("osv_vulnerabilities") && !db.Migrator().HasColumn(&models.OSVVulnerability{}, "catalog_generation_id") {
		if err := db.Exec(`ALTER TABLE osv_vulnerabilities ADD COLUMN catalog_generation_id BIGINT NOT NULL DEFAULT 0`).Error; err != nil {
			return fmt.Errorf("[Migration 141] add osv_vulnerabilities.catalog_generation_id: %w", err)
		}
	}
	if db.Migrator().HasTable("osv_packages") && !db.Migrator().HasColumn(&models.OSVPackage{}, "catalog_generation_id") {
		if err := db.Exec(`ALTER TABLE osv_packages ADD COLUMN catalog_generation_id BIGINT NOT NULL DEFAULT 0`).Error; err != nil {
			return fmt.Errorf("[Migration 141] add osv_packages.catalog_generation_id: %w", err)
		}
	}
	if db.Migrator().HasTable("osv_ranges") && !db.Migrator().HasColumn(&models.OSVRange{}, "catalog_generation_id") {
		if err := db.Exec(`ALTER TABLE osv_ranges ADD COLUMN catalog_generation_id BIGINT NOT NULL DEFAULT 0`).Error; err != nil {
			return fmt.Errorf("[Migration 141] add osv_ranges.catalog_generation_id: %w", err)
		}
	}

	if db.Migrator().HasTable("osv_vulnerabilities") {
		if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_osv_vulnerabilities_catalog_generation ON osv_vulnerabilities(catalog_generation_id)`).Error; err != nil {
			return fmt.Errorf("[Migration 141] create osv_vulnerabilities catalog generation index: %w", err)
		}
	}
	if db.Migrator().HasTable("osv_packages") {
		if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_osv_packages_generation_lookup ON osv_packages(catalog_generation_id, ecosystem, package_name)`).Error; err != nil {
			return fmt.Errorf("[Migration 141] create osv_packages generation lookup index: %w", err)
		}
	}
	if db.Migrator().HasTable("osv_ranges") {
		if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_osv_ranges_catalog_generation ON osv_ranges(catalog_generation_id, package_id)`).Error; err != nil {
			return fmt.Errorf("[Migration 141] create osv_ranges catalog generation index: %w", err)
		}
	}

	log.Println("[Migration 141] OSV mirror catalog generation provenance added")
	return nil
}
