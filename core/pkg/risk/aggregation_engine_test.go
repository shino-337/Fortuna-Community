package risk

import (
	"math"
	"testing"
)

func approxEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) < tolerance
}

// --- softCap unit tests (unchanged) ---

func TestSoftCap_BasicBehavior(t *testing.T) {
	tests := []struct {
		name   string
		x, cap float64
		want   float64
		tol    float64
	}{
		{"zero_input", 0, 15, 0, 0.01},
		{"zero_cap", 5, 0, 0, 0.01},
		{"small_input", 5, 15, 4.25, 0.1},
		{"equal_to_cap", 15, 15, 9.48, 0.1},
		{"large_input", 50, 15, 14.46, 0.1},
		{"very_large_input", 200, 15, 15.0, 0.01},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := softCap(tt.x, tt.cap)
			if !approxEqual(got, tt.want, tt.tol) {
				t.Errorf("softCap(%v, %v) = %v, want ~%v", tt.x, tt.cap, got, tt.want)
			}
		})
	}
}

func TestSoftCap_Monotonic(t *testing.T) {
	prev := 0.0
	for x := 1.0; x <= 100; x++ {
		got := softCap(x, 15)
		if got < prev {
			t.Errorf("softCap not monotonic: softCap(%v, 15)=%v < softCap(%v, 15)=%v", x, got, x-1, prev)
		}
		prev = got
	}
}

func TestSoftCap_BoundedByCap(t *testing.T) {
	for x := 1.0; x <= 1000; x += 10 {
		got := softCap(x, 15)
		if got > 15.0 {
			t.Errorf("softCap(%v, 15)=%v exceeds cap", x, got)
		}
	}
}

func TestSoftCap_MoreInputAlwaysMoreOutput(t *testing.T) {
	cap := 15.0
	one := softCap(1, cap)
	five := softCap(5, cap)
	fifty := softCap(50, cap)
	hundred := softCap(100, cap)

	if !(one < five && five < fifty && fifty < hundred) {
		t.Errorf("softCap should be strictly increasing: 1→%v, 5→%v, 50→%v, 100→%v",
			one, five, fifty, hundred)
	}
}

// --- NormalizeFactors tests ---

func TestNormalizeFactors_ConfidenceFromSource(t *testing.T) {
	engine := NewRiskAggregationEngineV3(
		map[string]float64{"vulnerability": 15},
		DefaultDimensionWeights,
		DefaultSourceConfidence,
		1.0,
	)
	factors := engine.NormalizeFactors([]RiskFactor{
		{FactorID: "f1", Category: "vulnerability", Source: "insight", Weight: 10},
		{FactorID: "f2", Category: "runtime", Source: "runtime", Weight: 10},
	})

	if factors[0].Confidence != 0.75 {
		t.Errorf("insight confidence: got %v, want 0.75", factors[0].Confidence)
	}
	if factors[1].Confidence != 1.0 {
		t.Errorf("runtime confidence: got %v, want 1.0", factors[1].Confidence)
	}
}

func TestNormalizeFactors_FreshnessApplied(t *testing.T) {
	engine := NewRiskAggregationEngine(map[string]float64{})
	factors := engine.NormalizeFactors([]RiskFactor{
		{FactorID: "f1", Category: "runtime", Source: "runtime", Weight: 10, Freshness: 0.5},
	})
	expected := 10 * 1.0 * 0.5 * 1.0 * 1.2
	if !approxEqual(factors[0].Contribution, expected, 0.01) {
		t.Errorf("contribution with freshness: got %v, want %v", factors[0].Contribution, expected)
	}
}

func TestNormalizeFactors_DimensionWeight(t *testing.T) {
	engine := NewRiskAggregationEngine(map[string]float64{})
	factors := engine.NormalizeFactors([]RiskFactor{
		{FactorID: "f1", Category: "capability", Source: "capability", Weight: 10},
		{FactorID: "f2", Category: "vulnerability", Source: "insight", Weight: 10},
	})
	if factors[0].Contribution <= factors[1].Contribution {
		t.Errorf("capability (%v) should outweigh vulnerability (%v) for same raw", factors[0].Contribution, factors[1].Contribution)
	}
}

// --- ApplyCategoryCaps tests ---

func TestApplyCategoryCaps_SoftCapApplied(t *testing.T) {
	engine := NewRiskAggregationEngine(map[string]float64{"vulnerability": 15})
	factors := []RiskFactor{
		{FactorID: "f1", Category: "vulnerability", Contribution: 30},
		{FactorID: "f2", Category: "vulnerability", Contribution: 20},
	}
	capped := engine.ApplyCategoryCaps(factors)

	totalCapped := 0.0
	for _, f := range capped {
		totalCapped += f.Contribution
	}
	if totalCapped >= 15 {
		t.Errorf("capped total %v should be < 15", totalCapped)
	}
	if totalCapped <= 0 {
		t.Errorf("capped total should be > 0, got %v", totalCapped)
	}
}

func TestApplyCategoryCaps_PreservesRatio(t *testing.T) {
	engine := NewRiskAggregationEngine(map[string]float64{"vulnerability": 15})
	factors := []RiskFactor{
		{FactorID: "f1", Category: "vulnerability", Contribution: 30},
		{FactorID: "f2", Category: "vulnerability", Contribution: 10},
	}
	capped := engine.ApplyCategoryCaps(factors)

	ratio := capped[0].Contribution / capped[1].Contribution
	if !approxEqual(ratio, 3.0, 0.01) {
		t.Errorf("ratio should be preserved at 3.0, got %v", ratio)
	}
}

// --- CrossFactorBoost tests ---

func TestCrossFactorBoost_VulnPlusExposure(t *testing.T) {
	engine := NewRiskAggregationEngine(map[string]float64{})
	factors := []RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Contribution: 10},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Contribution: 8},
	}
	catSums := map[string]float64{"vulnerability": 10, "exposure": 8}
	boost, _, combos := engine.applyCrossFactorBoost(factors, catSums)

	if boost <= 0 {
		t.Fatalf("expected positive boost for vuln+exposure, got %v", boost)
	}
	found := false
	for _, c := range combos {
		if c.Name == "cve_critical+internet_exposed" {
			found = true
			expectedBoost := 0.30 * (10 + 8)
			if !approxEqual(c.ComputedBoost, expectedBoost, 0.1) {
				t.Errorf("boost = %v, want ~%v", c.ComputedBoost, expectedBoost)
			}
		}
	}
	if !found {
		t.Error("cve_critical+internet_exposed combo not detected")
	}
}

func TestCrossFactorBoost_NoTriggerWhenInactive(t *testing.T) {
	engine := NewRiskAggregationEngine(map[string]float64{})
	factors := []RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Contribution: 10},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Contribution: 0},
	}
	catSums := map[string]float64{"vulnerability": 10, "exposure": 0}
	boost, _, _ := engine.applyCrossFactorBoost(factors, catSums)
	if boost != 0 {
		t.Errorf("expected zero boost when exposure is inactive, got %v", boost)
	}
}

// --- Tri-axial model tests ---

func TestComputeScore_TriAxial_AxesPopulated(t *testing.T) {
	engine := newTestEngine(1.0)
	result := engine.ComputeScore(fullRiskFactors())

	for _, name := range []string{"exploitability", "impact", "reachability"} {
		axis, ok := result.Axes[name]
		if !ok {
			t.Fatalf("missing axis %q", name)
		}
		if axis.NormScore <= 0 {
			t.Errorf("axis %q: NormScore should be > 0, got %v", name, axis.NormScore)
		}
		if axis.NormScore > 1 {
			t.Errorf("axis %q: NormScore should be <= 1, got %v", name, axis.NormScore)
		}
		if axis.Exponent <= 0 {
			t.Errorf("axis %q: Exponent should be > 0", name)
		}
		t.Logf("Axis %s: norm=%.3f raw=%.2f max=%.2f exp=%.1f comps=%v",
			name, axis.NormScore, axis.RawScore, axis.MaxScore, axis.Exponent, axis.Components)
	}
}

func TestComputeScore_GatingEffect(t *testing.T) {
	engine := newTestEngine(1.0)

	allAxes := engine.ComputeScore(fullRiskFactors())

	noImpact := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 15},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 0},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 10},
		{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 0},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 8},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 8},
		{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 0},
	})

	if noImpact.BaseRisk >= allAxes.BaseRisk {
		t.Errorf("missing impact axis should reduce base risk: noImpactBase=%.3f >= allAxesBase=%.3f",
			noImpact.BaseRisk, allAxes.BaseRisk)
	}
	t.Logf("Gating: allAxes total=%.2f base=%.3f; noImpact total=%.2f base=%.3f",
		allAxes.TotalScore, allAxes.BaseRisk, noImpact.TotalScore, noImpact.BaseRisk)
}

func TestComputeScore_ComboAmplification(t *testing.T) {
	engine := newTestEngine(1.0)

	withCombos := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 15},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 12},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 10},
		{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 13},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 8},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 8},
		{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 7},
	})

	if withCombos.ComboAmplifier != 1.0 {
		t.Errorf("combo amplifier should stay neutral after combo->axis injection refactor, got %v", withCombos.ComboAmplifier)
	}
	if len(withCombos.InteractionCombos) == 0 {
		t.Error("expected interaction combos for multi-dimension pod")
	}
	if withCombos.Axes["exploitability"].InjectionBoost <= 0 && withCombos.Axes["reachability"].InjectionBoost <= 0 {
		t.Error("expected combo signals to manifest via axis injection")
	}
	t.Logf("ComboAxisInjection: amplifier=%.3f combos=%d dE=%.3f dR=%.3f",
		withCombos.ComboAmplifier, len(withCombos.InteractionCombos),
		withCombos.Axes["exploitability"].InjectionBoost,
		withCombos.Axes["reachability"].InjectionBoost)
}

func TestComputeScore_OverrideRules(t *testing.T) {
	engine := newTestEngine(1.0)

	result := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 15},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 12},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 0},
		{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 0},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 0},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 8},
		{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 7},
	})

	if len(result.OverridesApplied) == 0 {
		t.Fatal("expected overrides for CVE critical + exposure + capability escape")
	}
	if result.TotalScore < 25 {
		t.Errorf("override injection should materially boost score, got %.2f", result.TotalScore)
	}
	if result.Axes["impact"].InjectionBoost <= 0 || result.Axes["reachability"].InjectionBoost <= 0 {
		t.Errorf("expected traceable axis injection, got impact=%.3f reach=%.3f",
			result.Axes["impact"].InjectionBoost, result.Axes["reachability"].InjectionBoost)
	}
	t.Logf("Override test: score=%.2f, overrides=%v", result.TotalScore, result.OverridesApplied)
}

func TestComputeScore_ContextMultiplier(t *testing.T) {
	capsMap := map[string]float64{
		"vulnerability": 15, "capability": 15, "attack_path": 15,
		"rbac_policy": 15, "runtime": 15, "exposure": 15, "blast_radius": 10,
	}
	// Use low weights that won't trigger override rules so context differences are visible.
	factors := []RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 8},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 5},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 3},
		{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 0},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 0},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 6},
		{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 2},
	}

	normal := NewRiskAggregationEngineV3(capsMap, DefaultDimensionWeights, DefaultSourceConfidence, 1.0).ComputeScore(factors)
	internet := NewRiskAggregationEngineV3(capsMap, DefaultDimensionWeights, DefaultSourceConfidence, 1.2).ComputeScore(factors)
	system := NewRiskAggregationEngineV3(capsMap, DefaultDimensionWeights, DefaultSourceConfidence, 0.9).ComputeScore(factors)

	t.Logf("context test: normal=%.2f internet=%.2f system=%.2f", normal.TotalScore, internet.TotalScore, system.TotalScore)

	if internet.TotalScore < normal.TotalScore {
		t.Errorf("internet-facing (%v) should score >= normal (%v)", internet.TotalScore, normal.TotalScore)
	}
	if system.TotalScore >= normal.TotalScore {
		t.Errorf("system pod (%v) should score < normal (%v)", system.TotalScore, normal.TotalScore)
	}
}

func TestComputeScore_ContextMultiplier_EnvSemantics(t *testing.T) {
	capsMap := map[string]float64{
		"vulnerability": 15, "capability": 15, "attack_path": 15,
		"rbac_policy": 15, "runtime": 15, "exposure": 15, "blast_radius": 10,
	}
	factors := []RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 10},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 7},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 4},
	}

	prod := NewRiskAggregationEngineV3(capsMap, DefaultDimensionWeights, DefaultSourceConfidence, 1.15).ComputeScore(factors)
	staging := NewRiskAggregationEngineV3(capsMap, DefaultDimensionWeights, DefaultSourceConfidence, 1.0).ComputeScore(factors)
	dev := NewRiskAggregationEngineV3(capsMap, DefaultDimensionWeights, DefaultSourceConfidence, 0.9).ComputeScore(factors)

	t.Logf("env context: prod=%.2f staging=%.2f dev=%.2f", prod.TotalScore, staging.TotalScore, dev.TotalScore)
	// Under the new Base*(1+Threat) model, prod/staging can both saturate at 100.
	// We require monotonic semantics (prod >= staging > dev), not strict prod > staging.
	if !(prod.TotalScore >= staging.TotalScore && staging.TotalScore > dev.TotalScore) {
		t.Errorf("expected prod >= staging > dev, got %.2f, %.2f, %.2f", prod.TotalScore, staging.TotalScore, dev.TotalScore)
	}
}

func TestComputeScore_ZeroFactors(t *testing.T) {
	engine := NewRiskAggregationEngine(map[string]float64{})
	result := engine.ComputeScore([]RiskFactor{})
	if result.TotalScore != 0 {
		t.Errorf("empty factors: score=%v, want 0", result.TotalScore)
	}
}

func TestComputeScore_KubeFlannel_Like(t *testing.T) {
	engine := NewRiskAggregationEngineV3(
		map[string]float64{
			"vulnerability": 15, "capability": 15, "attack_path": 15,
			"rbac_policy": 15, "runtime": 15, "exposure": 15, "blast_radius": 10,
		},
		DefaultDimensionWeights,
		DefaultSourceConfidence,
		1.2, // internet-facing
	)

	result := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 15},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 12},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 8},
		{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 13},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 0},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 8},
		{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 7},
	})

	// Acceptance: CVE critical + exposure + capability escape → must be >= 70 (CRITICAL)
	if result.TotalScore < 70 {
		t.Errorf("kube-flannel-like: score=%.2f, expected >= 70 (CRITICAL)", result.TotalScore)
	}
	if result.TotalScore > 100 {
		t.Errorf("kube-flannel-like: score=%.2f exceeds hard cap 100", result.TotalScore)
	}

	t.Logf("KubeFlannel: Score=%.2f, Boost=%.2f, Amplifier=%.3f, Overrides=%v",
		result.TotalScore, result.CrossFactorBoost, result.ComboAmplifier, result.OverridesApplied)
	for name, axis := range result.Axes {
		t.Logf("  Axis %s: norm=%.3f raw=%.2f comps=%v", name, axis.NormScore, axis.RawScore, axis.Components)
	}
	for _, c := range result.InteractionCombos {
		t.Logf("  Combo: %s → boost=%.2f", c.Name, c.ComputedBoost)
	}
}

func TestComputeScore_SingleCVE_NoOtherSignals(t *testing.T) {
	engine := newTestEngine(1.0)
	result := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 12},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 0},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 0},
		{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 0},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 0},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 0},
		{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 0},
	})

	// In rebalanced V3, single-CVE is latent structural risk in low-mid band.
	if result.TotalScore < 10 || result.TotalScore > 20 {
		t.Errorf("single CVE latent risk should stay in 10-20 band, got %.2f", result.TotalScore)
	}
	if len(result.OverridesApplied) > 0 {
		t.Errorf("no override should fire for single CVE, got %v", result.OverridesApplied)
	}
	t.Logf("SingleCVE: score=%.2f (latent base risk retained without active threat)", result.TotalScore)
}

func TestComputeScore_RuntimeExploit_Override(t *testing.T) {
	engine := newTestEngine(1.0)
	result := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 0},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 5},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 0},
		{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 0},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 12},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 6},
		{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 0},
	})

	if result.TotalScore < 35 {
		t.Errorf("active runtime exploit should receive strong amplification >= 35, got %.2f", result.TotalScore)
	}
	exp := result.Axes["exploitability"].InjectionBoost
	imp := result.Axes["impact"].InjectionBoost
	rea := result.Axes["reachability"].InjectionBoost
	if exp <= 0 || imp <= 0 || rea <= 0 {
		t.Errorf("runtime override should inject all axes, got dE=%.3f dI=%.3f dR=%.3f", exp, imp, rea)
	}
	if result.Axes["reachability"].InjectionK <= 0 {
		t.Errorf("expected injection_k to be exposed in explainability, got %.3f", result.Axes["reachability"].InjectionK)
	}
	if result.Axes["reachability"].Components["override_saturation_k"] <= 0 {
		t.Errorf("expected override_saturation_k in components")
	}
	if result.Axes["exploitability"].NormScore >= 1.0 && result.Axes["exploitability"].BaseNormScore < 0.95 {
		t.Errorf("soft saturation lost gradient for exploitability: base=%.3f norm=%.3f",
			result.Axes["exploitability"].BaseNormScore, result.Axes["exploitability"].NormScore)
	}
	t.Logf("RuntimeExploit: score=%.2f, overrides=%v", result.TotalScore, result.OverridesApplied)
}

func TestComputeScore_KubeFlannelEscapeHostAccess_NotUnderScored(t *testing.T) {
	engine := newTestEngine(1.2)
	result := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 15},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 12},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 0},
		{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 0},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 0},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 8},
		{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 0},
	})

	if result.Axes["exploitability"].NormScore < 0.7 {
		t.Fatalf("exploitability should be elevated by capability/exposure + override, got %.3f", result.Axes["exploitability"].NormScore)
	}
	if result.TotalScore < 70 {
		t.Fatalf("kube-flannel-like case is under-scored: got %.2f, expected >= 70", result.TotalScore)
	}
	t.Logf("KubeFlannel-like: score=%.2f E=%.3f I=%.3f R=%.3f combo=%.2f overrides=%v",
		result.TotalScore,
		result.Axes["exploitability"].NormScore,
		result.Axes["impact"].NormScore,
		result.Axes["reachability"].NormScore,
		result.ComboAmplifier,
		result.OverridesApplied,
	)
}

func TestComputeScore_ExposureNotDoubleCounted(t *testing.T) {
	engine := newTestEngine(1.0)

	lowExposure := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 5},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 2},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 0},
		{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 0},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 0},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 6},
		{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 0},
	})

	veryHighExposure := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 5},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 2},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 0},
		{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 0},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 0},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 12},
		{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 0},
	})

	if lowExposure.TotalScore <= 0 {
		t.Fatalf("low exposure scenario should have positive score")
	}
	ratio := veryHighExposure.TotalScore / lowExposure.TotalScore
	if ratio > 1.65 {
		t.Fatalf("exposure over-amplified: low=%.2f high=%.2f ratio=%.2f", lowExposure.TotalScore, veryHighExposure.TotalScore, ratio)
	}
	t.Logf("Exposure amplification ratio=%.2f (low=%.2f high=%.2f)", ratio, lowExposure.TotalScore, veryHighExposure.TotalScore)
}

func TestComputeScore_InternalOnlyPod_NotOverScored(t *testing.T) {
	engine := newTestEngine(1.0)
	result := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 6},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 4},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 0},
		{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 0},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 0},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 0},
		{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 0},
	})

	if result.TotalScore > 35 {
		t.Fatalf("internal-only pod should not be over-scored, got %.2f", result.TotalScore)
	}
	t.Logf("InternalOnly: score=%.2f E=%.3f I=%.3f R=%.3f", result.TotalScore,
		result.Axes["exploitability"].NormScore,
		result.Axes["impact"].NormScore,
		result.Axes["reachability"].NormScore)
}

func TestComputeScore_LatentRiskNotCollapsed(t *testing.T) {
	engine := newTestEngine(1.0)
	result := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 15},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 12},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 0},
		{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 6},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 0},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 0},
		{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 2},
	})

	if result.TotalScore < 20 {
		t.Fatalf("latent risk collapsed unexpectedly: got %.2f", result.TotalScore)
	}
	if result.Axes["reachability"].Components["internal_reachability"] <= 0 {
		t.Fatalf("internal reachability should contribute for latent internal threat model")
	}
	t.Logf("LatentRisk: score=%.2f, overrides=%v, reach_components=%v",
		result.TotalScore, result.OverridesApplied, result.Axes["reachability"].Components)
}

func TestDeduplicateFactors_MaxContribution(t *testing.T) {
	engine := NewRiskAggregationEngine(map[string]float64{})
	factors := []RiskFactor{
		{FactorID: "f1", Category: "vulnerability", DedupeKey: "cve-1", Contribution: 5},
		{FactorID: "f1", Category: "vulnerability", DedupeKey: "cve-1", Contribution: 8},
		{FactorID: "f2", Category: "vulnerability", DedupeKey: "cve-2", Contribution: 3},
	}
	deduped, agg := engine.DeduplicateFactors(factors)

	if len(deduped) != 2 {
		t.Fatalf("expected 2 deduped factors, got %d", len(deduped))
	}
	for _, a := range agg {
		if a.BaseFactor.DedupeKey == "cve-1" {
			if a.BaseFactor.Contribution != 8 {
				t.Errorf("expected max contribution 8, got %v", a.BaseFactor.Contribution)
			}
			if len(a.Instances) != 2 {
				t.Errorf("expected 2 instances, got %d", len(a.Instances))
			}
		}
	}
}

func TestComputeScore_BaseRiskInvariantAcrossContext(t *testing.T) {
	capsMap := map[string]float64{
		"vulnerability": 15, "capability": 15, "attack_path": 15,
		"rbac_policy": 15, "runtime": 15, "exposure": 15, "blast_radius": 10,
	}
	factors := []RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 10},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 8},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 4},
	}

	prod := NewRiskAggregationEngineV3(capsMap, DefaultDimensionWeights, DefaultSourceConfidence, 1.15).ComputeScore(factors)
	dev := NewRiskAggregationEngineV3(capsMap, DefaultDimensionWeights, DefaultSourceConfidence, 0.9).ComputeScore(factors)
	if !approxEqual(prod.BaseRisk, dev.BaseRisk, 0.001) {
		t.Fatalf("base risk must be context invariant: prod=%.3f dev=%.3f", prod.BaseRisk, dev.BaseRisk)
	}
}

func TestComputeScore_CapabilityGatingWhenNoVuln(t *testing.T) {
	engine := newTestEngine(1.0)
	highCapNoVuln := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 0},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 12},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 0},
		{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 0},
	})
	highCapWithVuln := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 12},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 12},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 0},
		{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 0},
	})
	if highCapNoVuln.BaseRisk >= highCapWithVuln.BaseRisk {
		t.Fatalf("capability gating broken: noVuln=%.3f withVuln=%.3f", highCapNoVuln.BaseRisk, highCapWithVuln.BaseRisk)
	}
}

func TestComputeScore_ThreatCapRelativeToBase(t *testing.T) {
	engine := newTestEngine(1.0)
	lowBase := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 0},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 2},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 10},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 6},
	})
	if lowBase.ThreatAmplifier > lowBase.MaxThreatAmplifier+0.001 {
		t.Fatalf("threat amplifier exceeds cap: amp=%.3f cap=%.3f", lowBase.ThreatAmplifier, lowBase.MaxThreatAmplifier)
	}
}

func TestComputeScore_Invariant_NoVulnNoRuntimeNoPathStaysLow(t *testing.T) {
	engine := newTestEngine(1.0)
	res := engine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 0},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 8},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 0},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 0},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 0},
		{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 0},
		{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 0},
	})
	if res.TotalScore >= 25 {
		t.Fatalf("invariant violated: no vuln/runtime/path should stay low, got %.2f", res.TotalScore)
	}
}

// --- helpers ---

func newTestEngine(ctxMul float64) *RiskAggregationEngine {
	return NewRiskAggregationEngineV3(
		map[string]float64{
			"vulnerability": 15, "capability": 15, "attack_path": 15,
			"rbac_policy": 15, "runtime": 15, "exposure": 15, "blast_radius": 10,
		},
		DefaultDimensionWeights,
		DefaultSourceConfidence,
		ctxMul,
	)
}

func fullRiskFactors() []RiskFactor {
	return []RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", Weight: 15},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", Weight: 12},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", Weight: 10},
		{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", Weight: 13},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", Weight: 8},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", Weight: 8},
		{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", Weight: 7},
	}
}
