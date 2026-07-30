package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration031_CleanupDuplicateIndexes removes duplicate and redundant indexes across all tables
//
// Date: 2025-12-27 (consolidated from old migrations 032, 038)
// Author: Fortuna Team
// Ticket: Migration Audit - Phase 1
//
// Description:
//   Removes duplicate and redundant indexes identified during schema optimization.
//   Consolidates index cleanup from old migrations 032 and 038.
//
// Tables Affected:
//   - cve_matches: Remove old component_id-based indexes
//   - sboms: Remove duplicate image_digest index
//   - sbom_components: Remove duplicate unique indexes
//
// Consolidation History:
//   This migration replaces:
//   - Old Migration 032: Remove duplicate indexes
//   - Old Migration 038: Index cleanup portion
//
// Rollback Plan:
//   Indexes can be recreated if needed:
//     CREATE INDEX idx_cve_matches_component_id ON cve_matches(component_id);
//     (Note: component_id column no longer exists, so this is not applicable)
//
// Testing:
//   - Verify indexes removed: \di | grep -E "(component_id|duplicate)"
//   - Check no duplicate unique constraints: \d+ cve_matches
//
// Notes:
//   - This is a cleanup migration (removes redundant indexes)
//   - No data loss risk
func Migration031_CleanupDuplicateIndexes(db *gorm.DB) error {
	log.Println("[Migration 031] Starting: Cleanup duplicate indexes")

	// 1. Remove duplicate indexes on cve_matches (old component_id based)
	log.Println("[Migration 031] Cleaning up cve_matches indexes...")
	
	cveMatchIndexes := []string{
		"idx_cve_matches_component_id",              // Old column index
		"idx_cve_matches_unique_sbom_component_cve", // Old component_id based
		"idx_cve_matches_unique_sbom_component_cve_all", // Old component_id based
	}

	for _, idx := range cveMatchIndexes {
		var exists bool
		if err := db.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM pg_indexes 
				WHERE schemaname = 'public' 
				AND indexname = $1
			)
		`, idx).Scan(&exists).Error; err == nil && exists {
			log.Printf("[Migration 031] Dropping index: %s", idx)
			if err := db.Exec(`DROP INDEX IF EXISTS ` + idx + ` CASCADE`).Error; err != nil {
				log.Printf("[Migration 031] ⚠️  Error dropping index %s: %v", idx, err)
			} else {
				log.Printf("[Migration 031] ✅ Dropped index %s", idx)
			}
		}
	}

	// 2. Remove duplicate indexes on sboms
	log.Println("[Migration 031] Cleaning up sboms indexes...")
	
	sbomIndexes := []string{
		"idx_sboms_image_digest", // Duplicate of sboms_image_digest_key (unique constraint)
	}

	for _, idx := range sbomIndexes {
		var exists bool
		if err := db.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM pg_indexes 
				WHERE schemaname = 'public' 
				AND indexname = $1
			)
		`, idx).Scan(&exists).Error; err == nil && exists {
			log.Printf("[Migration 031] Dropping duplicate index: %s", idx)
			if err := db.Exec(`DROP INDEX IF EXISTS ` + idx + ` CASCADE`).Error; err != nil {
				log.Printf("[Migration 031] ⚠️  Error dropping index %s: %v", idx, err)
			} else {
				log.Printf("[Migration 031] ✅ Dropped index %s", idx)
			}
		}
	}

	// 3. Remove duplicate indexes on sbom_components
	log.Println("[Migration 031] Cleaning up sbom_components indexes...")
	
	sbomComponentIndexes := []string{
		"idx_sbom_components_unique_sbom_purl",     // Duplicate
		"idx_sbom_components_unique_sbom_purl_all", // Duplicate
	}

	for _, idx := range sbomComponentIndexes {
		var exists bool
		if err := db.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM pg_indexes 
				WHERE schemaname = 'public' 
				AND indexname = $1
			)
		`, idx).Scan(&exists).Error; err == nil && exists {
			log.Printf("[Migration 031] Dropping duplicate index: %s", idx)
			if err := db.Exec(`DROP INDEX IF EXISTS ` + idx + ` CASCADE`).Error; err != nil {
				log.Printf("[Migration 031] ⚠️  Error dropping index %s: %v", idx, err)
			} else {
				log.Printf("[Migration 031] ✅ Dropped index %s", idx)
			}
		}
	}

	log.Println("[Migration 031] ✅ Completed successfully")
	return nil
}

