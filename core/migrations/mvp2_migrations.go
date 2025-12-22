package migrations

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gorm.io/gorm"

	"github.com/ksam/core/pkg/models"
)

// Migration012_AddRiskScores creates the risk_scores table
func Migration012_AddRiskScores(db *gorm.DB) error {
	log.Println("Running migration 012: Add risk_scores table")

	// Check if table already exists
	var exists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'risk_scores')").Scan(&exists).Error; err != nil {
		return fmt.Errorf("failed to check if risk_scores table exists: %w", err)
	}

	if exists {
		log.Println("risk_scores table already exists, skipping migration 012")
		return nil
	}

	// Read SQL from file
	sqlPath := filepath.Join("migrations", "mvp2", "001_risk_scores.sql")
	sqlBytes, err := os.ReadFile(sqlPath)
	if err != nil {
		// Fallback: use AutoMigrate
		log.Println("Migration file not found, using AutoMigrate")
		return db.AutoMigrate(&models.RiskScore{})
	}

	// Execute SQL
	if err := db.Exec(string(sqlBytes)).Error; err != nil {
		log.Printf("Warning: SQL migration had errors: %v. Attempting AutoMigrate fallback.", err)
		return db.AutoMigrate(&models.RiskScore{})
	}

	log.Println("Migration 012 completed: risk_scores table created")
	return nil
}

// Migration013_AddRiskScoresDeletedAt adds deleted_at column to risk_scores table
func Migration013_AddRiskScoresDeletedAt(db *gorm.DB) error {
	log.Println("Running migration 013: Add deleted_at column to risk_scores")

	// Check if table exists
	var tableExists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'risk_scores')").Scan(&tableExists).Error; err != nil {
		return fmt.Errorf("failed to check if risk_scores table exists: %w", err)
	}

	if !tableExists {
		log.Println("risk_scores table does not exist, skipping migration 013")
		return nil
	}

	// Check if column already exists
	var columnExists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'risk_scores' AND column_name = 'deleted_at')").Scan(&columnExists).Error; err != nil {
		return fmt.Errorf("failed to check if deleted_at column exists: %w", err)
	}

	if columnExists {
		log.Println("deleted_at column already exists, skipping migration 013")
		return nil
	}

	// Read SQL from file
	sqlPath := filepath.Join("migrations", "mvp2", "002_add_risk_scores_deleted_at.sql")
	sqlBytes, err := os.ReadFile(sqlPath)
	if err != nil {
		// Fallback: use DO block
		log.Println("Migration file not found, using inline SQL")
		sqlBytes = []byte(`
-- Add deleted_at column for soft delete support
DO $$ 
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'risk_scores' AND column_name = 'deleted_at'
    ) THEN
        ALTER TABLE risk_scores ADD COLUMN deleted_at TIMESTAMP WITH TIME ZONE;
    END IF;
END $$;

-- Add index for soft delete queries (if not exists)
CREATE INDEX IF NOT EXISTS idx_risk_scores_deleted_at ON risk_scores(deleted_at);
`)
	}

	// Execute SQL
	if err := db.Exec(string(sqlBytes)).Error; err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	log.Println("Migration 013 completed: deleted_at column added to risk_scores")
	return nil
}

// Migration014_AddPolicyTemplates creates the policy_templates table
func Migration014_AddPolicyTemplates(db *gorm.DB) error {
	log.Println("Running migration 014: Add policy_templates table")

	// Check if table already exists
	var exists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'policy_templates')").Scan(&exists).Error; err != nil {
		return fmt.Errorf("failed to check if policy_templates table exists: %w", err)
	}

	if exists {
		log.Println("policy_templates table already exists, skipping migration 014")
		return nil
	}

	// Read SQL from file
	sqlPath := filepath.Join("migrations", "mvp2", "003_policy_templates.sql")
	sqlBytes, err := os.ReadFile(sqlPath)
	if err != nil {
		// Fallback: use AutoMigrate
		log.Println("Migration file not found, using AutoMigrate")
		return db.AutoMigrate(&models.PolicyTemplate{})
	}

	// Execute SQL
	if err := db.Exec(string(sqlBytes)).Error; err != nil {
		log.Printf("Warning: SQL migration had errors: %v. Attempting AutoMigrate fallback.", err)
		return db.AutoMigrate(&models.PolicyTemplate{})
	}

	log.Println("Migration 014 completed: policy_templates table created")
	return nil
}

// Migration015_AddPolicyInstances creates the policy_instances table
func Migration015_AddPolicyInstances(db *gorm.DB) error {
	log.Println("Running migration 015: Add policy_instances table")

	// Check if table already exists
	var exists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'policy_instances')").Scan(&exists).Error; err != nil {
		return fmt.Errorf("failed to check if policy_instances table exists: %w", err)
	}

	if exists {
		log.Println("policy_instances table already exists, skipping migration 015")
		return nil
	}

	// Use AutoMigrate to create the table
	if err := db.AutoMigrate(&models.PolicyInstance{}); err != nil {
		return fmt.Errorf("failed to create policy_instances table: %w", err)
	}

	log.Println("Migration 015 completed: policy_instances table created")
	return nil
}

// Migration016_AddPolicyViolations creates the policy_violations table
func Migration016_AddPolicyViolations(db *gorm.DB) error {
	log.Println("Running migration 016: Add policy_violations table")

	// Check if table already exists
	var exists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'policy_violations')").Scan(&exists).Error; err != nil {
		return fmt.Errorf("failed to check if policy_violations table exists: %w", err)
	}

	if exists {
		log.Println("policy_violations table already exists, skipping migration 016")
		return nil
	}

	// Use AutoMigrate to create the table
	if err := db.AutoMigrate(&models.PolicyViolation{}); err != nil {
		return fmt.Errorf("failed to create policy_violations table: %w", err)
	}

	log.Println("Migration 016 completed: policy_violations table created")
	return nil
}

// Migration018_AddRiskScoresV2Columns adds V2 scoring columns to risk_scores table
func Migration018_AddRiskScoresV2Columns(db *gorm.DB) error {
	log.Println("========================================")
	log.Println("Running migration 018: Add V2 scoring columns to risk_scores")
	log.Println("========================================")

	// Check if table exists
	var exists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'risk_scores')").Scan(&exists).Error; err != nil {
		return fmt.Errorf("failed to check if risk_scores table exists: %w", err)
	}

	if !exists {
		log.Println("risk_scores table does not exist, skipping V2 columns migration")
		return nil
	}

	// Read SQL from file
	sqlPath := filepath.Join("migrations", "mvp2", "004_add_risk_scores_v2_columns.sql")
	sqlBytes, err := os.ReadFile(sqlPath)
	if err != nil {
		// Fallback: inline SQL
		log.Println("Migration file not found, using inline SQL")
		sqlBytes = []byte(`
-- Migration 004: Add V2 scoring columns to risk_scores table
-- MVP2 Phase 1.2: Risk Scoring V2 Enhancement
-- Date: 2025-12-11

-- Add V2 scoring columns
ALTER TABLE risk_scores 
ADD COLUMN IF NOT EXISTS exploitability_score DECIMAL(5,2) DEFAULT 0.0,
ADD COLUMN IF NOT EXISTS business_impact_score DECIMAL(5,2) DEFAULT 0.0,
ADD COLUMN IF NOT EXISTS scorer_version VARCHAR(10) DEFAULT 'v1';

-- Add comments
COMMENT ON COLUMN risk_scores.exploitability_score IS 'Exploitability score (0-30) for V2 scoring';
COMMENT ON COLUMN risk_scores.business_impact_score IS 'Business impact score (0-30) for V2 scoring';
COMMENT ON COLUMN risk_scores.scorer_version IS 'Scorer version: v1 (old) or v2 (new)';
`)
	}

	// Execute migration
	if err := db.Exec(string(sqlBytes)).Error; err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	log.Println("Migration 018 completed: V2 scoring columns added to risk_scores")
	return nil
}

// Migration019_AddCVETables creates CVE scanning tables
func Migration019_AddCVETables(db *gorm.DB) error {
	log.Println("Running migration 019: Add CVE scanning tables")

	// Check if cves table already exists
	var exists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'cves')").Scan(&exists).Error; err != nil {
		return fmt.Errorf("failed to check if cves table exists: %w", err)
	}

	if exists {
		log.Println("CVE tables already exist, skipping migration 019")
		return nil
	}

	// SQL file has been removed - use raw SQL directly
	// IMPORTANT: Create tables in order - cves must be created first
	// because package_vulnerabilities has a foreign key to cves
	log.Println("Creating CVE tables using raw SQL (SQL file removed, using inline SQL)...")

	// Create cves table first with correct column name 'cve_references' (not 'references')
	log.Println("Creating cves table first (required for foreign key constraints)...")
	cvesSQL := `CREATE TABLE IF NOT EXISTS cves (
		id SERIAL PRIMARY KEY,
		cve_id VARCHAR(20) UNIQUE NOT NULL,
		cvss_score DECIMAL(3,1),
		cvss_vector TEXT,
		cvss_version VARCHAR(10),
		severity VARCHAR(20) NOT NULL,
		title TEXT,
		description TEXT,
		published_date TIMESTAMP WITH TIME ZONE,
		last_modified_date TIMESTAMP WITH TIME ZONE,
		exploit_available BOOLEAN DEFAULT FALSE,
		exploit_maturity VARCHAR(20),
		exploit_sources TEXT[],
		"cve_references" JSONB,
		cwe_ids TEXT[],
		source VARCHAR(50) NOT NULL DEFAULT 'nvd',
		source_url TEXT,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		deleted_at TIMESTAMP WITH TIME ZONE
	);`

	// Log SQL before execution to verify it's correct
	log.Printf("DEBUG: Executing SQL to create cves table with column 'cve_references'")
	if err := db.Exec(cvesSQL).Error; err != nil {
		return fmt.Errorf("failed to create cves table: %w", err)
	}
	log.Println("✅ cves table created successfully")

	// Now create package_vulnerabilities (depends on cves)
	log.Println("Creating package_vulnerabilities table (depends on cves)...")
	packageVulnSQL := `CREATE TABLE IF NOT EXISTS package_vulnerabilities (
		id BIGSERIAL PRIMARY KEY,
		cve_id VARCHAR(20) NOT NULL,
		package_name VARCHAR(255) NOT NULL,
		package_type VARCHAR(50),
		ecosystem VARCHAR(50),
		affected_range TEXT,
		version_start_including VARCHAR(50),
		version_start_excluding VARCHAR(50),
		version_end_including VARCHAR(50),
		version_end_excluding VARCHAR(50),
		fixed_version VARCHAR(50),
		fixed_in_versions TEXT[],
		vendor VARCHAR(100),
		product VARCHAR(100),
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		deleted_at TIMESTAMP WITH TIME ZONE,
		CONSTRAINT fk_cves_package_vulnerabilities FOREIGN KEY (cve_id) REFERENCES cves(cve_id)
	);`
	if err := db.Exec(packageVulnSQL).Error; err != nil {
		return fmt.Errorf("failed to create package_vulnerabilities table: %w", err)
	}
	log.Println("✅ package_vulnerabilities table created successfully")

	// Create other CVE-related tables using AutoMigrate (these don't have the 'references' keyword issue)
	log.Println("Creating other CVE-related tables...")
	if err := db.AutoMigrate(&models.ImageScanResult{}, &models.PodImageScan{}); err != nil {
		return fmt.Errorf("failed to create CVE-related tables: %w", err)
	}
	log.Println("✅ Other CVE-related tables created successfully")

	// Also update insights table
	log.Println("Updating insights table...")
	if err := db.AutoMigrate(&models.Insight{}); err != nil {
		return fmt.Errorf("failed to update insights table: %w", err)
	}
	log.Println("✅ Insights table updated successfully")

	log.Println("Migration 019 completed: CVE tables created")
	return nil
}

// Migration020_AddSBOMTables creates SBOM-based scanning tables
func Migration020_AddSBOMTables(db *gorm.DB) error {
	// Force compiler to include these strings
	var _ = "MIGRATION_020_NEW_VERSION_2025_12_15"
	var _ = "STARTING_MIGRATION_020_WITH_SCHEMA_FIX"

	log.Println("========================================")
	log.Println("[Migration 020] ====== STARTING MIGRATION 020 ======")
	log.Println("[Migration 020] This is the NEW version with schema fix logic")
	log.Println("[Migration 020] Version: MIGRATION_020_NEW_VERSION_2025_12_15")
	log.Println("[Migration 020] ========================================")
	log.Println("Running migration 020: Add SBOM-based scanning tables")

	// Check if sboms table already exists
	var exists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'sboms')").Scan(&exists).Error; err != nil {
		return fmt.Errorf("failed to check if sboms table exists: %w", err)
	}
	log.Printf("[Migration 020] sboms table exists: %v", exists)

	// If tables already exist, skip this migration
	// Schema fixes are handled by Migration021_FixSBOMSchema
	if exists {
		log.Println("[Migration 020] SBOM tables already exist, skipping migration 020")
		log.Println("[Migration 020] Note: Schema fixes (p_url->purl, insights.source) handled by Migration 021")
		return nil
	}

	// Read SQL from file
	sqlPath := filepath.Join("migrations", "mvp2", "006_add_sbom_tables.sql")
	sqlBytes, err := os.ReadFile(sqlPath)
	if err != nil {
		// Fallback: use AutoMigrate
		log.Println("Migration file not found, using AutoMigrate")
		if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.CVEMatch{}); err != nil {
			return fmt.Errorf("failed to create SBOM tables: %w", err)
		}
		// Also update insights and pod_image_scans
		if err := db.AutoMigrate(&models.Insight{}, &models.PodImageScan{}); err != nil {
			return fmt.Errorf("failed to update tables: %w", err)
		}
		log.Println("Migration 020 completed: SBOM tables created via AutoMigrate")
		return nil
	}

	// Execute SQL
	if err := db.Exec(string(sqlBytes)).Error; err != nil {
		log.Printf("Warning: SQL migration had errors: %v. Attempting AutoMigrate fallback.", err)
		if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.CVEMatch{}); err != nil {
			return fmt.Errorf("failed to create SBOM tables: %w", err)
		}
		if err := db.AutoMigrate(&models.Insight{}, &models.PodImageScan{}); err != nil {
			return fmt.Errorf("failed to update tables: %w", err)
		}
	}

	log.Println("Migration 020 completed: SBOM tables created")
	return nil
}

// Migration021_FixSBOMSchema fixes schema issues for SBOM tables
// This migration ALWAYS runs to fix p_url -> purl and insights.source
func Migration021_FixSBOMSchema(db *gorm.DB) error {
	log.Println("========================================")
	log.Println("[Migration 021] ====== STARTING SCHEMA FIX ======")
	log.Println("[Migration 021] This migration ALWAYS runs to fix schema issues")
	log.Println("[Migration 021] ========================================")

	// Fix sbom_components.p_url -> purl
	log.Println("[Migration 021] Checking sbom_components table for p_url column...")
	var componentsTableExists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'sbom_components')").Scan(&componentsTableExists).Error; err != nil {
		log.Printf("[Migration 021] ⚠️  Failed to check if sbom_components table exists: %v", err)
	} else if componentsTableExists {
		var pUrlExists bool
		var purlExists bool
		if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'sbom_components' AND column_name = 'p_url')").Scan(&pUrlExists).Error; err != nil {
			log.Printf("[Migration 021] ⚠️  Failed to check for p_url column: %v", err)
		} else {
			log.Printf("[Migration 021] p_url column exists: %v", pUrlExists)
		}
		if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'sbom_components' AND column_name = 'purl')").Scan(&purlExists).Error; err != nil {
			log.Printf("[Migration 021] ⚠️  Failed to check for purl column: %v", err)
		} else {
			log.Printf("[Migration 021] purl column exists: %v", purlExists)
		}

		if pUrlExists && !purlExists {
			log.Println("[Migration 021] ⚠️  Found incorrect column 'p_url', renaming to 'purl'...")
			if err := db.Exec("ALTER TABLE sbom_components RENAME COLUMN p_url TO purl").Error; err != nil {
				log.Printf("[Migration 021] ⚠️  Failed to rename p_url to purl: %v", err)
			} else {
				log.Println("[Migration 021] ✅ Successfully renamed p_url to purl")
			}
		} else if !pUrlExists && purlExists {
			log.Println("[Migration 021] ✅ Schema is correct: purl column exists")
		} else if pUrlExists && purlExists {
			log.Println("[Migration 021] ⚠️  Both p_url and purl columns exist, dropping p_url...")
			if err := db.Exec("ALTER TABLE sbom_components DROP COLUMN IF EXISTS p_url").Error; err != nil {
				log.Printf("[Migration 021] ⚠️  Failed to drop p_url column: %v", err)
			} else {
				log.Println("[Migration 021] ✅ Dropped p_url column")
			}
		} else {
			log.Println("[Migration 021] ⚠️  Neither p_url nor purl column exists, adding purl...")
			if err := db.Exec("ALTER TABLE sbom_components ADD COLUMN IF NOT EXISTS purl VARCHAR(512)").Error; err != nil {
				log.Printf("[Migration 021] ⚠️  Failed to add purl column: %v", err)
			} else {
				log.Println("[Migration 021] ✅ Added purl column")
			}
		}
	} else {
		log.Println("[Migration 021] ⚠️  sbom_components table does not exist, skipping p_url fix")
	}

	// Fix insights.source column
	log.Println("[Migration 021] Checking insights table for source column...")
	var insightsSourceExists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'insights' AND column_name = 'source')").Scan(&insightsSourceExists).Error; err != nil {
		log.Printf("[Migration 021] ⚠️  Failed to check for insights.source column: %v", err)
	} else {
		log.Printf("[Migration 021] insights.source column exists: %v", insightsSourceExists)
	}

	if !insightsSourceExists {
		log.Println("[Migration 021] ⚠️  insights.source column missing, adding...")
		if err := db.Exec("ALTER TABLE insights ADD COLUMN IF NOT EXISTS source VARCHAR(50)").Error; err != nil {
			log.Printf("[Migration 021] ⚠️  Failed to add insights.source column: %v", err)
		} else {
			log.Println("[Migration 021] ✅ Added insights.source column")
			// Create index for source column
			if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_insights_source ON insights(source)").Error; err != nil {
				log.Printf("[Migration 021] ⚠️  Failed to create index on insights.source: %v", err)
			} else {
				log.Println("[Migration 021] ✅ Created index on insights.source")
			}
		}
	} else {
		log.Println("[Migration 021] ✅ insights.source column exists")
	}

	log.Println("[Migration 021] ========================================")
	log.Println("[Migration 021] Schema fix completed")
	log.Println("[Migration 021] ========================================")
	return nil
}

// Migration022_AddCVEColumnsToInsights adds CVE-specific columns to insights table
// This migration adds columns needed for CVE/vulnerability insights
func Migration022_AddCVEColumnsToInsights(db *gorm.DB) error {
	log.Println("========================================")
	log.Println("[Migration 022] ====== ADDING CVE COLUMNS TO INSIGHTS ======")
	log.Println("[Migration 022] Adding CVE-specific columns to insights table")
	log.Println("[Migration 022] ========================================")

	// Check if insights table exists
	var insightsTableExists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'insights')").Scan(&insightsTableExists).Error; err != nil {
		return fmt.Errorf("failed to check if insights table exists: %w", err)
	}

	if !insightsTableExists {
		log.Println("[Migration 022] ⚠️  insights table does not exist, skipping migration")
		return nil
	}

	// Define CVE columns to add
	type columnDef struct {
		name       string
		definition string
		indexSQL   string
	}

	columns := []columnDef{
		{
			name:       "cve_id",
			definition: "ALTER TABLE insights ADD COLUMN IF NOT EXISTS cve_id VARCHAR(20)",
			indexSQL:   "CREATE INDEX IF NOT EXISTS idx_insights_cve_id ON insights(cve_id)",
		},
		{
			name:       "cvss_score",
			definition: "ALTER TABLE insights ADD COLUMN IF NOT EXISTS cvss_score DECIMAL(3,1)",
			indexSQL:   "",
		},
		{
			name:       "cvss_vector",
			definition: "ALTER TABLE insights ADD COLUMN IF NOT EXISTS cvss_vector TEXT",
			indexSQL:   "",
		},
		{
			name:       "exploit_available",
			definition: "ALTER TABLE insights ADD COLUMN IF NOT EXISTS exploit_available BOOLEAN DEFAULT false",
			indexSQL:   "CREATE INDEX IF NOT EXISTS idx_insights_exploit_available ON insights(exploit_available)",
		},
		{
			name:       "package_name",
			definition: "ALTER TABLE insights ADD COLUMN IF NOT EXISTS package_name VARCHAR(255)",
			indexSQL:   "CREATE INDEX IF NOT EXISTS idx_insights_package_name ON insights(package_name)",
		},
		{
			name:       "installed_version",
			definition: "ALTER TABLE insights ADD COLUMN IF NOT EXISTS installed_version VARCHAR(50)",
			indexSQL:   "",
		},
		{
			name:       "fixed_version",
			definition: "ALTER TABLE insights ADD COLUMN IF NOT EXISTS fixed_version VARCHAR(50)",
			indexSQL:   "",
		},
	}

	// Add each column if it doesn't exist
	for _, col := range columns {
		var columnExists bool
		if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'insights' AND column_name = ?)", col.name).Scan(&columnExists).Error; err != nil {
			log.Printf("[Migration 022] ⚠️  Failed to check for %s column: %v", col.name, err)
			continue
		}

		if !columnExists {
			log.Printf("[Migration 022] Adding column %s...", col.name)
			if err := db.Exec(col.definition).Error; err != nil {
				log.Printf("[Migration 022] ⚠️  Failed to add %s column: %v", col.name, err)
			} else {
				log.Printf("[Migration 022] ✅ Added %s column", col.name)

				// Create index if specified
				if col.indexSQL != "" {
					if err := db.Exec(col.indexSQL).Error; err != nil {
						log.Printf("[Migration 022] ⚠️  Failed to create index for %s: %v", col.name, err)
					} else {
						log.Printf("[Migration 022] ✅ Created index for %s", col.name)
					}
				}
			}
		} else {
			log.Printf("[Migration 022] ✅ Column %s already exists", col.name)
		}
	}

	log.Println("[Migration 022] ========================================")
	log.Println("[Migration 022] CVE columns migration completed")
	log.Println("[Migration 022] ========================================")
	return nil
}
