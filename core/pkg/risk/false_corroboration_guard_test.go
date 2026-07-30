package risk

import (
	"testing"

	"github.com/fortuna/core/pkg/models"
)

// Production-oriented guard: ambiguous "kubernetes" context + non-API destination
// must not yield exploited token or a large toxic runtime bump.
func TestFalseCorroborationGuard_shellPlusNonK8sNetwork(t *testing.T) {
	signals := []models.RuntimeSignal{
		{SignalType: "INTERACTIVE_SHELL_EXEC", Confidence: 0.82, Count: 2},
		{SignalType: "NETWORK_QUEUE_ANOMALY", Confidence: 0.72, Count: 3, Evidence: `{"target":"8.8.8.8:443"}`},
	}
	events := []models.RuntimeEvent{
		{TargetPath: "external.cdn.example:443", Confidence: 0.9},
	}
	out, _ := mergeRuntimeDerivedPodCapabilities("p", "ns", nil, signals, events)
	var token *models.PodCapability
	for i := range out {
		if out[i].CapabilityID == runtimeDerivedIDTokenPod {
			token = &out[i]
			break
		}
	}
	if token == nil {
		t.Fatal("expected ID_TOKEN_POD row for NETWORK_QUEUE_ANOMALY")
	}
	if token.State == "exploited" {
		t.Fatalf("must not mark token exploited without K8s API corroboration, got %q", token.State)
	}

	score, meta := computeRuntimeThreatWithMeta(nil, 10, signals, 0, events, RuntimeTemporalMeta{CoherenceMultiplier: 1}, 1)
	if meta["runtime_toxic_combo_shell_api_bonus"].(float64) != 0 {
		t.Fatalf("toxic combo must be off, meta=%v", meta)
	}
	if score >= 8.0 {
		t.Fatalf("runtimeThreatScore should stay bounded without K8s toxic combo, got %.2f", score)
	}
}
