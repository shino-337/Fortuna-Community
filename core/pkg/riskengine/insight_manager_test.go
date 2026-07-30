package riskengine

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// TestCreateOrUpdateInsight_ExceptionPolicyPreventsReactivation verifies RP-5:
// a dismissed vulnerability insight with an active exception policy must NOT be
// re-activated on the next scan cycle.
func TestCreateOrUpdateInsight_ExceptionPolicyPreventsReactivation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Insight{}, &models.ExceptionPolicy{}); err != nil {
		t.Fatal(err)
	}

	m := NewInsightManager(db)
	uid := "cccccccc-dddd-dddd-dddd-cccccccccccc"
	cveID := "CVE-2024-1234"
	now := time.Now()

	// 1. Create a vulnerability insight in dismissed state (simulating a user dismiss action).
	dismissed := &models.Insight{
		ResourceType:  "Pod",
		ResourceName:  "web-pod",
		ResourceUID:   uid,
		InsightType:   "vulnerability",
		Severity:      "high",
		Title:         "High CVE in nginx",
		Description:   "desc",
		CVEID:         cveID,
		Status:        "dismissed",
		DetectedAt:    now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.Create(dismissed).Error; err != nil {
		t.Fatal(err)
	}

	// 2. Create an active exception policy for this resource+CVE.
	policy := &models.ExceptionPolicy{
		ResourceUID: uid,
		CVEID:       cveID,
		InsightType: "vulnerability",
		Reason:      "known false positive",
		CreatedBy:   "test",
	}
	if err := db.Create(policy).Error; err != nil {
		t.Fatal(err)
	}

	// 3. Simulate a re-scan by calling CreateOrUpdateInsight with the same vulnerability.
	rescan := &models.Insight{
		ResourceType:  "Pod",
		ResourceName:  "web-pod",
		ResourceUID:   uid,
		InsightType:   "vulnerability",
		Severity:      "high",
		Title:         "High CVE in nginx",
		Description:   "desc updated",
		CVEID:         cveID,
		Status:        "active",
		DetectedAt:    now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := m.CreateOrUpdateInsight(rescan); err != nil {
		t.Fatalf("CreateOrUpdateInsight failed: %v", err)
	}

	// 4. Verify the insight is STILL dismissed.
	var stored models.Insight
	if err := db.Where("resource_uid = ? AND cve_id = ?", uid, cveID).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != "dismissed" {
		t.Errorf("expected insight to remain dismissed, got status=%q", stored.Status)
	}
}

// TestCreateOrUpdateInsight_ExpiredExceptionPolicyAllowsReactivation verifies that
// an expired exception policy does NOT prevent re-activation.
func TestCreateOrUpdateInsight_ExpiredExceptionPolicyAllowsReactivation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Insight{}, &models.ExceptionPolicy{}); err != nil {
		t.Fatal(err)
	}

	m := NewInsightManager(db)
	uid := "eeeeeeee-ffff-ffff-ffff-eeeeeeeeeeee"
	cveID := "CVE-2024-5678"
	now := time.Now()
	past := now.Add(-24 * time.Hour)

	// Create dismissed insight
	dismissed := &models.Insight{
		ResourceType:  "Pod",
		ResourceName:  "api-pod",
		ResourceUID:   uid,
		InsightType:   "vulnerability",
		Severity:      "critical",
		Title:         "Critical CVE",
		Description:   "desc",
		CVEID:         cveID,
		Status:        "dismissed",
		DetectedAt:    now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.Create(dismissed).Error; err != nil {
		t.Fatal(err)
	}

	// Create an EXPIRED exception policy
	expiredPolicy := &models.ExceptionPolicy{
		ResourceUID: uid,
		CVEID:       cveID,
		InsightType: "vulnerability",
		Reason:      "expired reason",
		ExpiresAt:   &past,
		CreatedBy:   "test",
	}
	if err := db.Create(expiredPolicy).Error; err != nil {
		t.Fatal(err)
	}

	// Re-scan should re-activate because policy is expired
	rescan := &models.Insight{
		ResourceType:  "Pod",
		ResourceName:  "api-pod",
		ResourceUID:   uid,
		InsightType:   "vulnerability",
		Severity:      "critical",
		Title:         "Critical CVE",
		Description:   "desc updated",
		CVEID:         cveID,
		Status:        "active",
		DetectedAt:    now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := m.CreateOrUpdateInsight(rescan); err != nil {
		t.Fatalf("CreateOrUpdateInsight failed: %v", err)
	}

	var stored models.Insight
	if err := db.Where("resource_uid = ? AND cve_id = ?", uid, cveID).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != "active" {
		t.Errorf("expected insight to be re-activated (expired exception), got status=%q", stored.Status)
	}
}

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

// Empty cve_id still participates in UNIQUE(resource_uid, cve_id, insight_type).
// Title-only dedup misses when the engine changes the title; must upsert on (uid, '', type).
func TestCreateOrUpdateInsight_EmptyCVEKeyUpserts(t *testing.T) {
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
	uid := "bbbbbbbb-cccc-cccc-cccc-bbbbbbbbbbbb"
	now := time.Now()
	first := &models.Insight{
		ResourceType:      "ClusterRoleBinding",
		ResourceNamespace: "",
		ResourceName:      "cluster-admin",
		ResourceUID:       uid,
		InsightType:       "rbac_risk",
		Severity:          "high",
		Title:             "Original title",
		Description:       "first",
		CVEID:             "",
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
		ResourceName:      "cluster-admin",
		ResourceUID:       uid,
		InsightType:       "rbac_risk",
		Severity:          "critical",
		Title:             "Renamed title",
		Description:       "second pass same empty cve_id",
		CVEID:             "",
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
	if stored.Title != "Renamed title" || stored.Severity != "critical" {
		t.Fatalf("expected updated fields, got title=%q severity=%q", stored.Title, stored.Severity)
	}
}
