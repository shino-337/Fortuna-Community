package graph

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func canonicalContextScopes(s []string) []string {
	out := make([]string, 0, len(s))
	for _, x := range s {
		t := strings.TrimSpace(x)
		if t != "" {
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}

// Phase 4.1 shadow mode: embedded YAML parses to the same overlay rows as globals, and TechniqueByID
// merged output matches YAML semantics (MITRE refs + effective risk_weight). Computation still flows
// through TechniqueByID — YAML file is not a second scorer.
func TestPhase41_Shadow_OverlayYAMLMatchesMergedTechniqueByID(t *testing.T) {
	raw := TechniqueOverlayEmbeddedYAML()
	var f techniqueOverlayFile
	require.NoError(t, yaml.Unmarshal(raw, &f))

	// No silent drop: every overlay key in the file is present in the runtime index.
	require.Equal(t, len(f.Overlays), len(techniqueOverlayByID), "parsed overlay count must match techniqueOverlayByID (no missing keys)")

	for yamlID, row := range f.Overlays {
		key := strings.ToUpper(strings.TrimSpace(yamlID))
		stored, ok := techniqueOverlayByID[key]
		require.True(t, ok, "overlay key %s missing from runtime index", yamlID)

		// Canonical MITRE: order/alias/case must not matter; require full multiset match.
		require.ElementsMatch(t,
			CanonicalMitreTechniqueIDs(row.MitreTechniques),
			CanonicalMitreTechniqueIDs(stored.MitreTechniques),
			"YAML vs runtime MitreTechniques %s", yamlID)

		require.Equal(t, row.RiskWeight, stored.RiskWeight)
		require.Equal(t,
			canonicalContextScopes(row.ContextScope),
			canonicalContextScopes(stored.ContextScope),
			"context_scope drift %s", yamlID)

		if !reflect.DeepEqual(row.MitreTechniques, stored.MitreTechniques) || row.RiskWeight != stored.RiskWeight {
			t.Logf("DIFF overlay row %s (raw vs runtime): yaml=%v stored=%v", yamlID, row.MitreTechniques, stored.MitreTechniques)
		}

		merged, ok := TechniqueByID(key)
		if !ok {
			continue
		}

		require.ElementsMatch(t,
			CanonicalMitreTechniqueIDs(row.MitreTechniques),
			CanonicalMitreTechniqueIDs(merged.MitreTechniques),
			"merged TechniqueByID Mitre refs %s", yamlID)

		expRW := row.RiskWeight
		if expRW <= 0 {
			expRW = defaultOverlayRiskWeight()
			if expRW <= 0 {
				expRW = 1.0
			}
		}
		if merged.RiskWeight != expRW {
			t.Logf("DIFF risk_weight %s: merged=%v want effective %v", key, merged.RiskWeight, expRW)
		}
		require.Equal(t, expRW, merged.RiskWeight, "effective risk_weight %s", yamlID)
	}
}

func TestPhase41_ShadowReload_IdempotentMergedTechnique(t *testing.T) {
	t.Cleanup(func() {
		require.NoError(t, ReloadTechniqueOverlay(TechniqueOverlayEmbeddedYAML()))
	})

	baseline := TechniqueOverlayEmbeddedYAML()
	require.NoError(t, ReloadTechniqueOverlay(baseline))

	a1, ok := TechniqueByID("RBAC_PRIV_ESC")
	require.True(t, ok)
	require.NoError(t, ReloadTechniqueOverlay(baseline))
	a2, ok := TechniqueByID("RBAC_PRIV_ESC")
	require.True(t, ok)
	require.Equal(t, a1.RiskWeight, a2.RiskWeight)
	require.Equal(t,
		CanonicalMitreTechniqueIDs(a1.MitreTechniques),
		CanonicalMitreTechniqueIDs(a2.MitreTechniques))
}

func TestPhase41_Canonical_NoDuplicateAfterNormalize(t *testing.T) {
	refs := []MitreTechniqueRef{
		{ID: "t1611"}, {ID: "T1611"}, {ID: "T1611 "},
	}
	ids := CanonicalMitreTechniqueIDs(refs)
	require.Len(t, ids, 1)
	require.Equal(t, "T1611", ids[0])
}
