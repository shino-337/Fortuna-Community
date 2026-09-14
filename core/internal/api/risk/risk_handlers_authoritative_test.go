package risk

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

func setupRiskHandlersSQLite(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.RiskScore{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user", &models.User{Role: models.RoleAdmin}) })
	v1 := r.Group("/api/v1")
	v1.GET("/risk/scores", GetRiskScores(db))
	v1.GET("/risk/scores/:uid", GetRiskScore(db))
	return r, db
}

// Ensure list endpoint returns one authoritative row per resource (v3 only).
func TestGetRiskScores_PrefersV3Authoritative(t *testing.T) {
	router, db := setupRiskHandlersSQLite(t)
	now := time.Now()

	// Same logical resource, two v3 rows -> newest wins.
	_ = db.Create(&models.RiskScore{
		ResourceType: "Pod", ResourceUID: "pod-a", ResourceName: "pod-a", Namespace: "default", ClusterID: "c1",
		TotalScore: 78, PriorityLevel: "P1", ScorerVersion: "v3", CalculatedAt: now.Add(-5 * time.Minute),
	}).Error
	_ = db.Create(&models.RiskScore{
		ResourceType: "Pod", ResourceUID: "pod-a", ResourceName: "pod-a", Namespace: "default", ClusterID: "c1",
		TotalScore: 88, PriorityLevel: "P0", ScorerVersion: "v3", CalculatedAt: now,
	}).Error

	req := httptest.NewRequest(http.MethodGet, "/api/v1/risk/scores?sortBy=score&page=1&pageSize=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var out struct {
		Scores []models.RiskScore `json:"scores"`
		Total  int                `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if out.Total != 1 || len(out.Scores) != 1 {
		t.Fatalf("expected one authoritative row, total=%d len=%d", out.Total, len(out.Scores))
	}
	if out.Scores[0].ScorerVersion != "v3" {
		t.Fatalf("expected v3 row, got %s", out.Scores[0].ScorerVersion)
	}
}

// Ensure detail endpoint returns latest authoritative v3 version.
func TestGetRiskScore_DetailPrefersV3(t *testing.T) {
	router, db := setupRiskHandlersSQLite(t)
	now := time.Now()

	_ = db.Create(&models.RiskScore{
		ResourceType: "Pod", ResourceUID: "pod-b", ResourceName: "pod-b", Namespace: "default", ClusterID: "c1",
		TotalScore: 41, PriorityLevel: "P2", ScorerVersion: "v3", CalculatedAt: now.Add(-10 * time.Minute),
	}).Error
	_ = db.Create(&models.RiskScore{
		ResourceType: "Pod", ResourceUID: "pod-b", ResourceName: "pod-b", Namespace: "default", ClusterID: "c1",
		TotalScore: 79, PriorityLevel: "P1", ScorerVersion: "v3", CalculatedAt: now,
	}).Error

	req := httptest.NewRequest(http.MethodGet, "/api/v1/risk/scores/pod-b", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var out models.RiskScore
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if out.ScorerVersion != "v3" {
		t.Fatalf("expected v3 detail row, got %s", out.ScorerVersion)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("parse map: %v", err)
	}
	if payload["final_level"] != "critical" {
		t.Fatalf("expected final_level critical for score 79, got %v", payload["final_level"])
	}
	if payload["final_score"] != 79.0 && payload["final_score"] != 79 {
		t.Fatalf("expected final_score 79, got %v", payload["final_score"])
	}
}

