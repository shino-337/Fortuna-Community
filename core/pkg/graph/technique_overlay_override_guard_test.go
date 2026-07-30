package graph

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPhase42_YAMLOverlayUnknownKeysIgnoredAtUnmarshal(t *testing.T) {
	raw := `
ESCAPE_HOSTPATH:
  risk_weight: 2.0
  provides: ["CLUSTER_ADMIN"]
`
	var m map[string]techniqueOverlayRow
	require.NoError(t, yaml.Unmarshal([]byte(raw), &m))
	row := m["ESCAPE_HOSTPATH"]
	require.InDelta(t, 2.0, row.RiskWeight, 1e-9)
	require.Empty(t, row.MitreTechniques)
}

// TestPhase42_YAMLOverrides_AllowedFieldsOnly — overlay may change risk/MITRE/context only; Go registry Requires/Provides stay authoritative.
func TestPhase42_YAMLOverrides_AllowedFieldsOnly(t *testing.T) {
	t.Cleanup(func() {
		require.NoError(t, ReloadTechniqueOverlay(TechniqueOverlayEmbeddedYAML()))
	})

	base, ok := TechniqueByID("ESCAPE_HOSTPATH")
	require.True(t, ok)

	raw := TechniqueOverlayEmbeddedYAML()
	var f techniqueOverlayFile
	require.NoError(t, yaml.Unmarshal(raw, &f))

	row := f.Overlays["ESCAPE_HOSTPATH"]
	row.RiskWeight = 2.0
	f.Overlays["ESCAPE_HOSTPATH"] = row

	patched, err := yaml.Marshal(&f)
	require.NoError(t, err)

	require.NoError(t, ReloadTechniqueOverlay(patched))

	after, ok := TechniqueByID("ESCAPE_HOSTPATH")
	require.True(t, ok)

	require.InDelta(t, 2.0, after.RiskWeight, 1e-9)
	require.ElementsMatch(t, base.Provides, after.Provides)
	require.ElementsMatch(t, base.Requires, after.Requires)
}
