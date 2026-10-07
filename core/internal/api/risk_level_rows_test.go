package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
)

// Rule detail lists raw finding rows; each row must carry the risk level of its
// resource so the page does not show the rule severity as the finding's level.
func TestWithRiskLevels(t *testing.T) {
	db := setupExportTestDB(t)
	now := time.Now()
	if err := db.Create(&models.RiskScore{ClusterID: "c1", ResourceUID: "pod-1", ResourceType: "Pod", TotalScore: 74, ScorerVersion: "v3", CalculatedAt: now}).Error; err != nil {
		t.Fatalf("score: %v", err)
	}
	rows := withRiskLevels(db, []models.Insight{
		{ID: 1, ClusterID: "c1", ResourceUID: "pod-1", Severity: "low"},
		{ID: 2, ClusterID: "c1", ResourceUID: "pod-unscored", Severity: "high"},
	})
	if rows[0].FinalLevel != "critical" || rows[0].FinalScore == nil || *rows[0].FinalScore != 74 {
		t.Fatalf("scored row: got level %q score %v", rows[0].FinalLevel, rows[0].FinalScore)
	}
	if rows[1].FinalLevel != "" || rows[1].FinalScore != nil {
		t.Fatalf("unscored row should have no level, got %q", rows[1].FinalLevel)
	}
	raw, _ := json.Marshal(rows[0])
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	if decoded["finalLevel"] != "critical" || decoded["severity"] != "low" || decoded["id"] == nil {
		t.Fatalf("json should keep the finding fields and add finalLevel: %s", raw)
	}
}
