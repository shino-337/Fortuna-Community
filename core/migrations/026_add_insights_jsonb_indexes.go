package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration026_AddInsightsJSONBIndexes adds GIN indexes for efficient JSONB queries on insights table
// This addresses performance issues with text-based LIKE searches on JSONB fields
func Migration026_AddInsightsJSONBIndexes(db *gorm.DB) error {
	log.Println("========================================")
	log.Println("[Migration 026] Add GIN indexes for insights.affected_resources JSONB")
	log.Println("========================================")

	// Guard: only run if table exists
	var tableExists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'insights')").Scan(&tableExists).Error; err != nil {
		return fmt.Errorf("check insights table exists: %w", err)
	}

	if !tableExists {
		log.Println("[Migration 026] insights table does not exist, skipping")
		return nil
	}

	// Create GIN index for efficient JSONB queries on affected_resources (if column exists)
	// Note: This column was removed in refactoring, so check first
	var columnExists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'insights' 
			AND column_name = 'affected_resources'
		)
	`).Scan(&columnExists).Error; err == nil && columnExists {
		createGINIndexSQL := `
CREATE INDEX IF NOT EXISTS idx_insights_affected_resources_gin 
  ON insights USING GIN (affected_resources)
  WHERE affected_resources IS NOT NULL AND deleted_at IS NULL;
`
		if err := db.Exec(createGINIndexSQL).Error; err != nil {
			log.Printf("Warning: Could not create GIN index on affected_resources: %v", err)
		} else {
			log.Println("[Migration 026] ✅ Created GIN index on insights.affected_resources")
		}
	} else {
		log.Println("[Migration 026] Skipping affected_resources GIN index (column does not exist - removed in refactoring)")
	}

	// Create composite index for vulnerability insights (insight_type + cve_id + status)
	// Note: Column name changed from 'type' to 'insight_type' in refactoring
	// Check which column exists
	var hasType, hasInsightType bool
	db.Raw("SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='insights' AND column_name='type')").Scan(&hasType)
	db.Raw("SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='insights' AND column_name='insight_type')").Scan(&hasInsightType)
	
	if hasInsightType {
		// New schema uses insight_type
		createVulnIndexSQL := `
CREATE INDEX IF NOT EXISTS idx_insights_vuln_dedup 
  ON insights(insight_type, cve_id, status, deleted_at)
  WHERE insight_type = 'VULNERABILITY' AND cve_id IS NOT NULL;
`
		if err := db.Exec(createVulnIndexSQL).Error; err != nil {
			log.Printf("Warning: Could not create composite index for vulnerability insights: %v", err)
		} else {
			log.Println("[Migration 026] ✅ Created composite index for vulnerability insights")
		}
	} else if hasType {
		// Old schema uses type
		createVulnIndexSQL := `
CREATE INDEX IF NOT EXISTS idx_insights_vuln_dedup 
  ON insights(type, sbom_id, cve_id, status, deleted_at)
  WHERE type = 'vulnerability' AND sbom_id IS NOT NULL AND cve_id IS NOT NULL;
`
		if err := db.Exec(createVulnIndexSQL).Error; err != nil {
			log.Printf("Warning: Could not create composite index for vulnerability insights: %v", err)
		} else {
			log.Println("[Migration 026] ✅ Created composite index for vulnerability insights")
		}
	} else {
		log.Println("[Migration 026] Skipping composite index (insights table structure unknown)")
	}

	log.Println("[Migration 026] ✅ Completed")
	return nil
}

