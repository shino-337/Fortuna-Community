package policy

import (
	"context"
	"testing"

	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupTestDBViolation creates an in-memory SQLite database for testing
func setupTestDBViolation(t *testing.T) *gorm.DB {
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
			description TEXT,
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
			exemptions TEXT,
			created_by TEXT,
			updated_by TEXT
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
			status TEXT DEFAULT 'active',
			message TEXT,
			enforced_at DATETIME,
			enforcement_result TEXT,
			detected_at DATETIME,
			resolved_at DATETIME
		)
	`)

	return db
}

func TestViolationService_RecordViolation(t *testing.T) {
	db := setupTestDBViolation(t)
	service := NewViolationService(db)

	// Create template
	template := &models.PolicyTemplate{
		TemplateID:      "test-template",
		Version:         "1.0.0",
		Name:            "Test Template",
		Description:     "Test description",
		DefaultSeverity: "high",
	}
	db.Create(template)

	instance := &models.PolicyInstance{
		ID:              1,
		InstanceName:    "test-instance",
		TemplateID:      "test-template",
		TemplateVersion: "1.0.0",
		Severity:        "critical",
	}

	violation, err := service.RecordViolation(
		context.Background(),
		instance,
		"Pod",
		"uid-123",
		"test-pod",
		"default",
		"cluster-1",
		"block",
	)

	if err != nil {
		t.Errorf("RecordViolation() error = %v", err)
	}

	if violation == nil {
		t.Error("RecordViolation() should return violation")
	}

	if violation.InstanceID != instance.ID {
		t.Errorf("RecordViolation() InstanceID = %v, want %v", violation.InstanceID, instance.ID)
	}

	if violation.Severity != "critical" {
		t.Errorf("RecordViolation() Severity = %v, want 'critical'", violation.Severity)
	}

	if violation.Status != "active" {
		t.Errorf("RecordViolation() Status = %v, want 'active'", violation.Status)
	}
}

func TestViolationService_RecordViolation_ContextCancellation(t *testing.T) {
	db := setupTestDBViolation(t)
	service := NewViolationService(db)

	instance := &models.PolicyInstance{
		ID:              1,
		InstanceName:    "test-instance",
		TemplateID:      "test-template",
		TemplateVersion: "1.0.0",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.RecordViolation(ctx, instance, "Pod", "uid-123", "test-pod", "default", "cluster-1", "block")

	if err == nil {
		t.Error("RecordViolation() should return error on cancelled context")
	}
}
