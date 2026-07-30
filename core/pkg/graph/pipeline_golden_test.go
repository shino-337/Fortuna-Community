package graph

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func attackStepFromRegistry(t *testing.T, tid string) AttackStep {
	t.Helper()
	at, ok := TechniqueByID(tid)
	require.True(t, ok, "technique "+tid)
	return AttackStep{
		TechniqueID:     at.TechniqueID,
		Name:            at.Name,
		InputCaps:       at.Requires,
		OutputCaps:      at.Provides,
		Realism:         at.Realism,
		Cost:            at.Cost,
		MitreTechniques: at.MitreTechniques,
		ContextScopes:   at.ContextScopes,
	}
}

func techniqueIDs(steps []AttackStep) []string {
	var out []string
	for _, s := range steps {
		out = append(out, s.TechniqueID)
	}
	return out
}

func mitreIDsOnSteps(steps []AttackStep) []string {
	return distinctPathMitreIDs(steps)
}

// Minimal pipeline invariants: registry → steps → MITRE ids (Graph→Path→Chain surrogate without DB graph).

func TestGolden_RBAC_chain_has_priv_esc_mitre(t *testing.T) {
	steps := []AttackStep{attackStepFromRegistry(t, "RBAC_PRIV_ESC")}
	require.Contains(t, techniqueIDs(steps), "RBAC_PRIV_ESC")
	require.Contains(t, mitreIDsOnSteps(steps), "T1098.006")
}

func TestGolden_escape_chain_has_container_escape_mitre(t *testing.T) {
	steps := []AttackStep{attackStepFromRegistry(t, "ESCAPE_HOSTPATH")}
	require.Contains(t, mitreIDsOnSteps(steps), "T1611")
}

func TestGolden_escape_realism_respects_capability_when_no_gaps(t *testing.T) {
	steps := []AttackStep{attackStepFromRegistry(t, "ESCAPE_HOSTPATH")}
	ch := AttackChain{Steps: steps, Realism: 0.85}
	ch.CapabilityValidation = BuildCapabilityValidation(steps)
	applyCapabilityValidationToRealism(&ch)
	require.InDelta(t, 0.85, ch.Realism, 0.002)
}

func TestGolden_mixed_chain_MITRE_union(t *testing.T) {
	steps := []AttackStep{
		attackStepFromRegistry(t, "ESCAPE_HOSTPATH"),
		attackStepFromRegistry(t, "RBAC_PRIV_ESC"),
	}
	m := mitreIDsOnSteps(steps)
	require.Contains(t, m, "T1611")
	require.Contains(t, m, "T1098.006")
	ms := BuildMitreCoverage(m, map[string]bool{}, 0.8, 0.75)
	require.NotEmpty(t, ms)
	for _, row := range ms {
		if row.Status == "inferred" {
			require.NotNil(t, row.Confidence)
			require.GreaterOrEqual(t, *row.Confidence, 0.2)
			require.LessOrEqual(t, *row.Confidence, 1.0)
		}
	}
}
