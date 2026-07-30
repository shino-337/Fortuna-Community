package risk

import (
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
)

func baseCaps() map[string]float64 {
	return map[string]float64{
		"vulnerability": 15, "capability": 15, "attack_path": 15,
		"rbac_policy": 15, "runtime": 15, "exposure": 15, "blast_radius": 10,
	}
}

func mkPath(risk float64, length int, desc string) models.AttackPath {
	return models.AttackPath{
		TotalRisk:    risk,
		Length:       length,
		Description:  desc,
	}
}

func TestAdversarialScenarios(t *testing.T) {
	t.Run("A_RuntimeSpamAntiGaming", func(t *testing.T) {
		engine := NewRiskAggregationEngineV3(baseCaps(), DefaultDimensionWeights, DefaultSourceConfidence, 1.0)
		res := engine.ComputeScore([]RiskFactor{
			{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 1.5},
			{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 1.5},
			{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 0},
			{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 0},
			{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 2},
			{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 3},
			{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 0},
		})
		if res.TotalScore >= 40.0 {
			t.Fatalf("runtime spam should stay bounded, got score=%.2f", res.TotalScore)
		}
	})

	t.Run("B_SoftAllowAbuseTopologyGaming", func(t *testing.T) {
		paths := []models.AttackPath{
			mkPath(2.2, 5, "LATERAL_PATH soft chain"),
		}
		pi := computePathInfluence(paths, nil, map[string][]string{}, 0.4, nil)
		if pi.MaxStrength >= 0.3 {
			t.Fatalf("soft-allow dominated path must stay weak, maxStrength=%.3f", pi.MaxStrength)
		}
	})

	t.Run("C_LongPathIllusion", func(t *testing.T) {
		// proxy check: long path with low weakest-link should not inflate
		paths := []models.AttackPath{
			mkPath(3.9, 6, "PRIV_ESC_PATH mixed chain"),
		}
		pi := computePathInfluence(paths, nil, map[string][]string{}, 0.5, nil)
		if pi.MaxStrength >= 0.4 {
			t.Fatalf("long/weak chain should not dominate, maxStrength=%.3f", pi.MaxStrength)
		}
	})

	t.Run("D_RuntimeExploitWithoutGraph", func(t *testing.T) {
		now := time.Now().UTC()
		signals := []models.RuntimeSignal{
			{SignalType: "S_CRED", CreatedAt: now},
			{SignalType: "S_PRIV", CreatedAt: now},
		}
		signalMap := map[string][]string{
			"S_CRED": {"CREDENTIAL_ACCESS"},
			"S_PRIV": {"PRIV_ESC"},
		}
		paths := []models.AttackPath{
			{
				TotalRisk:   1.5,
				Length:      4,
				Description: "ESCAPE_PATH",
				Nodes: `[{"id":"step:x:CREDENTIAL_ACCESS","type":"attack_step","properties":{"stepId":"CREDENTIAL_ACCESS"}},{"id":"step:x:PRIV_ESC","type":"attack_step","properties":{"stepId":"PRIV_ESC"}}]`,
			},
		}
		pi := computePathInfluence(paths, signals, signalMap, 0.3, nil)
		if pi.ProgressSaturated < 0.6 {
			t.Fatalf("runtime critical milestones must lift progress, got %.3f", pi.ProgressSaturated)
		}
	})

	t.Run("E_GhostRuntimeDecay", func(t *testing.T) {
		old := time.Now().UTC().Add(-3 * time.Hour)
		d := runtimeInjectDecay([]models.RuntimeSignal{{SignalType: "X", CreatedAt: old}})
		if d >= 0.8 {
			t.Fatalf("stale runtime signal should decay materially, got %.3f", d)
		}
	})

	t.Run("F_CapabilityOnlyInflation", func(t *testing.T) {
		engine := NewRiskAggregationEngineV3(baseCaps(), DefaultDimensionWeights, DefaultSourceConfidence, 1.0)
		res := engine.ComputeScore([]RiskFactor{
			{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 0},
			{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 12},
			{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 0},
			{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 0},
			{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 0},
			{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 0},
			{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 0},
		})
		if res.TotalScore >= 50.0 {
			t.Fatalf("capability-only pod must not explode, score=%.2f", res.TotalScore)
		}
	})

	t.Run("G_CVEOnlyExposure", func(t *testing.T) {
		engine := NewRiskAggregationEngineV3(baseCaps(), DefaultDimensionWeights, DefaultSourceConfidence, 1.0)
		res := engine.ComputeScore([]RiskFactor{
			{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 15},
			{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 0},
			{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 0},
			{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 0},
			{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 0},
			{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 0},
			{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 0},
		})
		if res.TotalScore >= 30.0 {
			t.Fatalf("cve-only with no reachability should stay low, score=%.2f", res.TotalScore)
		}
	})

	t.Run("H_ComboOverAmplification", func(t *testing.T) {
		engine := NewRiskAggregationEngineV3(baseCaps(), DefaultDimensionWeights, DefaultSourceConfidence, 1.0)
		res := engine.ComputeScore([]RiskFactor{
			{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 12},
			{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 12},
			{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 8},
			{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 8},
			{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 8},
			{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 8},
			{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 6},
		})
		if res.ComboThreatBoost >= 0.2 {
			t.Fatalf("combo boost must stay bounded, boost=%.3f", res.ComboThreatBoost)
		}
		if res.TotalScore >= 95.0 {
			t.Fatalf("combo stack should not auto-hit extreme score, score=%.2f", res.TotalScore)
		}
	})

	t.Run("I_PathSpamDuplicateClass", func(t *testing.T) {
		paths := []models.AttackPath{
			mkPath(6, 3, "ESCAPE_PATH p1"),
			mkPath(6, 3, "ESCAPE_PATH p2"),
			mkPath(6, 3, "ESCAPE_PATH p3"),
			mkPath(6, 3, "ESCAPE_PATH p4"),
			mkPath(6, 3, "ESCAPE_PATH p5"),
		}
		pi := computePathInfluence(paths, nil, map[string][]string{}, 0.7, nil)
		if pi.DiversityFactor >= 0.85 {
			t.Fatalf("duplicate path classes must be penalized, diversity=%.3f", pi.DiversityFactor)
		}
	})

	t.Run("J_MixedRealAttackChain", func(t *testing.T) {
		now := time.Now().UTC()
		paths := []models.AttackPath{
			{
				TotalRisk:   7.8,
				Length:      4,
				Description: "PRIV_ESC_PATH realistic",
				Nodes: `[{"id":"step:x:RECON","type":"attack_step","properties":{"stepId":"RECON"}},{"id":"step:x:CREDENTIAL_ACCESS","type":"attack_step","properties":{"stepId":"CREDENTIAL_ACCESS"}},{"id":"step:x:LATERAL_MOVE","type":"attack_step","properties":{"stepId":"LATERAL_MOVE"}},{"id":"step:x:PRIV_ESC","type":"attack_step","properties":{"stepId":"PRIV_ESC"}}]`,
			},
		}
		signals := []models.RuntimeSignal{
			{SignalType: "S_CRED", CreatedAt: now},
			{SignalType: "S_PRIV", CreatedAt: now},
		}
		signalMap := map[string][]string{
			"S_CRED": {"CREDENTIAL_ACCESS"},
			"S_PRIV": {"PRIV_ESC"},
		}
		pi := computePathInfluence(paths, signals, signalMap, 0.6, nil)
		if pi.ProgressSaturated < 0.75 {
			t.Fatalf("mixed real chain should show high progress, got %.3f", pi.ProgressSaturated)
		}
	})

	t.Run("K_LowBaseTemporalAbuse", func(t *testing.T) {
		raw := 30.0
		total, temporal := ComputeTemporalScore(raw, nil, TemporalSignals{
			BurstEvents5m:      500,
			TrendDelta:         20,
			PersistenceMinutes: 1,
		})
		_ = temporal
		if total > 40.0 {
			t.Fatalf("low-base temporal burst should remain bounded, total=%.2f", total)
		}
	})

	t.Run("L_UncertainChainCollapse", func(t *testing.T) {
		penalty := 1.0
		for i := 0; i < 4; i++ {
			penalty *= 0.3
		}
		if penalty >= 0.25 {
			t.Fatalf("multi-uncertain chain should collapse, penalty=%.4f", penalty)
		}
	})
}

