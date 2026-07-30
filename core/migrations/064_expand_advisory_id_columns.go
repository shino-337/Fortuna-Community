package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration064_ExpandAdvisoryIDColumns expands advisory ID columns so we can ingest
// full OSV advisories (CVE + GHSA + distro-specific IDs), not only CVE-* IDs.
func Migration064_ExpandAdvisoryIDColumns(db *gorm.DB) error {
	log.Println("Running migration 064: Expand advisory ID columns to varchar(100)")

	statements := []string{
		"ALTER TABLE cves ALTER COLUMN cve_id TYPE VARCHAR(100);",
		"ALTER TABLE package_vulnerabilities ALTER COLUMN cve_id TYPE VARCHAR(100);",
		"ALTER TABLE cve_matches ALTER COLUMN cve_id TYPE VARCHAR(100);",
		"ALTER TABLE insights ALTER COLUMN cve_id TYPE VARCHAR(100);",
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}

	log.Println("Migration 064 completed successfully")
	return nil
}
