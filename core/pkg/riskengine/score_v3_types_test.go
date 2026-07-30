package riskengine

import "testing"

func TestMergeDimensionScores(t *testing.T) {
	total := MergeDimensionScores(map[ScoreDimension]float64{
		DimExposure:       10,
		DimRuntimeThreat:  5,
		DimSoftwareRisk:   0,
		DimCapabilityRisk: -1, // ignored negative for MVP sum
	})
	if total != 15 {
		t.Fatalf("got %v", total)
	}
}
