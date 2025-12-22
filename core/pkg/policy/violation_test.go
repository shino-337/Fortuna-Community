package policy

import (
	"context"
	"testing"
	"time"

	"github.com/ksam/core/pkg/models"
	"gorm.io/driver/sqlite"
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
		ID:             1,
		InstanceName:   "test-instance",
		TemplateID:     "test-template",
		TemplateVersion: "1.0.0",
		Severity:       "critical",
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
		ID:           1,
		InstanceName: "test-instance",
		TemplateID:   "test-template",
		TemplateVersion: "1.0.0",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.RecordViolation(ctx, instance, "Pod", "uid-123", "test-pod", "default", "cluster-1", "block")

	if err == nil {
		t.Error("RecordViolation() should return error on cancelled context")
	}
}

func TestViolationService_UpdateViolationStatus(t *testing.T) {
	db := setupTestDBViolation(t)
	service := NewViolationService(db)

	// Create violation
	violation := &models.PolicyViolation{
		InstanceID:  1,
		ResourceUID: "uid-123",
		ClusterID:   "cluster-1",
		Status:      "active",
		DetectedAt:  timePtr(time.Now()),
	}
	db.Create(violation)

	err := service.UpdateViolationStatus(context.Background(), violation.ID, "resolved")
	if err != nil {
		t.Errorf("UpdateViolationStatus() error = %v", err)
	}

	// Verify update
	var updated models.PolicyViolation
	db.First(&updated, violation.ID)

	if updated.Status != "resolved" {
		t.Errorf("UpdateViolationStatus() Status = %v, want 'resolved'", updated.Status)
	}
}

func TestViolationService_ResolveViolation(t *testing.T) {
	db := setupTestDBViolation(t)
	service := NewViolationService(db)

	// Create violation
	violation := &models.PolicyViolation{
		InstanceID:  1,
		ResourceUID: "uid-123",
		ClusterID:   "cluster-1",
		Status:      "active",
		DetectedAt:  timePtr(time.Now()),
	}
	db.Create(violation)

	err := service.ResolveViolation(context.Background(), violation.ID)
	if err != nil {
		t.Errorf("ResolveViolation() error = %v", err)
	}

	// Verify resolution
	var resolved models.PolicyViolation
	db.First(&resolved, violation.ID)

	if resolved.Status != "resolved" {
		t.Errorf("ResolveViolation() Status = %v, want 'resolved'", resolved.Status)
	}

	if resolved.ResolvedAt == nil {
		t.Error("ResolveViolation() should set ResolvedAt")
	}
}

func TestViolationService_GetViolations(t *testing.T) {
	db := setupTestDBViolation(t)
	service := NewViolationService(db)

	// Create test violations
	violations := []*models.PolicyViolation{
		{
			InstanceID:  1,
			ResourceUID: "uid-1",
			ClusterID:   "cluster-1",
			Status:      "active",
			DetectedAt:  timePtr(time.Now()),
		},
		{
			InstanceID:  1,
			ResourceUID: "uid-2",
			ClusterID:   "cluster-1",
			Status:      "resolved",
			DetectedAt:  timePtr(time.Now()),
		},
		{
			InstanceID:  2,
			ResourceUID: "uid-3",
			ClusterID:   "cluster-2",
			Status:      "active",
			DetectedAt:  timePtr(time.Now()),
		},
	}

	for _, v := range violations {
		db.Create(v)
	}

	// Test filter by status
	activeViolations, err := service.GetViolations(context.Background(), ViolationFilters{
		Status: "active",
	})
	if err != nil {
		t.Errorf("GetViolations() error = %v", err)
	}

	if len(activeViolations) != 2 {
		t.Errorf("GetViolations() count = %v, want 2", len(activeViolations))
	}

	// Test filter by cluster
	clusterViolations, err := service.GetViolations(context.Background(), ViolationFilters{
		ClusterID: "cluster-1",
	})
	if err != nil {
		t.Errorf("GetViolations() error = %v", err)
	}

	if len(clusterViolations) != 2 {
		t.Errorf("GetViolations() count = %v, want 2", len(clusterViolations))
	}
}

func TestViolationService_UpdateViolationTimestamp(t *testing.T) {
	db := setupTestDBViolation(t)
	service := NewViolationService(db)

	// Create violation
	violation := &models.PolicyViolation{
		InstanceID:  1,
		ResourceUID: "uid-123",
		ClusterID:   "cluster-1",
		Status:      "active",
		DetectedAt:  timePtr(time.Now()),
		UpdatedAt:   time.Now().Add(-1 * time.Hour),
	}
	db.Create(violation)

	oldUpdatedAt := violation.UpdatedAt

	// Wait a bit to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	err := service.UpdateViolationTimestamp(context.Background(), violation.ID)
	if err != nil {
		t.Errorf("UpdateViolationTimestamp() error = %v", err)
	}

	// Verify timestamp updated
	var updated models.PolicyViolation
	db.First(&updated, violation.ID)

	if !updated.UpdatedAt.After(oldUpdatedAt) {
		t.Error("UpdateViolationTimestamp() should update UpdatedAt")
	}
}

