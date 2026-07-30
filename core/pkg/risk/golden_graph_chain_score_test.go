package risk

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/fortuna/core/pkg/graph"
	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const (
	fixturePodUID      = "fixture-pod-1"
	fixtureRbacPodUID  = "fixture-rbac-pod-1"
	fixtureClusterID   = "golden-fixture-cluster"
	goldenRelativePath = "../../testdata/attack_path/expected/graph_chain_scores.golden.yaml"
)

type goldenScoreFile struct {
	Chains []goldenChainExpect `yaml:"chains"`
}

type goldenChainExpect struct {
	ID    string `yaml:"id"`
	Score struct {
		Min float64 `yaml:"min"`
		Max float64 `yaml:"max"`
	} `yaml:"score"`
	Components struct {
		AttackPathMin float64 `yaml:"attack_path_min"`
		AttackPathMax float64 `yaml:"attack_path_max"`
		CKDBMax       float64 `yaml:"ckdb_capability_risk_points_max"`
		MitreBoostMax float64 `yaml:"mitre_boost_max"`
	} `yaml:"components"`
	Sanity struct {
		RequiresTechniques []string `yaml:"requires_techniques"`
		RequiresMitre      []string `yaml:"requires_mitre"`
	} `yaml:"sanity"`
	Drift struct {
		ExpectedScore *float64 `yaml:"expected_score"`
		MaxDelta      *float64 `yaml:"max_delta"`
		// Soft % drift vs a documented baseline (preferred for tuning regressions).
		BaselineTotalScore *float64 `yaml:"baseline_total_score"`
		MaxDeltaPct        *float64 `yaml:"max_delta_pct"` // e.g. 20 means ±20%
	} `yaml:"drift"`
}

func loadGraphChainGolden(t *testing.T) goldenScoreFile {
	t.Helper()
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	p := filepath.Join(filepath.Dir(file), goldenRelativePath)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read golden %s: %v", p, err)
	}
	var doc goldenScoreFile
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse golden yaml: %v", err)
	}
	return doc
}

func materializeFixtureScorerDB(t *testing.T, paths []graph.AttackPath) *gorm.DB {
	t.Helper()
	return materializePathsForPod(t, fixturePodUID, "fixture", "ns-fix", paths, true)
}

// materializePathsForPod persists paths for a pod; insertEscCapability adds ESC_HOSTPATH_NODE row (escape fixture profile).
func materializePathsForPod(t *testing.T, podUID, podName, namespace string, paths []graph.AttackPath, insertEscCapability bool) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	now := time.Now()
	err = db.AutoMigrate(
		&models.Pod{},
		&models.AttackPath{},
		&models.PodCapability{},
		&models.RuntimeSignal{},
		&models.RuntimeEvent{},
		&models.Insight{},
		&models.RiskScore{},
	)
	require.NoError(t, err)
	_ = db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_test_risk_scores_key ON risk_scores(resource_type, resource_uid, cluster_id)")

	require.NoError(t, db.Create(&models.Pod{
		UID: podUID, Name: podName, Namespace: namespace, ClusterID: fixtureClusterID,
		ServiceAccount: "default", Containers: "[]",
		CreatedAt: now, UpdatedAt: now,
	}).Error)

	for i, p := range paths {
		pid := fmt.Sprintf("p%d", i)
		nodes, err := json.Marshal(p.Nodes)
		require.NoError(t, err)
		edges, err := json.Marshal(p.Edges)
		require.NoError(t, err)
		rec := models.AttackPath{
			PodUID: podUID, PathID: pid,
			Nodes: string(nodes), Edges: string(edges),
			TotalRisk: p.TotalRisk, Difficulty: p.Difficulty, Impact: p.Impact, Length: p.Length,
			Description: p.Description, EnrichedFromPCE: p.EnrichedFromPCE,
			CreatedAt: now, UpdatedAt: now,
		}
		require.NoError(t, db.Create(&rec).Error)
	}

	if insertEscCapability {
		require.NoError(t, db.Create(&models.PodCapability{
			PodUID: podUID, Namespace: namespace, CapabilityID: "ESC_HOSTPATH_NODE",
			CapabilityGroup: "ESC", Severity: "critical", State: "confirmed",
			Confidence: 0.9, Evidence: "{}", DerivedFrom: "{}",
			CreatedAt: now, UpdatedAt: now,
		}).Error)
	}

	requireMaterializationInvariantsOpts(t, db, podUID, insertEscCapability)

	return db
}

func requireMaterializationInvariants(t *testing.T, db *gorm.DB) {
	t.Helper()
	requireMaterializationInvariantsOpts(t, db, fixturePodUID, true)
}

func requireMaterializationInvariantsOpts(t *testing.T, db *gorm.DB, podUID string, expectCaps bool) {
	t.Helper()
	var nPaths, nCaps int64
	require.NoError(t, db.Model(&models.AttackPath{}).Where("pod_uid = ?", podUID).Count(&nPaths).Error)
	require.NoError(t, db.Model(&models.PodCapability{}).Where("pod_uid = ?", podUID).Count(&nCaps).Error)
	require.NotZero(t, nPaths, "attack_paths must be persisted for fixture pod")
	if expectCaps {
		require.Greater(t, nCaps, int64(0), "pod_capabilities must be present for CKDB/filter path")
	}
}

func collectPodPathIDs(t *testing.T, db *gorm.DB, podUID string) []string {
	t.Helper()
	var rows []models.AttackPath
	require.NoError(t, db.Where("pod_uid = ?", podUID).Find(&rows).Error)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.PathID)
	}
	sort.Strings(out)
	return out
}

func assertChainPathsExclusiveToMaterializedPaths(t *testing.T, ch *graph.AttackChain, db *gorm.DB, podUID string) {
	t.Helper()
	chainIDs := append([]string(nil), ch.Paths...)
	sort.Strings(chainIDs)
	all := collectPodPathIDs(t, db, podUID)
	require.Equal(t, len(chainIDs), len(all), "chain path count must match persisted rows (no stray paths)")
	require.True(t, slices.Equal(chainIDs, all),
		"materialized path_ids must equal chain.Paths exactly (subset both ways): chain=%v db=%v", chainIDs, all)
}

func loadPersistedAttackPaths(t *testing.T, db *gorm.DB, podUID string) []models.AttackPath {
	t.Helper()
	var rows []models.AttackPath
	require.NoError(t, db.Where("pod_uid = ?", podUID).Order("path_id").Find(&rows).Error)
	return rows
}

func loadPodCapabilities(t *testing.T, db *gorm.DB, podUID string) []models.PodCapability {
	t.Helper()
	var caps []models.PodCapability
	require.NoError(t, db.Where("pod_uid = ?", podUID).Find(&caps).Error)
	return caps
}

// assertCKDBUsesOnlyFlowCaps ensures FilterPodCapabilitiesForCKDB outputs only IDs present on path topology flow.
func assertCKDBUsesOnlyFlowCaps(t *testing.T, paths []models.AttackPath, caps []models.PodCapability, usedCapIDs []string) {
	t.Helper()
	flow := graph.CapabilityFlowSetFromAttackPaths(paths)
	for _, raw := range usedCapIDs {
		up := strings.ToUpper(strings.TrimSpace(raw))
		can := strings.ToUpper(graph.CanonicalCapability(raw))
		ok := flow[up] || flow[can] || flow[strings.ToUpper(strings.TrimSpace(graph.CanonicalCapability(up)))]
		require.True(t, ok, "CKDB used cap %q must appear on capability flow from attack_paths topology", raw)
	}
}

func requireUniquePathIDs(t *testing.T, paths []graph.AttackPath) {
	t.Helper()
	seen := map[string]bool{}
	for _, p := range paths {
		id := strings.TrimSpace(p.PathID)
		require.NotEmpty(t, id)
		require.False(t, seen[id], "duplicate PathID %q", id)
		seen[id] = true
	}
}

// assignStablePathIDs sets PathID to p0..pn — must stay aligned with chain_detection.normalizePaths indexing.
func assignStablePathIDs(t *testing.T, paths []graph.AttackPath) {
	t.Helper()
	for i := range paths {
		want := fmt.Sprintf("p%d", i)
		paths[i].PathID = want
		require.Equal(t, want, paths[i].PathID, "path index %d PathID invariant", i)
	}
	requireUniquePathIDs(t, paths)
}

func pathsByID(paths []graph.AttackPath) map[string]graph.AttackPath {
	m := make(map[string]graph.AttackPath, len(paths))
	for _, p := range paths {
		if p.PathID != "" {
			m[p.PathID] = p
		}
	}
	return m
}

// assertChainWorkloadBinding ensures chain.Paths reference materialized paths that start at the workload pod UID.
func assertChainWorkloadBinding(t *testing.T, ch *graph.AttackChain, paths []graph.AttackPath, podUID string) {
	t.Helper()
	require.NotEmpty(t, ch.Paths, "chain must reference AttackPath IDs (p0, p1, …)")
	byID := pathsByID(paths)
	for _, pid := range ch.Paths {
		ap, ok := byID[pid]
		require.True(t, ok, "chain references path id %s not found in fixture paths", pid)
		require.NotEmpty(t, ap.Nodes, "path %s must have nodes", pid)
		got := strings.TrimSpace(ap.Nodes[0].ID)
		require.Equal(t, podUID, got, "path %s must anchor at workload pod %s", pid, podUID)
	}
}

func dimensionAttackPathFromFactors(f map[string]interface{}) float64 {
	dims, ok := f["dimensions"].(map[string]interface{})
	if !ok || dims == nil {
		return 0
	}
	return factorFloat(dims, "attack_path")
}

func distinctPathMitreIDsFromChain(ch *graph.AttackChain) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range ch.Steps {
		for _, m := range s.MitreTechniques {
			id := strings.TrimSpace(strings.ToUpper(m.ID))
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func findChainByType(chains []graph.AttackChain, typ string) *graph.AttackChain {
	for i := range chains {
		if chains[i].Type == typ {
			return &chains[i]
		}
	}
	return nil
}

func chainHasTechniques(ch *graph.AttackChain, want []string) bool {
	have := map[string]bool{}
	for _, s := range ch.Steps {
		have[s.TechniqueID] = true
	}
	for _, id := range want {
		if !have[id] {
			return false
		}
	}
	return true
}

func chainHasPathMitre(ch *graph.AttackChain, want []string) bool {
	have := map[string]bool{}
	for _, s := range ch.Steps {
		for _, m := range s.MitreTechniques {
			id := strings.TrimSpace(m.ID)
			if id != "" {
				have[strings.ToUpper(id)] = true
			}
		}
	}
	for _, id := range want {
		if !have[strings.ToUpper(strings.TrimSpace(id))] {
			return false
		}
	}
	return true
}

func factorFloat(m map[string]interface{}, key string) float64 {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	default:
		return 0
	}
}

func assertScoreComponents(t *testing.T, sc *UnifiedScoreV3, g goldenChainExpect) {
	t.Helper()
	f := sc.Factors
	require.Greater(t, sc.AttackPathScore, 0.0, "attack_path dimension must be non-zero when graph→path→persist path holds")
	apFactors := dimensionAttackPathFromFactors(f)
	require.InEpsilon(t, sc.AttackPathScore, apFactors, 0.02, "Factors.dimensions.attack_path must match AttackPathScore")

	if g.Components.CKDBMax > 0 {
		ck := factorFloat(f, "ckdb_capability_risk_points")
		require.GreaterOrEqual(t, ck, 0.0, "ckdb_capability_risk_points")
		require.LessOrEqual(t, ck, 4.0, "CKDB injection hard cap (engine + test)")
		require.LessOrEqual(t, ck, g.Components.CKDBMax, "ckdb_capability_risk_points golden max")
	}
	if g.Components.MitreBoostMax > 0 {
		mb := factorFloat(f, "mitre_runtime_attack_path_boost")
		require.GreaterOrEqual(t, mb, 0.0)
		require.LessOrEqual(t, mb, g.Components.MitreBoostMax, "mitre_runtime_attack_path_boost")
	}
	if g.Components.AttackPathMin > 0 || g.Components.AttackPathMax > 0 {
		ap := sc.AttackPathScore
		if g.Components.AttackPathMin > 0 {
			require.GreaterOrEqual(t, ap, g.Components.AttackPathMin, "AttackPathScore min")
		}
		if g.Components.AttackPathMax > 0 {
			require.LessOrEqual(t, ap, g.Components.AttackPathMax, "AttackPathScore max")
		}
	}
}

func assertDriftOptional(t *testing.T, score float64, g goldenChainExpect) {
	t.Helper()
	if g.Drift.ExpectedScore != nil && g.Drift.MaxDelta != nil {
		delta := math.Abs(score - *g.Drift.ExpectedScore)
		require.LessOrEqual(t, delta, *g.Drift.MaxDelta,
			"score drift vs golden expected_score (set drift.max_delta wider if intentional tuning)")
	}
	if g.Drift.BaselineTotalScore != nil && g.Drift.MaxDeltaPct != nil && *g.Drift.BaselineTotalScore > 0 {
		baseline := *g.Drift.BaselineTotalScore
		pct := *g.Drift.MaxDeltaPct / 100.0
		rel := math.Abs(score-baseline) / baseline
		require.LessOrEqual(t, rel, pct,
			"TotalScore drift vs drift.baseline_total_score exceeds drift.max_delta_pct (%.1f%%)", *g.Drift.MaxDeltaPct)
		// Directional band: catch dangerous inflate vs accidental collapse under same % window.
		if score > baseline {
			require.LessOrEqual(t, score, baseline*(1+pct), "inflate guard: score above baseline must stay within +max_delta_pct")
		} else {
			require.GreaterOrEqual(t, score, baseline*(1-pct), "collapse guard: score below baseline must stay within -max_delta_pct")
		}
	}
}

// TestGolden_GraphChain_ScoreRanges locks workload-level Unified V3 output for the escape→priv-esc
// fixture (same graph as graph golden). Chain row identifies scenario type; TotalScore is pod V3.
func TestGolden_GraphChain_ScoreRanges(t *testing.T) {
	paths := graph.AttackPathsFromEscapeClusterAdminFixture(fixturePodUID)
	assignStablePathIDs(t, paths)

	chains := graph.DetectChainsWithHints(paths, graph.DefaultHardeningHints())
	require.NotEmpty(t, chains)

	golden := loadGraphChainGolden(t)
	db := materializeFixtureScorerDB(t, paths)
	scorer := NewUnifiedScorerV3(db)
	sc, err := scorer.CalculateScoreV3(context.Background(), fixturePodUID)
	require.NoError(t, err)

	for _, g := range golden.Chains {
		c := findChainByType(chains, g.ID)
		require.NotNil(t, c, "chain type %s", g.ID)
		assertChainWorkloadBinding(t, c, paths, fixturePodUID)
		assertChainPathsExclusiveToMaterializedPaths(t, c, db, fixturePodUID)

		require.True(t, chainHasTechniques(c, g.Sanity.RequiresTechniques),
			"techniques missing: want %v steps=%v", g.Sanity.RequiresTechniques, chainStepTechniqueIDs(c))
		require.True(t, chainHasPathMitre(c, g.Sanity.RequiresMitre),
			"path MITRE missing: want %v", g.Sanity.RequiresMitre)

		t.Logf("golden chain_id=%s (type=%s) TotalScore=%.2f AttackPathDim=%.2f chain_paths=%v",
			g.ID, c.Type, sc.TotalScore, sc.AttackPathScore, c.Paths)

		require.GreaterOrEqual(t, sc.TotalScore, g.Score.Min)
		require.LessOrEqual(t, sc.TotalScore, g.Score.Max)
		assertScoreComponents(t, sc, g)
		assertDriftOptional(t, sc.TotalScore, g)

		modelPaths := loadPersistedAttackPaths(t, db, fixturePodUID)
		caps := loadPodCapabilities(t, db, fixturePodUID)
		used := graph.FilterPodCapabilitiesForCKDB(modelPaths, caps)
		assertCKDBUsesOnlyFlowCaps(t, modelPaths, caps, used)
	}

	sc2, err := scorer.CalculateScoreV3(context.Background(), fixturePodUID)
	require.NoError(t, err)
	require.InDelta(t, sc.TotalScore, sc2.TotalScore, 0.01, "CalculateScoreV3 must be idempotent on same DB state")
	mb1 := factorFloat(sc.Factors, "mitre_runtime_attack_path_boost")
	mb2 := factorFloat(sc2.Factors, "mitre_runtime_attack_path_boost")
	require.InDelta(t, mb1, mb2, 0.01, "MITRE boost factors must be stable across identical scorer runs")
}

func chainStepTechniqueIDs(ch *graph.AttackChain) []string {
	var out []string
	for _, s := range ch.Steps {
		out = append(out, s.TechniqueID)
	}
	return out
}

// TestGolden_GraphChain_NoisyRuntimeMitigation ensures spammy runtime_events do not inflate MITRE boost
// past the relaxed noise ceiling or pretend high overlay correlation precision.
func TestGolden_GraphChain_NoisyRuntimeMitigation(t *testing.T) {
	paths := graph.AttackPathsFromEscapeClusterAdminFixture(fixturePodUID)
	assignStablePathIDs(t, paths)
	chains := graph.DetectChainsWithHints(paths, graph.DefaultHardeningHints())
	c := findChainByType(chains, "ESCAPE_TO_PRIV_ESC")
	require.NotNil(t, c)

	db := materializeFixtureScorerDB(t, paths)

	scorer := NewUnifiedScorerV3(db)
	scClean, err := scorer.CalculateScoreV3(context.Background(), fixturePodUID)
	require.NoError(t, err)
	mbClean := factorFloat(scClean.Factors, "mitre_runtime_attack_path_boost")

	now := time.Now()
	const noiseMitre = "T_GOLDEN_FIXTURE_NOISE_UNMAPPED"
	for i := 0; i < 8; i++ {
		require.NoError(t, db.Create(&models.RuntimeEvent{
			PodUID: fixturePodUID, Namespace: "ns-fix", Mitre: noiseMitre,
			Syscall: "execve", CreatedAt: now, Confidence: 0.85,
		}).Error)
	}
	require.NoError(t, db.Create(&models.RuntimeEvent{
		PodUID: fixturePodUID, Namespace: "ns-fix", Mitre: "T1098.006",
		Syscall: "open", CreatedAt: now, Confidence: 0.85,
	}).Error)

	sc, err := scorer.CalculateScoreV3(context.Background(), fixturePodUID)
	require.NoError(t, err)
	mb := factorFloat(sc.Factors, "mitre_runtime_attack_path_boost")
	cp := factorFloat(sc.Factors, "mitre_correlation_precision_scorer")
	t.Logf("noisy_profile mitre_boost=%.3f corr_prec=%.3f (clean_boost=%.3f)", mb, cp, mbClean)
	// Clean→noisy can move boost from 0 into low single digits; cap absolute swing (engine scale), not 0.5 pt.
	require.InDelta(t, mbClean, mb, 2.5, "MITRE path boost change from clean→noisy must stay within small absolute band")
	require.Less(t, mb, 3.5, "sparse unmapped spam must not push MITRE boost near cap")
	require.Less(t, cp, 0.6, "overlay-correlation precision must stay low under mostly-unmapped spam")

	scAgain, err := scorer.CalculateScoreV3(context.Background(), fixturePodUID)
	require.NoError(t, err)
	mbAgain := factorFloat(scAgain.Factors, "mitre_runtime_attack_path_boost")
	require.InDelta(t, mb, mbAgain, 0.01, "noisy scorer must be idempotent on same DB snapshot")

	graph.EnrichAttackChainsWithRuntimeMitre(db, fixtureClusterID, chains, paths)
	c2 := findChainByType(chains, "ESCAPE_TO_PRIV_ESC")
	require.NotNil(t, c2.MitreSummary)
	require.Less(t, c2.MitreSummary.CorrelationPrecision, 0.6)

	pathMitre := distinctPathMitreIDsFromChain(c2)
	require.NotContains(t, pathMitre, "T1046", "path narrative MITRE must not include noisy discovery T1046")
}

// TestGolden_GraphChain_ScoreDropsWithoutAttackPaths proves attack_paths rows drive V3 TotalScore down when removed (negative guard).
func TestGolden_GraphChain_ScoreDropsWithoutAttackPaths(t *testing.T) {
	paths := graph.AttackPathsFromEscapeClusterAdminFixture(fixturePodUID)
	assignStablePathIDs(t, paths)

	dbWith := materializeFixtureScorerDB(t, paths)
	scorer := NewUnifiedScorerV3(dbWith)
	scWith, err := scorer.CalculateScoreV3(context.Background(), fixturePodUID)
	require.NoError(t, err)
	require.Greater(t, scWith.AttackPathScore, 0.0)

	dbEmpty, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	now := time.Now()
	require.NoError(t, dbEmpty.AutoMigrate(&models.Pod{}, &models.PodCapability{}, &models.AttackPath{}, &models.RuntimeSignal{}, &models.RuntimeEvent{}, &models.Insight{}, &models.RiskScore{}))
	_ = dbEmpty.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_test_risk_scores_key2 ON risk_scores(resource_type, resource_uid, cluster_id)")
	require.NoError(t, dbEmpty.Create(&models.Pod{
		UID: fixturePodUID, Name: "fixture", Namespace: "ns-fix", ClusterID: fixtureClusterID,
		ServiceAccount: "default", Containers: "[]",
		CreatedAt: now, UpdatedAt: now,
	}).Error)
	require.NoError(t, dbEmpty.Create(&models.PodCapability{
		PodUID: fixturePodUID, Namespace: "ns-fix", CapabilityID: "ESC_HOSTPATH_NODE",
		CapabilityGroup: "ESC", Severity: "critical", State: "confirmed",
		Confidence: 0.9, Evidence: "{}", DerivedFrom: "{}",
		CreatedAt: now, UpdatedAt: now,
	}).Error)
	var nEmpty int64
	require.NoError(t, dbEmpty.Model(&models.AttackPath{}).Where("pod_uid = ?", fixturePodUID).Count(&nEmpty).Error)
	require.Zero(t, nEmpty, "negative test DB must have no attack_paths")

	scorerEmpty := NewUnifiedScorerV3(dbEmpty)
	scEmpty, err := scorerEmpty.CalculateScoreV3(context.Background(), fixturePodUID)
	require.NoError(t, err)

	require.Equal(t, 0.0, scEmpty.AttackPathScore, "attack_path dimension must be exactly zero without persisted paths (no hidden floor)")
	require.InDelta(t, 0.0, scEmpty.AttackPathScore, 1e-9)
	require.Greater(t, scWith.AttackPathScore, scEmpty.AttackPathScore)
	require.Greater(t, scWith.TotalScore, scEmpty.TotalScore)
}

// TestGolden_GraphChain_AttackPathSensitivityOnePathRemoved asserts attack_path dimension drops when topology loses one persisted path.
func TestGolden_GraphChain_AttackPathSensitivityOnePathRemoved(t *testing.T) {
	paths := graph.AttackPathsFromEscapeClusterAdminFixture(fixturePodUID)
	assignStablePathIDs(t, paths)
	require.GreaterOrEqual(t, len(paths), 2)

	dbFull := materializeFixtureScorerDB(t, paths)
	scFull, err := NewUnifiedScorerV3(dbFull).CalculateScoreV3(context.Background(), fixturePodUID)
	require.NoError(t, err)

	pathLess := paths[:len(paths)-1]
	assignStablePathIDs(t, pathLess)
	dbLess := materializePathsForPod(t, fixturePodUID, "fixture", "ns-fix", pathLess, true)
	scLess, err := NewUnifiedScorerV3(dbLess).CalculateScoreV3(context.Background(), fixturePodUID)
	require.NoError(t, err)

	require.Greater(t, scFull.AttackPathScore, scLess.AttackPathScore,
		"attack_path dimension must respond when one graph path row is omitted")
	require.Greater(t, scFull.TotalScore, scLess.TotalScore)
}

// TestGolden_RbacOnlyClusterAdmin_NoEscapeInflation ensures RBAC-only topology scores below escape fixture and omits escape graph hops.
func TestGolden_RbacOnlyClusterAdmin_NoEscapeInflation(t *testing.T) {
	pathsRBAC := graph.AttackPathsFromRbacOnlyClusterAdminFixture(fixtureRbacPodUID)
	assignStablePathIDs(t, pathsRBAC)
	require.False(t, pathsTouchNode(pathsRBAC), "RBAC-only fixture must not include a worker node hop")
	require.True(t, hasPrivEscClass(pathsRBAC))
	for _, p := range pathsRBAC {
		if p.Explainability != nil {
			require.NotEqual(t, graph.PathClassEscape, p.Explainability.Class)
		}
	}

	pathsEscape := graph.AttackPathsFromEscapeClusterAdminFixture(fixturePodUID)
	assignStablePathIDs(t, pathsEscape)
	require.True(t, pathsTouchNode(pathsEscape))

	dbRbac := materializePathsForPod(t, fixtureRbacPodUID, "rbac-fix", "ns-rbac", pathsRBAC, false)
	scRbac, err := NewUnifiedScorerV3(dbRbac).CalculateScoreV3(context.Background(), fixtureRbacPodUID)
	require.NoError(t, err)

	dbEscape := materializeFixtureScorerDB(t, pathsEscape)
	scEscape, err := NewUnifiedScorerV3(dbEscape).CalculateScoreV3(context.Background(), fixturePodUID)
	require.NoError(t, err)

	require.Less(t, scRbac.TotalScore, scEscape.TotalScore)
	require.Less(t, scRbac.AttackPathScore, scEscape.AttackPathScore)
	t.Logf("rbac-only fixture baseline: TotalScore=%.2f attack_path_dim=%.2f; escape fixture: Total=%.2f attack_path_dim=%.2f",
		scRbac.TotalScore, scRbac.AttackPathScore, scEscape.TotalScore, scEscape.AttackPathScore)
	// Minimal DB (paths only, no ESC_HOSTPATH_NODE CKDB row): tuned band after temporal/MITRE/runtime guards — see docs/03-components/COMPONENTS.md#attack-path-and-risk-signals (Golden score regression).
	require.GreaterOrEqual(t, scRbac.TotalScore, 5.0)
	require.LessOrEqual(t, scRbac.TotalScore, 9.5)
	require.GreaterOrEqual(t, scRbac.AttackPathScore, 6.0)
	require.LessOrEqual(t, scRbac.AttackPathScore, 10.5)

	refs := graph.MitreRefsForTechnique("RBAC_PRIV_ESC")
	require.Len(t, refs, 1)
	require.Equal(t, "T1098.006", refs[0].ID)
}

// TestGolden_FalseEscalationIllusion_RbacShellNetworkNoiseWithoutK8sAPI seals the “looks like escalation” narrative:
// cluster-admin path + shell + heavy DNS-like network noise, but no K8s API corroboration — must not satisfy SA_TOKEN,
// must not apply shell+API toxic bonus, and must stay below the escape workload score.
func TestGolden_FalseEscalationIllusion_RbacShellNetworkNoiseWithoutK8sAPI(t *testing.T) {
	pathsRBAC := graph.AttackPathsFromRbacOnlyClusterAdminFixture(fixtureRbacPodUID)
	assignStablePathIDs(t, pathsRBAC)
	pathsEscape := graph.AttackPathsFromEscapeClusterAdminFixture(fixturePodUID)
	assignStablePathIDs(t, pathsEscape)

	db := materializePathsForPod(t, fixtureRbacPodUID, "rbac-fix", "ns-rbac", pathsRBAC, false)
	now := time.Now().UTC()
	dnsNoiseEvidence := `{"dst":"kube-dns.kube-system.svc.cluster.local","port":53,"protocol":"tcp"}`
	// “High” noise: multiple queue rows + sizeable counts (DNS-like evidence is explicitly non–K8s API).
	signals := []models.RuntimeSignal{
		{
			PodUID: fixtureRbacPodUID, SignalType: "INTERACTIVE_SHELL_EXEC", Category: "runtime",
			Confidence: 0.82, Evidence: "{}", Count: 2, CreatedAt: now,
		},
		{
			PodUID: fixtureRbacPodUID, SignalType: "NETWORK_QUEUE_ANOMALY", Category: "network",
			Confidence: 0.7, Evidence: dnsNoiseEvidence, Count: 12, CreatedAt: now,
		},
		{
			PodUID: fixtureRbacPodUID, SignalType: "NETWORK_QUEUE_ANOMALY", Category: "network",
			Confidence: 0.65, Evidence: dnsNoiseEvidence, Count: 8, CreatedAt: now,
		},
	}
	for i := range signals {
		require.NoError(t, db.Create(&signals[i]).Error)
	}

	var noEvents []models.RuntimeEvent
	require.False(t, CorroboratesK8sAPIAccess(noEvents, signals), "DNS-style queue evidence must not count as K8s API corroboration")
	require.Empty(t, runtimeSatisfiedRequirements(noEvents, signals), "SA_TOKEN must not be runtime-satisfied without K8s API evidence")

	derived, _ := mergeRuntimeDerivedPodCapabilities(fixtureRbacPodUID, "ns-rbac", nil, signals, noEvents)
	var token *models.PodCapability
	for i := range derived {
		if derived[i].CapabilityID == runtimeDerivedIDTokenPod {
			token = &derived[i]
			break
		}
	}
	require.NotNil(t, token, "network anomaly still injects ID_TOKEN_POD for scoring context")
	require.NotEqual(t, "exploited", strings.ToLower(token.State), "without K8s API corroboration token must not reach exploited (no false escalation)")
	require.Equal(t, "confirmed", strings.ToLower(token.State))

	scorer := NewUnifiedScorerV3(db)
	sc, err := scorer.CalculateScoreV3(context.Background(), fixtureRbacPodUID)
	require.NoError(t, err)

	require.Less(t, factorFloat(sc.Factors, "sa_token_satisfaction_weight"), 0.28)
	reqSat := factorStringSliceFromFactors(sc.Factors, "runtime_satisfied_requirements")
	require.NotContains(t, reqSat, "SA_TOKEN")
	require.Zero(t, factorFloat(sc.Factors, "runtime_toxic_combo_shell_api_bonus"), "no toxic shell+K8s API bonus without corroboration")
	require.False(t, factorBoolFromFactors(sc.Factors, "runtime_k8s_api_corroborated"))

	// “Medium” runtime threat: clearly non-zero from shell + volume of noisy network signals, but not near dimension ceiling.
	require.Greater(t, sc.RuntimeThreatScore, 3.5, "expected audible runtime layer from shell + network churn")
	require.Less(t, sc.RuntimeThreatScore, 11.5, "must not spike as if K8s API toxic combo fired")

	// A/B: identical RBAC + signals plus one strong K8s API runtime_event → strictly higher unified score
	// (runtime-derived caps + toxic bonus). Clean escape baseline (~16.6) is *without* this runtime load;
	// comparing illusion to clean escape is misleading once RUNTIME_THREAT and capability lifts apply.
	dbAB := materializePathsForPod(t, fixtureRbacPodUID, "rbac-fix", "ns-rbac", pathsRBAC, false)
	for i := range signals {
		require.NoError(t, dbAB.Create(&signals[i]).Error)
	}
	require.NoError(t, dbAB.Create(&models.RuntimeEvent{
		PodUID: fixtureRbacPodUID, Namespace: "ns-rbac", Syscall: "connect",
		SourceRule: "Contact K8s API Server From Container", Confidence: 0.9,
		CreatedAt: now,
	}).Error)
	scUp, err := NewUnifiedScorerV3(dbAB).CalculateScoreV3(context.Background(), fixtureRbacPodUID)
	require.NoError(t, err)
	require.Greater(t, scUp.TotalScore, sc.TotalScore+0.5,
		"K8s API–corroborated profile must score materially higher than the false-illusion profile")
	require.Contains(t, factorStringSliceFromFactors(scUp.Factors, "runtime_satisfied_requirements"), "SA_TOKEN")
	require.Greater(t, factorFloat(scUp.Factors, "runtime_toxic_combo_shell_api_bonus"), 0.0)

	dbEscape := materializeFixtureScorerDB(t, pathsEscape)
	scEscape, err := NewUnifiedScorerV3(dbEscape).CalculateScoreV3(context.Background(), fixturePodUID)
	require.NoError(t, err)
	t.Logf("false-illusion total=%.2f runtime=%.2f; clean escape total=%.2f (escape has no runtime seed)", sc.TotalScore, sc.RuntimeThreatScore, scEscape.TotalScore)
	_ = scEscape
}

func factorStringSliceFromFactors(m map[string]interface{}, key string) []string {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	switch x := v.(type) {
	case []string:
		return x
	case []interface{}:
		out := make([]string, 0, len(x))
		for _, e := range x {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func factorBoolFromFactors(m map[string]interface{}, key string) bool {
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	switch x := v.(type) {
	case bool:
		return x
	default:
		return false
	}
}

func pathsTouchNode(paths []graph.AttackPath) bool {
	for _, p := range paths {
		for _, n := range p.Nodes {
			if strings.EqualFold(n.Type, graph.NodeTypeNode) {
				return true
			}
		}
	}
	return false
}

func hasPrivEscClass(paths []graph.AttackPath) bool {
	for _, p := range paths {
		if p.Explainability != nil && p.Explainability.Class == graph.PathClassPrivEsc {
			return true
		}
	}
	return false
}
