package migrations

import (
	"fmt"
	"log"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Migration156_VulnEcosystemRelease records which distro release an advisory range applies to
// (OSV "Debian:12" → "12") so a Debian 12 image is not matched against Debian 11 ranges.
// Existing rows keep "" and apply to every release until the CVE catalog is reloaded.
func Migration156_VulnEcosystemRelease(db *gorm.DB) error {
	log.Println("[Migration 156] Adding ecosystem_release to package_vulnerabilities and osv_packages...")

	if db.Migrator().HasTable("package_vulnerabilities") && !db.Migrator().HasColumn(&models.PackageVulnerability{}, "ecosystem_release") {
		if err := db.Exec(`ALTER TABLE package_vulnerabilities ADD COLUMN ecosystem_release VARCHAR(64) NOT NULL DEFAULT ''`).Error; err != nil {
			return fmt.Errorf("[Migration 156] add package_vulnerabilities.ecosystem_release: %w", err)
		}
	}
	if db.Migrator().HasTable("osv_packages") && !db.Migrator().HasColumn(&models.OSVPackage{}, "ecosystem_release") {
		if err := db.Exec(`ALTER TABLE osv_packages ADD COLUMN ecosystem_release VARCHAR(64) NOT NULL DEFAULT ''`).Error; err != nil {
			return fmt.Errorf("[Migration 156] add osv_packages.ecosystem_release: %w", err)
		}
	}

	log.Println("[Migration 156] ecosystem_release added")
	return nil
}
