package graph

import "testing"

func TestBuildMitreCoverage_unknownWhenNotListed(t *testing.T) {
	rt := map[string]bool{"T1611": true}
	path := []string{"T1611", "T999999"}
	cov := BuildMitreCoverage(path, rt, 1.0, 1.0)
	if len(cov) != 2 {
		t.Fatalf("want 2 rows, got %d", len(cov))
	}
	st := map[string]string{}
	for _, c := range cov {
		st[c.MitreID] = c.Status
	}
	if st["T1611"] != "observed" {
		t.Errorf("T1611: want observed got %q", st["T1611"])
	}
	if st["T999999"] != "unknown" {
		t.Errorf("unknown T-id: want unknown got %q", st["T999999"])
	}
	if cov[1].Confidence != nil {
		t.Errorf("unknown should omit confidence, got %#v", cov[1].Confidence)
	}
}

func TestMitreHasDetectionRule_known(t *testing.T) {
	if !MitreHasDetectionRule("T1611") {
		t.Fatal("expected T1611 in detection allowlist")
	}
}

func TestBuildMitreCoverage_inferredWhenNoRuntimeButListed(t *testing.T) {
	rt := map[string]bool{}
	path := []string{"T1611"}
	cov := BuildMitreCoverage(path, rt, 0.85, 0.8)
	if len(cov) != 1 || cov[0].Status != "inferred" {
		t.Fatalf("want inferred, got %+v", cov)
	}
	if cov[0].Confidence == nil || *cov[0].Confidence < 0.5 {
		t.Fatalf("want inference confidence set, got %#v", cov[0].Confidence)
	}
	if cov[0].Priority != "MEDIUM" {
		t.Fatalf("want MEDIUM priority for inferred, got %q", cov[0].Priority)
	}
}

func TestMitreCoveragePriority_rules(t *testing.T) {
	high := 0.85
	if mitreCoveragePriority("not_covered", &high) != "HIGH" {
		t.Errorf("not_covered+conf>0.7 => HIGH")
	}
	low := 0.5
	if mitreCoveragePriority("not_covered", &low) != "LOW" {
		t.Errorf("not_covered+low conf => LOW")
	}
	if mitreCoveragePriority("inferred", &low) != "MEDIUM" {
		t.Errorf("inferred => MEDIUM")
	}
}
