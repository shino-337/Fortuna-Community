package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration067_ExpandPackageVulnerabilitiesVersionColumns expands version/fixed/package_type/ecosystem
// columns in package_vulnerabilities so OSV data (e.g. long AlmaLinux version strings) fits.
func Migration067_ExpandPackageVulnerabilitiesVersionColumns(db *gorm.DB) error {
	log.Println("Running migration 067: Expand package_vulnerabilities string columns to varchar(255)")

	statements := []string{
		"ALTER TABLE package_vulnerabilities ALTER COLUMN package_type TYPE VARCHAR(255);",
		"ALTER TABLE package_vulnerabilities ALTER COLUMN ecosystem TYPE VARCHAR(255);",
		"ALTER TABLE package_vulnerabilities ALTER COLUMN version_start_including TYPE VARCHAR(255);",
		"ALTER TABLE package_vulnerabilities ALTER COLUMN version_start_excluding TYPE VARCHAR(255);",
		"ALTER TABLE package_vulnerabilities ALTER COLUMN version_end_including TYPE VARCHAR(255);",
		"ALTER TABLE package_vulnerabilities ALTER COLUMN version_end_excluding TYPE VARCHAR(255);",
		"ALTER TABLE package_vulnerabilities ALTER COLUMN fixed_version TYPE VARCHAR(255);",
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}

	log.Println("Migration 067 completed successfully")
	return nil
}
