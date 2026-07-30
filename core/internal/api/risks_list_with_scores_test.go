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

// TestGetInsightsList_WithScores_IncludesScore verifies GET /risks?withScores=1 returns totalScore and unified final_level from risk_scores.
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
			TotalScore           *float64 `json:"totalScore"`
			SeverityHint         string   `json:"severity_hint"`
			FinalScore           *float64 `json:"final_score"`
			FinalLevel           string   `json:"final_level"`
			ExploitabilityScore  *float64 `json:"exploitabilityScore"`
			BusinessImpactScore  *float64 `json:"businessImpactScore"`
			TimeDecay            *float64 `json:"timeDecay"`
			Title                string   `json:"title"`
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
	if out.Insights[0].SeverityHint != "high" {
		t.Errorf("expected severity_hint high, got %q", out.Insights[0].SeverityHint)
	}
	if out.Insights[0].FinalScore == nil || *out.Insights[0].FinalScore != 75.5 {
		t.Errorf("expected final_score 75.5, got %v", out.Insights[0].FinalScore)
	}
	if out.Insights[0].FinalLevel != "critical" {
		t.Errorf("expected final_level critical (75.5), got %q", out.Insights[0].FinalLevel)
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

// TestGetInsightsList_WithScores_NoMatchingScore verifies that when risk_scores has no row for resource_uid, score fields are omitted.
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
			TotalScore           *float64 `json:"totalScore"`
			SeverityHint         string   `json:"severity_hint"`
			FinalScore           *float64 `json:"final_score"`
			FinalLevel           string   `json:"final_level"`
			ExploitabilityScore  *float64 `json:"exploitabilityScore"`
			BusinessImpactScore  *float64 `json:"businessImpactScore"`
			TimeDecay            *float64 `json:"timeDecay"`
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
	if out.Insights[0].SeverityHint != "high" {
		t.Errorf("expected severity_hint high, got %q", out.Insights[0].SeverityHint)
	}
	if out.Insights[0].FinalScore != nil {
		t.Errorf("expected nil final_score when no matching risk_score, got %v", *out.Insights[0].FinalScore)
	}
	if out.Insights[0].FinalLevel != "" {
		t.Errorf("expected empty final_level when no matching risk_score, got %q", out.Insights[0].FinalLevel)
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

// TestGetInsightsList_WithScores_UsesV3 verifies withScores mapping uses v3 rows.
func TestGetInsightsList_WithScores_UsesV3(t *testing.T) {
	db := setupRisksListTestDB(t)
	now := time.Now()
	_ = db.Create(&models.Insight{
		ResourceType: "Pod", ResourceUID: "pod-1", ResourceName: "p1", ResourceNamespace: "default",
		InsightType: "vulnerability", Severity: "high", Title: "Test", Description: "Desc",
		Status: "active", DetectedAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error

	_ = db.Create(&models.RiskScore{
		ResourceType: "Pod", ResourceUID: "pod-1", ResourceName: "p1", Namespace: "default", ClusterID: "c1",
		TotalScore: 86.0, PriorityLevel: "P0", ScorerVersion: "v3", CalculatedAt: now, CreatedAt: now, UpdatedAt: now,
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
			TotalScore  *float64 `json:"totalScore"`
			FinalLevel  string   `json:"final_level"`
		} `json:"insights"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if len(out.Insights) != 1 || out.Insights[0].TotalScore == nil {
		t.Fatalf("expected one scored insight, got len=%d score=%v", len(out.Insights), out.Insights[0].TotalScore)
	}
	if *out.Insights[0].TotalScore != 86.0 || out.Insights[0].FinalLevel != "critical" {
		t.Fatalf("expected v3 score=86 and final_level=critical, got %.1f/%s", *out.Insights[0].TotalScore, out.Insights[0].FinalLevel)
	}
}

// TestGetInsightsList_ScoreBinFiltersByPreferredScore verifies scoreBin=70 keeps only rows whose preferred score is in [70,80).
func TestGetInsightsList_ScoreBinFiltersByPreferredScore(t *testing.T) {
	db := setupRisksListTestDB(t)
	now := time.Now()
	_ = db.Create(&models.Pod{UID: "pod-2", Name: "p2", Namespace: "default", ClusterID: "c1"}).Error

	_ = db.Create(&models.Insight{
		ResourceType: "Pod", ResourceUID: "pod-1", ResourceName: "p1", ResourceNamespace: "default",
		InsightType: "vulnerability", Severity: "high", Title: "A", Description: "D",
		CVEID: "CVE-2024-1", Status: "active", DetectedAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error
	_ = db.Create(&models.Insight{
		ResourceType: "Pod", ResourceUID: "pod-2", ResourceName: "p2", ResourceNamespace: "default",
		InsightType: "vulnerability", Severity: "medium", Title: "B", Description: "D",
		CVEID: "CVE-2024-2", Status: "active", DetectedAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error

	_ = db.Create(&models.RiskScore{
		ResourceType: "Pod", ResourceUID: "pod-1", ResourceName: "p1", Namespace: "default", ClusterID: "c1",
		TotalScore: 76, PriorityLevel: "P1", ScorerVersion: "v3", CalculatedAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error
	_ = db.Create(&models.RiskScore{
		ResourceType: "Pod", ResourceUID: "pod-2", ResourceName: "p2", Namespace: "default", ClusterID: "c1",
		TotalScore: 42, PriorityLevel: "P2", ScorerVersion: "v3", CalculatedAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/risks?clusterId=c1&page=1&pageSize=20&withScores=1&scoreBin=70", nil)

	GetInsightsList(db)(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Insights []json.RawMessage `json:"insights"`
		Total      int64             `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if out.Total != 1 || len(out.Insights) != 1 {
		t.Fatalf("expected total=1 and one row, got total=%d len=%d", out.Total, len(out.Insights))
	}
}

// TestGetInsightsList_View_Group aggregates findings by insight_type + CVE/title key.
func TestGetInsightsList_View_Group(t *testing.T) {
	db := setupRisksListTestDB(t)
	now := time.Now()
	_ = db.Create(&models.Pod{UID: "pod-2", Name: "p2", Namespace: "default", ClusterID: "c1"}).Error

	_ = db.Create(&models.Insight{
		ResourceType: "Pod", ResourceUID: "pod-1", ResourceName: "p1", ResourceNamespace: "default",
		InsightType: "vulnerability", Severity: "high", Title: "Same CVE", Description: "D",
		CVEID: "CVE-2024-999", Status: "active", DetectedAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error
	_ = db.Create(&models.Insight{
		ResourceType: "Pod", ResourceUID: "pod-2", ResourceName: "p2", ResourceNamespace: "default",
		InsightType: "vulnerability", Severity: "medium", Title: "Same CVE other pod", Description: "D",
		CVEID: "CVE-2024-999", Status: "active", DetectedAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error

	_ = db.Create(&models.RiskScore{
		ResourceType: "Pod", ResourceUID: "pod-1", ResourceName: "p1", Namespace: "default", ClusterID: "c1",
		TotalScore: 60, ScorerVersion: "v3", CalculatedAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error
	_ = db.Create(&models.RiskScore{
		ResourceType: "Pod", ResourceUID: "pod-2", ResourceName: "p2", Namespace: "default", ClusterID: "c1",
		TotalScore: 80, ScorerVersion: "v3", CalculatedAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/risks?clusterId=c1&view=group&page=1&pageSize=20", nil)

	GetInsightsList(db)(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		View   string `json:"view"`
		Total  int64  `json:"total"`
		Groups []struct {
			GroupKey    string  `json:"group_key"`
			MemberCount int64   `json:"member_count"`
			MaxScore    float64 `json:"max_score"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if out.View != "group" {
		t.Fatalf("expected view=group, got %q", out.View)
	}
	if out.Total != 1 || len(out.Groups) != 1 {
		t.Fatalf("expected one group, got total=%d groups=%d", out.Total, len(out.Groups))
	}
	if out.Groups[0].MemberCount != 2 {
		t.Fatalf("expected member_count=2, got %d", out.Groups[0].MemberCount)
	}
	if out.Groups[0].GroupKey != "cve-2024-999" {
		t.Fatalf("expected group_key cve-2024-999, got %q", out.Groups[0].GroupKey)
	}
	if out.Groups[0].MaxScore != 80 {
		t.Fatalf("expected max_score 80, got %v", out.Groups[0].MaxScore)
	}
}
