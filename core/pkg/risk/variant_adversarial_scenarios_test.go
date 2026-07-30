package risk

import (
	"math"
	"testing"

	"github.com/fortuna/core/pkg/models"
)

func TestExecutionSemantics_PartialExecutionIllusion(t *testing.T) {
	engine := NewRiskAggregationEngineV3(baseCaps(), DefaultDimensionWeights, DefaultSourceConfidence, 1.0)
	res := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 12},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 2},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 1.5},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 3},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 8},
	})
	if res.TotalScore >= 40 {
		t.Fatalf("partial execution illusion should stay <40, got %.2f", res.TotalScore)
	}
}

func TestExecutionSemantics_FakeStructuralExecution(t *testing.T) {
	engine := NewRiskAggregationEngineV3(baseCaps(), DefaultDimensionWeights, DefaultSourceConfidence, 1.0)
	res := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 3},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 13},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 0},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 0},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 0},
		{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 0},
	})
	if res.TotalScore >= 35 {
		t.Fatalf("fake structural execution should stay <35, got %.2f", res.TotalScore)
	}
}

func TestAxisBehavior_HighEI_LowR(t *testing.T) {
	engine := NewRiskAggregationEngineV3(baseCaps(), DefaultDimensionWeights, DefaultSourceConfidence, 1.0)
	res := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 15},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 12},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 0},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 0},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 0},
	})
	if res.TotalScore >= 50 {
		t.Fatalf("high E/I with low R should remain bounded, got %.2f", res.TotalScore)
	}
}

func TestAxisBehavior_NearSaturationNot100(t *testing.T) {
	engine := NewRiskAggregationEngineV3(baseCaps(), DefaultDimensionWeights, DefaultSourceConfidence, 1.0)
	res := engine.ComputeScore(fullRiskFactors())
	if res.TotalScore >= 95 {
		t.Fatalf("near saturation should not hit 100 easily, got %.2f", res.TotalScore)
	}
}

func TestInjectionBudget_OverrideVsComboCompetition(t *testing.T) {
	engine := NewRiskAggregationEngineV3(baseCaps(), DefaultDimensionWeights, DefaultSourceConfidence, 1.0)
	res := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 10},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 8},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 12},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 8},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 4},
		{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 4},
	})
	if res.TotalScore < 65 || res.TotalScore > 85 {
		t.Fatalf("override/combo competition should remain in 65..85, got %.2f", res.TotalScore)
	}
}

func TestInjectionBudget_ComboOnlyMultiChain(t *testing.T) {
	engine := NewRiskAggregationEngineV3(baseCaps(), DefaultDimensionWeights, DefaultSourceConfidence, 1.0)
	res := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 12},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 12},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 12},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 8},
		{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 10},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 0},
	})
	if res.TotalScore >= 90 {
		t.Fatalf("combo-only multichain should stay <90, got %.2f", res.TotalScore)
	}
}

func TestTemporalBehavior_BurstDecayConflict(t *testing.T) {
	score, detail := ComputeTemporalScore(60, nil, TemporalSignals{
		BurstEvents5m:       120,
		UniqueSignalTypes5m: 1,
		InactivityMinutes:   720,
	})
	if detail.Multiplier > 1.1 {
		t.Fatalf("burst/decay conflict must keep multiplier <=1.1, got %.3f", detail.Multiplier)
	}
	if score > 66 {
		t.Fatalf("temporal score should stay bounded, got %.2f", score)
	}
}

func TestPathInfluence_StrongPathZeroRuntime(t *testing.T) {
	paths := []models.AttackPath{
		mkPath(7.0, 3, "PRIV_ESC_PATH"),
		mkPath(6.5, 3, "LATERAL_PATH"),
	}
	pi := computePathInfluence(paths, nil, map[string][]string{}, 0.6, nil)
	total := pi.EInjectPoints + pi.RInjectPoints + pi.IInjectPoints
	if total < 5.0 || total > 11.0 {
		t.Fatalf("strong path without runtime should be latent-high but bounded, inject=%.2f", total)
	}
}

func TestPathInfluence_DuplicateDisguisedSemantics(t *testing.T) {
	paths := []models.AttackPath{
		{TotalRisk: 6.0, Length: 3, Description: "ESCAPE_PATH", Edges: `[{"from":"p1","to":"rb1"},{"from":"rb1","to":"cr1"}]`},
		{TotalRisk: 6.1, Length: 3, Description: "ESCAPE_PATH", Edges: `[{"from":"p2","to":"rb2"},{"from":"rb2","to":"cr2"}]`},
		{TotalRisk: 5.9, Length: 3, Description: "ESCAPE_PATH", Edges: `[{"from":"p3","to":"rb3"},{"from":"rb3","to":"cr3"}]`},
	}
	pi := computePathInfluence(paths, nil, map[string][]string{}, 0.7, nil)
	if pi.DiversityFactor >= 0.85 {
		t.Fatalf("disguised duplicate paths must still lower diversity, got %.3f", pi.DiversityFactor)
	}
}

func TestBaseRiskIntegrity_HighExposureLowVuln(t *testing.T) {
	engine := NewRiskAggregationEngineV3(baseCaps(), DefaultDimensionWeights, DefaultSourceConfidence, 1.0)
	res := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 1.5},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 13},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 0},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 0},
	})
	if res.TotalScore >= 30 {
		t.Fatalf("high exposure + low vuln should stay <30, got %.2f", res.TotalScore)
	}
}

func TestBaseRiskIntegrity_HighRBACNoPath(t *testing.T) {
	engine := NewRiskAggregationEngineV3(baseCaps(), DefaultDimensionWeights, DefaultSourceConfidence, 1.0)
	res := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 14},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 0},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 0},
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 4},
	})
	if res.TotalScore < 40 || res.TotalScore > 60 {
		t.Fatalf("high RBAC + no path should stay in 40..60, got %.2f", res.TotalScore)
	}
}

func TestBoundaryInvariant_ZeroAndMax(t *testing.T) {
	engine := NewRiskAggregationEngineV3(baseCaps(), DefaultDimensionWeights, DefaultSourceConfidence, 1.0)
	zero := engine.ComputeScore([]RiskFactor{})
	if zero.TotalScore != 0 {
		t.Fatalf("zero everything should be zero, got %.2f", zero.TotalScore)
	}
	maxed := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 100},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 100},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 100},
		{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 100},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 100},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 100},
		{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 100},
	})
	if math.IsNaN(maxed.TotalScore) || maxed.TotalScore > 100 {
		t.Fatalf("max bound invariant violated, score=%v", maxed.TotalScore)
	}
}

func TestBoundaryInvariant_Monotonicity(t *testing.T) {
	engine := NewRiskAggregationEngineV3(baseCaps(), DefaultDimensionWeights, DefaultSourceConfidence, 1.0)
	low := engine.ComputeScore([]RiskFactor{
		{FactorID: "V", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 4},
		{FactorID: "R", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 1},
		{FactorID: "P", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 1},
	})
	highV := engine.ComputeScore([]RiskFactor{
		{FactorID: "V", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 10},
		{FactorID: "R", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 1},
		{FactorID: "P", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 1},
	})
	highR := engine.ComputeScore([]RiskFactor{
		{FactorID: "V", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 4},
		{FactorID: "R", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 8},
		{FactorID: "P", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 1},
	})
	highP := engine.ComputeScore([]RiskFactor{
		{FactorID: "V", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 4},
		{FactorID: "R", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 1},
		{FactorID: "P", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 8},
	})
	if highV.TotalScore < low.TotalScore || highR.TotalScore < low.TotalScore || highP.TotalScore < low.TotalScore {
		t.Fatalf("monotonicity violated: low=%.2f highV=%.2f highR=%.2f highP=%.2f",
			low.TotalScore, highV.TotalScore, highR.TotalScore, highP.TotalScore)
	}
}

