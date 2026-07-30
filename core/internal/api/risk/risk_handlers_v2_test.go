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

func setupCalculateRiskScoreSQLite(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Pod{},
		&models.Insight{},
		&models.PodCapability{},
		&models.AttackPath{},
		&models.RiskScore{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_test_risk_scores_key ON risk_scores(resource_type, resource_uid, cluster_id)").Error; err != nil {
		t.Fatalf("create unique index: %v", err)
	}
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.GET("/risk/scores", GetRiskScores(db))
	v1.GET("/risk/scores/:uid", GetRiskScore(db))
	v1.POST("/risk/scores/:uid/calculate", CalculateRiskScore(db))
	return r, db
}

// TestCalculateRiskScore_V3Only verifies POST calculate persists unified V3 only.
func TestCalculateRiskScore_V3Only(t *testing.T) {
	router, db := setupCalculateRiskScoreSQLite(t)
	now := time.Now()
	podUID := "test-uid-api"
	_ = db.Create(&models.Pod{
		UID: podUID, Name: "test-pod", Namespace: "default", ClusterID: "c1",
		ServiceAccount: "default", Containers: "[]",
	}).Error
	_ = db.Create(&models.Insight{
		ResourceType: "Pod", ResourceUID: podUID, ResourceName: "test-pod", ResourceNamespace: "default",
		InsightType: "security", Severity: "critical", Title: "Test", Description: "D",
		Status: "active", DetectedAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/risk/scores/"+podUID+"/calculate", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if response["mode"] != "v3" {
		t.Fatalf("expected mode=v3, got %v", response["mode"])
	}
	if _, ok := response["v3"].(map[string]interface{}); !ok {
		t.Fatalf("expected v3 object: %#v", response["v3"])
	}
	var saved models.RiskScore
	if err := db.Where("resource_uid = ? AND scorer_version = ?", podUID, "v3").First(&saved).Error; err != nil {
		t.Fatalf("v3 row missing: %v", err)
	}
}

// TestCalculateRiskScore_RejectsV2Mode verifies deprecated v2 mode returns 400.
func TestCalculateRiskScore_RejectsV2Mode(t *testing.T) {
	router, _ := setupCalculateRiskScoreSQLite(t)
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/risk/scores/x/calculate?mode=v2", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestGetRiskScores_V3Fields tests list surfaces v3 rows.
func TestGetRiskScores_V3Fields(t *testing.T) {
	router, db := setupCalculateRiskScoreSQLite(t)
	now := time.Now()
	_ = db.Create(&models.RiskScore{
		ResourceType: "Pod", ResourceUID: "test-uid-list", ResourceName: "p", Namespace: "default", ClusterID: "c1",
		TotalScore: 75.0, BaseScore: 30, ExploitabilityScore: 25, BusinessImpactScore: 20,
		ScorerVersion: "v3", PriorityLevel: "P1", CalculatedAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/risk/scores", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("parse: %v", err)
	}
	scores, ok := response["scores"].([]interface{})
	if !ok || len(scores) == 0 {
		t.Fatal("missing scores")
	}
	first := scores[0].(map[string]interface{})
	if first["scorerVersion"] != "v3" {
		t.Fatalf("expected v3 row, got %v", first["scorerVersion"])
	}
}
