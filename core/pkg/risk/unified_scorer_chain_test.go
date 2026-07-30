package risk

import (
	"context"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// Wave 5 regression shield:
// Ensure Layer 3 (attack_paths) + capability state are reflected in Layer 4 (Unified Scorer V3).
func TestUnifiedScorerV3_UsesAttackPathAndCapabilitySignals(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&models.Pod{},
		&models.Insight{},
		&models.PodCapability{},
		&models.AttackPath{},
		&models.RuntimeSignal{},
		&models.RiskScore{},
	); err != nil {
		t.Fatal(err)
	}
	// Match production conflict target used by SaveScoreV3.
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_test_risk_scores_key ON risk_scores(resource_type, resource_uid, cluster_id)").Error; err != nil {
		t.Fatalf("create unique index: %v", err)
	}

	podUID := "pod-wave5-1"
	now := time.Now()
	if err := db.Create(&models.Pod{
		UID:            podUID,
		Name:           "wave5-pod",
		Namespace:      "default",
		ClusterID:      "c1",
		ServiceAccount: "default",
		HostNetwork:    true, // contributes to exposure dimension + toxic combo condition
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Create(&models.Insight{
		ResourceType:      "Pod",
		ResourceUID:       podUID,
		ResourceName:      "wave5-pod",
		ResourceNamespace: "default",
		InsightType:       "vulnerability",
		Severity:          "critical",
		Title:             "critical cve",
		Status:            "active",
		CVSS:              9.8,
		DetectedAt:        now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Create(&models.PodCapability{
		PodUID:          podUID,
		Namespace:       "default",
		CapabilityID:    "ESC_PRIV_POD",
		CapabilityGroup: "ESC",
		Severity:        "critical",
		State:           "exploited",
		Confidence:      0.95,
		Evidence:        "{}",
		DerivedFrom:     "{}",
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Create(&models.AttackPath{
		PodUID:          podUID,
		PathID:          "p1",
		Nodes:           `[{"id":"step:pod-wave5-1:NETWORK_SNIFFING","type":"attack_step","properties":{"stepId":"NETWORK_SNIFFING"}}]`,
		Edges:           "[]",
		TotalRisk:       9.5,
		Difficulty:      0.2,
		Impact:          1.0,
		Length:          3,
		Description:     "cluster-admin path with full cluster access",
		EnrichedFromPCE: true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.RuntimeSignal{
		PodUID:     podUID,
		SignalType: "CAPABILITY_MISUSE",
		Category:   "runtime",
		Confidence: 0.9,
		Evidence:   `{"src":"test"}`,
		Count:      1,
		CreatedAt:  now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	scorer := NewUnifiedScorerV3(db)
	score, err := scorer.CalculateScoreV3(context.Background(), podUID)
	if err != nil {
		t.Fatalf("calculate score v3: %v", err)
	}

	if score.AttackPathScore <= 0 {
		t.Fatalf("expected attackPath dimension > 0, got %.2f", score.AttackPathScore)
	}
	if score.BlastRadiusScore <= 0 {
		t.Fatalf("expected blastRadius dimension > 0, got %.2f", score.BlastRadiusScore)
	}
	if len(score.ToxicCombos) == 0 {
		t.Fatalf("expected at least one toxic combo, got none")
	}
	if score.TotalScore <= 0 {
		t.Fatalf("expected total score > 0, got %.2f", score.TotalScore)
	}
	reasoning, ok := score.Factors["attack_path_reasoning"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected attack_path_reasoning block in factors")
	}
	if v, ok := reasoning["e_inject_points"].(float64); !ok || v <= 0 {
		t.Fatalf("expected e_inject_points > 0, got %#v", reasoning["e_inject_points"])
	}
	if v, ok := reasoning["r_inject_points"].(float64); !ok || v <= 0 {
		t.Fatalf("expected r_inject_points > 0, got %#v", reasoning["r_inject_points"])
	}
	if v, ok := reasoning["runtime_step_progress"].(float64); !ok || v <= 0 {
		t.Fatalf("expected runtime_step_progress > 0, got %#v", reasoning["runtime_step_progress"])
	}

	if err := scorer.SaveScoreV3(context.Background(), score); err != nil {
		t.Fatalf("save score v3: %v", err)
	}

	var saved models.RiskScore
	if err := db.Where("resource_uid = ?", podUID).First(&saved).Error; err != nil {
		t.Fatalf("load saved score: %v", err)
	}
	if saved.ScorerVersion != "v3" {
		t.Fatalf("expected scorer_version=v3, got %q", saved.ScorerVersion)
	}
	if saved.AttackPathScore <= 0 || saved.BlastRadiusScore <= 0 {
		t.Fatalf("expected persisted v3 dimension scores > 0, got attackPath=%.2f blastRadius=%.2f", saved.AttackPathScore, saved.BlastRadiusScore)
	}
}

func TestUnifiedScorerV3_NoAttackPathNoHeuristicFloor(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&models.Pod{},
		&models.Insight{},
		&models.PodCapability{},
		&models.AttackPath{},
		&models.RiskScore{},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_test_risk_scores_key ON risk_scores(resource_type, resource_uid, cluster_id)").Error; err != nil {
		t.Fatalf("create unique index: %v", err)
	}

	podUID := "pod-system-daemonset-1"
	now := time.Now()
	if err := db.Create(&models.Pod{
		UID:            podUID,
		Name:           "cni-node-agent",
		Namespace:      "kube-system",
		ClusterID:      "c1",
		ServiceAccount: "default",
		OwnerKind:      "DaemonSet",
		HostNetwork:    true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Insight{
		ResourceType:      "Pod",
		ResourceUID:       podUID,
		ResourceName:      "cni-node-agent",
		ResourceNamespace: "kube-system",
		InsightType:       "vulnerability",
		Severity:          "critical",
		Title:             "critical cve",
		Status:            "active",
		CVSS:              9.8,
		DetectedAt:        now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	scorer := NewUnifiedScorerV3(db)
	score, err := scorer.CalculateScoreV3(context.Background(), podUID)
	if err != nil {
		t.Fatalf("calculate score v3: %v", err)
	}
	if score.AttackPathScore != 0 {
		t.Fatalf("expected attack-path score to remain 0 without materialized paths, got %.2f", score.AttackPathScore)
	}
}

func TestLoadRuntimeSignalStepMappings_DBOverride(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.RuntimeSignalStepMapping{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.RuntimeSignalStepMapping{
		SignalType: "CUSTOM_SIG",
		StepID:     "CUSTOM_STEP",
		Enabled:    true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	m := loadRuntimeSignalStepMappings(db)
	steps := m["CUSTOM_SIG"]
	if len(steps) != 1 || steps[0] != "CUSTOM_STEP" {
		t.Fatalf("expected DB mapping CUSTOM_SIG->CUSTOM_STEP, got %#v", steps)
	}
}
