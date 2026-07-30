package risk

import (
	"context"
	"math"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/fortuna/core/pkg/graph"
)

const (
	phase42AbsTol      = 0.05
	phase42RelTol      = 0.02
	phase42FactorTol   = 0.1
	incompleteAbsDrift = 2.0
	incompleteRelDrift = 0.15
	extraScoreTol      = 0.5
	extraMitreBoostTol = 0.5
)

func scoreDeltaWithinPhase42(a, b float64) bool {
	delta := math.Abs(a - b)
	if delta <= phase42AbsTol {
		return true
	}
	denom := math.Abs(a)
	if denom < 1e-9 {
		return delta <= phase42AbsTol
	}
	return delta/denom <= phase42RelTol
}

func scoreDriftOKIncompleteOverlay(baseline, degraded float64) bool {
	delta := math.Abs(baseline - degraded)
	if delta <= incompleteAbsDrift {
		return true
	}
	denom := math.Abs(baseline)
	if denom < 1e-9 {
		return delta <= incompleteAbsDrift
	}
	return delta/denom <= incompleteRelDrift
}

// calculateFixtureScore builds the escape→cluster-admin fixture DB and returns Unified V3 for fixturePodUID.
// Overlay content must already be loaded by the caller via ReloadTechniqueOverlay (does not mutate overlay itself).
func calculateFixtureScore(t *testing.T) *UnifiedScoreV3 {
	t.Helper()
	paths := graph.AttackPathsFromEscapeClusterAdminFixture(fixturePodUID)
	assignStablePathIDs(t, paths)
	db := materializeFixtureScorerDB(t, paths)
	scorer := NewUnifiedScorerV3(db)
	sc, err := scorer.CalculateScoreV3(context.Background(), fixturePodUID)
	require.NoError(t, err)
	require.NotNil(t, sc)
	return sc
}

func assertEscapeToPrivEscContainsTechnique(t *testing.T, techniqueID string) {
	t.Helper()
	paths := graph.AttackPathsFromEscapeClusterAdminFixture(fixturePodUID)
	chains := graph.DetectChainsWithHints(paths, graph.DefaultHardeningHints())
	require.NotEmpty(t, chains)
	var escPriv *graph.AttackChain
	for i := range chains {
		if chains[i].Type == "ESCAPE_TO_PRIV_ESC" {
			escPriv = &chains[i]
			break
		}
	}
	require.NotNil(t, escPriv, "expected ESCAPE_TO_PRIV_ESC chain")
	var stepTech []string
	for _, s := range escPriv.Steps {
		stepTech = append(stepTech, s.TechniqueID)
	}
	require.Contains(t, stepTech, techniqueID, "core technique must remain on chain steps (YAML must not hide registry bridge)")
}

// Phase 4.2 A/B: Unified V3 stable across overlay reset/reload + component parity + non-regression bounds.
func TestPhase42_AB_UnifiedScoreStableAcrossOverlayReload(t *testing.T) {
	t.Cleanup(func() {
		_ = os.Unsetenv("FORTUNA_TECHNIQUE_SOURCE")
		require.NoError(t, graph.ReloadTechniqueOverlay(graph.TechniqueOverlayEmbeddedYAML()))
	})

	paths := graph.AttackPathsFromEscapeClusterAdminFixture(fixturePodUID)
	assignStablePathIDs(t, paths)

	db := materializeFixtureScorerDB(t, paths)

	op1, mr1 := graph.OverlayDerivedMapsIdentity()
	require.NoError(t, graph.ResetTechniqueRegistry(graph.TechniqueOverlayEmbeddedYAML()))
	op2, mr2 := graph.OverlayDerivedMapsIdentity()
	require.NotEqual(t, op1, op2, "overlay table must be reallocated after ResetTechniqueRegistry")
	require.NotEqual(t, mr1, mr2, "mitreRiskIndex must be reallocated after ResetTechniqueRegistry")

	scorer := NewUnifiedScorerV3(db)
	scGo, err := scorer.CalculateScoreV3(context.Background(), fixturePodUID)
	require.NoError(t, err)

	require.NoError(t, graph.ReloadTechniqueOverlay(graph.TechniqueOverlayEmbeddedYAML()))
	scYaml, err := scorer.CalculateScoreV3(context.Background(), fixturePodUID)
	require.NoError(t, err)

	require.True(t, scoreDeltaWithinPhase42(scGo.TotalScore, scYaml.TotalScore),
		"TotalScore delta abs≤%.g or rel≤%.g%% got go=%.6f yaml=%.6f", phase42AbsTol, phase42RelTol*100, scGo.TotalScore, scYaml.TotalScore)
	require.True(t, scoreDeltaWithinPhase42(scGo.AttackPathScore, scYaml.AttackPathScore),
		"AttackPathScore delta go=%.6f yaml=%.6f", scGo.AttackPathScore, scYaml.AttackPathScore)

	require.InDelta(t, scGo.AttackPathScore, scYaml.AttackPathScore, phase42AbsTol)
	require.InDelta(t, factorFloat(scGo.Factors, "ckdb_capability_risk_points"), factorFloat(scYaml.Factors, "ckdb_capability_risk_points"), phase42FactorTol)
	require.InDelta(t, factorFloat(scGo.Factors, "mitre_runtime_attack_path_boost"), factorFloat(scYaml.Factors, "mitre_runtime_attack_path_boost"), phase42FactorTol)

	require.Greater(t, scYaml.AttackPathScore, 0.0)
	require.LessOrEqual(t, factorFloat(scYaml.Factors, "mitre_runtime_attack_path_boost"), 5.0)
}

// Phase 4.3 prep: runtime rollback switch — go mode disables overlay semantics; both paths must produce finite scores.
// TestPhase43_GoSource_IgnoresYAMLOverrides — FORTUNA_TECHNIQUE_SOURCE=go must ignore destructive overlay rows (no leak into scores vs embedded baseline).
func TestPhase43_GoSource_IgnoresYAMLOverrides(t *testing.T) {
	t.Cleanup(func() {
		_ = os.Unsetenv("FORTUNA_TECHNIQUE_SOURCE")
		require.NoError(t, graph.ReloadTechniqueOverlay(graph.TechniqueOverlayEmbeddedYAML()))
	})

	require.NoError(t, os.Setenv("FORTUNA_TECHNIQUE_SOURCE", "go"))

	rawEmbed := graph.TechniqueOverlayEmbeddedYAML()
	var f graphTechniqueOverlayFileClone
	require.NoError(t, yaml.Unmarshal(rawEmbed, &f))

	row := f.Overlays["ESCAPE_HOSTPATH"]
	row.RiskWeight = 9.99
	row.MitreTechniques = []graph.MitreTechniqueRef{{ID: "T9999", Name: "Synthetic", Tactic: "Impact"}}
	f.Overlays["ESCAPE_HOSTPATH"] = row

	patched, err := yaml.Marshal(&f)
	require.NoError(t, err)

	require.NoError(t, graph.ReloadTechniqueOverlay(patched))
	scoreGo := calculateFixtureScore(t)

	require.NoError(t, graph.ReloadTechniqueOverlay(graph.TechniqueOverlayEmbeddedYAML()))
	scoreBaseline := calculateFixtureScore(t)

	require.InDelta(t, scoreBaseline.TotalScore, scoreGo.TotalScore, 0.01)
	require.InDelta(t, scoreBaseline.AttackPathScore, scoreGo.AttackPathScore, 0.01)
}

func TestPhase43_FeatureFlag_TechniqueSourceYamlVsGo(t *testing.T) {
	t.Cleanup(func() {
		_ = os.Unsetenv("FORTUNA_TECHNIQUE_SOURCE")
		require.NoError(t, graph.ReloadTechniqueOverlay(graph.TechniqueOverlayEmbeddedYAML()))
	})

	paths := graph.AttackPathsFromEscapeClusterAdminFixture(fixturePodUID)
	assignStablePathIDs(t, paths)
	db := materializeFixtureScorerDB(t, paths)
	scorer := NewUnifiedScorerV3(db)

	require.NoError(t, os.Setenv("FORTUNA_TECHNIQUE_SOURCE", "yaml"))
	scY, err := scorer.CalculateScoreV3(context.Background(), fixturePodUID)
	require.NoError(t, err)

	require.NoError(t, os.Setenv("FORTUNA_TECHNIQUE_SOURCE", "go"))
	scG, err := scorer.CalculateScoreV3(context.Background(), fixturePodUID)
	require.NoError(t, err)

	require.False(t, math.IsNaN(scY.TotalScore))
	require.False(t, math.IsNaN(scG.TotalScore))
	require.Greater(t, scY.TotalScore, 0.0)
	require.Greater(t, scG.TotalScore, 0.0)

	mbY := factorFloat(scY.Factors, "mitre_runtime_attack_path_boost")
	mbG := factorFloat(scG.Factors, "mitre_runtime_attack_path_boost")
	require.GreaterOrEqual(t, mbY, mbG, "yaml semantic mode should not reduce MITRE boost vs go rollback for same fixture")

	_ = os.Unsetenv("FORTUNA_TECHNIQUE_SOURCE")
}

// TestPhase42_OverlayYAMLIncomplete_FallbackSafety: strip MITRE + zero risk_weight on ESCAPE_HOSTPATH in a **full** embedded patch — score stays bounded; CKDB isolated; chain steps still use core registry technique.
func TestPhase42_OverlayYAMLIncomplete_FallbackSafety(t *testing.T) {
	t.Cleanup(func() {
		require.NoError(t, graph.ReloadTechniqueOverlay(graph.TechniqueOverlayEmbeddedYAML()))
	})

	require.NoError(t, graph.ReloadTechniqueOverlay(graph.TechniqueOverlayEmbeddedYAML()))
	baseline := calculateFixtureScore(t)

	raw := graph.TechniqueOverlayEmbeddedYAML()
	var f graphTechniqueOverlayFileClone
	require.NoError(t, yaml.Unmarshal(raw, &f))

	row := f.Overlays["ESCAPE_HOSTPATH"]
	row.MitreTechniques = nil
	row.RiskWeight = 0 // explicit missing weight → defaults in merge
	f.Overlays["ESCAPE_HOSTPATH"] = row

	patched, err := yaml.Marshal(&f)
	require.NoError(t, err)

	require.NoError(t, graph.ReloadTechniqueOverlay(patched))
	degraded := calculateFixtureScore(t)

	require.Greater(t, degraded.TotalScore, 0.0)
	require.True(t, scoreDriftOKIncompleteOverlay(baseline.TotalScore, degraded.TotalScore),
		"score drift too large: baseline=%.4f degraded=%.4f", baseline.TotalScore, degraded.TotalScore)

	require.Greater(t, degraded.AttackPathScore, 0.0)

	mbDeg := factorFloat(degraded.Factors, "mitre_runtime_attack_path_boost")
	require.GreaterOrEqual(t, mbDeg, 0.0)

	require.InDelta(t,
		factorFloat(baseline.Factors, "ckdb_capability_risk_points"),
		factorFloat(degraded.Factors, "ckdb_capability_risk_points"),
		0.5,
		"CKDB path must not move when MITRE/risk overlay row is degraded")

	assertEscapeToPrivEscContainsTechnique(t, "ESCAPE_HOSTPATH")
}

// TestPhase42_OverlayYAMLExtra_UnregisteredTechniqueIgnored: orphan overlay keys must not move scores or inflate MITRE boost; TechniqueByID rejects unknown IDs.
func TestPhase42_OverlayYAMLExtra_UnregisteredTechniqueIgnored(t *testing.T) {
	t.Cleanup(func() {
		require.NoError(t, graph.ReloadTechniqueOverlay(graph.TechniqueOverlayEmbeddedYAML()))
	})

	require.NoError(t, graph.ReloadTechniqueOverlay(graph.TechniqueOverlayEmbeddedYAML()))
	baseline := calculateFixtureScore(t)

	raw := graph.TechniqueOverlayEmbeddedYAML()
	var f graphTechniqueOverlayFileClone
	require.NoError(t, yaml.Unmarshal(raw, &f))
	f.Overlays["FAKE_TECH"] = graphTechniqueOverlayRowClone{
		RiskWeight:      999,
		MitreTechniques: []graph.MitreTechniqueRef{{ID: "T9999", Name: "Synthetic", Tactic: "Impact"}},
	}

	patched, err := yaml.Marshal(&f)
	require.NoError(t, err)

	require.NoError(t, graph.ReloadTechniqueOverlay(patched))
	mutated := calculateFixtureScore(t)

	mbBase := factorFloat(baseline.Factors, "mitre_runtime_attack_path_boost")
	mbMut := factorFloat(mutated.Factors, "mitre_runtime_attack_path_boost")

	require.InDelta(t, baseline.TotalScore, mutated.TotalScore, extraScoreTol)
	require.InDelta(t, baseline.AttackPathScore, mutated.AttackPathScore, extraScoreTol)
	require.LessOrEqual(t, mbMut, mbBase+extraMitreBoostTol, "MITRE boost must not spike from unused overlay key")

	_, ok := graph.TechniqueByID("FAKE_TECH")
	require.False(t, ok, "YAML cannot register new AttackTechnique IDs without Go registry entry")
}

// shuffleOverlayYAMLKeyOrder rebuilds overlays map iteration order (deterministic reverse) — serialized YAML key order differs.
func shuffleOverlayYAMLKeyOrder(raw []byte) ([]byte, error) {
	var f graphTechniqueOverlayFileClone
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(f.Overlays))
	for k := range f.Overlays {
		keys = append(keys, k)
	}
	for i, j := 0, len(keys)-1; i < j; i, j = i+1, j-1 {
		keys[i], keys[j] = keys[j], keys[i]
	}
	shuf := make(map[string]graphTechniqueOverlayRowClone, len(f.Overlays))
	for _, k := range keys {
		shuf[k] = f.Overlays[k]
	}
	f.Overlays = shuf
	return yaml.Marshal(&f)
}

func TestPhase41_YAMLOverlayOrder_DoesNotAffectFixtureScore(t *testing.T) {
	t.Cleanup(func() {
		require.NoError(t, graph.ReloadTechniqueOverlay(graph.TechniqueOverlayEmbeddedYAML()))
	})

	y1 := graph.TechniqueOverlayEmbeddedYAML()
	y2, err := shuffleOverlayYAMLKeyOrder(y1)
	require.NoError(t, err)

	require.NoError(t, graph.ReloadTechniqueOverlay(y1))
	s1 := calculateFixtureScore(t)

	require.NoError(t, graph.ReloadTechniqueOverlay(y2))
	s2 := calculateFixtureScore(t)

	require.InDelta(t, s1.TotalScore, s2.TotalScore, 0.01)
	require.InDelta(t, s1.AttackPathScore, s2.AttackPathScore, 0.01)
}

func TestPhase42_ReloadOverlay_Concurrent_Idempotent(t *testing.T) {
	t.Cleanup(func() {
		require.NoError(t, graph.ReloadTechniqueOverlay(graph.TechniqueOverlayEmbeddedYAML()))
	})

	require.NoError(t, graph.ReloadTechniqueOverlay(graph.TechniqueOverlayEmbeddedYAML()))
	base := calculateFixtureScore(t)

	patched := graph.TechniqueOverlayEmbeddedYAML()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = graph.ReloadTechniqueOverlay(patched)
		}()
	}
	wg.Wait()

	require.NoError(t, graph.ReloadTechniqueOverlay(patched))
	after := calculateFixtureScore(t)

	require.InDelta(t, base.TotalScore, after.TotalScore, 0.05)
	require.InDelta(t, base.AttackPathScore, after.AttackPathScore, 0.05)
}

func TestPhase42_YAMLMissingScope_NoInflationVsEmbedded(t *testing.T) {
	t.Cleanup(func() {
		require.NoError(t, graph.ReloadTechniqueOverlay(graph.TechniqueOverlayEmbeddedYAML()))
	})

	raw := graph.TechniqueOverlayEmbeddedYAML()
	var f graphTechniqueOverlayFileClone
	require.NoError(t, yaml.Unmarshal(raw, &f))

	row := f.Overlays["KUBELET_API_PROBE"]
	row.ContextScope = nil
	f.Overlays["KUBELET_API_PROBE"] = row

	patched, err := yaml.Marshal(&f)
	require.NoError(t, err)

	require.NoError(t, graph.ReloadTechniqueOverlay(patched))
	s := calculateFixtureScore(t)

	require.NoError(t, graph.ReloadTechniqueOverlay(graph.TechniqueOverlayEmbeddedYAML()))
	base := calculateFixtureScore(t)

	mbS := factorFloat(s.Factors, "mitre_runtime_attack_path_boost")
	mbB := factorFloat(base.Factors, "mitre_runtime_attack_path_boost")
	require.LessOrEqual(t, mbS, mbB+0.5,
		"stripping context_scope must not inflate MITRE boost vs embedded baseline")
}

// Local clones — yaml struct tags match graph package (avoid exporting overlay file types).
type graphTechniqueOverlayFileClone struct {
	Version         int                                      `yaml:"version"`
	Defaults        graphOverlayDefaultsClone                `yaml:"defaults"`
	TacticModifiers map[string]float64                       `yaml:"tactic_modifiers"`
	Overlays        map[string]graphTechniqueOverlayRowClone `yaml:"overlays"`
}

type graphOverlayDefaultsClone struct {
	RiskWeight      float64 `yaml:"risk_weight"`
	HalfLifeHours   float64 `yaml:"half_life_hours"`
	MitreBoostCap   float64 `yaml:"mitre_boost_cap"`
	MitreBoostScale float64 `yaml:"mitre_boost_scale"`
}

type graphTechniqueOverlayRowClone struct {
	MitreTechniques []graph.MitreTechniqueRef `yaml:"mitre_techniques"`
	RiskWeight      float64                   `yaml:"risk_weight,omitempty"`
	ContextScope    []string                  `yaml:"context_scope,omitempty"`
}
