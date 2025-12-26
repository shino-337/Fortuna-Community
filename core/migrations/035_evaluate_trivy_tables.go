package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration035_EvaluateTrivyTables evaluates and optionally drops Trivy-related tables
// Since we're using Agent-based SBOM extraction, Trivy tables are no longer needed
func Migration035_EvaluateTrivyTables(db *gorm.DB) error {
	log.Println("[Migration 035] Starting: Evaluate Trivy tables")

	// List of Trivy-related tables that may exist
	trivyTables := []string{
		"trivy_scans",
		"trivy_vulnerabilities",
		"trivy_artifacts",
		"trivy_results",
		"trivy_reports",
	}

	var existingTables []string
	for _, tableName := range trivyTables {
		var exists bool
		if err := db.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables 
				WHERE table_schema = 'public' 
				AND table_name = ?
			)
		`, tableName).Scan(&exists).Error; err != nil {
			log.Printf("[Migration 035] ⚠️  Error checking table %s: %v", tableName, err)
			continue
		}
		if exists {
			existingTables = append(existingTables, tableName)
		}
	}

	if len(existingTables) == 0 {
		log.Println("[Migration 035] ✅ No Trivy tables found - system is using Agent-based SBOM extraction")
		log.Println("[Migration 035] ✅ No action needed")
		return nil
	}

	log.Printf("[Migration 035] Found %d Trivy table(s): %v", len(existingTables), existingTables)

	// Check if tables have data
	for _, tableName := range existingTables {
		var rowCount int64
		if err := db.Raw(`SELECT COUNT(*) FROM ` + tableName).Scan(&rowCount).Error; err != nil {
			log.Printf("[Migration 035] ⚠️  Error counting rows in %s: %v", tableName, err)
			continue
		}
		log.Printf("[Migration 035] Table %s has %d rows", tableName, rowCount)
	}

	// Decision: Keep tables but mark as deprecated
	// Dropping tables could cause issues if there are foreign key references
	// Instead, we'll just log that they're deprecated and not used
	log.Println("[Migration 035] ℹ️  Trivy tables are deprecated but will be kept for now")
	log.Println("[Migration 035] ℹ️  System is using Agent-based SBOM extraction (no Trivy dependency)")
	log.Println("[Migration 035] ℹ️  Trivy tables can be manually dropped if no longer needed")

	// Optional: Add a comment to mark tables as deprecated
	for _, tableName := range existingTables {
		if err := db.Exec(`
			COMMENT ON TABLE ` + tableName + ` IS 'DEPRECATED: This table is no longer used. System now uses Agent-based SBOM extraction.';
		`).Error; err != nil {
			log.Printf("[Migration 035] ⚠️  Error adding comment to %s: %v", tableName, err)
		} else {
			log.Printf("[Migration 035] ✅ Added deprecation comment to %s", tableName)
		}
	}

	log.Println("[Migration 035] ✅ Completed successfully")
	return nil
}

