package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

func setupRisksListTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Insight{}, &models.Pod{}, &models.Cluster{}, &models.RiskScore{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_ = db.Create(&models.Cluster{ID: "c1", Name: "cluster1"}).Error
	_ = db.Create(&models.Pod{UID: "pod-1", Name: "p1", Namespace: "default", ClusterID: "c1"}).Error
	return db
}

// TestGetInsightsList_WithScores_Empty verifies GET /risks without withScores returns insights without score fields.
func TestGetInsightsList_WithScores_Empty(t *testing.T) {
	db := setupRisksListTestDB(t)
	now := time.Now()
	_ = db.Create(&models.Insight{
		ResourceType: "Pod", ResourceUID: "pod-1", ResourceName: "p1", ResourceNamespace: "default",
		InsightType: "vulnerability", Severity: "high", Title: "Test", Description: "Desc",
		Status: "active", DetectedAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/risks?clusterId=c1&page=1&pageSize=20", nil)

	GetInsightsList(db)(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Insights []map[string]interface{} `json:"insights"`
		Total    int                     `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if out.Total != 1 || len(out.Insights) != 1 {
		t.Errorf("expected 1 insight, got total=%d len=%d", out.Total, len(out.Insights))
	}
	if _, ok := out.Insights[0]["totalScore"]; ok {
		t.Error("expected no totalScore when withScores not set")
	}
}

// TestGetInsightsList_WithScores_IncludesScore verifies GET /risks?withScores=1 returns totalScore and priorityLevel from risk_scores.
func TestGetInsightsList_WithScores_IncludesScore(t *testing.T) {
	db := setupRisksListTestDB(t)
	now := time.Now()
	_ = db.Create(&models.Insight{
		ResourceType: "Pod", ResourceUID: "pod-1", ResourceName: "p1", ResourceNamespace: "default",
		InsightType: "vulnerability", Severity: "high", Title: "Test", Description: "Desc",
		Status: "active", DetectedAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error
	_ = db.Create(&models.RiskScore{
		ResourceType: "Pod", ResourceUID: "pod-1", ResourceName: "p1", Namespace: "default", ClusterID: "c1",
		TotalScore: 75.5, PriorityLevel: "P1", CalculatedAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/risks?clusterId=c1&page=1&pageSize=20&withScores=1", nil)

	GetInsightsList(db)(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Insights []struct {
			TotalScore          *float64 `json:"totalScore"`
			PriorityLevel       string   `json:"priorityLevel"`
			ExploitabilityScore *float64 `json:"exploitabilityScore"`
			BusinessImpactScore *float64 `json:"businessImpactScore"`
			TimeDecay           *float64 `json:"timeDecay"`
			Title               string   `json:"title"`
		} `json:"insights"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if out.Total != 1 || len(out.Insights) != 1 {
		t.Errorf("expected 1 insight, got total=%d len=%d", out.Total, len(out.Insights))
	}
	if out.Insights[0].TotalScore == nil || *out.Insights[0].TotalScore != 75.5 {
		t.Errorf("expected totalScore 75.5, got %v", out.Insights[0].TotalScore)
	}
	if out.Insights[0].PriorityLevel != "P1" {
		t.Errorf("expected priorityLevel P1, got %q", out.Insights[0].PriorityLevel)
	}
	if out.Insights[0].ExploitabilityScore == nil {
		t.Error("expected exploitabilityScore to be present when risk_score exists")
	}
	if out.Insights[0].BusinessImpactScore == nil {
		t.Error("expected businessImpactScore to be present when risk_score exists")
	}
	if out.Insights[0].TimeDecay == nil {
		t.Error("expected timeDecay to be present when risk_score exists")
	}
}

// TestGetInsightsList_WithScores_NoMatchingScore verifies that when risk_scores has no row for resource_uid, totalScore/priorityLevel are omitted.
func TestGetInsightsList_WithScores_NoMatchingScore(t *testing.T) {
	db := setupRisksListTestDB(t)
	now := time.Now()
	_ = db.Create(&models.Insight{
		ResourceType: "Pod", ResourceUID: "pod-1", ResourceName: "p1", ResourceNamespace: "default",
		InsightType: "vulnerability", Severity: "high", Title: "Test", Description: "Desc",
		Status: "active", DetectedAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error
	// risk_score for different resource
	_ = db.Create(&models.RiskScore{
		ResourceType: "Pod", ResourceUID: "other-pod", ResourceName: "other", Namespace: "default", ClusterID: "c1",
		TotalScore: 50, PriorityLevel: "P2", CalculatedAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/risks?clusterId=c1&page=1&pageSize=20&withScores=1", nil)

	GetInsightsList(db)(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var out struct {
		Insights []struct {
			TotalScore          *float64 `json:"totalScore"`
			PriorityLevel       string   `json:"priorityLevel"`
			ExploitabilityScore *float64 `json:"exploitabilityScore"`
			BusinessImpactScore *float64 `json:"businessImpactScore"`
			TimeDecay           *float64 `json:"timeDecay"`
		} `json:"insights"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if len(out.Insights) != 1 {
		t.Fatalf("expected 1 insight, got %d", len(out.Insights))
	}
	if out.Insights[0].TotalScore != nil {
		t.Errorf("expected nil totalScore when no matching risk_score, got %v", *out.Insights[0].TotalScore)
	}
	if out.Insights[0].PriorityLevel != "" {
		t.Errorf("expected empty priorityLevel, got %q", out.Insights[0].PriorityLevel)
	}
	if out.Insights[0].ExploitabilityScore != nil {
		t.Errorf("expected nil exploitabilityScore when no matching risk_score, got %v", *out.Insights[0].ExploitabilityScore)
	}
	if out.Insights[0].BusinessImpactScore != nil {
		t.Errorf("expected nil businessImpactScore when no matching risk_score, got %v", *out.Insights[0].BusinessImpactScore)
	}
	if out.Insights[0].TimeDecay != nil {
		t.Errorf("expected nil timeDecay when no matching risk_score, got %v", *out.Insights[0].TimeDecay)
	}
}
