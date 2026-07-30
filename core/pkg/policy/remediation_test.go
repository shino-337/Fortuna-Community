package policy

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
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

func TestRemediationService_ValidateNamespace(t *testing.T) {
	db := setupTestDB(t)
	service := NewRemediationService(db)

	tests := []struct {
		name      string
		namespace string
		wantErr   bool
	}{
		{
			name:      "Block kube-system",
			namespace: "kube-system",
			wantErr:   true,
		},
		{
			name:      "Block kube-public",
			namespace: "kube-public",
			wantErr:   true,
		},
		{
			name:      "Allow default",
			namespace: "default",
			wantErr:   false,
		},
		{
			name:      "Allow custom namespace",
			namespace: "production",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateNamespace(tt.namespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateNamespace() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRemediationService_IsResourceTypeAllowed(t *testing.T) {
	db := setupTestDB(t)
	service := NewRemediationService(db)

	tests := []struct {
		name         string
		resourceType string
		want         bool
	}{
		{
			name:         "Allow Pod",
			resourceType: "Pod",
			want:         true,
		},
		{
			name:         "Allow Deployment",
			resourceType: "Deployment",
			want:         true,
		},
		{
			name:         "Block Service",
			resourceType: "Service",
			want:         false,
		},
		{
			name:         "Block ConfigMap",
			resourceType: "ConfigMap",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.isResourceTypeAllowed(tt.resourceType)
			if got != tt.want {
				t.Errorf("isResourceTypeAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemediationService_SetNestedField_ArrayHandling(t *testing.T) {
	db := setupTestDB(t)
	service := NewRemediationService(db)

	tests := []struct {
		name     string
		resource map[string]interface{}
		path     string
		value    interface{}
		wantErr  bool
		verify   func(map[string]interface{}) bool
	}{
		{
			name: "Set container securityContext",
			resource: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name": "nginx",
						},
					},
				},
			},
			path:    "/spec/containers/0/securityContext/runAsNonRoot",
			value:   true,
			wantErr: false,
			verify: func(res map[string]interface{}) bool {
				spec, ok := res["spec"].(map[string]interface{})
				if !ok {
					return false
				}
				containers, ok := spec["containers"].([]interface{})
				if !ok || len(containers) == 0 {
					return false
				}
				container, ok := containers[0].(map[string]interface{})
				if !ok {
					return false
				}
				sc, ok := container["securityContext"].(map[string]interface{})
				if !ok {
					return false
				}
				runAsNonRoot, ok := sc["runAsNonRoot"].(bool)
				return ok && runAsNonRoot == true
			},
		},
		{
			name: "Set nested field without array",
			resource: map[string]interface{}{
				"spec": map[string]interface{}{},
			},
			path:    "/spec/securityContext/runAsNonRoot",
			value:   true,
			wantErr: false,
			verify: func(res map[string]interface{}) bool {
				spec := res["spec"].(map[string]interface{})
				sc := spec["securityContext"].(map[string]interface{})
				return sc["runAsNonRoot"] == true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.setNestedField(tt.resource, tt.path, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("setNestedField() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.verify != nil {
				if !tt.verify(tt.resource) {
					t.Errorf("setNestedField() verification failed")
				}
			}
		})
	}
}

func TestRemediationService_ValidatePatchFields(t *testing.T) {
	db := setupTestDB(t)
	service := NewRemediationService(db)

	tests := []struct {
		name     string
		template string
		wantErr  bool
	}{
		{
			name: "Allow securityContext",
			template: `{
				"type": "patch",
				"operations": [
					{"op": "add", "path": "/spec/securityContext/runAsNonRoot", "value": true}
				]
			}`,
			wantErr: false,
		},
		{
			name: "Allow container securityContext",
			template: `{
				"type": "patch",
				"operations": [
					{"op": "add", "path": "/spec/containers/0/securityContext/privileged", "value": false}
				]
			}`,
			wantErr: false,
		},
		{
			name: "Block nodeSelector",
			template: `{
				"type": "patch",
				"operations": [
					{"op": "add", "path": "/spec/nodeSelector", "value": {"key": "value"}}
				]
			}`,
			wantErr: true,
		},
		{
			name: "Block volumes",
			template: `{
				"type": "patch",
				"operations": [
					{"op": "add", "path": "/spec/volumes", "value": []}
				]
			}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validatePatchFields(tt.template)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePatchFields() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRemediationService_CaptureResourceState(t *testing.T) {
	db := setupTestDB(t)
	service := NewRemediationService(db)

	resource := map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name": "nginx",
				},
			},
		},
	}

	state := service.captureResourceState(resource)

	// Verify it's a deep copy (check that modifying one doesn't affect the other)
	stateCopy := state
	if stateCopy["spec"] == nil {
		t.Error("captureResourceState() should preserve content")
	}
	// Modify state and verify resource is unchanged
	if spec, ok := state["spec"].(map[string]interface{}); ok {
		spec["test"] = "modified"
	}
	// If it was a shallow copy, resource would be modified too
	if spec, ok := resource["spec"].(map[string]interface{}); ok {
		if spec["test"] != nil {
			t.Error("captureResourceState() should return a deep copy")
		}
	}

	// Verify content is the same
	if state["spec"] == nil {
		t.Error("captureResourceState() should preserve content")
	}
}

func TestRemediationService_ComputeDiff(t *testing.T) {
	db := setupTestDB(t)
	service := NewRemediationService(db)

	before := map[string]interface{}{
		"spec": map[string]interface{}{
			"securityContext": map[string]interface{}{
				"runAsNonRoot": false,
			},
		},
	}

	after := map[string]interface{}{
		"spec": map[string]interface{}{
			"securityContext": map[string]interface{}{
				"runAsNonRoot": true,
			},
		},
	}

	diff := service.computeDiff(before, after)

	if len(diff) == 0 {
		t.Error("computeDiff() should detect changes")
	}

	// Check that diff contains the change
	found := false
	for _, d := range diff {
		if strings.Contains(d, "runAsNonRoot") {
			found = true
			break
		}
	}

	if !found {
		t.Error("computeDiff() should include runAsNonRoot change")
	}
}
