package rep

import "testing"

func TestAllREP_CDetectors_UniqueIncidentTypes(t *testing.T) {
	seen := map[string]string{}
	for _, d := range AllREP_CDetectors() {
		if d.ID == "" || d.IncidentType == "" {
			t.Fatalf("incomplete detector: %+v", d)
		}
		if prev, ok := seen[d.IncidentType]; ok {
			t.Fatalf("duplicate incident type %q: %s and %s", d.IncidentType, prev, d.ID)
		}
		seen[d.IncidentType] = d.ID
		if d.Confidence <= 0 || d.Confidence > 1 {
			t.Fatalf("confidence out of (0,1]: %s %v", d.ID, d.Confidence)
		}
		if d.Cooldown <= 0 || d.BucketWindow <= 0 {
			t.Fatalf("cooldown/bucket must be positive: %s", d.ID)
		}
	}
}

func TestDetectorReconBurst_CountThresholdMatchesCorrelator(t *testing.T) {
	if DetectorReconBurst.MinDistinctFacts != 6 {
		t.Fatalf("documented recon threshold is 6 distinct facts")
	}
}
