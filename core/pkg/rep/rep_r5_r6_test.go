package rep

import (
	"context"
	"testing"

	"github.com/fortuna/core/pkg/models"
)

func TestClassifySignal_NetworkQueueSpike(t *testing.T) {
	signal, mitre, score := classifySignal("connect", "1.2.3.4:443", "NETWORK_TXRX_QUEUE_SPIKE", nil, context.Background(), "")
	if signal != "NETWORK_QUEUE_ANOMALY" {
		t.Fatalf("expected NETWORK_QUEUE_ANOMALY, got %s", signal)
	}
	if mitre != "T1046" {
		t.Fatalf("expected mitre T1046, got %s", mitre)
	}
	if score != 35 {
		t.Fatalf("expected score 35, got %d", score)
	}
}

func TestClassifyEventToSignal_NetworkQueueSpike(t *testing.T) {
	ev := &models.RuntimeEvent{
		Syscall:    "connect",
		Capability: "NETWORK_TXRX_QUEUE_SPIKE",
		TargetPath: "8.8.8.8:53 proto=udp q=12000 avg=1000",
	}
	signal, category, confidence := classifyEventToSignal(ev)
	if signal != "NETWORK_QUEUE_ANOMALY" {
		t.Fatalf("expected NETWORK_QUEUE_ANOMALY, got %s", signal)
	}
	if category != "NETWORK" {
		t.Fatalf("expected NETWORK category, got %s", category)
	}
	if confidence < 0.6 {
		t.Fatalf("expected confidence >= 0.6, got %.2f", confidence)
	}
}
