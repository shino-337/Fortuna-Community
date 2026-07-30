package migrations

import (
	"fmt"
	"log"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Migration140_AddCVECatalogGenerationProvenance links CVE catalog rows to load generations.
func Migration140_AddCVECatalogGenerationProvenance(db *gorm.DB) error {
	log.Println("[Migration 140] Adding catalog_generation_id to CVE catalog tables...")

	if db.Migrator().HasTable("cves") && !db.Migrator().HasColumn(&models.CVE{}, "catalog_generation_id") {
		if err := db.Exec(`ALTER TABLE cves ADD COLUMN catalog_generation_id BIGINT NOT NULL DEFAULT 0`).Error; err != nil {
			return fmt.Errorf("[Migration 140] add cves.catalog_generation_id: %w", err)
		}
	}
	if db.Migrator().HasTable("package_vulnerabilities") && !db.Migrator().HasColumn(&models.PackageVulnerability{}, "catalog_generation_id") {
		if err := db.Exec(`ALTER TABLE package_vulnerabilities ADD COLUMN catalog_generation_id BIGINT NOT NULL DEFAULT 0`).Error; err != nil {
			return fmt.Errorf("[Migration 140] add package_vulnerabilities.catalog_generation_id: %w", err)
		}
	}
	if db.Migrator().HasTable("cves") {
		if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_cves_catalog_generation ON cves(catalog_generation_id)`).Error; err != nil {
			return fmt.Errorf("[Migration 140] create cves catalog generation index: %w", err)
		}
	}
	if db.Migrator().HasTable("package_vulnerabilities") {
		if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_package_vulnerabilities_catalog_generation ON package_vulnerabilities(catalog_generation_id)`).Error; err != nil {
			return fmt.Errorf("[Migration 140] create package_vulnerabilities catalog generation index: %w", err)
		}
		if err := db.Exec(`
CREATE INDEX IF NOT EXISTS idx_package_vulnerabilities_generation_lookup
ON package_vulnerabilities(catalog_generation_id, ecosystem, package_name)
WHERE deleted_at IS NULL
`).Error; err != nil {
			return fmt.Errorf("[Migration 140] create package vulnerability generation lookup index: %w", err)
		}
	}

	log.Println("[Migration 140] CVE catalog generation provenance added")
	return nil
}
