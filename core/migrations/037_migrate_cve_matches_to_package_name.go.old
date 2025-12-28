package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration037_MigrateCVEMatchesToPackageName migrates cve_matches from component_id to package_name
func Migration037_MigrateCVEMatchesToPackageName(db *gorm.DB) error {
	log.Println("[Migration 037] Starting: Migrate cve_matches to package_name")

	// Check if package_name column exists
	var hasPackageName bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'cve_matches' 
			AND column_name = 'package_name'
		)
	`).Scan(&hasPackageName).Error; err != nil {
		log.Printf("[Migration 037] ⚠️  Error checking package_name column: %v", err)
		return err
	}

	if !hasPackageName {
		log.Println("[Migration 037] Adding package_name column to cve_matches...")
		if err := db.Exec(`
			ALTER TABLE cve_matches 
			ADD COLUMN IF NOT EXISTS package_name VARCHAR(255);
		`).Error; err != nil {
			log.Printf("[Migration 037] ⚠️  Error adding package_name: %v", err)
			return err
		}
		log.Println("[Migration 037] ✅ Added package_name column")
	} else {
		log.Println("[Migration 037] ✅ package_name column already exists")
	}

	// Check if component_id column exists
	var hasComponentID bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'cve_matches' 
			AND column_name = 'component_id'
		)
	`).Scan(&hasComponentID).Error; err != nil {
		log.Printf("[Migration 037] ⚠️  Error checking component_id column: %v", err)
	}

	if hasComponentID {
		log.Println("[Migration 037] Migrating data from component_id to package_name...")
		// Migrate data: get package_name from sbom_components table
		if err := db.Exec(`
			UPDATE cve_matches cm
			SET package_name = sc.component_name
			FROM sbom_components sc
			WHERE cm.component_id = sc.id
			AND cm.package_name IS NULL;
		`).Error; err != nil {
			log.Printf("[Migration 037] ⚠️  Error migrating data: %v", err)
			// Non-fatal, continue
		} else {
			log.Println("[Migration 037] ✅ Migrated data from component_id to package_name")
		}

		// Note: We don't drop component_id column yet as it may still be referenced
		// It can be dropped in a later migration after verifying all code uses package_name
		log.Println("[Migration 037] ℹ️  component_id column kept for now (can be dropped later)")
	} else {
		log.Println("[Migration 037] ✅ component_id column does not exist (already migrated)")
	}

	// Add index on package_name if not exists
	log.Println("[Migration 037] Ensuring index on package_name...")
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_cve_matches_package_name 
		ON cve_matches(package_name) 
		WHERE deleted_at IS NULL;
	`).Error; err != nil {
		log.Printf("[Migration 037] ⚠️  Error creating index: %v", err)
	} else {
		log.Println("[Migration 037] ✅ Created index on package_name")
	}

	log.Println("[Migration 037] ✅ Completed successfully")
	return nil
}

