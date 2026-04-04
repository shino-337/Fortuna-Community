package riskengine

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Regression: idx_insights_unique_resource_cve_type_all is on (resource_uid, cve_id, insight_type).
// YAML insights use CVEID = rule.ID; dedupe must use that triple, not title alone, or the second
// evaluation hits 23505 (e.g. duplicate binding rows or legacy status not matching title query).
func TestCreateOrUpdateInsight_NonVulnCVEIDKeyUpserts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Insight{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_insights_unique_resource_cve_type_all ON insights(resource_uid, cve_id, insight_type)`).Error; err != nil {
		t.Fatal(err)
	}

	m := NewInsightManager(db)
	uid := "aaaaaaaa-bbbb-bbbb-bbbb-aaaaaaaaaaaa"
	now := time.Now()
	first := &models.Insight{
		ResourceType:      "ClusterRoleBinding",
		ResourceNamespace: "",
		ResourceName:      "rc-cluster-admin-sa-binding",
		ResourceUID:       uid,
		InsightType:       "rbac_risk",
		Severity:          "high",
		Title:             "Original title",
		Description:       "first",
		CVEID:             "cluster-admin-binding-detailed",
		Status:            "active",
		DetectedAt:        now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := m.CreateOrUpdateInsight(first); err != nil {
		t.Fatal(err)
	}

	second := &models.Insight{
		ResourceType:      "ClusterRoleBinding",
		ResourceNamespace: "",
		ResourceName:      "rc-cluster-admin-sa-binding",
		ResourceUID:       uid,
		InsightType:       "rbac_risk",
		Severity:          "critical",
		Title:             "Updated title",
		Description:       "second pass same rule id",
		CVEID:             "cluster-admin-binding-detailed",
		Status:            "active",
		DetectedAt:        now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := m.CreateOrUpdateInsight(second); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	var count int64
	if err := db.Model(&models.Insight{}).Where("resource_uid = ?", uid).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 insight row, got %d", count)
	}
	var stored models.Insight
	if err := db.Where("resource_uid = ?", uid).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Title != "Updated title" || stored.Severity != "critical" {
		t.Fatalf("expected updated fields, got title=%q severity=%q", stored.Title, stored.Severity)
	}
}
