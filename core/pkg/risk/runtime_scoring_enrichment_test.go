package risk

import (
	"encoding/json"
	"testing"

	"github.com/fortuna/core/pkg/models"
)

func TestMergeRuntimeDerivedPodCapabilities_toxicComboUpgradesToken(t *testing.T) {
	signals := []models.RuntimeSignal{
		{SignalType: "SUSPICIOUS_EXEC_FROM_SNAPSHOT", Confidence: 0.82},
		{SignalType: "NETWORK_QUEUE_ANOMALY", Confidence: 0.72},
	}
	events := []models.RuntimeEvent{
		{SourceRule: "Contact K8s API Server From Container", Confidence: 0.9},
	}
	out, inj := mergeRuntimeDerivedPodCapabilities("pod-1", "ns", nil, signals, events)
	if len(out) < 2 {
		t.Fatalf("expected at least 2 derived caps, got %d", len(out))
	}
	var token *models.PodCapability
	for i := range out {
		if out[i].CapabilityID == runtimeDerivedIDTokenPod {
			token = &out[i]
			break
		}
	}
	if token == nil {
		t.Fatal("missing ID_TOKEN_POD injection")
	}
	if token.State != "exploited" {
		t.Fatalf("toxic combo with K8s corroboration should mark token exploited, got %q", token.State)
	}
	found := false
	for _, id := range inj {
		if id == runtimeDerivedIDTokenPod {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("injected list should mention %s: %v", runtimeDerivedIDTokenPod, inj)
	}
	var prov map[string]interface{}
	if err := json.Unmarshal([]byte(token.DerivedFrom), &prov); err != nil {
		t.Fatalf("derived_from JSON: %v", err)
	}
	if prov["source"] != "runtime" {
		t.Fatalf("expected source=runtime, got %v", prov["source"])
	}
}

func TestMergeRuntimeDerivedPodCapabilities_networkWithoutK8sStaysConfirmed(t *testing.T) {
	signals := []models.RuntimeSignal{
		{SignalType: "SUSPICIOUS_EXEC_FROM_SNAPSHOT", Confidence: 0.82},
		{SignalType: "NETWORK_QUEUE_ANOMALY", Confidence: 0.72},
	}
	out, _ := mergeRuntimeDerivedPodCapabilities("pod-1", "ns", nil, signals, nil)
	var token *models.PodCapability
	for i := range out {
		if out[i].CapabilityID == runtimeDerivedIDTokenPod {
			token = &out[i]
			break
		}
	}
	if token == nil {
		t.Fatal("missing ID_TOKEN_POD injection")
	}
	if token.State != "confirmed" {
		t.Fatalf("without K8s API corroboration, token should stay confirmed, got %q", token.State)
	}
}

func TestComputeRuntimeThreatWithMeta_toxicComboBonus(t *testing.T) {
	signals := []models.RuntimeSignal{
		{SignalType: "INTERACTIVE_SHELL_EXEC", Confidence: 0.8},
		{SignalType: "NETWORK_QUEUE_ANOMALY", Confidence: 0.7},
	}
	events := []models.RuntimeEvent{
		{SourceRule: "Contact K8s API Server From Container", Confidence: 0.9},
	}
	score, meta := computeRuntimeThreatWithMeta(nil, 7, signals, 0, events, RuntimeTemporalMeta{CoherenceMultiplier: 1}, 1)
	if meta["runtime_toxic_combo_shell_api_bonus"].(float64) != 4.5 {
		t.Fatalf("toxic bonus: %+v", meta)
	}
	if score < 4.5 {
		t.Fatalf("expected score to include toxic bonus, got %.2f", score)
	}
}

func TestMergeRuntimeDerivedPodCapabilities_tokenExploitRequiresHigherConfidence(t *testing.T) {
	signals := []models.RuntimeSignal{
		{SignalType: "SUSPICIOUS_EXEC_FROM_SNAPSHOT", Confidence: 0.82},
		{SignalType: "NETWORK_QUEUE_ANOMALY", Confidence: 0.65},
	}
	events := []models.RuntimeEvent{
		{SourceRule: "Contact K8s API Server From Container", Confidence: 0.9},
	}
	out, _ := mergeRuntimeDerivedPodCapabilities("pod-1", "ns", nil, signals, events)
	var token *models.PodCapability
	for i := range out {
		if out[i].CapabilityID == runtimeDerivedIDTokenPod {
			token = &out[i]
			break
		}
	}
	if token == nil {
		t.Fatal("missing ID_TOKEN_POD injection")
	}
	if token.State != "confirmed" {
		t.Fatalf("ID_TOKEN_POD requires confidence >= 0.7 for exploited, got %q", token.State)
	}
}

func TestMergeRuntimeDerivedPodCapabilities_k8sToxicLowNetworkConfidenceStaysConfirmed(t *testing.T) {
	signals := []models.RuntimeSignal{
		{SignalType: "SUSPICIOUS_EXEC_FROM_SNAPSHOT", Confidence: 0.82},
		{SignalType: "NETWORK_QUEUE_ANOMALY", Confidence: 0.55},
	}
	events := []models.RuntimeEvent{
		{SourceRule: "Contact K8s API Server From Container", Confidence: 0.9},
	}
	out, _ := mergeRuntimeDerivedPodCapabilities("pod-1", "ns", nil, signals, events)
	var token *models.PodCapability
	for i := range out {
		if out[i].CapabilityID == runtimeDerivedIDTokenPod {
			token = &out[i]
			break
		}
	}
	if token == nil {
		t.Fatal("missing ID_TOKEN_POD injection")
	}
	if token.State != "confirmed" {
		t.Fatalf("with network confidence < 0.6, max state is confirmed, got %q", token.State)
	}
}

func TestComputeRuntimeThreatWithMeta_noK8sCorroborationNoToxic(t *testing.T) {
	signals := []models.RuntimeSignal{
		{SignalType: "INTERACTIVE_SHELL_EXEC", Confidence: 0.8},
		{SignalType: "NETWORK_QUEUE_ANOMALY", Confidence: 0.7},
	}
	_, meta := computeRuntimeThreatWithMeta(nil, 7, signals, 0, nil, RuntimeTemporalMeta{CoherenceMultiplier: 1}, 1)
	if meta["runtime_toxic_combo_shell_api_bonus"].(float64) != 0 {
		t.Fatalf("expected no toxic bonus without K8s corroboration, got %+v", meta)
	}
}
