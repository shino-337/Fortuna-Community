package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration030_MigrateInsightsToNewSchema migrates insights table from OLD schema to NEW schema
// OLD Schema: type, affected_resources (JSONB), sbom_id, cve_match_id
// NEW Schema: insight_type, resource_type, resource_uid, resource_namespace, resource_name (direct fields)
func Migration030_MigrateInsightsToNewSchema(db *gorm.DB) error {
	log.Println("Starting Migration 030: Migrate Insights to New Schema")

	// Step 1: Add new columns
	log.Println("Step 1: Adding new columns...")
	
	// Add insight_type column (rename from type later)
	if err := db.Exec(`
		ALTER TABLE insights 
		ADD COLUMN IF NOT EXISTS insight_type VARCHAR(50);
	`).Error; err != nil {
		log.Printf("Warning: Failed to add insight_type column: %v", err)
	}

	// Add resource fields
	if err := db.Exec(`
		ALTER TABLE insights 
		ADD COLUMN IF NOT EXISTS resource_type VARCHAR(50),
		ADD COLUMN IF NOT EXISTS resource_uid VARCHAR(255),
		ADD COLUMN IF NOT EXISTS resource_namespace VARCHAR(255),
		ADD COLUMN IF NOT EXISTS resource_name VARCHAR(255);
	`).Error; err != nil {
		log.Printf("Warning: Failed to add resource columns: %v", err)
	}

	// Add new fields for CVE insights
	if err := db.Exec(`
		ALTER TABLE insights 
		ADD COLUMN IF NOT EXISTS affected_component VARCHAR(255),
		ADD COLUMN IF NOT EXISTS affected_version VARCHAR(100),
		ADD COLUMN IF NOT EXISTS cvss REAL;
	`).Error; err != nil {
		log.Printf("Warning: Failed to add CVE fields: %v", err)
	}

	// Add recommendation field (rename from recommended_action later)
	if err := db.Exec(`
		ALTER TABLE insights 
		ADD COLUMN IF NOT EXISTS recommendation TEXT;
	`).Error; err != nil {
		log.Printf("Warning: Failed to add recommendation column: %v", err)
	}

	// Add detected_at field
	if err := db.Exec(`
		ALTER TABLE insights 
		ADD COLUMN IF NOT EXISTS detected_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;
	`).Error; err != nil {
		log.Printf("Warning: Failed to add detected_at column: %v", err)
	}

	// Add title field (for new schema)
	if err := db.Exec(`
		ALTER TABLE insights 
		ADD COLUMN IF NOT EXISTS title VARCHAR(500);
	`).Error; err != nil {
		log.Printf("Warning: Failed to add title column: %v", err)
	}

	// Step 2: Migrate data from OLD schema to NEW schema
	log.Println("Step 2: Migrating data from OLD to NEW schema...")

	// Migrate type -> insight_type
	if err := db.Exec(`
		UPDATE insights 
		SET insight_type = type 
		WHERE insight_type IS NULL AND type IS NOT NULL;
	`).Error; err != nil {
		log.Printf("Warning: Failed to migrate type to insight_type: %v", err)
	}

	// Migrate recommended_action -> recommendation
	if err := db.Exec(`
		UPDATE insights 
		SET recommendation = recommended_action 
		WHERE recommendation IS NULL AND recommended_action IS NOT NULL;
	`).Error; err != nil {
		log.Printf("Warning: Failed to migrate recommended_action to recommendation: %v", err)
	}

	// Migrate cvss_score -> cvss
	if err := db.Exec(`
		UPDATE insights 
		SET cvss = cvss_score::REAL 
		WHERE cvss IS NULL AND cvss_score IS NOT NULL;
	`).Error; err != nil {
		log.Printf("Warning: Failed to migrate cvss_score to cvss: %v", err)
	}

	// Migrate package_name -> affected_component
	if err := db.Exec(`
		UPDATE insights 
		SET affected_component = package_name 
		WHERE affected_component IS NULL AND package_name IS NOT NULL;
	`).Error; err != nil {
		log.Printf("Warning: Failed to migrate package_name to affected_component: %v", err)
	}

	// Migrate installed_version -> affected_version
	if err := db.Exec(`
		UPDATE insights 
		SET affected_version = installed_version 
		WHERE affected_version IS NULL AND installed_version IS NOT NULL;
	`).Error; err != nil {
		log.Printf("Warning: Failed to migrate installed_version to affected_version: %v", err)
	}

	// Migrate detected_at from created_at if not set
	if err := db.Exec(`
		UPDATE insights 
		SET detected_at = created_at 
		WHERE detected_at IS NULL AND created_at IS NOT NULL;
	`).Error; err != nil {
		log.Printf("Warning: Failed to set detected_at: %v", err)
	}

	// Generate title from description if not set
	if err := db.Exec(`
		UPDATE insights 
		SET title = LEFT(description, 500)
		WHERE title IS NULL AND description IS NOT NULL;
	`).Error; err != nil {
		log.Printf("Warning: Failed to set title: %v", err)
	}

	// Step 3: Parse affected_resources JSONB and extract resource fields
	log.Println("Step 3: Parsing affected_resources JSONB...")

	// Extract resource information from JSONB
	// Format: [{"uid": "...", "name": "...", "type": "Pod", "namespace": "...", ...}]
	if err := db.Exec(`
		UPDATE insights 
		SET 
			resource_uid = COALESCE(
				NULLIF(resource_uid, ''),
				(affected_resources->0->>'uid')
			),
			resource_name = COALESCE(
				NULLIF(resource_name, ''),
				(affected_resources->0->>'name')
			),
			resource_type = COALESCE(
				NULLIF(resource_type, ''),
				(affected_resources->0->>'type')
			),
			resource_namespace = COALESCE(
				NULLIF(resource_namespace, ''),
				(affected_resources->0->>'namespace')
			)
		WHERE affected_resources IS NOT NULL 
		AND jsonb_array_length(affected_resources) > 0
		AND (resource_uid IS NULL OR resource_uid = '' OR resource_name IS NULL OR resource_name = '' OR resource_type IS NULL OR resource_type = '');
	`).Error; err != nil {
		log.Printf("Warning: Failed to parse affected_resources JSONB: %v", err)
	}

	// Step 4: Add indexes for new columns
	log.Println("Step 4: Adding indexes for new columns...")

	// Index for insight_type
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_insights_insight_type 
		ON insights(insight_type) 
		WHERE deleted_at IS NULL;
	`).Error; err != nil {
		log.Printf("Warning: Failed to create index on insight_type: %v", err)
	}

	// Index for resource_uid
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_insights_resource_uid 
		ON insights(resource_uid) 
		WHERE deleted_at IS NULL AND resource_uid IS NOT NULL;
	`).Error; err != nil {
		log.Printf("Warning: Failed to create index on resource_uid: %v", err)
	}

	// Index for resource_type
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_insights_resource_type 
		ON insights(resource_type) 
		WHERE deleted_at IS NULL AND resource_type IS NOT NULL;
	`).Error; err != nil {
		log.Printf("Warning: Failed to create index on resource_type: %v", err)
	}

	// Index for resource_namespace
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_insights_resource_namespace 
		ON insights(resource_namespace) 
		WHERE deleted_at IS NULL AND resource_namespace IS NOT NULL;
	`).Error; err != nil {
		log.Printf("Warning: Failed to create index on resource_namespace: %v", err)
	}

	// Index for resource_name
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_insights_resource_name 
		ON insights(resource_name) 
		WHERE deleted_at IS NULL AND resource_name IS NOT NULL;
	`).Error; err != nil {
		log.Printf("Warning: Failed to create index on resource_name: %v", err)
	}

	// Index for detected_at
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_insights_detected_at 
		ON insights(detected_at) 
		WHERE deleted_at IS NULL;
	`).Error; err != nil {
		log.Printf("Warning: Failed to create index on detected_at: %v", err)
	}

	// Step 5: Set default values for NULL fields (before NOT NULL constraints)
	log.Println("Step 5: Setting default values for NULL fields...")

	// Set default for resource_type (use 'Unknown' if not set)
	if err := db.Exec(`
		UPDATE insights 
		SET resource_type = 'Unknown'
		WHERE resource_type IS NULL OR resource_type = '';
	`).Error; err != nil {
		log.Printf("Warning: Failed to set default resource_type: %v", err)
	}

	// Set default for resource_name (use 'Unknown' if not set)
	if err := db.Exec(`
		UPDATE insights 
		SET resource_name = 'Unknown'
		WHERE resource_name IS NULL OR resource_name = '';
	`).Error; err != nil {
		log.Printf("Warning: Failed to set default resource_name: %v", err)
	}

	// Set default for resource_uid (use 'unknown-' || id if not set)
	if err := db.Exec(`
		UPDATE insights 
		SET resource_uid = 'unknown-' || id::text
		WHERE resource_uid IS NULL OR resource_uid = '';
	`).Error; err != nil {
		log.Printf("Warning: Failed to set default resource_uid: %v", err)
	}

	// Set default for insight_type (use 'unknown' if not set)
	if err := db.Exec(`
		UPDATE insights 
		SET insight_type = 'unknown'
		WHERE insight_type IS NULL OR insight_type = '';
	`).Error; err != nil {
		log.Printf("Warning: Failed to set default insight_type: %v", err)
	}

	// Step 6: Make new columns NOT NULL where appropriate (after data migration)
	log.Println("Step 6: Setting NOT NULL constraints...")

	// Set NOT NULL for insight_type (after migration)
	if err := db.Exec(`
		ALTER TABLE insights 
		ALTER COLUMN insight_type SET NOT NULL;
	`).Error; err != nil {
		log.Printf("Warning: Failed to set NOT NULL on insight_type (may have NULL values): %v", err)
	}

	// Set NOT NULL for resource_type (after migration)
	if err := db.Exec(`
		ALTER TABLE insights 
		ALTER COLUMN resource_type SET NOT NULL;
	`).Error; err != nil {
		log.Printf("Warning: Failed to set NOT NULL on resource_type (may have NULL values): %v", err)
	}

	// Set NOT NULL for resource_name (after migration)
	if err := db.Exec(`
		ALTER TABLE insights 
		ALTER COLUMN resource_name SET NOT NULL;
	`).Error; err != nil {
		log.Printf("Warning: Failed to set NOT NULL on resource_name (may have NULL values): %v", err)
	}

	// Set NOT NULL for resource_uid (after migration)
	if err := db.Exec(`
		ALTER TABLE insights 
		ALTER COLUMN resource_uid SET NOT NULL;
	`).Error; err != nil {
		log.Printf("Warning: Failed to set NOT NULL on resource_uid (may have NULL values): %v", err)
	}

	// Set NOT NULL for detected_at (after migration)
	if err := db.Exec(`
		ALTER TABLE insights 
		ALTER COLUMN detected_at SET NOT NULL;
	`).Error; err != nil {
		log.Printf("Warning: Failed to set NOT NULL on detected_at (may have NULL values): %v", err)
	}

	// Set NOT NULL for title (after migration)
	if err := db.Exec(`
		ALTER TABLE insights 
		ALTER COLUMN title SET NOT NULL;
	`).Error; err != nil {
		log.Printf("Warning: Failed to set NOT NULL on title (may have NULL values): %v", err)
	}

	log.Println("Migration 030 completed: Insights migrated to new schema")
	return nil
}

