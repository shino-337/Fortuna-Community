package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration027_AddCVEFileMetadata creates table for tracking CVE file metadata
// This enables incremental updates by tracking file modification times
func Migration027_AddCVEFileMetadata(db *gorm.DB) error {
	log.Println("========================================")
	log.Println("[Migration 027] Add CVE file metadata tracking table")
	log.Println("========================================")

	// Create cve_file_metadata table
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS cve_file_metadata (
		id SERIAL PRIMARY KEY,
		file_path VARCHAR(500) UNIQUE NOT NULL,
		cve_id VARCHAR(255) NOT NULL,
		file_size BIGINT NOT NULL,
		file_mtime TIMESTAMPTZ NOT NULL,
		file_hash VARCHAR(64),
		last_processed_at TIMESTAMPTZ NOT NULL,
		processing_status VARCHAR(20) DEFAULT 'success',
		error_message TEXT,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		updated_at TIMESTAMPTZ DEFAULT NOW()
	);
	`

	if err := db.Exec(createTableSQL).Error; err != nil {
		return fmt.Errorf("failed to create cve_file_metadata table: %w", err)
	}

	log.Println("[Migration 027] Created table: cve_file_metadata")

	// Create indexes for efficient queries
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_cve_file_metadata_cve_id ON cve_file_metadata(cve_id)",
		"CREATE INDEX IF NOT EXISTS idx_cve_file_metadata_mtime ON cve_file_metadata(file_mtime)",
		"CREATE INDEX IF NOT EXISTS idx_cve_file_metadata_status ON cve_file_metadata(processing_status)",
		"CREATE INDEX IF NOT EXISTS idx_cve_file_metadata_last_processed ON cve_file_metadata(last_processed_at DESC)",
	}

	for _, indexSQL := range indexes {
		if err := db.Exec(indexSQL).Error; err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	log.Println("[Migration 027] Created indexes on cve_file_metadata")

	// Add additional indexes on cves table for better query performance
	additionalIndexes := []string{
		// Composite index for ecosystem + package queries
		`CREATE INDEX IF NOT EXISTS idx_pkg_vuln_ecosystem_package
		 ON package_vulnerabilities(ecosystem, package_name, deleted_at)
		 WHERE deleted_at IS NULL`,

		// Index for incremental updates (find recently modified CVEs)
		`CREATE INDEX IF NOT EXISTS idx_cves_last_modified_desc
		 ON cves(last_modified_date DESC NULLS LAST)
		 WHERE deleted_at IS NULL`,

		// BRIN index for time-series queries (much smaller than B-tree)
		`CREATE INDEX IF NOT EXISTS idx_cves_created_brin
		 ON cves USING BRIN (created_at) WITH (pages_per_range = 128)`,

		// Partial index for critical CVEs (fast critical vulnerability queries)
		`CREATE INDEX IF NOT EXISTS idx_cves_critical
		 ON cves(cve_id, cvss_score)
		 WHERE severity = 'CRITICAL' AND deleted_at IS NULL`,

		// Partial index for high severity CVEs
		`CREATE INDEX IF NOT EXISTS idx_cves_high
		 ON cves(cve_id, cvss_score)
		 WHERE severity = 'HIGH' AND deleted_at IS NULL`,
	}

	for _, indexSQL := range additionalIndexes {
		if err := db.Exec(indexSQL).Error; err != nil {
			// Log but don't fail if index already exists
			log.Printf("[Migration 027] Warning: Could not create additional index: %v", err)
		}
	}

	log.Println("[Migration 027] Created additional performance indexes")

	// Add table comment
	commentSQL := `
	COMMENT ON TABLE cve_file_metadata IS
	'Tracks CVE JSON file metadata for incremental updates.
	 Stores file modification times and processing status to avoid re-processing unchanged files.';
	`

	db.Exec(commentSQL)

	log.Println("[Migration 027] ✅ Completed")
	return nil
}
