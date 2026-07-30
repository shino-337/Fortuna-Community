package policy

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newTestPolicyWorkerDB(t *testing.T) *gorm.DB {
	t.Helper()
	// Same manual DDL as setupTestDBViolation (AutoMigrate breaks on SQLite for policy models).
	db := setupTestDBViolation(t)
	require.NoError(t, db.Exec(`
		CREATE TABLE IF NOT EXISTS insights (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			resource_type TEXT NOT NULL,
			resource_namespace TEXT,
			resource_name TEXT NOT NULL,
			resource_uid TEXT NOT NULL,
			insight_type TEXT NOT NULL,
			severity TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL,
			recommendation TEXT,
			match_confidence TEXT NOT NULL DEFAULT '',
			component_confidence TEXT NOT NULL DEFAULT '',
			sbom_confidence TEXT NOT NULL DEFAULT '',
			final_risk_confidence TEXT NOT NULL DEFAULT '',
			degraded BOOLEAN NOT NULL DEFAULT 0,
			cve_id TEXT,
			affected_component TEXT,
			affected_version TEXT,
			fixed_version TEXT,
			cvss REAL,
			evidence TEXT,
			violated_rules TEXT,
			risk_explanation TEXT,
			remediation TEXT,
			status TEXT,
			sensitivity TEXT NOT NULL DEFAULT 'internal',
			detected_at DATETIME NOT NULL,
			resolved_at DATETIME
		)
	`).Error)
	return db
}

func TestPolicyWorker_ProcessViolationEvent_CreatesBaselineInsights(t *testing.T) {
	db := newTestPolicyWorkerDB(t)
	worker := NewPolicyWorker(db, nil)

	// Seed template + instance so ViolationService.RecordViolation can resolve severity/message.
	tpl := models.PolicyTemplate{
		TemplateID:          "tpl-1",
		Version:             "v1",
		Name:                "ExampleTemplate",
		Description:         "template description",
		Category:            "security",
		DefaultSeverity:     "high",
		CELExpression:       "true",
		DefaultScope:        "{}",
		DefaultAction:       "block",
		SupportsRemediation: false,
	}
	require.NoError(t, db.Create(&tpl).Error)

	inst := models.PolicyInstance{
		TemplateID:      tpl.TemplateID,
		TemplateVersion: tpl.Version,
		InstanceName:    "inst-1",
		Enabled:         true,
		Action:          "block",
		Severity:        "high",
		// Keep empty collections for scope filters.
		Clusters:      models.StringArray{},
		Namespaces:    models.StringArray{},
		ResourceTypes: models.StringArray{},
		LabelSelectors: "{}",
		Exemptions:    "[]",
	}
	require.NoError(t, db.Create(&inst).Error)

	ev := ViolationEvent{
		Type:      "policy.violation.detected",
		Timestamp: time.Now().Unix(),
		Violations: []*Violation{
			{
				InstanceID:   inst.ID,
				InstanceName: inst.InstanceName,
				TemplateID:   tpl.TemplateID,
				TemplateName: tpl.Name,
				ResourceType: "Pod",
				ResourceUID:  "pod-uid-1",
				ResourceName: "pod-1",
				Namespace:    "default",
				ClusterID:    "cluster-1",
				Severity:     "high",
				Action:       "block",
				Message:      "instance violation message",
			},
		},
		Resource: map[string]interface{}{
			"type":      "Pod",
			"name":      "pod-1",
		},
		Request: map[string]interface{}{
			"uid":       "req-1",
		},
	}

	raw, err := json.Marshal(ev)
	require.NoError(t, err)

	require.NoError(t, worker.ProcessViolationEvent(context.Background(), raw))

	var violations int64
	require.NoError(t, db.Model(&models.PolicyViolation{}).Count(&violations).Error)
	require.Equal(t, int64(1), violations, "expected 1 persisted violation")

	var insights int64
	require.NoError(t, db.Model(&models.Insight{}).Count(&insights).Error)
	require.Equal(t, int64(1), insights, "expected baseline insight to be created")

	var insight models.Insight
	require.NoError(t, db.Where("insight_type = ?", "policy_violation").First(&insight).Error)
	require.Equal(t, "policy_violation", insight.InsightType)
	require.Equal(t, "high", insight.Severity)
	require.Contains(t, insight.Title, tpl.Name)
}

