package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration029_AddInsightsUniqueConstraint adds the unique constraint required for batch UPSERT
// The insights batch upsert uses: ON CONFLICT (resource_uid, cve_id, insight_type) WHERE deleted_at IS NULL
func Migration029_AddInsightsUniqueConstraint(db *gorm.DB) error {
	log.Println("========================================")
	log.Println("[Migration 029] Add unique constraint for insights batch UPSERT")
	log.Println("========================================")

	// Step 1: Remove any duplicate insights first
	log.Println("[Migration 029] Step 1: Removing duplicate insights...")
	if err := db.Exec(`
DELETE FROM insights a
USING insights b
WHERE a.id > b.id
  AND a.resource_uid = b.resource_uid
  AND a.cve_id = b.cve_id
  AND a.insight_type = b.insight_type
  AND a.deleted_at IS NULL
  AND b.deleted_at IS NULL;
`).Error; err != nil {
		return fmt.Errorf("dedup insights for unique constraint: %w", err)
	}

	var deletedCount int64
	db.Raw(`
SELECT COUNT(*) FROM (
	SELECT resource_uid, cve_id, insight_type, COUNT(*) as cnt
	FROM insights
	WHERE deleted_at IS NULL
	GROUP BY resource_uid, cve_id, insight_type
	HAVING COUNT(*) > 1
) AS dups
`).Scan(&deletedCount)
	log.Printf("[Migration 029] Removed %d duplicate insights\n", deletedCount)

	// Step 2: Create partial unique index (for soft deletes)
	log.Println("[Migration 029] Step 2: Creating partial unique index...")
	if err := db.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS idx_insights_unique_resource_cve_type
  ON insights(resource_uid, cve_id, insight_type)
  WHERE deleted_at IS NULL;
`).Error; err != nil {
		return fmt.Errorf("create partial unique index insights(resource_uid, cve_id, insight_type): %w", err)
	}
	log.Println("[Migration 029] ✅ Created partial unique index: idx_insights_unique_resource_cve_type")

	// Step 3: Create non-partial unique index (for ON CONFLICT inference)
	// PostgreSQL cannot infer partial indexes in ON CONFLICT unless the WHERE clause matches
	log.Println("[Migration 029] Step 3: Creating non-partial unique index for UPSERT...")
	if err := db.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS idx_insights_unique_resource_cve_type_all
  ON insights(resource_uid, cve_id, insight_type);
`).Error; err != nil {
		return fmt.Errorf("create non-partial unique index insights(resource_uid, cve_id, insight_type): %w", err)
	}
	log.Println("[Migration 029] ✅ Created non-partial unique index: idx_insights_unique_resource_cve_type_all")

	log.Println("[Migration 029] ✅ Completed successfully")
	return nil
}
