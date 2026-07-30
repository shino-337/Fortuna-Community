package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration112_ExpandAdvisoryIDColumnsV2 widens advisory ID columns across CVE-related
// tables to support long IDs (GHSA/OSV/vendor advisories), not only CVE-* IDs.
func Migration112_ExpandAdvisoryIDColumnsV2(db *gorm.DB) error {
	log.Println("Running migration 112: Expand advisory ID columns to varchar(255)")

	statements := []string{
		"ALTER TABLE cves ALTER COLUMN cve_id TYPE VARCHAR(255);",
		"ALTER TABLE package_vulnerabilities ALTER COLUMN cve_id TYPE VARCHAR(255);",
		"ALTER TABLE cve_matches ALTER COLUMN cve_id TYPE VARCHAR(255);",
		"ALTER TABLE insights ALTER COLUMN cve_id TYPE VARCHAR(255);",
		"ALTER TABLE cve_file_metadata ALTER COLUMN cve_id TYPE VARCHAR(255);",
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}

	log.Println("Migration 112 completed successfully")
	return nil
}
