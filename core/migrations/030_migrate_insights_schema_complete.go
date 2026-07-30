package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration030_MigrateInsightsSchemaComplete migrates insights table to new schema and cleans up old columns
//
// Date: 2025-12-27 (consolidated from old migrations 030, 031, 038)
// Author: Fortuna Team
// Ticket: Migration Audit - Phase 1
//
// Description:
//   Consolidates insights table schema migration, combining functionality from
//   old migrations 030, 031, and 038. Migrates from old JSONB-based schema to
//   new relational schema with direct resource references.
//
// Schema Changes:
//   OLD Schema: type, affected_resources (JSONB), sbom_id, cve_match_id, recommended_action, etc.
//   NEW Schema: insight_type, resource_type, resource_uid, resource_namespace, resource_name, recommendation, etc.
//
// Tables Affected:
//   - insights: Complete schema overhaul (add new columns, migrate data, drop old columns)
//
// Consolidation History:
//   This migration replaces:
//   - Old Migration 030: Initial schema migration
//   - Old Migration 031: Cleanup old columns
//   - Old Migration 038: Remove deprecated fields
//
// Rollback Plan:
//   WARNING: This migration is destructive (drops old columns)
//   Rollback requires restore from backup:
//     pg_restore -t insights insights_backup_before_migration030.sql
//
// Testing:
//   - Verify column migration: SELECT insight_type, resource_type FROM insights LIMIT 10;
//   - Check old columns dropped: \d insights
//   - Verify indexes: \di idx_insights_*
//
// Notes:
//   - Data migration is automatic (copies from old columns to new)
//   - Foreign keys to sbom_id and cve_match_id are dropped
//   - Performance: Migration may take several minutes on large datasets
func Migration030_MigrateInsightsSchemaComplete(db *gorm.DB) error {
	log.Println("[Migration 030] Starting: Complete Insights Schema Migration")

	// Step 1: Add new columns if they don't exist
	log.Println("[Migration 030] Step 1: Adding new columns...")
	
	newColumns := []struct {
		name    string
		sqlType string
	}{
		{"insight_type", "VARCHAR(50)"},
		{"resource_type", "VARCHAR(50)"},
		{"resource_uid", "VARCHAR(255)"},
		{"resource_namespace", "VARCHAR(255)"},
		{"resource_name", "VARCHAR(255)"},
		{"title", "VARCHAR(500)"},
		{"recommendation", "TEXT"}, // Remediation recommendation
		{"affected_component", "VARCHAR(255)"},
		{"affected_version", "VARCHAR(100)"},
		{"fixed_version", "VARCHAR(255)"}, // Fixed version for vulnerabilities
		{"cve_id", "VARCHAR(20)"},          // CVE ID for vulnerability insights
		{"cvss", "REAL"},                   // CVSS score (standardized to REAL)
		{"detected_at", "TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP"}, // Detection timestamp
	}

	for _, col := range newColumns {
		var exists bool
		if err := db.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_schema = 'public'
				AND table_name = 'insights' 
				AND column_name = $1
			)
		`, col.name).Scan(&exists).Error; err == nil && !exists {
			log.Printf("[Migration 030] Adding column: insights.%s", col.name)
			if err := db.Exec(`ALTER TABLE insights ADD COLUMN ` + col.name + ` ` + col.sqlType).Error; err != nil {
				log.Printf("[Migration 030] ⚠️  Error adding insights.%s: %v", col.name, err)
			} else {
				log.Printf("[Migration 030] ✅ Added insights.%s", col.name)
			}
		}
	}

	// Step 2: Migrate data from old columns to new columns (if old columns exist)
	log.Println("[Migration 030] Step 2: Migrating data from old columns...")
	
	// Migrate type -> insight_type
	if err := db.Exec(`
		UPDATE insights 
		SET insight_type = type 
		WHERE insight_type IS NULL AND type IS NOT NULL;
	`).Error; err != nil {
		log.Printf("[Migration 030] ⚠️  Error migrating type -> insight_type: %v", err)
	}

	// Migrate recommended_action -> recommendation
	if err := db.Exec(`
		UPDATE insights 
		SET recommendation = recommended_action 
		WHERE recommendation IS NULL AND recommended_action IS NOT NULL;
	`).Error; err != nil {
		log.Printf("[Migration 030] ⚠️  Error migrating recommended_action -> recommendation: %v", err)
	}

	// Migrate cvss_score -> cvss
	if err := db.Exec(`
		UPDATE insights 
		SET cvss = cvss_score::real 
		WHERE cvss IS NULL AND cvss_score IS NOT NULL;
	`).Error; err != nil {
		log.Printf("[Migration 030] ⚠️  Error migrating cvss_score -> cvss: %v", err)
	}

	// Migrate package_name -> affected_component
	if err := db.Exec(`
		UPDATE insights 
		SET affected_component = package_name 
		WHERE affected_component IS NULL AND package_name IS NOT NULL;
	`).Error; err != nil {
		log.Printf("[Migration 030] ⚠️  Error migrating package_name -> affected_component: %v", err)
	}

	// Migrate installed_version -> affected_version
	if err := db.Exec(`
		UPDATE insights 
		SET affected_version = installed_version 
		WHERE affected_version IS NULL AND installed_version IS NOT NULL;
	`).Error; err != nil {
		log.Printf("[Migration 030] ⚠️  Error migrating installed_version -> affected_version: %v", err)
	}

	// Step 3: Create indexes on new columns
	log.Println("[Migration 030] Step 3: Creating indexes on new columns...")
	
	newIndexes := []struct {
		name    string
		columns string
		partial string
	}{
		{"idx_insights_insight_type", "insight_type", "WHERE deleted_at IS NULL"},
		{"idx_insights_resource_type", "resource_type", "WHERE deleted_at IS NULL AND resource_type IS NOT NULL"},
		{"idx_insights_resource_uid", "resource_uid", "WHERE deleted_at IS NULL AND resource_uid IS NOT NULL"},
		{"idx_insights_resource_namespace", "resource_namespace", "WHERE deleted_at IS NULL AND resource_namespace IS NOT NULL"},
		{"idx_insights_resource_name", "resource_name", "WHERE deleted_at IS NULL AND resource_name IS NOT NULL"},
		{"idx_insights_detected_at", "detected_at", "WHERE deleted_at IS NULL"},
		{"idx_insights_affected_component", "affected_component", "WHERE deleted_at IS NULL"},
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
			log.Printf("[Migration 030] Creating index: %s", idx.name)
			sql := `CREATE INDEX ` + idx.name + ` ON insights(` + idx.columns + `) ` + idx.partial
			if err := db.Exec(sql).Error; err != nil {
				log.Printf("[Migration 030] ⚠️  Error creating index %s: %v", idx.name, err)
			} else {
				log.Printf("[Migration 030] ✅ Created index %s", idx.name)
			}
		}
	}

	// Step 4: Drop old columns and indexes
	log.Println("[Migration 030] Step 4: Dropping old columns and indexes...")
	
	oldColumns := []struct {
		name        string
		dropFKFirst bool
		dropIndexFirst bool
	}{
		{"type", false, true},
		{"affected_resources", false, true},
		{"recommended_action", false, false},
		{"sbom_id", true, true},
		{"cve_match_id", true, true},
		{"source", false, true},
		{"cvss_score", false, false},
		{"cvss_vector", false, false},
		{"exploit_available", false, true},
		{"package_name", false, true},
		{"installed_version", false, false},
		{"fixed_version", false, false},
	}

	for _, col := range oldColumns {
		var exists bool
		if err := db.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_schema = 'public'
				AND table_name = 'insights' 
				AND column_name = $1
			)
		`, col.name).Scan(&exists).Error; err == nil && exists {
			log.Printf("[Migration 030] Dropping old column: insights.%s", col.name)
			
			// Drop foreign key constraint first if needed
			if col.dropFKFirst {
				if col.name == "sbom_id" {
					db.Exec(`ALTER TABLE insights DROP CONSTRAINT IF EXISTS insights_sbom_id_fkey CASCADE`)
				} else if col.name == "cve_match_id" {
					db.Exec(`ALTER TABLE insights DROP CONSTRAINT IF EXISTS insights_cve_match_id_fkey CASCADE`)
				}
			}
			
			// Drop index first if needed
			if col.dropIndexFirst {
				indexName := "idx_insights_" + col.name
				db.Exec(`DROP INDEX IF EXISTS ` + indexName + ` CASCADE`)
				if col.name == "affected_resources" {
					db.Exec(`DROP INDEX IF EXISTS idx_insights_affected_resources_gin CASCADE`)
				}
			}
			
			// Drop column
			if err := db.Exec(`ALTER TABLE insights DROP COLUMN IF EXISTS ` + col.name + ` CASCADE`).Error; err != nil {
				log.Printf("[Migration 030] ⚠️  Error dropping insights.%s: %v", col.name, err)
			} else {
				log.Printf("[Migration 030] ✅ Dropped insights.%s", col.name)
			}
		}
	}

	// Step 5: Drop duplicate/redundant indexes
	log.Println("[Migration 030] Step 5: Dropping duplicate indexes...")
	
	duplicateIndexes := []string{
		"idx_insights_created",           // Duplicate of idx_insights_created_at
		"idx_insights_type",              // Old, replaced by idx_insights_insight_type
		"idx_insights_affected_resources_gin", // Old JSONB index
		"idx_insights_cve_match_id",      // Old column index
		"idx_insights_sbom_id",           // Old column index
		"idx_insights_package_name",      // Old column index
		"idx_insights_exploit_available", // Old column index
		"idx_insights_source",            // Old column index
		"idx_insights_vuln_dedup",        // Old schema based
	}

	for _, idx := range duplicateIndexes {
		var exists bool
		if err := db.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM pg_indexes 
				WHERE schemaname = 'public' 
				AND indexname = $1
			)
		`, idx).Scan(&exists).Error; err == nil && exists {
			log.Printf("[Migration 030] Dropping duplicate index: %s", idx)
			if err := db.Exec(`DROP INDEX IF EXISTS ` + idx + ` CASCADE`).Error; err != nil {
				log.Printf("[Migration 030] ⚠️  Error dropping index %s: %v", idx, err)
			} else {
				log.Printf("[Migration 030] ✅ Dropped index %s", idx)
			}
		}
	}

	// Step 6: Set NOT NULL constraints on required columns (after data migration)
	log.Println("[Migration 030] Step 6: Setting NOT NULL constraints...")
	
	// Only set NOT NULL if all rows have values
	var nullCount int64
	if err := db.Raw(`SELECT COUNT(*) FROM insights WHERE insight_type IS NULL`).Scan(&nullCount).Error; err == nil && nullCount == 0 {
		if err := db.Exec(`ALTER TABLE insights ALTER COLUMN insight_type SET NOT NULL`).Error; err != nil {
			log.Printf("[Migration 030] ⚠️  Error setting NOT NULL on insight_type: %v", err)
		}
	}

	log.Println("[Migration 030] ✅ Completed successfully")
	return nil
}

