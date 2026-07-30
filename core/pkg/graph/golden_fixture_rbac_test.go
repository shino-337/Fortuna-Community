package graph

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFixture_RbacOnlyGraph_NoEscapeHop(t *testing.T) {
	paths := AttackPathsFromRbacOnlyClusterAdminFixture("fixture-rbac-pod-1")
	require.NotEmpty(t, paths)
	var hasPriv bool
	for _, p := range paths {
		if p.Explainability != nil && p.Explainability.Class == PathClassPrivEsc {
			hasPriv = true
		}
		if p.Explainability != nil {
			require.NotEqual(t, PathClassEscape, p.Explainability.Class)
		}
	}
	require.True(t, hasPriv, "expected at least one PRIV_ESC_PATH toward cluster-admin")
	for _, p := range paths {
		for _, n := range p.Nodes {
			require.NotEqual(t, NodeTypeNode, n.Type, "RBAC-only golden must not introduce node lateral hop")
		}
	}
}
