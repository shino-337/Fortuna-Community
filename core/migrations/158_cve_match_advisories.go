package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration158_CVEMatchAdvisories adds the advisory list and the severity source to cve_matches.
// From resolver v1.7 a match is one finding per (component, CVE): cve_id holds the CVE (or the
// advisory ID when there is none), advisory_ids every advisory whose range matched, and
// severity_source where the severity came from (vendor, advisory_cvss, nvd, cve_cvss,
// errata_cvss, default).
func Migration158_CVEMatchAdvisories(db *gorm.DB) error {
	log.Println("[Migration 158] Adding advisory_ids and severity_source to cve_matches...")
	if !db.Migrator().HasTable("cve_matches") {
		return nil
	}
	for _, stmt := range []string{
		`ALTER TABLE cve_matches ADD COLUMN IF NOT EXISTS advisory_ids TEXT[]`,
		`ALTER TABLE cve_matches ADD COLUMN IF NOT EXISTS severity_source VARCHAR(30) NOT NULL DEFAULT ''`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("migration 158: %w", err)
		}
	}
	return nil
}
