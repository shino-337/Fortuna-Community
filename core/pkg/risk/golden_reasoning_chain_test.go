package risk

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fortuna/core/pkg/graph"
	"github.com/fortuna/core/pkg/models"
)

// TestGoldenReasoningChain_E2E locks graph fixture + runtime corroboration + V3 explainability shape.
func TestGoldenReasoningChain_E2E(t *testing.T) {
	paths := graph.AttackPathsFromEscapeClusterAdminFixture(fixturePodUID)
	assignStablePathIDs(t, paths)

	db := materializeFixtureScorerDB(t, paths)
	now := time.Now().UTC()

	require.NoError(t, db.Create(&models.RuntimeEvent{
		PodUID:     fixturePodUID,
		Namespace:  "ns-fix",
		Syscall:    "connect",
		TargetPath: "kubernetes.default:443",
		SourceRule: "Contact K8s API Server From Container",
		Mitre:      "T1078",
		CreatedAt:  now,
		Confidence: 0.92,
	}).Error)

	require.NoError(t, db.Create(&models.RuntimeSignal{
		PodUID:     fixturePodUID,
		SignalType: "INTERACTIVE_SHELL_EXEC",
		Category:   "EXECUTION",
		Confidence: 0.88,
		Evidence:   `{"source":"test"}`,
		CreatedAt:  now,
	}).Error)

	chains := graph.DetectChainsWithHints(paths, graph.DefaultHardeningHints())
	graph.EnrichAttackChainsWithRuntimeMitre(db, fixtureClusterID, chains, paths)
	var esc *graph.AttackChain
	for i := range chains {
		if chains[i].Type == "ESCAPE_TO_PRIV_ESC" {
			esc = &chains[i]
			break
		}
	}
	require.NotNil(t, esc)
	require.NotNil(t, esc.CapabilityValidation)
	require.NotContains(t, esc.CapabilityValidation.Gaps, "SA_TOKEN", "K8s API runtime should close SA_TOKEN gap")

	scorer := NewUnifiedScorerV3(db)
	sc, err := scorer.CalculateScoreV3(context.Background(), fixturePodUID)
	require.NoError(t, err)
	require.Greater(t, sc.TotalScore, 4.0)
	require.Less(t, sc.TotalScore, 95.0)

	raw, err := json.Marshal(sc.Factors)
	require.NoError(t, err)
	var factors map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &factors))

	sat, ok := factors["runtime_satisfied_requirements"].([]interface{})
	require.True(t, ok, "runtime_satisfied_requirements must be JSON array")
	found := false
	for _, x := range sat {
		if fmt.Sprint(x) == "SA_TOKEN" {
			found = true
			break
		}
	}
	require.True(t, found, "factors.runtime_satisfied_requirements must list SA_TOKEN")

	ex, ok := factors["explainability"].(map[string]interface{})
	require.True(t, ok)
	rt, ok := ex["runtime_threat"].(map[string]interface{})
	require.True(t, ok)
	require.Contains(t, rt, "k8s_api_corroboration")
	require.Contains(t, rt, "satisfied_requirements")

	require.NotNil(t, factors["mitre_runtime_attack_path_boost"])
	if m, ok := factors["runtime_influence_capability_mul"].(float64); ok {
		require.GreaterOrEqual(t, m, 1.0)
	}
	if m, ok := factors["runtime_influence_blast_mul"].(float64); ok {
		require.GreaterOrEqual(t, m, 1.0)
	}
}
