package worker

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCoverageStats_RawMatchRatio_Invariant(t *testing.T) {
	components := []CoverageComponent{
		{ID: "c1"},
		{ID: "c2"},
		{ID: "c3"},
	}

	matches := map[string][]CoverageMatch{
		"c1": {{ID: "m1"}},
		"c2": {},
		// c3 missing => treated as unmatched
	}

	stats := ComputeSBOMCoverageStats(components, matches)
	require.Equal(t, 3, stats.TotalComponents)
	require.Equal(t, 1, stats.MatchedComponents)
	require.Equal(t, 2, stats.NotMatchedComponents)

	ratio := stats.RawMatchRatio()
	expected := float64(stats.MatchedComponents) / float64(stats.TotalComponents)
	require.True(t, math.Abs(ratio-expected) <= 1e-9)
}

func TestCoverageStats_EdgeCases(t *testing.T) {
	t.Run("empty SBOM", func(t *testing.T) {
		stats := ComputeSBOMCoverageStats([]CoverageComponent{}, map[string][]CoverageMatch{})
		require.Equal(t, 0, stats.TotalComponents)
		require.Equal(t, 0, stats.MatchedComponents)
		require.Equal(t, 0, stats.NotMatchedComponents)
		require.Equal(t, 0.0, stats.RawMatchRatio())
	})

	t.Run("all matched", func(t *testing.T) {
		components := []CoverageComponent{{ID: "c1"}, {ID: "c2"}}
		matches := map[string][]CoverageMatch{
			"c1": {{ID: "m1"}},
			"c2": {{ID: "m2"}},
		}
		stats := ComputeSBOMCoverageStats(components, matches)
		require.Equal(t, 2, stats.TotalComponents)
		require.Equal(t, 2, stats.MatchedComponents)
		require.Equal(t, 0, stats.NotMatchedComponents)
		require.Equal(t, 1.0, stats.RawMatchRatio())
	})

	t.Run("all unmatched", func(t *testing.T) {
		components := []CoverageComponent{{ID: "c1"}, {ID: "c2"}}
		matches := map[string][]CoverageMatch{
			"c1": {},
			"c2": {},
		}
		stats := ComputeSBOMCoverageStats(components, matches)
		require.Equal(t, 2, stats.TotalComponents)
		require.Equal(t, 0, stats.MatchedComponents)
		require.Equal(t, 2, stats.NotMatchedComponents)
		require.Equal(t, 0.0, stats.RawMatchRatio())
	})

	t.Run("duplicate matches count per-component only", func(t *testing.T) {
		components := []CoverageComponent{{ID: "c1"}}
		matches := map[string][]CoverageMatch{
			"c1": {{ID: "m1"}, {ID: "m2"}, {ID: "m3"}},
		}
		stats := ComputeSBOMCoverageStats(components, matches)
		require.Equal(t, 1, stats.TotalComponents)
		require.Equal(t, 1, stats.MatchedComponents)
		require.Equal(t, 0, stats.NotMatchedComponents)
		require.Equal(t, 1.0, stats.RawMatchRatio())
	})
}

