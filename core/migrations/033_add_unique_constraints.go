package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration033_AddUniqueConstraints adds proper unique constraints for data integrity
func Migration033_AddUniqueConstraints(db *gorm.DB) error {
	log.Println("[Migration 033] Starting: Add unique constraints")

	// 1. Ensure sboms has unique constraint on image_digest (if not exists)
	log.Println("[Migration 033] Checking sboms.image_digest unique constraint...")
	var hasUniqueConstraint bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_constraint 
			WHERE conname = 'sboms_image_digest_key'
		)
	`).Scan(&hasUniqueConstraint).Error; err != nil {
		log.Printf("[Migration 033] ⚠️  Error checking constraint: %v", err)
	}

	if !hasUniqueConstraint {
		log.Println("[Migration 033] Creating unique constraint on sboms.image_digest...")
		if err := db.Exec(`
			ALTER TABLE sboms 
			ADD CONSTRAINT sboms_image_digest_key UNIQUE (image_digest);
		`).Error; err != nil {
			log.Printf("[Migration 033] ⚠️  Error creating constraint: %v", err)
		} else {
			log.Println("[Migration 033] ✅ Created unique constraint on sboms.image_digest")
		}
	} else {
		log.Println("[Migration 033] ✅ Unique constraint on sboms.image_digest already exists")
	}

	// 2. Ensure cve_matches has unique constraint on (sbom_id, package_name, cve_id)
	// Check if using package_name (new) or component_id (old)
	log.Println("[Migration 033] Checking cve_matches unique constraint...")
	var hasPackageNameConstraint bool
	var hasComponentIDConstraint bool

	// Check for package_name-based constraint
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes 
			WHERE schemaname = 'public' 
			AND tablename = 'cve_matches' 
			AND indexname LIKE '%package_name%'
			AND indexdef LIKE '%UNIQUE%'
		)
	`).Scan(&hasPackageNameConstraint).Error; err != nil {
		log.Printf("[Migration 033] ⚠️  Error checking package_name constraint: %v", err)
	}

	// Check for component_id-based constraint
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes 
			WHERE schemaname = 'public' 
			AND tablename = 'cve_matches' 
			AND indexname LIKE '%component_id%'
			AND indexdef LIKE '%UNIQUE%'
		)
	`).Scan(&hasComponentIDConstraint).Error; err != nil {
		log.Printf("[Migration 033] ⚠️  Error checking component_id constraint: %v", err)
	}

	// Check if package_name column exists
	var hasPackageNameColumn bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'cve_matches' 
			AND column_name = 'package_name'
		)
	`).Scan(&hasPackageNameColumn).Error; err != nil {
		log.Printf("[Migration 033] ⚠️  Error checking package_name column: %v", err)
	}

	if hasPackageNameColumn && !hasPackageNameConstraint {
		log.Println("[Migration 033] Creating unique constraint on cve_matches(sbom_id, package_name, cve_id)...")
		// Create non-partial unique index for ON CONFLICT inference
		if err := db.Exec(`
			CREATE UNIQUE INDEX IF NOT EXISTS idx_cve_matches_unique_sbom_package_cve_all
			ON cve_matches(sbom_id, package_name, cve_id);
		`).Error; err != nil {
			log.Printf("[Migration 033] ⚠️  Error creating constraint: %v", err)
		} else {
			log.Println("[Migration 033] ✅ Created unique constraint on cve_matches(sbom_id, package_name, cve_id)")
		}
	} else if hasPackageNameConstraint {
		log.Println("[Migration 033] ✅ Unique constraint on cve_matches(sbom_id, package_name, cve_id) already exists")
	}

	// 3. Ensure insights has unique constraint on (resource_uid, cve_id, insight_type)
	log.Println("[Migration 033] Checking insights unique constraint...")
	var hasInsightsConstraint bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes 
			WHERE schemaname = 'public' 
			AND tablename = 'insights' 
			AND indexname LIKE '%unique_resource_cve_type%'
		)
	`).Scan(&hasInsightsConstraint).Error; err != nil {
		log.Printf("[Migration 033] ⚠️  Error checking insights constraint: %v", err)
	}

	if !hasInsightsConstraint {
		log.Println("[Migration 033] Creating unique constraint on insights(resource_uid, cve_id, insight_type)...")
		// Create non-partial unique index for ON CONFLICT inference
		if err := db.Exec(`
			CREATE UNIQUE INDEX IF NOT EXISTS idx_insights_unique_resource_cve_type_all
			ON insights(resource_uid, cve_id, insight_type);
		`).Error; err != nil {
			log.Printf("[Migration 033] ⚠️  Error creating constraint: %v", err)
		} else {
			log.Println("[Migration 033] ✅ Created unique constraint on insights(resource_uid, cve_id, insight_type)")
		}
	} else {
		log.Println("[Migration 033] ✅ Unique constraint on insights(resource_uid, cve_id, insight_type) already exists")
	}

	// 4. Ensure sbom_components has unique constraint on (sbom_id, purl)
	log.Println("[Migration 033] Checking sbom_components unique constraint...")
	var hasComponentsConstraint bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes 
			WHERE schemaname = 'public' 
			AND tablename = 'sbom_components' 
			AND indexname LIKE '%unique_sbom_purl%'
		)
	`).Scan(&hasComponentsConstraint).Error; err != nil {
		log.Printf("[Migration 033] ⚠️  Error checking sbom_components constraint: %v", err)
	}

	if !hasComponentsConstraint {
		log.Println("[Migration 033] Creating unique constraint on sbom_components(sbom_id, purl)...")
		if err := db.Exec(`
			CREATE UNIQUE INDEX IF NOT EXISTS idx_sbom_components_unique_sbom_purl_all
			ON sbom_components(sbom_id, purl);
		`).Error; err != nil {
			log.Printf("[Migration 033] ⚠️  Error creating constraint: %v", err)
		} else {
			log.Println("[Migration 033] ✅ Created unique constraint on sbom_components(sbom_id, purl)")
		}
	} else {
		log.Println("[Migration 033] ✅ Unique constraint on sbom_components(sbom_id, purl) already exists")
	}

	log.Println("[Migration 033] ✅ Completed successfully")
	return nil
}

