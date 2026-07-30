package graph

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func techniqueIDsFromSteps(steps []AttackStep) []string {
	var out []string
	for _, s := range steps {
		out = append(out, s.TechniqueID)
	}
	return out
}

func pathClasses(paths []AttackPath) []string {
	var out []string
	for _, p := range paths {
		if p.Explainability != nil && p.Explainability.Class != "" {
			out = append(out, p.Explainability.Class)
		}
	}
	return out
}

func TestFixture_GraphToPaths_toNode_and_toClusterRole(t *testing.T) {
	paths := AttackPathsFromEscapeClusterAdminFixture("fixture-pod-1")
	require.GreaterOrEqual(t, len(paths), 2, "need escape + priv_esc paths: classes=%v", pathClasses(paths))
	cls := pathClasses(paths)
	var hasEscape, hasPriv bool
	for _, c := range cls {
		if c == PathClassEscape {
			hasEscape = true
		}
		if c == PathClassPrivEsc {
			hasPriv = true
		}
	}
	require.True(t, hasEscape, "expected ESCAPE_PATH, got %v", cls)
	require.True(t, hasPriv, "expected PRIV_ESC path, got %v", cls)
}

func TestFixture_GraphPathChain_EscapeToPrivEsc_chain(t *testing.T) {
	paths := AttackPathsFromEscapeClusterAdminFixture("fixture-pod-1")
	require.GreaterOrEqual(t, len(paths), 2)

	chains := DetectChainsWithHints(paths, DefaultHardeningHints())
	require.NotEmpty(t, chains, "DetectChainsWithHints should emit at least one chain")

	var escPriv *AttackChain
	for i := range chains {
		if chains[i].Type == "ESCAPE_TO_PRIV_ESC" {
			escPriv = &chains[i]
			break
		}
	}
	require.NotNil(t, escPriv, "expected ESCAPE_TO_PRIV_ESC among %v", chainTypes(chains))
	require.Contains(t, techniqueIDsFromSteps(escPriv.Steps), "ESCAPE_HOSTPATH")
	require.Contains(t, techniqueIDsFromSteps(escPriv.Steps), "KUBELET_TOKEN_HARVEST")
	require.Contains(t, techniqueIDsFromSteps(escPriv.Steps), "RBAC_PRIV_ESC")

	m := distinctPathMitreIDs(escPriv.Steps)
	require.Contains(t, m, "T1098.006")
	require.Contains(t, m, "T1611")
}

func chainTypes(chains []AttackChain) []string {
	var out []string
	for _, c := range chains {
		out = append(out, c.Type)
	}
	return out
}
