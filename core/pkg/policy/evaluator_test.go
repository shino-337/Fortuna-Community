package policy

import (
	"context"
	"testing"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDBEvaluator creates an in-memory SQLite database for testing
func setupTestDBEvaluator(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create tables manually without CHECK constraints for SQLite
	db.Exec(`
		CREATE TABLE IF NOT EXISTS policy_templates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			template_id TEXT NOT NULL,
			version TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			category TEXT,
			default_severity TEXT,
			cel_expression TEXT NOT NULL,
			cel_program_cache BLOB,
			default_scope TEXT,
			default_action TEXT,
			supports_remediation BOOLEAN DEFAULT 0,
			remediation_template TEXT,
			rationale TEXT,
			"references" TEXT,
			examples TEXT,
			created_by TEXT,
			is_system BOOLEAN DEFAULT 0,
			UNIQUE(template_id, version)
		)
	`)
	
	db.Exec(`
		CREATE TABLE IF NOT EXISTS policy_instances (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			template_id TEXT NOT NULL,
			template_version TEXT NOT NULL,
			instance_name TEXT NOT NULL UNIQUE,
			enabled BOOLEAN DEFAULT 1,
			clusters TEXT,
			namespaces TEXT,
			resource_types TEXT,
			label_selectors TEXT,
			action TEXT,
			severity TEXT,
			custom_message TEXT,
			auto_remediate BOOLEAN DEFAULT 0,
			remediation_dry_run BOOLEAN DEFAULT 0,
			exemptions TEXT
		)
	`)

	return db
}

func TestEvaluator_EvaluateInstance(t *testing.T) {
	db := setupTestDBEvaluator(t)
	evaluator, err := NewEvaluator(db)
	if err != nil {
		t.Fatalf("NewEvaluator() error = %v", err)
	}

	// Create template
	template := &models.PolicyTemplate{
		TemplateID:      "test-template",
		Version:         "1.0.0",
		Name:            "Test Template",
		Category:        "security", // Required field
		CELExpression:   "resource.securityContext.runAsNonRoot == true",
		DefaultAction:   "block",
		DefaultSeverity: "high",
	}
	db.Create(template)

	// Create instance
	instance := &models.PolicyInstance{
		ID:             1,
		InstanceName:   "test-instance",
		TemplateID:     "test-template",
		TemplateVersion: "1.0.0",
		Enabled:        true,
		Clusters:       []string{"*"},
		Namespaces:     []string{"default"},
		ResourceTypes:  []string{"Pod"},
	}

	// Reload evaluator to pick up template
	evaluator.ReloadTemplates()
	evaluator.ReloadInstances()

	// Test compliant resource
	compliantResource := &Resource{
		Type:      "Pod",
		UID:       "uid-1",
		Name:      "test-pod",
		Namespace: "default",
		ClusterID: "cluster-1",
		Spec: map[string]interface{}{
			"securityContext": map[string]interface{}{
				"runAsNonRoot": true,
			},
		},
		Labels: map[string]string{},
	}

	violates, err := evaluator.EvaluateInstance(context.Background(), instance, compliantResource)
	if err != nil {
		t.Errorf("EvaluateInstance() error = %v", err)
	}

	if violates {
		t.Error("EvaluateInstance() should not violate for compliant resource")
	}

	// Test non-compliant resource
	nonCompliantResource := &Resource{
		Type:      "Pod",
		UID:       "uid-2",
		Name:      "test-pod-2",
		Namespace: "default",
		ClusterID: "cluster-1",
		Spec: map[string]interface{}{
			"securityContext": map[string]interface{}{
				"runAsNonRoot": false,
			},
		},
		Labels: map[string]string{},
	}

	violates, err = evaluator.EvaluateInstance(context.Background(), instance, nonCompliantResource)
	if err != nil {
		t.Errorf("EvaluateInstance() error = %v", err)
	}

	if !violates {
		t.Error("EvaluateInstance() should violate for non-compliant resource")
	}
}

func TestEvaluator_EvaluateInstance_DisabledInstance(t *testing.T) {
	db := setupTestDBEvaluator(t)
	evaluator, err := NewEvaluator(db)
	if err != nil {
		t.Fatalf("NewEvaluator() error = %v", err)
	}

	instance := &models.PolicyInstance{
		ID:           1,
		InstanceName: "test-instance",
		TemplateID:   "test-template",
		TemplateVersion: "1.0.0",
		Enabled:     false, // Disabled
	}

	resource := &Resource{
		Type:      "Pod",
		UID:       "uid-1",
		Name:      "test-pod",
		Namespace: "default",
		ClusterID: "cluster-1",
		Spec:      map[string]interface{}{},
		Labels:    map[string]string{},
	}

	violates, err := evaluator.EvaluateInstance(context.Background(), instance, resource)
	if err != nil {
		t.Errorf("EvaluateInstance() error = %v", err)
	}

	if violates {
		t.Error("EvaluateInstance() should not violate for disabled instance")
	}
}

func TestEvaluator_EvaluateInstance_ContextCancellation(t *testing.T) {
	db := setupTestDBEvaluator(t)
	evaluator, err := NewEvaluator(db)
	if err != nil {
		t.Fatalf("NewEvaluator() error = %v", err)
	}

	instance := &models.PolicyInstance{
		ID:           1,
		InstanceName: "test-instance",
		TemplateID:   "test-template",
		TemplateVersion: "1.0.0",
		Enabled:     true,
	}

	resource := &Resource{
		Type:      "Pod",
		UID:       "uid-1",
		Name:      "test-pod",
		Namespace: "default",
		ClusterID: "cluster-1",
		Spec:      map[string]interface{}{},
		Labels:    map[string]string{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = evaluator.EvaluateInstance(ctx, instance, resource)

	if err == nil {
		t.Error("EvaluateInstance() should return error on cancelled context")
	}
}

