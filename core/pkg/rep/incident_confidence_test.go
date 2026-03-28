package rep

import "testing"

func TestMergeIncidentConfidenceOnSuppress_Reinforce(t *testing.T) {
	got := mergeIncidentConfidenceOnSuppress(0.7, 0.7, 2, 5)
	if got < 0.73 || got > 1.01 {
		t.Fatalf("expected bump on more evidence, got %v", got)
	}
}

func TestMergeIncidentConfidenceOnSuppress_DecayRepeat(t *testing.T) {
	got := mergeIncidentConfidenceOnSuppress(0.8, 0.85, 3, 3)
	if got >= 0.8 {
		t.Fatalf("expected slight decay on same evidence count, got %v", got)
	}
	if got < 0.75 {
		t.Fatalf("decay too aggressive: %v", got)
	}
}

func TestEvidenceRefCountJSON(t *testing.T) {
	if n := evidenceRefCountJSON(`["a","b"]`); n != 2 {
		t.Fatalf("got %d", n)
	}
	if n := evidenceRefCountJSON(`{}`); n != 0 {
		t.Fatalf("got %d", n)
	}
}
