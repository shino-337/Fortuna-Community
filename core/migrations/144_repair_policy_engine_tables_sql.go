package migrations

import (
	"fmt"
	"log"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Migration144_RepairPolicyEngineTablesSQL repairs policy engine tables using
// explicit DDL. Earlier AutoMigrate-based repairs can be marked applied by the
// runner even when PostgreSQL relation checks fail, leaving Policy Rules
// Instances broken after a partial reset.
func Migration144_RepairPolicyEngineTablesSQL(db *gorm.DB) error {
	log.Println("Running migration 144: repair policy engine tables with SQL DDL")

	statements := []string{
		`CREATE TABLE IF NOT EXISTS policy_templates (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ,
			deleted_at TIMESTAMPTZ,
			template_id TEXT NOT NULL,
			version TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			category TEXT NOT NULL CHECK (category IN ('security', 'compliance', 'operational', 'governance')),
			default_severity TEXT NOT NULL,
			cel_expression TEXT NOT NULL,
			cel_program_cache BYTEA,
			default_scope JSONB,
			default_action TEXT DEFAULT 'alert' CHECK (default_action IN ('alert', 'block', 'audit')),
			supports_remediation BOOLEAN DEFAULT FALSE,
			remediation_template JSONB,
			rationale TEXT,
			"references" TEXT[],
			examples JSONB,
			created_by TEXT DEFAULT 'system',
			is_system BOOLEAN DEFAULT TRUE,
			CONSTRAINT idx_template_id_version UNIQUE (template_id, version)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_policy_templates_deleted_at ON policy_templates(deleted_at);`,
		`CREATE INDEX IF NOT EXISTS idx_policy_templates_template_version ON policy_templates(template_id, version);`,
		`ALTER TABLE policy_templates DROP CONSTRAINT IF EXISTS policy_templates_template_id_key;`,
		`DROP INDEX IF EXISTS idx_policy_templates_template_id;`,
		`CREATE TABLE IF NOT EXISTS policy_instances (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ,
			deleted_at TIMESTAMPTZ,
			template_id TEXT NOT NULL,
			template_version TEXT NOT NULL,
			instance_name TEXT NOT NULL UNIQUE,
			description TEXT,
			enabled BOOLEAN DEFAULT TRUE,
			clusters TEXT[],
			namespaces TEXT[],
			resource_types TEXT[],
			label_selectors JSONB,
			action TEXT CHECK (action IN ('alert', 'block', 'audit', 'remediate')),
			severity TEXT CHECK (severity IN ('low', 'medium', 'high', 'critical')),
			custom_message TEXT,
			auto_remediate BOOLEAN DEFAULT FALSE,
			remediation_dry_run BOOLEAN DEFAULT TRUE,
			exemptions JSONB,
			created_by TEXT,
			updated_by TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_policy_instances_deleted_at ON policy_instances(deleted_at);`,
		`CREATE INDEX IF NOT EXISTS idx_policy_instances_template ON policy_instances(template_id, template_version);`,
		`CREATE INDEX IF NOT EXISTS idx_policy_instances_enabled ON policy_instances(enabled);`,
		`CREATE TABLE IF NOT EXISTS policy_violations (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ,
			deleted_at TIMESTAMPTZ,
			instance_id BIGINT NOT NULL,
			instance_name TEXT NOT NULL,
			template_id TEXT NOT NULL,
			template_name TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			resource_uid TEXT NOT NULL,
			resource_name TEXT,
			namespace TEXT,
			cluster_id TEXT NOT NULL,
			severity TEXT NOT NULL CHECK (severity IN ('low', 'medium', 'high', 'critical')),
			action TEXT NOT NULL CHECK (action IN ('alert', 'block', 'audit', 'remediate')),
			status TEXT DEFAULT 'active' CHECK (status IN ('active', 'resolved', 'dismissed')),
			message TEXT,
			enforced_at TIMESTAMPTZ,
			enforcement_result TEXT,
			detected_at TIMESTAMPTZ DEFAULT NOW(),
			resolved_at TIMESTAMPTZ
		);`,
		`CREATE INDEX IF NOT EXISTS idx_policy_violations_deleted_at ON policy_violations(deleted_at);`,
		`CREATE INDEX IF NOT EXISTS idx_policy_violations_instance_id ON policy_violations(instance_id);`,
		`CREATE INDEX IF NOT EXISTS idx_policy_violations_cluster_id ON policy_violations(cluster_id);`,
		`CREATE INDEX IF NOT EXISTS idx_violations_resource ON policy_violations(resource_type, resource_uid);`,
		`CREATE INDEX IF NOT EXISTS idx_policy_violations_detected_at ON policy_violations(detected_at);`,
	}

	for _, stmt := range statements {
		if err := execDDL(db, stmt); err != nil {
			return fmt.Errorf("migration 144 policy engine DDL failed: %w", err)
		}
	}

	var tplCount int64
	if err := db.Model(&models.PolicyTemplate{}).Count(&tplCount).Error; err != nil {
		return fmt.Errorf("migration 144 count policy_templates: %w", err)
	}
	if tplCount == 0 {
		if err := seedBaselinePolicyTemplates(db); err != nil {
			return err
		}
	}

	var instCount int64
	if err := db.Model(&models.PolicyInstance{}).Count(&instCount).Error; err != nil {
		return fmt.Errorf("migration 144 count policy_instances: %w", err)
	}
	if instCount == 0 {
		if err := seedBaselinePolicyInstances(db); err != nil {
			return err
		}
	}

	log.Printf("Migration 144 completed: policy_templates=%d policy_instances=%d", tplCount, instCount)
	return nil
}
