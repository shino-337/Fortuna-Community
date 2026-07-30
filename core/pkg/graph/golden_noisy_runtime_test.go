package graph

import "testing"

// “Killer” regression: many irrelevant runtime events (e.g. Discovery) must not look like
// good chain correlation; path MITRE set must not include noise techniques.
func TestNoisyRuntime_RBACPathExcludesScanMitre(t *testing.T) {
	steps := []AttackStep{attackStepFromRegistry(t, "RBAC_PRIV_ESC")}
	for _, id := range distinctPathMitreIDs(steps) {
		if id == "T1046" {
			t.Fatalf("RBAC chain must not list T1046 in path MITRE; got %v", id)
		}
	}
}

func TestNoisyRuntime_SpamUnrelatedMitreDrivesDownChainPrecision(t *testing.T) {
	steps := []AttackStep{attackStepFromRegistry(t, "RBAC_PRIV_ESC")}
	var evs []runtimeEventLite
	for i := 0; i < 50; i++ {
		evs = append(evs, runtimeEventLite{Mitre: "T1046"})
	}
	// one event that could match RBAC path (T1098.006) so we are not at 0
	evs = append(evs, runtimeEventLite{Mitre: "T1098.006"})

	matched := countEventsMatchingSteps(steps, evs)
	cp := float64(matched) / float64(mitreMaxDen(1, len(evs)))
	if cp >= 0.6 {
		t.Fatalf("expected chain precision < 0.6 with mostly noise, got %v (matched=%d n=%d)", cp, matched, len(evs))
	}
}
