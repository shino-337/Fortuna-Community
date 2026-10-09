package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration160_DropLegacyVulnTables drops the vulnerability tables the versioned catalog
// (migration 157) replaced: cves, package_vulnerabilities and the OSV mirror (osv_vulnerabilities,
// osv_packages, osv_ranges). Nothing has written them since the loader moved to the versioned
// catalog, and the matcher stopped reading them in the release after. Their content is reloaded
// from OSV, never converted.
func Migration160_DropLegacyVulnTables(db *gorm.DB) error {
	log.Println("[Migration 160] Dropping the legacy vulnerability tables...")
	// Children first: package_vulnerabilities references cves, osv_ranges osv_packages.
	for _, table := range []string{"package_vulnerabilities", "cves", "osv_ranges", "osv_packages", "osv_vulnerabilities"} {
		if err := db.Exec(`DROP TABLE IF EXISTS ` + table).Error; err != nil {
			return fmt.Errorf("[Migration 160] drop %s: %w", table, err)
		}
	}
	log.Println("[Migration 160] Legacy vulnerability tables dropped")
	return nil
}
