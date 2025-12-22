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

	// Create GIN index for efficient JSONB queries on affected_resources
	// This enables fast queries using JSONB operators (@>, ?, ?&, ?|) instead of text LIKE
	createGINIndexSQL := `
CREATE INDEX IF NOT EXISTS idx_insights_affected_resources_gin 
  ON insights USING GIN (affected_resources)
  WHERE affected_resources IS NOT NULL AND deleted_at IS NULL;
`
	if err := db.Exec(createGINIndexSQL).Error; err != nil {
		return fmt.Errorf("create GIN index on insights.affected_resources failed: %w", err)
	}

	log.Println("[Migration 026] ✅ Created GIN index on insights.affected_resources")

	// Create composite index for vulnerability insights (sbom_id + cve_id + status)
	// This optimizes the vulnerability deduplication query
	createVulnIndexSQL := `
CREATE INDEX IF NOT EXISTS idx_insights_vuln_dedup 
  ON insights(type, sbom_id, cve_id, status, deleted_at)
  WHERE type = 'vulnerability' AND sbom_id IS NOT NULL AND cve_id IS NOT NULL;
`
	if err := db.Exec(createVulnIndexSQL).Error; err != nil {
		return fmt.Errorf("create composite index for vulnerability insights failed: %w", err)
	}

	log.Println("[Migration 026] ✅ Created composite index for vulnerability insights")

	log.Println("[Migration 026] ✅ Completed")
	return nil
}

