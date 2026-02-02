package risk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// setupTestRouter creates a test router with risk handlers
func setupTestRouterV2(t *testing.T) (*gin.Engine, *gorm.DB) {
	gin.SetMode(gin.TestMode)

	dsn := "host=localhost user=postgres password=postgres dbname=fortuna_test port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("Skipping test: cannot connect to test database: %v", err)
		return nil, nil
	}

	// Auto-migrate
	db.AutoMigrate(&models.RiskScore{}, &models.Insight{})

	router := gin.New()
	v1 := router.Group("/api/v1")
	{
		v1.GET("/risk/scores", GetRiskScores(db))
		v1.GET("/risk/scores/:uid", GetRiskScore(db))
		v1.POST("/risk/scores/:uid/calculate", CalculateRiskScore(db))
	}

	return router, db
}

// TestCalculateRiskScore_V2 tests the CalculateRiskScore endpoint with V2 scorer
func TestCalculateRiskScore_V2(t *testing.T) {
	router, db := setupTestRouterV2(t)
	if router == nil {
		return
	}

	// Create test insight
	insight := models.Insight{
		ResourceType:      "Pod",
		ResourceNamespace: "default",
		ResourceName:      "test-pod",
		ResourceUID:       "test-uid-api",
		InsightType:       "security",
		Severity:          "critical",
		Title:             "Test critical security issue",
		Description:       "Test critical security issue",
		Status:            "active",
		DetectedAt:        time.Now(),
	}
	db.Create(&insight)

	// Test POST /api/v1/risk/scores/:uid/calculate
	req, _ := http.NewRequest("POST", "/api/v1/risk/scores/test-uid-api/calculate", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify V2 fields
	if version, ok := response["scorerVersion"].(string); !ok || version != "v2" {
		t.Errorf("Expected scorerVersion=v2, got %v", response["scorerVersion"])
	}

	if _, ok := response["exploitabilityScore"]; !ok {
		t.Error("Response missing exploitabilityScore field")
	}

	if _, ok := response["businessImpactScore"]; !ok {
		t.Error("Response missing businessImpactScore field")
	}

	// Verify score is saved to database
	var saved models.RiskScore
	if err := db.Where("resource_uid = ?", "test-uid-api").First(&saved).Error; err != nil {
		t.Errorf("Score not saved to database: %v", err)
	} else {
		if saved.ScorerVersion != "v2" {
			t.Errorf("Database score has wrong version: %s", saved.ScorerVersion)
		}
	}
}

// TestGetRiskScores_V2Fields tests that GetRiskScores returns V2 fields
func TestGetRiskScores_V2Fields(t *testing.T) {
	router, db := setupTestRouterV2(t)
	if router == nil {
		return
	}

	// Create test risk score with V2 fields
	score := models.RiskScore{
		ResourceUID:         "test-uid-list",
		ResourceType:        "Pod",
		TotalScore:          75.0,
		BaseScore:           30.0,
		ExploitabilityScore: 25.0,
		BusinessImpactScore: 20.0,
		ScorerVersion:       "v2",
		PriorityLevel:       "P1",
	}
	db.Create(&score)

	// Test GET /api/v1/risk/scores
	req, _ := http.NewRequest("GET", "/api/v1/risk/scores", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	scores, ok := response["scores"].([]interface{})
	if !ok || len(scores) == 0 {
		t.Error("Response missing scores array")
		return
	}

	firstScore := scores[0].(map[string]interface{})
	if firstScore["scorerVersion"] != "v2" {
		t.Errorf("Expected scorerVersion=v2, got %v", firstScore["scorerVersion"])
	}

	if firstScore["exploitabilityScore"] == nil {
		t.Error("Response missing exploitabilityScore field")
	}

	if firstScore["businessImpactScore"] == nil {
		t.Error("Response missing businessImpactScore field")
	}
}
