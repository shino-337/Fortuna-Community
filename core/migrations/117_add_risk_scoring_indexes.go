package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration117_AddRiskScoringIndexes adds composite indexes that speed up the most
// common risk-scoring query patterns.
//
// Transaction context: RunMigrations invokes each migration outside an explicit
// transaction, so CREATE INDEX CONCURRENTLY is valid here.
//
// Rollback:
//
//	DROP INDEX CONCURRENTLY IF EXISTS idx_insights_resource_type_status;
//	DROP INDEX CONCURRENTLY IF EXISTS idx_cve_matches_sbom_severity;
//	DROP INDEX CONCURRENTLY IF EXISTS idx_insights_active;
func Migration117_AddRiskScoringIndexes(db *gorm.DB) error {
	log.Println("Running migration 117: add risk scoring performance indexes")

	indexes := []struct {
		name  string
		query string
	}{
		{
			name: "idx_insights_resource_type_status",
			query: `
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_insights_resource_type_status
    ON insights(resource_uid, insight_type, status)
    WHERE deleted_at IS NULL;
`,
		},
		{
			name: "idx_cve_matches_sbom_severity",
			query: `
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_cve_matches_sbom_severity
    ON cve_matches(sbom_id, severity)
    WHERE deleted_at IS NULL;
`,
		},
		{
			name: "idx_insights_active",
			query: `
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_insights_active
    ON insights(status)
    WHERE status = 'active' AND deleted_at IS NULL;
`,
		},
	}

	for _, idx := range indexes {
		log.Printf("[Migration 117] Creating index: %s", idx.name)
		if err := execDDL(db, idx.query); err != nil {
			return fmt.Errorf("create index %s: %w", idx.name, err)
		}
		log.Printf("[Migration 117] ✅ Created index: %s", idx.name)
	}

	log.Println("[Migration 117] ✅ Completed - All risk scoring indexes created")
	return nil
}
