package risk

import (
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
)

func TestResolveSignalCapability_unknownFallsBackToProbe(t *testing.T) {
	b := ResolveSignalCapability("TOTALLY_UNKNOWN_SIGNAL_XYZ")
	if b.CapabilityID != "ESC_RUNTIME_PROBE" {
		t.Fatalf("got %q", b.CapabilityID)
	}
}

func TestComputeRuntimeTemporalCoherence_disorderWhenAPIBeforeShell(t *testing.T) {
	// Keep |shell−api| inside the pairing window so disorder logic runs (not early window exit).
	shellTime := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	apiTime := time.Date(2026, 4, 23, 11, 50, 0, 0, time.UTC)
	signals := []models.RuntimeSignal{
		{SignalType: "INTERACTIVE_SHELL_EXEC", Confidence: 0.8, CreatedAt: shellTime},
		{SignalType: "SERVICEACCOUNT_TOKEN_READ", Confidence: 0.85, CreatedAt: apiTime},
	}
	meta := ComputeRuntimeTemporalCoherence(nil, signals)
	if meta.DisorderPenalty <= 0 {
		t.Fatalf("expected disorder penalty, got %+v", meta)
	}
	if meta.CoherenceMultiplier >= 1.0 {
		t.Fatalf("expected coherence < 1 when API precedes shell, got %v", meta.CoherenceMultiplier)
	}
}

func TestComputeRuntimeTemporalCoherence_sequenceBonusShellFirst(t *testing.T) {
	t1 := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 4, 23, 10, 10, 0, 0, time.UTC)
	signals := []models.RuntimeSignal{
		{SignalType: "INTERACTIVE_SHELL_EXEC", Confidence: 0.8, CreatedAt: t1},
		{SignalType: "SERVICEACCOUNT_TOKEN_READ", Confidence: 0.85, CreatedAt: t2},
	}
	meta := ComputeRuntimeTemporalCoherence(nil, signals)
	if meta.SequenceBonus <= 0 {
		t.Fatalf("expected sequence bonus, got %+v", meta)
	}
}

func TestComputeRuntimeTemporalCoherence_pairingGapBeyondWindowDecouples(t *testing.T) {
	shellTime := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)
	apiTime := time.Date(2026, 4, 23, 10, 45, 0, 0, time.UTC)
	signals := []models.RuntimeSignal{
		{SignalType: "INTERACTIVE_SHELL_EXEC", Confidence: 0.8, CreatedAt: shellTime},
		{SignalType: "SERVICEACCOUNT_TOKEN_READ", Confidence: 0.85, CreatedAt: apiTime},
	}
	meta := ComputeRuntimeTemporalCoherence(nil, signals)
	if !meta.PairingWindowExceeded {
		t.Fatalf("expected pairing window exceeded for 45m gap, got %+v", meta)
	}
	if meta.CoherenceMultiplier >= 0.99 {
		t.Fatalf("expected coherence damped when pairing window exceeded, got %v", meta.CoherenceMultiplier)
	}
}
