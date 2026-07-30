package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration145_SeedPolicyEngineBaselineSQL seeds baseline policy templates and
// instances using SQL. It intentionally does not rely on GORM model type
// inference because existing clusters may have policy tables created by older
// SQL migrations.
func Migration145_SeedPolicyEngineBaselineSQL(db *gorm.DB) error {
	log.Println("Running migration 145: seed policy engine baseline with SQL")

	statements := []string{
		`INSERT INTO policy_templates (
			created_at, updated_at, template_id, version, name, description,
			category, default_severity, cel_expression, default_scope,
			default_action, supports_remediation, rationale, created_by, is_system
		) VALUES (
			NOW(), NOW(), 'k8s-no-privileged-container', '1.0.0',
			'Disallow privileged containers',
			'Flags Pod workloads that run any container with privileged=true.',
			'security', 'high',
			'has(object.spec) && has(object.spec.containers) && object.spec.containers.exists(c, has(c.securityContext) && has(c.securityContext.privileged) && c.securityContext.privileged == true)',
			'{"resourceTypes":["Pod"]}',
			'alert', false,
			'Privileged containers weaken isolation boundaries and increase host takeover risk.',
			'system', true
		) ON CONFLICT (template_id, version) DO NOTHING;`,
		`INSERT INTO policy_templates (
			created_at, updated_at, template_id, version, name, description,
			category, default_severity, cel_expression, default_scope,
			default_action, supports_remediation, rationale, created_by, is_system
		) VALUES (
			NOW(), NOW(), 'k8s-no-host-namespace-sharing', '1.0.0',
			'Restrict host namespace sharing',
			'Flags Pod workloads enabling hostNetwork, hostPID, or hostIPC.',
			'security', 'high',
			'has(object.spec) && ((has(object.spec.hostNetwork) && object.spec.hostNetwork == true) || (has(object.spec.hostPID) && object.spec.hostPID == true) || (has(object.spec.hostIPC) && object.spec.hostIPC == true))',
			'{"resourceTypes":["Pod"]}',
			'alert', false,
			'Host namespace sharing broadens lateral movement and process/network visibility on the node.',
			'system', true
		) ON CONFLICT (template_id, version) DO NOTHING;`,
		`INSERT INTO policy_instances (
			created_at, updated_at, template_id, template_version, instance_name,
			description, enabled, action, severity, created_by, updated_by
		) VALUES (
			NOW(), NOW(), 'k8s-no-privileged-container', '1.0.0',
			'baseline-no-privileged-container',
			'Default baseline guard for privileged containers.',
			true, 'alert', 'high', 'system', 'system'
		) ON CONFLICT (instance_name) DO NOTHING;`,
		`INSERT INTO policy_instances (
			created_at, updated_at, template_id, template_version, instance_name,
			description, enabled, action, severity, created_by, updated_by
		) VALUES (
			NOW(), NOW(), 'k8s-no-host-namespace-sharing', '1.0.0',
			'baseline-no-host-namespace-sharing',
			'Default baseline guard for host namespace sharing.',
			true, 'alert', 'high', 'system', 'system'
		) ON CONFLICT (instance_name) DO NOTHING;`,
	}

	for _, stmt := range statements {
		if err := execDDL(db, stmt); err != nil {
			return fmt.Errorf("migration 145 policy baseline seed failed: %w", err)
		}
	}

	log.Println("Migration 145 completed: baseline policy templates and instances ensured")
	return nil
}
