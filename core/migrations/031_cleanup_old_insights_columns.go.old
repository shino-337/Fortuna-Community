package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration031_CleanupOldInsightsColumns removes deprecated columns from insights table
// These columns were migrated to new schema in Migration 030 and are no longer needed
func Migration031_CleanupOldInsightsColumns(db *gorm.DB) error {
	log.Println("Starting Migration 031: Cleanup Old Insights Columns")

	// Step 1: Verify migration 030 completed (check new columns exist)
	log.Println("Step 1: Verifying Migration 030 completed...")

	var count int64
	if err := db.Raw(`
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_name = 'insights'
		AND column_name IN ('insight_type', 'resource_uid', 'resource_type', 'resource_name');
	`).Scan(&count).Error; err != nil {
		log.Printf("Error checking new columns: %v", err)
		return err
	}

	if count < 4 {
		log.Printf("⚠️  Migration 030 appears incomplete (only %d/4 new columns found). Skipping cleanup.", count)
		return nil
	}
	log.Printf("✅ Migration 030 verified: all new columns exist")

	// Step 2: Drop old columns that were replaced in Migration 030
	log.Println("Step 2: Dropping deprecated columns...")

	// Drop 'type' column (replaced by 'insight_type')
	if err := db.Exec(`
		ALTER TABLE insights
		DROP COLUMN IF EXISTS type;
	`).Error; err != nil {
		log.Printf("Warning: Failed to drop 'type' column: %v", err)
	} else {
		log.Println("  ✅ Dropped column: type")
	}

	// Drop 'recommended_action' column (replaced by 'recommendation')
	if err := db.Exec(`
		ALTER TABLE insights
		DROP COLUMN IF EXISTS recommended_action;
	`).Error; err != nil {
		log.Printf("Warning: Failed to drop 'recommended_action' column: %v", err)
	} else {
		log.Println("  ✅ Dropped column: recommended_action")
	}

	// Drop 'cvss_score' column (replaced by 'cvss')
	if err := db.Exec(`
		ALTER TABLE insights
		DROP COLUMN IF EXISTS cvss_score;
	`).Error; err != nil {
		log.Printf("Warning: Failed to drop 'cvss_score' column: %v", err)
	} else {
		log.Println("  ✅ Dropped column: cvss_score")
	}

	// Drop 'package_name' column (replaced by 'affected_component')
	if err := db.Exec(`
		ALTER TABLE insights
		DROP COLUMN IF EXISTS package_name;
	`).Error; err != nil {
		log.Printf("Warning: Failed to drop 'package_name' column: %v", err)
	} else {
		log.Println("  ✅ Dropped column: package_name")
	}

	// Drop 'installed_version' column (replaced by 'affected_version')
	if err := db.Exec(`
		ALTER TABLE insights
		DROP COLUMN IF EXISTS installed_version;
	`).Error; err != nil {
		log.Printf("Warning: Failed to drop 'installed_version' column: %v", err)
	} else {
		log.Println("  ✅ Dropped column: installed_version")
	}

	// Drop 'affected_resources' JSONB column (replaced by direct fields)
	if err := db.Exec(`
		ALTER TABLE insights
		DROP COLUMN IF EXISTS affected_resources;
	`).Error; err != nil {
		log.Printf("Warning: Failed to drop 'affected_resources' column: %v", err)
	} else {
		log.Println("  ✅ Dropped column: affected_resources")
	}

	// Step 3: Drop old foreign key references (if they exist)
	log.Println("Step 3: Dropping old foreign key constraints...")

	// Drop sbom_id foreign key constraint (if exists)
	if err := db.Exec(`
		ALTER TABLE insights
		DROP CONSTRAINT IF EXISTS fk_insights_sbom;
	`).Error; err != nil {
		log.Printf("Warning: Failed to drop fk_insights_sbom constraint: %v", err)
	} else {
		log.Println("  ✅ Dropped constraint: fk_insights_sbom")
	}

	// Drop cve_match_id foreign key constraint (if exists)
	if err := db.Exec(`
		ALTER TABLE insights
		DROP CONSTRAINT IF EXISTS fk_insights_cve_match;
	`).Error; err != nil {
		log.Printf("Warning: Failed to drop fk_insights_cve_match constraint: %v", err)
	} else {
		log.Println("  ✅ Dropped constraint: fk_insights_cve_match")
	}

	// Drop sbom_id column (no longer needed, insights are linked via resource_uid)
	if err := db.Exec(`
		ALTER TABLE insights
		DROP COLUMN IF EXISTS sbom_id;
	`).Error; err != nil {
		log.Printf("Warning: Failed to drop 'sbom_id' column: %v", err)
	} else {
		log.Println("  ✅ Dropped column: sbom_id")
	}

	// Drop cve_match_id column (no longer needed)
	if err := db.Exec(`
		ALTER TABLE insights
		DROP COLUMN IF EXISTS cve_match_id;
	`).Error; err != nil {
		log.Printf("Warning: Failed to drop 'cve_match_id' column: %v", err)
	} else {
		log.Println("  ✅ Dropped column: cve_match_id")
	}

	log.Println("Migration 031 completed: Old insights columns cleaned up")
	return nil
}
