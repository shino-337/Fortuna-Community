package risk

import "testing"

func TestDeriveFinalLevelFromScore_Bands(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{0, "low"},
		{19.9, "low"},
		{20, "medium"},
		{39, "medium"},
		{40, "high"},
		{69, "high"},
		{70, "critical"},
		{100, "critical"},
	}
	for _, tc := range cases {
		if got := DeriveFinalLevelFromScore(tc.score); got != tc.want {
			t.Errorf("score %v: want %q got %q", tc.score, tc.want, got)
		}
	}
}

func TestParseBreakdownFromFactorsJSON(t *testing.T) {
	raw := `{"aggregation":{"risk_factors":[{"factor_id":"f1","category":"c","scope":"pod","source":"s","contribution":1.25,"evidence_refs":["e1"]}]}}`
	got := ParseBreakdownFromFactorsJSON(raw)
	if len(got) != 1 {
		t.Fatalf("want 1 row, got %d", len(got))
	}
	if got[0].FactorID != "f1" || got[0].Contribution != 1.25 || len(got[0].EvidenceRefs) != 1 || got[0].EvidenceRefs[0] != "e1" {
		t.Fatalf("unexpected row: %+v", got[0])
	}
	if ParseBreakdownFromFactorsJSON("") != nil {
		t.Fatal("empty input should be nil")
	}
}

func TestParseDimensionScoresV3FromFactorsJSON(t *testing.T) {
	raw := `{"dimensions":{"vulnerability":1.5,"capability_exposure":2,"attack_path":3,"rbac_policy":4,"runtime_threat":5,"exposure":6,"blast_radius":7}}`
	got := ParseDimensionScoresV3FromFactorsJSON(raw)
	if got == nil {
		t.Fatal("expected dimensions")
	}
	if got.Vulnerability != 1.5 || got.CapabilityExposure != 2 || got.AttackPath != 3 || got.RbacPolicy != 4 ||
		got.RuntimeThreat != 5 || got.Exposure != 6 || got.BlastRadius != 7 {
		t.Fatalf("unexpected dims: %+v", got)
	}
	if ParseDimensionScoresV3FromFactorsJSON("") != nil {
		t.Fatal("empty input should be nil")
	}
	if ParseDimensionScoresV3FromFactorsJSON(`{}`) != nil {
		t.Fatal("no dimensions key should be nil")
	}
}
