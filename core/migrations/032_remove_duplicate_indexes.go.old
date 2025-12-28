package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration032_RemoveDuplicateIndexes removes duplicate and redundant indexes
func Migration032_RemoveDuplicateIndexes(db *gorm.DB) error {
	log.Println("[Migration 032] Starting: Remove duplicate indexes")

	// 1. Remove duplicate indexes on cve_matches
	// Keep the non-partial index (needed for ON CONFLICT), drop the partial one
	// But first check if they use component_id (old) or package_name (new)
	var hasComponentIDIndex bool
	var hasPackageNameIndex bool
	
	// Check for component_id index
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes 
			WHERE schemaname = 'public' 
			AND tablename = 'cve_matches' 
			AND indexname = 'idx_cve_matches_unique_sbom_component_cve'
		)
	`).Scan(&hasComponentIDIndex).Error; err != nil {
		log.Printf("[Migration 032] ⚠️  Error checking component_id index: %v", err)
	}
	
	// Check for package_name index
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes 
			WHERE schemaname = 'public' 
			AND tablename = 'cve_matches' 
			AND indexname LIKE '%package_name%'
		)
	`).Scan(&hasPackageNameIndex).Error; err != nil {
		log.Printf("[Migration 032] ⚠️  Error checking package_name index: %v", err)
	}

	// Drop old component_id-based indexes if they exist
	if hasComponentIDIndex {
		log.Println("[Migration 032] Dropping old component_id-based unique index...")
		if err := db.Exec(`
			DROP INDEX IF EXISTS idx_cve_matches_unique_sbom_component_cve;
		`).Error; err != nil {
			log.Printf("[Migration 032] ⚠️  Error dropping component_id index: %v", err)
		} else {
			log.Println("[Migration 032] ✅ Dropped idx_cve_matches_unique_sbom_component_cve")
		}
	}

	// 2. Remove duplicate created_at indexes on insights
	log.Println("[Migration 032] Removing duplicate created_at indexes on insights...")
	if err := db.Exec(`
		DROP INDEX IF EXISTS idx_insights_created;
	`).Error; err != nil {
		log.Printf("[Migration 032] ⚠️  Error dropping idx_insights_created: %v", err)
	} else {
		log.Println("[Migration 032] ✅ Dropped idx_insights_created (keeping idx_insights_created_at)")
	}

	// 3. Remove old type index if insight_type index exists
	log.Println("[Migration 032] Checking for old type index on insights...")
	var hasTypeIndex bool
	var hasInsightTypeIndex bool
	
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes 
			WHERE schemaname = 'public' 
			AND tablename = 'insights' 
			AND indexname = 'idx_insights_type'
		)
	`).Scan(&hasTypeIndex).Error; err == nil && hasTypeIndex {
		if err := db.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM pg_indexes 
				WHERE schemaname = 'public' 
				AND tablename = 'insights' 
				AND indexname = 'idx_insights_insight_type'
			)
		`).Scan(&hasInsightTypeIndex).Error; err == nil && hasInsightTypeIndex {
			// Both exist, drop the old one
			if err := db.Exec(`
				DROP INDEX IF EXISTS idx_insights_type;
			`).Error; err != nil {
				log.Printf("[Migration 032] ⚠️  Error dropping idx_insights_type: %v", err)
			} else {
				log.Println("[Migration 032] ✅ Dropped idx_insights_type (keeping idx_insights_insight_type)")
			}
		}
	}

	// 4. Remove duplicate image_digest indexes on sboms
	// Keep the unique constraint, drop the regular index
	log.Println("[Migration 032] Removing duplicate image_digest index on sboms...")
	if err := db.Exec(`
		DROP INDEX IF EXISTS idx_sboms_image_digest;
	`).Error; err != nil {
		log.Printf("[Migration 032] ⚠️  Error dropping idx_sboms_image_digest: %v", err)
	} else {
		log.Println("[Migration 032] ✅ Dropped idx_sboms_image_digest (keeping sboms_image_digest_key unique constraint)")
	}

	// 5. Remove old component_id index on cve_matches if package_name is being used
	if hasPackageNameIndex {
		log.Println("[Migration 032] Removing old component_id index on cve_matches...")
		if err := db.Exec(`
			DROP INDEX IF EXISTS idx_cve_matches_component_id;
		`).Error; err != nil {
			log.Printf("[Migration 032] ⚠️  Error dropping idx_cve_matches_component_id: %v", err)
		} else {
			log.Println("[Migration 032] ✅ Dropped idx_cve_matches_component_id")
		}
	}

	log.Println("[Migration 032] ✅ Completed successfully")
	return nil
}

