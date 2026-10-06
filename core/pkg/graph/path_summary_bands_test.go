package graph

import (
	"context"
	"testing"
)

// The summary counts must use the same bands as each path's label, the page and the alerts.
func TestSummarizePathsUsesPathLabelBands(t *testing.T) {
	b := &RelationalPathBuilder{}
	var paths []AttackPath
	for _, r := range []float64{9.4, 8.2, 7.0, 6.9, 4.0, 3.2, 1.8} {
		paths = append(paths, AttackPath{TotalRisk: r})
	}
	s, err := b.summarizePaths(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	if s.CriticalPaths != 1 || s.HighPaths != 2 || s.MediumPaths != 2 || s.LowPaths != 2 {
		t.Fatalf("bands = critical %d high %d medium %d low %d, want 1/2/2/2", s.CriticalPaths, s.HighPaths, s.MediumPaths, s.LowPaths)
	}
}
