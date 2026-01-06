package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration032_MigrateCVEMatchesComplete migrates cve_matches from component_id to package_name and adds missing columns
//
// Date: 2025-12-27 (consolidated from old migrations 037, 039)
// Author: KSAM Team
// Ticket: Migration Audit - Phase 1
//
// Description:
//   Complete migration of cve_matches table from component_id-based schema to
//   package_name-based schema. Adds missing columns and migrates existing data.
//
// Schema Changes:
//   OLD Schema: component_id (FK to sbom_components), matcher, db_version
//   NEW Schema: package_name, package_version, purl, pod_uid, container_name, matched_by
//
// Tables Affected:
//   - cve_matches: Add new columns, migrate data, drop old columns
//
// Consolidation History:
//   This migration replaces:
//   - Old Migration 037: Migrate component_id to package_name
//   - Old Migration 039: Add missing columns
//
// Rollback Plan:
//   WARNING: This migration is destructive (drops component_id column)
//   Rollback requires restore from backup:
//     pg_restore -t cve_matches cve_matches_backup_before_migration032.sql
//
// Testing:
//   - Verify new columns: SELECT package_name, pod_uid FROM cve_matches LIMIT 10;
//   - Check old columns dropped: \d cve_matches
//   - Verify indexes: \di idx_cve_matches_*
//
// Notes:
//   - Data migration copies from sbom_components table
//   - Foreign key to sbom_components is dropped
//   - Performance: Migration may take time on large datasets
func Migration032_MigrateCVEMatchesComplete(db *gorm.DB) error {
	log.Println("[Migration 032] Starting: Complete CVE Matches Migration")

	// Step 1: Add new columns if they don't exist
	log.Println("[Migration 032] Step 1: Adding new columns...")
	
	newColumns := []struct {
		name    string
		sqlType string
	}{
		{"package_name", "VARCHAR(255)"},
		{"package_version", "VARCHAR(100)"},
		{"purl", "VARCHAR(500)"},
		{"p_url", "VARCHAR(500)"}, // Alternative name for purl (used in some code paths)
		{"pod_uid", "VARCHAR(255)"},
		{"container_name", "VARCHAR(255)"},
		{"matched_by", "VARCHAR(255)"},
		{"cvss", "DECIMAL(4,1)"}, // CVSS score (standardized to DECIMAL(4,1))
	}

	for _, col := range newColumns {
		var exists bool
		if err := db.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_schema = 'public'
				AND table_name = 'cve_matches' 
				AND column_name = $1
			)
		`, col.name).Scan(&exists).Error; err != nil {
			log.Printf("[Migration 032] ⚠️  Error checking if cve_matches.%s exists: %v", col.name, err)
		} else if !exists {
			log.Printf("[Migration 032] Adding column: cve_matches.%s", col.name)
			if err := db.Exec(`ALTER TABLE cve_matches ADD COLUMN ` + col.name + ` ` + col.sqlType).Error; err != nil {
				log.Printf("[Migration 032] ⚠️  Error adding cve_matches.%s: %v", col.name, err)
			} else {
				log.Printf("[Migration 032] ✅ Added cve_matches.%s", col.name)
			}
		}
	}

	// Step 2: Migrate data from component_id to package_name (if component_id exists)
	log.Println("[Migration 032] Step 2: Migrating data from component_id to package_name...")
	
	var hasComponentID bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_schema = 'public'
			AND table_name = 'cve_matches' 
			AND column_name = 'component_id'
		)
	`).Scan(&hasComponentID).Error; err == nil && hasComponentID {
		log.Println("[Migration 032] Migrating component_id -> package_name...")
		if err := db.Exec(`
			UPDATE cve_matches cm
			SET package_name = sc.component_name,
			    package_version = sc.component_version,
			    purl = sc.purl
			FROM sbom_components sc
			WHERE cm.component_id = sc.id
			AND cm.package_name IS NULL;
		`).Error; err != nil {
			log.Printf("[Migration 032] ⚠️  Error migrating component_id data: %v", err)
		} else {
			log.Println("[Migration 032] ✅ Migrated component_id data to package_name")
		}
	}

	// Step 3: Create indexes on new columns
	log.Println("[Migration 032] Step 3: Creating indexes on new columns...")
	
	newIndexes := []struct {
		name    string
		columns string
	}{
		{"idx_cve_matches_package_name", "package_name"},
		{"idx_cve_matches_pod_uid", "pod_uid"},
		{"idx_cve_matches_container_name", "container_name"},
		{"idx_cve_matches_package_version", "package_version"},
	}

	for _, idx := range newIndexes {
		var exists bool
		if err := db.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM pg_indexes 
				WHERE schemaname = 'public' 
				AND indexname = $1
			)
		`, idx.name).Scan(&exists).Error; err == nil && !exists {
			log.Printf("[Migration 032] Creating index: %s", idx.name)
			sql := `CREATE INDEX ` + idx.name + ` ON cve_matches(` + idx.columns + `) WHERE deleted_at IS NULL`
			if err := db.Exec(sql).Error; err != nil {
				log.Printf("[Migration 032] ⚠️  Error creating index %s: %v", idx.name, err)
			} else {
				log.Printf("[Migration 032] ✅ Created index %s", idx.name)
			}
		}
	}

	// Step 4: Drop old columns and foreign keys
	log.Println("[Migration 032] Step 4: Dropping old columns...")
	
	oldColumns := []struct {
		name        string
		dropFKFirst bool
		dropIndexFirst bool
	}{
		{"component_id", true, true},
		{"matcher", false, false},
		{"db_version", false, false},
	}

	for _, col := range oldColumns {
		var exists bool
		if err := db.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_schema = 'public'
				AND table_name = 'cve_matches' 
				AND column_name = $1
			)
		`, col.name).Scan(&exists).Error; err == nil && exists {
			log.Printf("[Migration 032] Dropping old column: cve_matches.%s", col.name)
			
			// Drop foreign key constraint first if needed
			if col.dropFKFirst {
				if col.name == "component_id" {
					db.Exec(`ALTER TABLE cve_matches DROP CONSTRAINT IF EXISTS cve_matches_component_id_fkey CASCADE`)
				}
			}
			
			// Drop index first if needed
			if col.dropIndexFirst {
				indexName := "idx_cve_matches_" + col.name
				db.Exec(`DROP INDEX IF EXISTS ` + indexName + ` CASCADE`)
				// Also drop unique indexes that use component_id
				if col.name == "component_id" {
					db.Exec(`DROP INDEX IF EXISTS idx_cve_matches_unique_sbom_component_cve CASCADE`)
					db.Exec(`DROP INDEX IF EXISTS idx_cve_matches_unique_sbom_component_cve_all CASCADE`)
				}
			}
			
			// Drop column
			if err := db.Exec(`ALTER TABLE cve_matches DROP COLUMN IF EXISTS ` + col.name + ` CASCADE`).Error; err != nil {
				log.Printf("[Migration 032] ⚠️  Error dropping cve_matches.%s: %v", col.name, err)
			} else {
				log.Printf("[Migration 032] ✅ Dropped cve_matches.%s", col.name)
			}
		}
	}

	log.Println("[Migration 032] ✅ Completed successfully")
	return nil
}

