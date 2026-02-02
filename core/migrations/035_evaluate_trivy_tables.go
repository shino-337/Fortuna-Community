package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration035_EvaluateTrivyTables evaluates and optionally drops Trivy-related tables
// Since we're using Agent-based SBOM extraction, Trivy tables are no longer needed
func Migration035_EvaluateTrivyTables(db *gorm.DB) error {
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
		// Silently skip if no Trivy tables exist (system never used Trivy)
		// No need to log - this is expected for Agent-based deployments
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

	// Decision: Drop Trivy tables since they're no longer used
	// System now uses Agent-based SBOM extraction, Trivy tables are obsolete
	log.Printf("[Migration 035] Found %d deprecated Trivy table(s), dropping them...", len(existingTables))
	
	for _, tableName := range existingTables {
		// Check for foreign key constraints before dropping
		var hasFK bool
		if err := db.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.table_constraints 
				WHERE table_name = ? 
				AND constraint_type = 'FOREIGN KEY'
			)
		`, tableName).Scan(&hasFK).Error; err != nil {
			log.Printf("[Migration 035] ⚠️  Error checking constraints for %s: %v", tableName, err)
			continue
		}
		
		if hasFK {
			log.Printf("[Migration 035] ⚠️  Table %s has foreign key constraints, skipping drop (manual cleanup required)", tableName)
			// Add deprecation comment instead
			if err := db.Exec(`COMMENT ON TABLE ` + tableName + ` IS 'DEPRECATED: This table is no longer used. System now uses Agent-based SBOM extraction.'`).Error; err != nil {
				log.Printf("[Migration 035] ⚠️  Error adding comment to %s: %v", tableName, err)
			}
		} else {
			// Safe to drop - no foreign key constraints
			if err := db.Exec(`DROP TABLE IF EXISTS ` + tableName + ` CASCADE`).Error; err != nil {
				log.Printf("[Migration 035] ⚠️  Error dropping table %s: %v", tableName, err)
			} else {
				log.Printf("[Migration 035] ✅ Dropped deprecated table: %s", tableName)
			}
		}
	}

	log.Println("[Migration 035] ✅ Completed successfully")
	return nil
}

