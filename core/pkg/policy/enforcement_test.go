package policy

import (
	"context"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupTestDBEnforcement creates an in-memory SQLite database for testing
func setupTestDBEnforcement(t *testing.T) *gorm.DB {
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
			instance_name TEXT NOT NULL,
			template_id TEXT NOT NULL,
			template_version TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 1,
			scope TEXT,
			action TEXT,
			severity TEXT,
			custom_message TEXT,
			auto_remediate BOOLEAN DEFAULT 0,
			cluster_id TEXT,
			UNIQUE(instance_name)
		)
	`)

	db.Exec(`
		CREATE TABLE IF NOT EXISTS policy_violations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			instance_id INTEGER NOT NULL,
			instance_name TEXT NOT NULL,
			template_id TEXT NOT NULL,
			template_name TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			resource_uid TEXT NOT NULL,
			resource_name TEXT,
			namespace TEXT,
			cluster_id TEXT NOT NULL,
			severity TEXT NOT NULL,
			action TEXT NOT NULL,
			status TEXT DEFAULT "active",
			message TEXT,
			enforced_at DATETIME,
			enforcement_result TEXT,
			detected_at DATETIME DEFAULT (datetime('now')),
			resolved_at DATETIME
		)
	`)

	return db
}

func TestEnforcementService_EnforcePolicy_ContextCancellation(t *testing.T) {
	db := setupTestDBEnforcement(t)
	evaluator, _ := NewEvaluator(db)
	service := NewEnforcementService(db, evaluator)

	instance := &models.PolicyInstance{
		ID:              1,
		InstanceName:    "test-instance",
		TemplateID:      "test-template",
		TemplateVersion: "1.0.0",
		Enabled:         true,
		Action:          "block",
	}

	resource := map[string]interface{}{
		"spec": map[string]interface{}{},
	}

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.EnforcePolicy(ctx, instance, resource, "Pod", "uid-123", "test-pod", "default", "cluster-1")

	if err == nil {
		t.Error("EnforcePolicy() should return error on cancelled context")
	}
}

func TestEnforcementService_EnforcePolicy_ContextTimeout(t *testing.T) {
	db := setupTestDBEnforcement(t)
	evaluator, _ := NewEvaluator(db)
	service := NewEnforcementService(db, evaluator)

	instance := &models.PolicyInstance{
		ID:              1,
		InstanceName:    "test-instance",
		TemplateID:      "test-template",
		TemplateVersion: "1.0.0",
		Enabled:         true,
		Action:          "block",
	}

	resource := map[string]interface{}{
		"spec": map[string]interface{}{},
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(2 * time.Nanosecond) // Ensure timeout

	_, err := service.EnforcePolicy(ctx, instance, resource, "Pod", "uid-123", "test-pod", "default", "cluster-1")

	if err == nil {
		t.Error("EnforcePolicy() should return error on timeout")
	}
}

func TestEnforcementService_EnforcePolicy_DisabledInstance(t *testing.T) {
	db := setupTestDBEnforcement(t)
	evaluator, _ := NewEvaluator(db)
	service := NewEnforcementService(db, evaluator)

	instance := &models.PolicyInstance{
		ID:              1,
		InstanceName:    "test-instance",
		TemplateID:      "test-template",
		TemplateVersion: "1.0.0",
		Enabled:         false, // Disabled
		Action:          "block",
	}

	resource := map[string]interface{}{
		"spec": map[string]interface{}{},
	}

	result, err := service.EnforcePolicy(context.Background(), instance, resource, "Pod", "uid-123", "test-pod", "default", "cluster-1")

	if err != nil {
		t.Errorf("EnforcePolicy() error = %v, want nil", err)
	}

	if !result.Allowed {
		t.Error("EnforcePolicy() should allow resource when instance is disabled")
	}

	if result.Action != "skipped" {
		t.Errorf("EnforcePolicy() action = %v, want 'skipped'", result.Action)
	}
}

func TestEnforcementService_FindActiveViolation(t *testing.T) {
	db := setupTestDBEnforcement(t)
	evaluator, _ := NewEvaluator(db)
	service := NewEnforcementService(db, evaluator)

	// Create test violation
	violation := &models.PolicyViolation{
		InstanceID:  1,
		ResourceUID: "uid-123",
		ClusterID:   "cluster-1",
		Status:      "active",
		DetectedAt:  timePtr(time.Now()),
	}
	db.Create(violation)

	// Test finding existing violation
	found, err := service.findActiveViolation(context.Background(), 1, "uid-123", "cluster-1")
	if err != nil {
		t.Errorf("findActiveViolation() error = %v", err)
	}

	if found == nil {
		t.Error("findActiveViolation() should find existing violation")
	}

	if found.ID != violation.ID {
		t.Errorf("findActiveViolation() ID = %v, want %v", found.ID, violation.ID)
	}

	// Test not finding non-existent violation
	notFound, err := service.findActiveViolation(context.Background(), 1, "uid-999", "cluster-1")
	if err != nil {
		t.Errorf("findActiveViolation() error = %v", err)
	}

	if notFound != nil {
		t.Error("findActiveViolation() should not find non-existent violation")
	}
}
