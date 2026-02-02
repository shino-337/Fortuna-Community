package risk

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// setupTestDB creates a test database connection
func setupTestDB(t *testing.T) *gorm.DB {
	// Use in-memory SQLite for testing (or configure test PostgreSQL)
	dsn := "host=localhost user=postgres password=postgres dbname=fortuna_test port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("Skipping test: cannot connect to test database: %v", err)
		return nil
	}

	// Auto-migrate test tables
	db.AutoMigrate(&models.Insight{}, &models.RiskScore{}, &models.Pod{}, &models.ServiceAccount{})

	return db
}

// TestScorerV2_CalculateScore_NoInsights tests scoring with no insights
func TestScorerV2_CalculateScore_NoInsights(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}

	scorer := NewScorer(db)
	score, err := scorer.CalculateScore(context.Background(), "test-uid-123")

	if err != nil {
		t.Fatalf("CalculateScore failed: %v", err)
	}

	if score.TotalScore != 0 {
		t.Errorf("Expected TotalScore=0 for no insights, got %.2f", score.TotalScore)
	}

	if score.PriorityLevel != "P4" {
		t.Errorf("Expected PriorityLevel=P4 for no insights, got %s", score.PriorityLevel)
	}

	if score.ScorerVersion != "v2" {
		t.Errorf("Expected ScorerVersion=v2, got %s", score.ScorerVersion)
	}
}

// TestScorerV2_CalculateScore_WithInsights tests scoring with insights
func TestScorerV2_CalculateScore_WithInsights(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}

	// Create test insight
	insight := models.Insight{
		ResourceType:      "Pod",
		ResourceNamespace: "default",
		ResourceName:      "test-pod",
		ResourceUID:       "test-uid-123",
		InsightType:       "security",
		Severity:          "critical",
		Title:             "Test critical security issue",
		Description:       "Test critical security issue",
		Status:            "active",
		DetectedAt:        time.Now(),
	}

	if err := db.Create(&insight).Error; err != nil {
		t.Fatalf("Failed to create test insight: %v", err)
	}

	scorer := NewScorer(db)
	score, err := scorer.CalculateScore(context.Background(), "test-uid-123")

	if err != nil {
		t.Fatalf("CalculateScore failed: %v", err)
	}

	// Verify score components
	if score.BaseScore < 0 || score.BaseScore > 40 {
		t.Errorf("BaseScore should be 0-40, got %.2f", score.BaseScore)
	}

	if score.ExploitabilityScore < 0 || score.ExploitabilityScore > 30 {
		t.Errorf("ExploitabilityScore should be 0-30, got %.2f", score.ExploitabilityScore)
	}

	if score.BusinessImpactScore < 0 || score.BusinessImpactScore > 30 {
		t.Errorf("BusinessImpactScore should be 0-30, got %.2f", score.BusinessImpactScore)
	}

	if score.TimeDecay < 0.7 || score.TimeDecay > 1.0 {
		t.Errorf("TimeDecay should be 0.7-1.0, got %.2f", score.TimeDecay)
	}

	// Verify total score formula: (Base + Exploit + Impact) × TimeDecay
	expectedTotal := (score.BaseScore + score.ExploitabilityScore + score.BusinessImpactScore) * score.TimeDecay
	if score.TotalScore < 0 || score.TotalScore > 100 {
		t.Errorf("TotalScore should be 0-100, got %.2f", score.TotalScore)
	}

	// Allow small floating point differences
	if score.TotalScore < expectedTotal-0.01 || score.TotalScore > expectedTotal+0.01 {
		t.Errorf("TotalScore formula mismatch: expected ~%.2f, got %.2f", expectedTotal, score.TotalScore)
	}

	// Verify priority level
	if score.PriorityLevel == "" {
		t.Error("PriorityLevel should not be empty")
	}

	// Verify scorer version
	if score.ScorerVersion != "v2" {
		t.Errorf("Expected ScorerVersion=v2, got %s", score.ScorerVersion)
	}
}

// TestScorerV2_SaveScore tests saving score to database
func TestScorerV2_SaveScore(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}

	scorer := NewScorer(db)
	score := &RiskScoreV2{
		ResourceUID:         "test-uid-456",
		ResourceType:        "Pod",
		ResourceName:        "test-pod",
		Namespace:           "default",
		ClusterID:           "test-cluster",
		BaseScore:           25.0,
		ExploitabilityScore: 20.0,
		BusinessImpactScore: 15.0,
		TimeDecay:           0.9,
		TotalScore:          54.0, // (25+20+15)*0.9
		PriorityLevel:       "P2",
		ScorerVersion:       "v2",
		InsightsCount:       1,
		HighestSeverity:     "critical",
		Factors:             map[string]interface{}{"test": "value"},
	}

	err := scorer.SaveScore(context.Background(), score)
	if err != nil {
		t.Fatalf("SaveScore failed: %v", err)
	}

	// Verify saved score
	var saved models.RiskScore
	if err := db.Where("resource_uid = ?", "test-uid-456").First(&saved).Error; err != nil {
		t.Fatalf("Failed to retrieve saved score: %v", err)
	}

	if saved.TotalScore != 54.0 {
		t.Errorf("Expected TotalScore=54.0, got %.2f", saved.TotalScore)
	}

	if saved.ExploitabilityScore != 20.0 {
		t.Errorf("Expected ExploitabilityScore=20.0, got %.2f", saved.ExploitabilityScore)
	}

	if saved.BusinessImpactScore != 15.0 {
		t.Errorf("Expected BusinessImpactScore=15.0, got %.2f", saved.BusinessImpactScore)
	}

	if saved.ScorerVersion != "v2" {
		t.Errorf("Expected ScorerVersion=v2, got %s", saved.ScorerVersion)
	}
}

// TestScorerV2_PriorityLevels tests priority level calculation
func TestScorerV2_PriorityLevels(t *testing.T) {
	testCases := []struct {
		score       float64
		expected    string
		description string
	}{
		{95.0, "P0", "Critical score"},
		{80.0, "P0", "Critical threshold"},
		{79.0, "P1", "High score"},
		{60.0, "P1", "High threshold"},
		{59.0, "P2", "Medium score"},
		{35.0, "P2", "Medium threshold"},
		{34.0, "P3", "Low score"},
		{10.0, "P3", "Low threshold"},
		{9.0, "P4", "Minimal score"},
		{0.0, "P4", "Zero score"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			priority := NewScorer(nil).determinePriority(tc.score)
			if priority != tc.expected {
				t.Errorf("Score %.2f: expected priority %s, got %s", tc.score, tc.expected, priority)
			}
		})
	}
}
