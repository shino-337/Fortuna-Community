package risk

import (
	"math"
	"sort"
	"strings"
)

// RiskFactor is the canonical normalized unit used by the aggregation engine.
type RiskFactor struct {
	FactorID     string   `json:"factor_id"`
	Category     string   `json:"category"`
	Scope        string   `json:"scope"`
	Source       string   `json:"source"`
	DedupeKey    string   `json:"dedupe_key"`
	Weight       float64  `json:"weight"`
	Confidence   float64  `json:"confidence"`
	Freshness    float64  `json:"freshness"`
	Contribution float64  `json:"contribution"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

// AggregatedFactor preserves all factor instances sharing the same dedupe key.
type AggregatedFactor struct {
	BaseFactor          RiskFactor   `json:"base_factor"`
	Instances           []RiskFactor `json:"instances"`
	AggregationStrategy string       `json:"aggregation_strategy"`
}

// InteractionCombo describes a detected toxic combination and its computed boost.
type InteractionCombo struct {
	Name           string  `json:"name"`
	Multiplier     float64 `json:"multiplier"`
	RelatedDimSum  float64 `json:"related_dim_sum"`
	ComputedBoost  float64 `json:"computed_boost"`
}

// AxisScore represents one of the three risk axes in the multiplicative model.
// Risk = 100 × Exploitability^α × Impact^β × Reachability^γ
type AxisScore struct {
	Name            string             `json:"name"`
	RawScore        float64            `json:"raw_score"`
	NormScore       float64            `json:"norm_score"`
	BaseNormScore   float64            `json:"base_norm_score"`
	InjectionBoost  float64            `json:"injection_boost"`
	InjectionK      float64            `json:"injection_k"`
	MaxScore        float64            `json:"max_score"`
	Exponent        float64            `json:"exponent"`
	Components      map[string]float64 `json:"components"`
}

// AggregationResult provides score breakdown for explainability.
type AggregationResult struct {
	Factors            []RiskFactor            `json:"factors"`
	AggregatedFactors  []AggregatedFactor      `json:"aggregated_factors,omitempty"`
	CategorySums       map[string]float64      `json:"category_sums"`
	CategoryCaps       map[string]float64      `json:"category_caps"`
	PreCapScoreRaw     float64                 `json:"pre_cap_score_raw"`
	PostCapScoreRaw    float64                 `json:"post_cap_score_raw"`
	CrossFactorBoost   float64                 `json:"cross_factor_boost"`
	TotalScoreRaw      float64                 `json:"total_score_raw"`
	TotalScore         float64                 `json:"total_score"`
	ExplanationSummary string                  `json:"explanation_summary,omitempty"`
	RecommendedActions []string                `json:"recommended_actions,omitempty"`
	TopContributors    []map[string]interface{} `json:"top_contributors,omitempty"`

	InteractionCombos  []InteractionCombo      `json:"interaction_combos,omitempty"`
	ActiveDimCount     int                     `json:"active_dim_count"`
	TotalDimCount      int                     `json:"total_dim_count"`
	DimNormMultiplier  float64                 `json:"dim_norm_multiplier"`
	ContextMultiplier  float64                 `json:"context_multiplier"`

	Axes             map[string]AxisScore `json:"axes,omitempty"`
	ComboAmplifier   float64             `json:"combo_amplifier"`
	BaseRisk         float64             `json:"base_risk"`
	ThreatCore       float64             `json:"threat_core"`
	ThreatAmplifierRaw float64           `json:"threat_amplifier_raw"`
	ThreatAmplifier  float64             `json:"threat_amplifier"`
	ComboThreatBoost float64             `json:"combo_threat_boost,omitempty"`
	MaxThreatAmplifier float64           `json:"max_threat_amplifier"`
	FinalFormula     string              `json:"final_formula,omitempty"`
	OverridesApplied []string            `json:"overrides_applied,omitempty"`
}

// RiskAggregationEngine normalizes factors and computes final score.
type RiskAggregationEngine struct {
	categoryCaps      map[string]float64
	dimensionWeights  map[string]float64
	sourceConfidence  map[string]float64
	scopeImpactByName map[string]float64
	contextMultiplier float64
}

// DefaultSourceConfidence provides confidence priors per data source.
// Runtime signals are highest (observed), static CVE matching lowest (inferred).
var DefaultSourceConfidence = map[string]float64{
	"runtime":     1.0,
	"capability":  0.85,
	"pod":         0.95,
	"policy":      0.90,
	"insight":     0.75,
	"attack_path": 0.90,
	"interaction": 1.0,
}

// DefaultDimensionWeights scale dimensions by operational importance.
// Capability and runtime are elevated (actionable execution power);
// vulnerability is slightly reduced (noise-prone, scan-dependent).
var DefaultDimensionWeights = map[string]float64{
	"capability":  1.20,
	"runtime":     1.20,
	"attack_path": 1.10,
	"rbac_policy": 1.00,
	"exposure":    1.00,
	"blast_radius":1.00,
	"vulnerability":0.90,
	"interaction": 1.00,
}

func NewRiskAggregationEngine(categoryCaps map[string]float64) *RiskAggregationEngine {
	return NewRiskAggregationEngineV3(categoryCaps, DefaultDimensionWeights, DefaultSourceConfidence, 1.0)
}

func NewRiskAggregationEngineV3(
	categoryCaps map[string]float64,
	dimWeights map[string]float64,
	sourceConf map[string]float64,
	contextMul float64,
) *RiskAggregationEngine {
	caps := map[string]float64{}
	for k, v := range categoryCaps {
		caps[strings.ToLower(strings.TrimSpace(k))] = v
	}
	weights := map[string]float64{}
	for k, v := range dimWeights {
		weights[strings.ToLower(strings.TrimSpace(k))] = v
	}
	conf := map[string]float64{}
	for k, v := range sourceConf {
		conf[strings.ToLower(strings.TrimSpace(k))] = v
	}
	if contextMul <= 0 {
		contextMul = 1.0
	}
	return &RiskAggregationEngine{
		categoryCaps:     caps,
		dimensionWeights: weights,
		sourceConfidence: conf,
		scopeImpactByName: map[string]float64{
			"pod":       1.0,
			"workload":  1.2,
			"namespace": 1.5,
			"cluster":   2.0,
		},
		contextMultiplier: contextMul,
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// softCap applies exponential saturation: cap * (1 - e^(-x/cap)).
// Unlike hard cap, 56 CVEs scores higher than 1 CVE, while still bounded.
func softCap(x, cap float64) float64 {
	if cap <= 0 || x <= 0 {
		return 0
	}
	return cap * (1.0 - math.Exp(-x/cap))
}

// Tri-axial multiplicative model parameters.
// Risk = 100 × E^α × I^β × R^γ
const (
	alphaExploitability = 1.2
	betaImpact          = 1.3
	gammaReachability   = 1.1
	axisFloor           = 0.15
	axisFloorElevated   = 0.25
	axisFloorHighSignal = 0.30
	maxToxicBoost       = 20.0
	maxComboAmplifier   = 1.8
	maxThreatAmplifier  = 1.5
	overrideSoftCapK    = 6.0
	baseWeightVuln       = 0.40
	baseWeightCapability = 0.40
	baseWeightExposure  = 0.10
	baseWeightRBAC      = 0.05
)

func (e *RiskAggregationEngine) getCap(category string) float64 {
	if c, ok := e.categoryCaps[category]; ok && c > 0 {
		return c
	}
	return 15.0
}

// computeEffectiveCatMax returns the practical soft-capped maximum per category,
// derived from actual factor metadata (confidence, freshness, scope, dim weight).
func (e *RiskAggregationEngine) computeEffectiveCatMax(normalized []RiskFactor) map[string]float64 {
	result := make(map[string]float64, len(normalized))
	for _, f := range normalized {
		if _, ok := result[f.Category]; ok {
			continue
		}
		cap := e.getCap(f.Category)
		scopeImpact := e.scopeImpactByName[f.Scope]
		if scopeImpact <= 0 {
			scopeImpact = 1.0
		}
		dimWeight := 1.0
		if w, ok := e.dimensionWeights[f.Category]; ok && w > 0 {
			dimWeight = w
		}
		simContrib := cap * f.Confidence * f.Freshness * scopeImpact * dimWeight
		result[f.Category] = softCap(simContrib, cap)
	}
	return result
}

func catNormValue(sum, effectiveMax float64) float64 {
	if effectiveMax <= 0 || sum <= 0 {
		return 0
	}
	return math.Min(1.0, sum/effectiveMax)
}

func (e *RiskAggregationEngine) computeAxisScores(
	categorySums map[string]float64,
	effectiveCatMax map[string]float64,
	interactionBoost float64,
) map[string]AxisScore {
	getMax := func(cat string) float64 {
		if m, ok := effectiveCatMax[cat]; ok && m > 0 {
			return m
		}
		cap := e.getCap(cat)
		return softCap(cap*0.85, cap)
	}

	runtimeNorm := catNormValue(categorySums["runtime"], getMax("runtime"))
	attackPathNorm := catNormValue(categorySums["attack_path"], getMax("attack_path"))
	rawExposureEnabler := catNormValue(categorySums["exposure"], getMax("exposure"))
	exposureEnabler := math.Sqrt(rawExposureEnabler)
	// Hard gate: no execution evidence (runtime/path) means exposure must not
	// leak into exploitability axis.
	if runtimeNorm == 0 && attackPathNorm == 0 {
		exposureEnabler = 0
	}
	exploitComps := map[string]float64{
		"vulnerability":      catNormValue(categorySums["vulnerability"], getMax("vulnerability")),
		"runtime":            runtimeNorm,
		"attack_path":        attackPathNorm,
		"exposure_enabler":   exposureEnabler,
	}
	exploitNorm := 0.6*exploitComps["vulnerability"] +
		0.2*exploitComps["runtime"] +
		0.15*exploitComps["attack_path"] +
		0.05*exploitComps["exposure_enabler"]

	impactComps := map[string]float64{
		"capability":   catNormValue(categorySums["capability"], getMax("capability")),
		"rbac_policy":  catNormValue(categorySums["rbac_policy"], getMax("rbac_policy")),
		"blast_radius": catNormValue(categorySums["blast_radius"], getMax("blast_radius")),
	}
	capabilityImpactBoost := math.Pow(impactComps["capability"], 0.75)
	impactComps["capability_boosted"] = capabilityImpactBoost
	impactNorm := 0.6*capabilityImpactBoost + 0.25*impactComps["rbac_policy"] + 0.15*impactComps["blast_radius"]

	_ = interactionBoost
	internalReachNorm := clamp01(
		0.55*impactComps["rbac_policy"] +
			0.45*exploitComps["attack_path"],
	)
	reachComps := map[string]float64{
		"exposure":             catNormValue(categorySums["exposure"], getMax("exposure")),
		"internal_reachability": internalReachNorm,
	}
	reachNorm := 0.45*reachComps["exposure"] + 0.55*reachComps["internal_reachability"]

	exploitRaw := 0.6*categorySums["vulnerability"] +
		0.2*categorySums["runtime"] +
		0.15*categorySums["attack_path"] +
		0.05*exposureEnabler*getMax("exposure")
	impactRaw := 0.6*categorySums["capability"] + 0.25*categorySums["rbac_policy"] + 0.15*categorySums["blast_radius"]
	reachRaw := 0.45*categorySums["exposure"] +
		0.55*(0.55*categorySums["rbac_policy"]+0.45*categorySums["attack_path"])

	exploitMax := 0.6*getMax("vulnerability") +
		0.2*getMax("runtime") +
		0.15*getMax("attack_path") +
		0.05*getMax("exposure")
	impactMax := 0.6*getMax("capability") + 0.25*getMax("rbac_policy") + 0.15*getMax("blast_radius")
	reachMax := 0.45*getMax("exposure") +
		0.55*(0.55*getMax("rbac_policy")+0.45*getMax("attack_path"))

	r2 := func(v float64) float64 { return math.Round(v*100) / 100 }
	r3 := func(v float64) float64 { return math.Round(v*1000) / 1000 }
	rmap := func(m map[string]float64) map[string]float64 {
		out := make(map[string]float64, len(m))
		for k, v := range m {
			out[k] = r3(v)
		}
		return out
	}

	return map[string]AxisScore{
		"exploitability": {Name: "exploitability", RawScore: r2(exploitRaw), NormScore: r3(exploitNorm), BaseNormScore: r3(exploitNorm), MaxScore: r2(exploitMax), Exponent: alphaExploitability, Components: rmap(exploitComps)},
		"impact":         {Name: "impact", RawScore: r2(impactRaw), NormScore: r3(impactNorm), BaseNormScore: r3(impactNorm), MaxScore: r2(impactMax), Exponent: betaImpact, Components: rmap(impactComps)},
		"reachability":   {Name: "reachability", RawScore: r2(reachRaw), NormScore: r3(reachNorm), BaseNormScore: r3(reachNorm), MaxScore: r2(reachMax), Exponent: gammaReachability, Components: rmap(reachComps)},
	}
}

func (e *RiskAggregationEngine) computeBaseRisk(
	categorySums map[string]float64,
	effectiveCatMax map[string]float64,
) float64 {
	getMax := func(cat string) float64 {
		if m, ok := effectiveCatMax[cat]; ok && m > 0 {
			return m
		}
		cap := e.getCap(cat)
		return softCap(cap*0.85, cap)
	}
	vulnNorm := catNormValue(categorySums["vulnerability"], getMax("vulnerability"))
	capNorm := catNormValue(categorySums["capability"], getMax("capability"))
	exposureNorm := catNormValue(categorySums["exposure"], getMax("exposure"))
	rbacNorm := catNormValue(categorySums["rbac_policy"], getMax("rbac_policy"))
	pathNorm := catNormValue(categorySums["attack_path"], getMax("attack_path"))
	vulnTerm := math.Pow(vulnNorm, 1.3)
	capTerm := math.Pow(capNorm, 1.1)
	if vulnNorm < 0.2 {
		capTerm *= 0.5
	}
	base := baseWeightVuln*vulnTerm +
		baseWeightCapability*capTerm +
		baseWeightExposure*exposureNorm +
		baseWeightRBAC*rbacNorm
	if rbacNorm > 0.7 {
		base = math.Max(base, 0.4+0.3*rbacNorm)
	}
	pathPresenceFactor := 0.0
	if categorySums["attack_path"] > 0 && pathNorm >= 0.25 {
		structuralPair := math.Min(1.0, catNormValue(categorySums["blast_radius"], getMax("blast_radius"))+pathNorm)
		pathPresenceFactor = math.Min(0.2, 0.03+0.05*structuralPair)
	}
	if vulnNorm < 0.1 {
		pathPresenceFactor *= 0.5
	}
	base += pathPresenceFactor
	if vulnNorm < 0.1 {
		base *= 0.7
	}
	if vulnNorm > 0.7 && base < 0.3 {
		base = 0.3
	}
	return clamp01(base)
}

func axisNormWithFloor(axis AxisScore, floor float64) float64 {
	if axis.NormScore > 0 {
		return axis.NormScore
	}
	return floor
}

func dynamicAxisFloor(categorySums map[string]float64) float64 {
	if categorySums["runtime"] <= 0 && categorySums["attack_path"] <= 0 && categorySums["exposure"] <= 0 {
		return 0.05
	}
	floor := axisFloor
	if categorySums["vulnerability"] >= 3 || categorySums["capability"] >= 3 || categorySums["rbac_policy"] >= 3 {
		floor = axisFloorElevated
	}
	if categorySums["runtime"] >= 5 {
		floor = axisFloorHighSignal
	}
	return floor
}

func computeExecutionFactor(runtimeNorm, pathNorm float64, hasCriticalStep bool) float64 {
	_ = pathNorm // keep signature stable; execution gating now runtime-first.
	progressRaw := clamp(runtimeNorm, 0, 1)
	progressSat := 1 - math.Exp(-2.3*progressRaw)
	var executionFactor float64
	switch {
	case progressRaw < 0.3:
		executionFactor = 0.12 * progressSat
	case progressRaw < 0.6:
		executionFactor = 0.40 * progressSat
	default:
		executionFactor = 0.85 + 0.15*progressSat
	}
	if !hasCriticalStep {
		executionFactor *= 0.55
	}
	return clamp(executionFactor, 0.2, 1.0)
}

func computeStructuralExecutionFactor(capNorm, blastNorm, exposureNorm, runtimeNorm float64) float64 {
	structural := math.Max(capNorm, 0.8*blastNorm)
	if capNorm > 0.7 {
		if exposureNorm < 0.3 && runtimeNorm == 0 {
			return 0.3 * capNorm
		}
		return clamp(0.6+0.4*structural, 0.0, 1.0)
	}
	return 0.0
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func compressAxisForSaturation(a float64) float64 {
	a = clamp(a, 0, 1)
	a1 := 1 - math.Pow(1-a, 1.8)
	return 1 - math.Pow(1-a1, 1.5)
}

func cloneAxes(in map[string]AxisScore) map[string]AxisScore {
	out := make(map[string]AxisScore, len(in))
	for k, v := range in {
		comps := make(map[string]float64, len(v.Components))
		for ck, cv := range v.Components {
			comps[ck] = cv
		}
		v.Components = comps
		out[k] = v
	}
	return out
}

func budgetAndCapAxisInjections(baseAxes, comboAxes, finalAxes map[string]AxisScore, baseRisk float64, runtimeActiveOverride bool, comboScale float64, comboCount int) map[string]AxisScore {
	out := cloneAxes(finalAxes)
	axes := []string{"exploitability", "impact", "reachability"}
	comboDelta := map[string]float64{}
	overrideDelta := map[string]float64{}
	sumCombo := 0.0
	sumOverride := 0.0
	for _, n := range axes {
		b := baseAxes[n].NormScore
		c := comboAxes[n].NormScore
		f := finalAxes[n].NormScore
		cd := math.Max(0, c-b)
		od := math.Max(0, f-c)
		// Saturation resistance: as base axis approaches 1.0, added injection
		// should diminish to avoid cascade saturation.
		headroom := clamp(1.0-baseAxes[n].BaseNormScore, 0.15, 1.0)
		cd *= headroom
		od *= math.Sqrt(headroom)
		comboDelta[n] = cd
		overrideDelta[n] = od
		sumCombo += cd
		sumOverride += od
	}
	maxTotal := 0.45 + 0.45*baseRisk
	maxOverride := 0.30 + 0.35*baseRisk
	maxCombo := 0.10 + 0.20*baseRisk
	for _, n := range axes {
		overrideDelta[n] *= 0.65
		comboDelta[n] *= 0.35
	}
	sumCombo = 0
	sumOverride = 0
	for _, n := range axes {
		sumCombo += comboDelta[n]
		sumOverride += overrideDelta[n]
	}
	effectiveCombo := 1 - math.Exp(-0.8*float64(maxInt(comboCount, 1)))
	for _, n := range axes {
		comboDelta[n] *= effectiveCombo * clamp(comboScale, 0.2, 1.0)
		if comboDelta[n] > 0 {
			comboDelta[n] = math.Max(comboDelta[n], 0.05)
		}
	}
	sumCombo = 0
	for _, n := range axes {
		sumCombo += comboDelta[n]
	}
	if sumCombo > maxCombo && sumCombo > 0 {
		s := maxCombo / sumCombo
		for _, n := range axes {
			comboDelta[n] *= s
		}
		sumCombo = maxCombo
	}
	if sumOverride > maxOverride && sumOverride > 0 {
		s := maxOverride / sumOverride
		for _, n := range axes {
			overrideDelta[n] *= s
		}
		sumOverride = maxOverride
	}
	if sumCombo+sumOverride > maxTotal && (sumCombo+sumOverride) > 0 {
		s := maxTotal / (sumCombo + sumOverride)
		for _, n := range axes {
			comboDelta[n] *= s
			overrideDelta[n] *= s
		}
	}
	axisCap := 0.84 + 0.09*baseRisk
	for _, n := range axes {
		base := finalAxes[n]
		norm := base.BaseNormScore + comboDelta[n] + overrideDelta[n]
		if norm > axisCap {
			norm = axisCap
		}
		compressed := compressAxisForSaturation(clamp(norm, 0, 1))
		base.NormScore = math.Max(base.BaseNormScore, compressed)
		if runtimeActiveOverride && overrideDelta[n] > 0 {
			base.NormScore = math.Max(base.NormScore, base.BaseNormScore+0.02)
		}
		base.InjectionBoost = math.Round((base.NormScore-base.BaseNormScore)*1000) / 1000
		base.Components["combo_injection"] = math.Round(comboDelta[n]*1000) / 1000
		base.Components["override_injection"] = math.Round(overrideDelta[n]*1000) / 1000
		if _, ok := base.Components["combo_injection_k"]; !ok {
			base.Components["combo_injection_k"] = 4.0
		}
		if _, ok := base.Components["override_saturation_k"]; !ok && overrideDelta[n] > 0 {
			base.Components["override_saturation_k"] = overrideSoftCapK
		}
		if base.InjectionK == 0 && overrideDelta[n] > 0 {
			base.InjectionK = overrideSoftCapK
		}
		out[n] = base
	}
	return out
}

type scoreOverrideRule struct {
	name      string
	axisBoost map[string]float64
	saturationK float64
	detect    func(categorySums map[string]float64) bool
}

var defaultOverrideRules = []scoreOverrideRule{
	{
		name:      "runtime_active_exploit",
		axisBoost: map[string]float64{"exploitability": 0.30, "impact": 0.20, "reachability": 0.25},
		saturationK: 7.0,
		detect: func(cs map[string]float64) bool {
			return cs["runtime"] >= 6
		},
	},
	{
		name:      "escape+host_access",
		axisBoost: map[string]float64{"exploitability": 0.20, "impact": 0.30, "reachability": 0.25},
		saturationK: 6.0,
		detect: func(cs map[string]float64) bool {
			return cs["capability"] >= 5 && (cs["exposure"] >= 3 || cs["blast_radius"] >= 4)
		},
	},
	{
		name:      "cve_critical+exposure",
		axisBoost: map[string]float64{"exploitability": 0.20, "reachability": 0.20},
		saturationK: 6.0,
		detect: func(cs map[string]float64) bool {
			return cs["vulnerability"] >= 5 && cs["exposure"] >= 2
		},
	},
	{
		name:      "latent_cve+dangerous_capability",
		axisBoost: map[string]float64{"exploitability": 0.18, "impact": 0.22, "reachability": 0.12},
		saturationK: 5.5,
		detect: func(cs map[string]float64) bool {
			return cs["vulnerability"] >= 4 && cs["capability"] >= 4
		},
	},
}

// applyOverrideInjections converts high-confidence edge conditions into axis
// boosts instead of hard floor jumps. This keeps score changes explainable in
// the same multiplicative formula.
func applyOverrideInjections(
	axes map[string]AxisScore,
	categorySums map[string]float64,
) (map[string]AxisScore, []string) {
	out := map[string]AxisScore{}
	for k, v := range axes {
		copyComps := map[string]float64{}
		for ck, cv := range v.Components {
			copyComps[ck] = cv
		}
		v.Components = copyComps
		out[k] = v
	}

	applied := make([]string, 0, len(defaultOverrideRules))
	for _, rule := range defaultOverrideRules {
		if !rule.detect(categorySums) {
			continue
		}
		applied = append(applied, rule.name)
		for axisName, boost := range rule.axisBoost {
			axis, ok := out[axisName]
			if !ok || boost <= 0 {
				continue
			}
			k := rule.saturationK
			if k <= 0 {
				k = overrideSoftCapK
			}
			// Soft saturation keeps gradient near 1.0, unlike hard clamp.
			// E' = 1 - (1 - E) * exp(-k * boost)
			base := axis.NormScore
			saturated := 1.0 - (1.0-base)*math.Exp(-k*boost)
			axis.NormScore = math.Min(1.0, saturated)
			axis.InjectionBoost = math.Round((axis.NormScore-axis.BaseNormScore)*1000) / 1000
			axis.InjectionK = math.Round(k*1000) / 1000
			axis.Components["override_injection"] = math.Round(axis.InjectionBoost*1000) / 1000
			axis.Components["override_saturation_k"] = math.Round(k*1000) / 1000
			out[axisName] = axis
		}
		// Apply only the highest-priority matching rule to avoid stack inflation.
		break
	}
	return out, applied
}

func applyInteractionComboInjections(axes map[string]AxisScore, combos []InteractionCombo) map[string]AxisScore {
	boostByCombo := map[string]map[string]float64{
		"cve_critical+internet_exposed": {"exploitability": 0.15, "reachability": 0.20},
		"capability+runtime_threat":     {"exploitability": 0.18, "impact": 0.15},
		"rbac_policy+exposure":          {"impact": 0.10, "reachability": 0.12},
		"priv_esc+cluster_admin_path":   {"impact": 0.20, "reachability": 0.15},
	}
	axisTotals := map[string]float64{}
	for _, combo := range combos {
		axisBoosts, ok := boostByCombo[combo.Name]
		if !ok {
			continue
		}
		for axisName, boost := range axisBoosts {
			axisTotals[axisName] += boost
		}
	}
	for axisName, boost := range axisTotals {
		axis, ok := axes[axisName]
		if !ok || boost <= 0 {
			continue
		}
		k := 4.0
		base := axis.NormScore
		saturated := 1.0 - (1.0-base)*math.Exp(-k*boost)
		axis.NormScore = math.Min(1.0, saturated)
		axis.InjectionBoost = math.Round((axis.NormScore-axis.BaseNormScore)*1000) / 1000
		axis.InjectionK = math.Round(k*1000) / 1000
		axis.Components["combo_injection"] = math.Round(axis.InjectionBoost*1000) / 1000
		axis.Components["combo_injection_k"] = k
		axes[axisName] = axis
	}
	return axes
}

func (e *RiskAggregationEngine) NormalizeFactors(in []RiskFactor) []RiskFactor {
	out := make([]RiskFactor, 0, len(in))
	for _, f := range in {
		n := f
		n.FactorID = strings.TrimSpace(n.FactorID)
		n.Category = strings.ToLower(strings.TrimSpace(n.Category))
		n.Scope = strings.ToLower(strings.TrimSpace(n.Scope))
		n.Source = strings.ToLower(strings.TrimSpace(n.Source))
		n.DedupeKey = strings.TrimSpace(n.DedupeKey)
		if n.Weight < 0 {
			n.Weight = 0
		}

		// Apply source-based confidence prior when caller didn't set explicit confidence
		if n.Confidence == 0 || n.Confidence == 1.0 {
			if sc, ok := e.sourceConfidence[n.Source]; ok {
				n.Confidence = sc
			} else {
				n.Confidence = 0.8
			}
		}
		if n.Freshness == 0 {
			n.Freshness = 1
		}
		n.Confidence = clamp01(n.Confidence)
		n.Freshness = clamp01(n.Freshness)

		scopeImpact := e.scopeImpactByName[n.Scope]
		if scopeImpact <= 0 {
			scopeImpact = 1.0
		}

		// Dimension weight multiplier
		dimWeight := 1.0
		if w, ok := e.dimensionWeights[n.Category]; ok && w > 0 {
			dimWeight = w
		}

		n.Contribution = n.Weight * n.Confidence * n.Freshness * scopeImpact * dimWeight
		out = append(out, n)
	}
	return out
}

// DeduplicateFactors aggregates factor instances per dedupe key and preserves context.
func (e *RiskAggregationEngine) DeduplicateFactors(in []RiskFactor) ([]RiskFactor, []AggregatedFactor) {
	clustered := make(map[string][]RiskFactor, len(in))
	singletons := make([]RiskFactor, 0)
	for _, f := range in {
		if f.DedupeKey == "" {
			singletons = append(singletons, f)
			continue
		}
		clustered[f.DedupeKey] = append(clustered[f.DedupeKey], f)
	}

	aggregated := make([]AggregatedFactor, 0, len(clustered)+len(singletons))
	out := make([]RiskFactor, 0, len(clustered)+len(singletons))

	for _, f := range singletons {
		aggregated = append(aggregated, AggregatedFactor{
			BaseFactor:          f,
			Instances:           []RiskFactor{f},
			AggregationStrategy: "identity",
		})
		out = append(out, f)
	}

	for _, instances := range clustered {
		best := instances[0]
		for _, inst := range instances[1:] {
			if inst.Contribution > best.Contribution {
				best = inst
			}
		}
		aggregated = append(aggregated, AggregatedFactor{
			BaseFactor:          best,
			Instances:           instances,
			AggregationStrategy: "max",
		})
		out = append(out, best)
	}
	return out, aggregated
}

// ApplyCategoryCaps uses exponential soft cap per category.
// softCap(x, cap) = cap * (1 - e^(-x/cap))
// This preserves signal from volume (56 CVEs > 1 CVE) while bounding contribution.
func (e *RiskAggregationEngine) ApplyCategoryCaps(in []RiskFactor) []RiskFactor {
	if len(e.categoryCaps) == 0 {
		return in
	}
	sumByCategory := map[string]float64{}
	for _, f := range in {
		sumByCategory[f.Category] += f.Contribution
	}

	out := make([]RiskFactor, 0, len(in))
	for _, f := range in {
		capValue, hasCap := e.categoryCaps[f.Category]
		if !hasCap || capValue <= 0 {
			out = append(out, f)
			continue
		}
		curSum := sumByCategory[f.Category]
		if curSum <= 0 {
			out = append(out, f)
			continue
		}
		cappedSum := softCap(curSum, capValue)
		scale := cappedSum / curSum
		c := f
		c.Contribution = c.Contribution * scale
		out = append(out, c)
	}
	return out
}

// interactionComboRule defines a toxic combination that acts as a multiplier
// on related dimension scores, not a flat additive boost.
type interactionComboRule struct {
	name             string
	multiplier       float64
	relatedCategories []string
	detect           func([]RiskFactor) bool
}

// applyCrossFactorBoost computes interaction boosts as multipliers on related
// dimension scores. This models the security reality that CVE+exposure is
// more dangerous than sum(CVE, exposure).
func (e *RiskAggregationEngine) applyCrossFactorBoost(in []RiskFactor, categorySums map[string]float64) (float64, []string, []InteractionCombo) {
	idSet := map[string]bool{}
	catActive := map[string]bool{}
	for _, f := range in {
		idSet[strings.ToUpper(f.FactorID)] = true
		if f.Contribution > 0 {
			catActive[f.Category] = true
		}
	}

	rules := []interactionComboRule{
		{
			name:       "cve_critical+internet_exposed",
			multiplier: 0.30,
			relatedCategories: []string{"vulnerability", "exposure"},
			detect: func(_ []RiskFactor) bool {
				return catActive["vulnerability"] && catActive["exposure"]
			},
		},
		{
			name:       "capability+runtime_threat",
			multiplier: 0.40,
			relatedCategories: []string{"capability", "runtime"},
			detect: func(_ []RiskFactor) bool {
				return catActive["capability"] && catActive["runtime"]
			},
		},
		{
			name:       "rbac_policy+exposure",
			multiplier: 0.30,
			relatedCategories: []string{"rbac_policy", "exposure"},
			detect: func(_ []RiskFactor) bool {
				return catActive["rbac_policy"] && catActive["exposure"]
			},
		},
		{
			name:       "priv_esc+cluster_admin_path",
			multiplier: 0.35,
			relatedCategories: []string{"capability", "attack_path"},
			detect: func(_ []RiskFactor) bool {
				return catActive["capability"] && catActive["attack_path"]
			},
		},
	}

	totalBoost := 0.0
	reasons := make([]string, 0, 4)
	combos := make([]InteractionCombo, 0, 4)

	for _, rule := range rules {
		if !rule.detect(in) {
			continue
		}
		relatedSum := 0.0
		for _, cat := range rule.relatedCategories {
			relatedSum += categorySums[cat]
		}
		boost := rule.multiplier * relatedSum
		if boost > 0 {
			totalBoost += boost
			reasons = append(reasons, rule.name)
			combos = append(combos, InteractionCombo{
				Name:          rule.name,
				Multiplier:    rule.multiplier,
				RelatedDimSum: math.Round(relatedSum*100) / 100,
				ComputedBoost: math.Round(boost*100) / 100,
			})
		}
	}
	return totalBoost, reasons, combos
}

func (e *RiskAggregationEngine) ComputeScore(in []RiskFactor) AggregationResult {
	if len(in) == 0 {
		return AggregationResult{
			CategorySums:     map[string]float64{},
			CategoryCaps:     e.categoryCaps,
			Axes:             map[string]AxisScore{},
			ComboAmplifier:   1.0,
			BaseRisk:         0.0,
			ThreatCore:       0.0,
			ThreatAmplifierRaw: 0.0,
			ThreatAmplifier:    0.0,
			ComboThreatBoost:   0.0,
			MaxThreatAmplifier: 0.0,
			FinalFormula:     "score = 100 * base_risk * (1 + threat_amplifier) * context_multiplier",
			ContextMultiplier: e.contextMultiplier,
		}
	}

	normalized := e.NormalizeFactors(in)
	deduped, aggregated := e.DeduplicateFactors(normalized)
	preCapRaw := 0.0
	for _, f := range deduped {
		preCapRaw += f.Contribution
	}
	capped := e.ApplyCategoryCaps(deduped)
	postCapRaw := 0.0
	for _, f := range capped {
		postCapRaw += f.Contribution
	}
	sort.SliceStable(capped, func(i, j int) bool {
		return capped[i].Contribution > capped[j].Contribution
	})

	categorySums := map[string]float64{}
	for _, f := range capped {
		categorySums[f.Category] += f.Contribution
	}

	crossBoost, crossReasons, interactionCombos := e.applyCrossFactorBoost(capped, categorySums)

	// --- Tri-axial multiplicative model ---
	effectiveCatMax := e.computeEffectiveCatMax(normalized)
	axesBase := e.computeAxisScores(categorySums, effectiveCatMax, crossBoost)
	baseRisk := e.computeBaseRisk(categorySums, effectiveCatMax)
	axesAfterCombo := applyInteractionComboInjections(cloneAxes(axesBase), interactionCombos)
	axesAfterBoth, overridesApplied := applyOverrideInjections(cloneAxes(axesAfterCombo), categorySums)
	runtimeActiveOverride := false
	for _, ov := range overridesApplied {
		if ov == "runtime_active_exploit" {
			runtimeActiveOverride = true
			break
		}
	}
	criticalSignal := runtimeActiveOverride
	execForBudget := computeExecutionFactor(
		axesBase["exploitability"].Components["runtime"],
		0,
		criticalSignal,
	)
	comboScale := 0.5 + 0.5*execForBudget
	axes := budgetAndCapAxisInjections(
		axesBase, axesAfterCombo, axesAfterBoth,
		baseRisk, runtimeActiveOverride, comboScale, len(interactionCombos),
	)
	floor := dynamicAxisFloor(categorySums)

	eNorm := axisNormWithFloor(axes["exploitability"], floor)
	iNorm := axisNormWithFloor(axes["impact"], floor)
	rNorm := axisNormWithFloor(axes["reachability"], floor)
	if rNorm < 0.3 {
		rNorm = math.Pow(rNorm, 2.2)
	} else {
		rNorm = math.Pow(rNorm, 1.4)
	}
	runtimeNorm := axes["exploitability"].Components["runtime"]
	pathNorm := axes["exploitability"].Components["attack_path"]
	vulnNorm := axes["exploitability"].Components["vulnerability"]
	capNorm := axes["impact"].Components["capability"]
	blastNorm := axes["impact"].Components["blast_radius"]
	if runtimeNorm == 0 && pathNorm == 0 {
		eCap := 0.35 + 0.15*vulnNorm
		eNorm = math.Min(eNorm, eCap)
	}
	// Structural execution enablers (e.g. escape/host-access posture) should
	// prevent exploitability from being under-scored even without runtime hits.
	if capNorm > 0.7 && blastNorm > 0.6 {
		eNorm = math.Max(eNorm, 0.68)
		eAxis := axes["exploitability"]
		eAxis.NormScore = math.Max(eAxis.NormScore, 0.68)
		eAxis.InjectionBoost = math.Round((eAxis.NormScore-eAxis.BaseNormScore)*1000) / 1000
		axes["exploitability"] = eAxis
	}
	if runtimeNorm == 0 && pathNorm == 0 && capNorm > 0.6 &&
		axes["reachability"].Components["exposure"] > 0.5 {
		eNorm = math.Max(eNorm, 0.7)
		eAxis := axes["exploitability"]
		eAxis.NormScore = math.Max(eAxis.NormScore, 0.7)
		eAxis.InjectionBoost = math.Round((eAxis.NormScore-eAxis.BaseNormScore)*1000) / 1000
		axes["exploitability"] = eAxis
	}

	threatCore := math.Pow(eNorm, alphaExploitability) *
		math.Pow(iNorm, betaImpact) *
		math.Pow(rNorm, gammaReachability)
	execFactor := computeExecutionFactor(runtimeNorm, pathNorm, runtimeActiveOverride)
	exposureNorm := axes["reachability"].Components["exposure"]
	execFactor = math.Max(execFactor, computeStructuralExecutionFactor(capNorm, blastNorm, exposureNorm, runtimeNorm))
	for _, ov := range overridesApplied {
		if ov == "runtime_active_exploit" {
			execFactor = math.Max(execFactor, 1.1)
			break
		}
	}
	threatCore *= execFactor
	comboPressure := axes["exploitability"].Components["combo_injection"] +
		axes["impact"].Components["combo_injection"] +
		axes["reachability"].Components["combo_injection"]
	if comboPressure > 0 {
		threatCore *= clamp(1.0-0.6*comboPressure, 0.4, 1.0)
		baseRisk *= clamp(1.0-0.25*comboPressure, 0.75, 1.0)
	}
	// BaseRisk should not dominate when there is no execution evidence.
	if !runtimeActiveOverride && runtimeNorm == 0 && pathNorm == 0 {
		if execFactor <= 0.45 {
			baseRisk *= 0.7
		} else if execFactor < 0.6 {
			baseRisk *= 0.85
		}
	}
	if len(interactionCombos) >= 2 && runtimeNorm > 0.3 {
		baseRisk *= 0.8
	}
	for _, ov := range overridesApplied {
		if ov == "runtime_active_exploit" && baseRisk < 0.3 {
			baseRisk = 0.35
			break
		}
	}

	comboAmplifier := 1.0
	threatAmplifierRaw := threatCore
	maxAmpByBase := math.Min(maxThreatAmplifier, 1.0+0.7*baseRisk)
	for _, ov := range overridesApplied {
		if ov == "runtime_active_exploit" {
			maxAmpByBase = maxThreatAmplifier
			break
		}
	}
	threatAmpCore := math.Min(maxAmpByBase, math.Max(0.0, threatAmplifierRaw))
	comboThreatBoost := 0.0
	threatAmplifier := math.Min(maxAmpByBase, threatAmpCore+comboThreatBoost)
	effectiveThreatCap := math.Min(maxAmpByBase, 0.25+1.1*math.Pow(baseRisk, 0.85))
	if runtimeNorm == 0 && pathNorm == 0 {
		effectiveThreatCap = math.Min(effectiveThreatCap, 0.45)
	}
	reachPenalty := math.Pow(clamp(rNorm, 0, 1), 1.8)
	effectiveThreat := math.Min(threatAmplifier*reachPenalty, effectiveThreatCap)
	if len(interactionCombos) > 0 {
		comboAwareCap := 0.40 + 0.18*baseRisk
		effectiveThreat = math.Min(effectiveThreat, comboAwareCap)
		effectiveThreat *= clamp(1.0-0.8*comboPressure, 0.2, 1.0)
	}
	// Re-balanced aggregation shape:
	// 1) Base reshape (soft, no hard cut)
	adjustedBase := clamp(baseRisk*0.75, 0, 1)
	effectiveBase := math.Pow(adjustedBase, 1.2)
	// Preserve latent structural bands for CVE-only and high-RBAC-no-path cases.
	if runtimeNorm == 0 && pathNorm == 0 {
		if vulnNorm > 0.6 {
			effectiveBase = math.Max(effectiveBase, 0.15)
		}
		if axes["impact"].Components["rbac_policy"] > 0.7 {
			effectiveBase = math.Max(effectiveBase, 0.40)
		}
	}
	// 2) Threat reshape with re-amplify
	reshapedThreat := math.Pow(clamp(effectiveThreat, 0, 1), 0.85) * 1.15
	// Path latent recovery: strong path should retain latent threat even without runtime.
	if pathNorm > 0.3 && runtimeNorm == 0 {
		reshapedThreat += 0.1 * pathNorm
	}
	reshapedThreat = clamp(reshapedThreat, 0, 1)
	// 3) Combine (no multiplicative chain)
	combined := effectiveBase + (1.0-effectiveBase)*reshapedThreat
	// 4) Context applies mostly to threat, softly to base.
	contextApplied := effectiveBase*(0.85+0.15*e.contextMultiplier) +
		(combined-effectiveBase)*e.contextMultiplier
	totalRaw := 100.0 * clamp(contextApplied, 0, 1)
	// Active dimension counting (backward compat)
	totalDimCount := 0
	activeDimCount := 0
	seenCategories := map[string]bool{}
	for _, f := range capped {
		cat := f.Category
		if cat == "interaction" {
			continue
		}
		if !seenCategories[cat] {
			seenCategories[cat] = true
			totalDimCount++
			if f.Contribution > 0 {
				activeDimCount++
			}
		}
	}
	if totalDimCount == 0 {
		totalDimCount = 1
	}
	if activeDimCount == 0 {
		activeDimCount = 1
	}
	dimNormMultiplier := 1.0
	if activeDimCount < totalDimCount && activeDimCount > 0 {
		rawRatio := float64(totalDimCount) / float64(activeDimCount)
		dimNormMultiplier = 1.0 + (rawRatio-1.0)*0.4
	}

	total := math.Min(100.0, math.Max(0.0, totalRaw))
	total = math.Round(total*100) / 100

	top := make([]map[string]interface{}, 0, 3)
	for i, f := range capped {
		if i >= 3 {
			break
		}
		top = append(top, map[string]interface{}{
			"factor_id":     f.FactorID,
			"category":      f.Category,
			"scope":         f.Scope,
			"source":        f.Source,
			"contribution":  math.Round(f.Contribution*100) / 100,
			"evidence_refs": f.EvidenceRefs,
		})
	}
	summaryParts := []string{}
	if len(crossReasons) > 0 {
		summaryParts = append(summaryParts, "interaction: "+strings.Join(crossReasons, ", "))
	}
	if len(overridesApplied) > 0 {
		summaryParts = append(summaryParts, "override_injection: "+strings.Join(overridesApplied, ", "))
	}
	if len(top) > 0 {
		summaryParts = append(summaryParts, "top factors drive most risk")
	}

	return AggregationResult{
		Factors:            capped,
		AggregatedFactors:  aggregated,
		CategorySums:       categorySums,
		CategoryCaps:       e.categoryCaps,
		PreCapScoreRaw:     math.Round(preCapRaw*100) / 100,
		PostCapScoreRaw:    math.Round(postCapRaw*100) / 100,
		CrossFactorBoost:   math.Round(crossBoost*100) / 100,
		TotalScoreRaw:      math.Round(totalRaw*100) / 100,
		TotalScore:         total,
		ExplanationSummary: strings.Join(summaryParts, "; "),
		RecommendedActions: []string{"remove privileged/host access vectors", "reduce token exposure and outbound egress"},
		TopContributors:    top,
		InteractionCombos:  interactionCombos,
		ActiveDimCount:     activeDimCount,
		TotalDimCount:      totalDimCount,
		DimNormMultiplier:  math.Round(dimNormMultiplier*1000) / 1000,
		ContextMultiplier:  e.contextMultiplier,
		Axes:               axes,
		BaseRisk:           math.Round(baseRisk*1000) / 1000,
		ThreatCore:         math.Round(threatCore*1000) / 1000,
		ThreatAmplifierRaw: math.Round(threatAmplifierRaw*1000) / 1000,
		ThreatAmplifier:    math.Round(reshapedThreat*1000) / 1000,
		ComboThreatBoost:   math.Round(comboThreatBoost*1000) / 1000,
		MaxThreatAmplifier: math.Round(maxAmpByBase*1000) / 1000,
		FinalFormula:       "score = 100 * base_risk * (1 + threat_amplifier) * context_multiplier",
		ComboAmplifier:     math.Round(comboAmplifier*1000) / 1000,
		OverridesApplied:   overridesApplied,
	}
}

