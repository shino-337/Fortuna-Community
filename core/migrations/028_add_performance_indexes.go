package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration028_AddPerformanceIndexes adds critical missing indexes for query optimization
func Migration028_AddPerformanceIndexes(db *gorm.DB) error {
	log.Println("========================================")
	log.Println("[Migration 028] Add performance indexes for CVE matching and insights")
	log.Println("========================================")

	indexes := []struct {
		name  string
		query string
	}{
		{
			name: "idx_package_vulnerabilities_ecosystem_package",
			query: `
CREATE INDEX IF NOT EXISTS idx_package_vulnerabilities_ecosystem_package
  ON package_vulnerabilities(ecosystem, package_name)
  WHERE deleted_at IS NULL;
`,
		},
		{
			name: "idx_insights_resource_uid_type_status",
			query: `
CREATE INDEX IF NOT EXISTS idx_insights_resource_uid_type_status
  ON insights(resource_uid, insight_type, status)
  WHERE deleted_at IS NULL;
`,
		},
		{
			name: "idx_insights_resource_uid_cve_status",
			query: `
CREATE INDEX IF NOT EXISTS idx_insights_resource_uid_cve_status
  ON insights(resource_uid, cve_id, status)
  WHERE deleted_at IS NULL AND insight_type = 'vulnerability';
`,
		},
		{
			name: "idx_sbom_components_sbom_id_component_name",
			query: `
CREATE INDEX IF NOT EXISTS idx_sbom_components_sbom_id_component_name
  ON sbom_components(sbom_id, component_name)
  WHERE deleted_at IS NULL;
`,
		},
		{
			name: "idx_cve_matches_sbom_id",
			query: `
CREATE INDEX IF NOT EXISTS idx_cve_matches_sbom_id
  ON cve_matches(sbom_id)
  WHERE deleted_at IS NULL;
`,
		},
		{
			name: "idx_package_vulnerabilities_package_name",
			query: `
CREATE INDEX IF NOT EXISTS idx_package_vulnerabilities_package_name
  ON package_vulnerabilities(package_name)
  WHERE deleted_at IS NULL;
`,
		},
		{
			name: "idx_insights_detected_at",
			query: `
CREATE INDEX IF NOT EXISTS idx_insights_detected_at
  ON insights(detected_at DESC)
  WHERE deleted_at IS NULL;
`,
		},
		{
			name: "idx_sboms_pod_uid_deleted_at",
			query: `
CREATE INDEX IF NOT EXISTS idx_sboms_pod_uid_deleted_at
  ON sboms(pod_uid)
  WHERE deleted_at IS NULL;
`,
		},
	}

	for _, idx := range indexes {
		log.Printf("[Migration 028] Creating index: %s", idx.name)
		if err := db.Exec(idx.query).Error; err != nil {
			return fmt.Errorf("create index %s failed: %w", idx.name, err)
		}
		log.Printf("[Migration 028] ✅ Created index: %s", idx.name)
	}

	log.Println("[Migration 028] ✅ Completed - All performance indexes created")
	return nil
}
