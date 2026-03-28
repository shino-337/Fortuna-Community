package rep

import (
	"context"
	"testing"

	"github.com/fortuna/core/pkg/models"
)

func TestClassifySignal_NetworkQueueSpike(t *testing.T) {
	signal, mitre, score := classifySignal("connect", "key=k dst=1.2.3.4:443 proto=tcp q=12000 avg=1000 ratio=12.00 samples=9", "NETWORK_TXRX_QUEUE_SPIKE", "", nil, context.Background(), "")
	if signal != "NETWORK_QUEUE_ANOMALY" {
		t.Fatalf("expected NETWORK_QUEUE_ANOMALY, got %s", signal)
	}
	if mitre != "T1046" {
		t.Fatalf("expected mitre T1046, got %s", mitre)
	}
	if score != 65 {
		t.Fatalf("expected dynamic score 65, got %d", score)
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

func TestClassifySignal_EBPFExecTrace(t *testing.T) {
	signal, mitre, score := classifySignal("execve", "/bin/sh", "EBPF_EXEC_TRACE", "", nil, context.Background(), "")
	if signal != "EBPF_EXEC_ACTIVITY" {
		t.Fatalf("expected EBPF_EXEC_ACTIVITY, got %s", signal)
	}
	if mitre != "T1059" {
		t.Fatalf("expected mitre T1059, got %s", mitre)
	}
	if score != 40 {
		t.Fatalf("expected score 40, got %d", score)
	}
}

func TestClassifyEventToSignal_EBPFConnectTrace(t *testing.T) {
	ev := &models.RuntimeEvent{
		Syscall:    "connect",
		Capability: "EBPF_CONNECT_TRACE",
		TargetPath: "8.8.8.8:53",
	}
	signal, category, confidence := classifyEventToSignal(ev)
	if signal != "EBPF_CONNECT_ACTIVITY" {
		t.Fatalf("expected EBPF_CONNECT_ACTIVITY, got %s", signal)
	}
	if category != "NETWORK" {
		t.Fatalf("expected NETWORK category, got %s", category)
	}
	if confidence < 0.6 {
		t.Fatalf("expected confidence >= 0.6, got %.2f", confidence)
	}
}

func TestClassifyEventToSignal_FalcoExecveSuspicious(t *testing.T) {
	ev := &models.RuntimeEvent{
		Syscall:    "execve",
		Runtime:    "falco",
		TargetPath: "/bin/sh -c id",
	}
	signal, category, confidence := classifyEventToSignal(ev)
	if signal != "SUSPICIOUS_EXEC_FROM_SNAPSHOT" {
		t.Fatalf("expected SUSPICIOUS_EXEC_FROM_SNAPSHOT, got %s", signal)
	}
	if category != "EXECUTION" {
		t.Fatalf("expected EXECUTION category, got %s", category)
	}
	if confidence < 0.69 {
		t.Fatalf("expected confidence about 0.7, got %.2f", confidence)
	}
}

func TestClassifyEventToSignal_FalcoConnectNetwork(t *testing.T) {
	ev := &models.RuntimeEvent{
		Syscall:    "connect",
		Runtime:    "falco",
		TargetPath: "dst=8.8.8.8:53 proto=udp",
	}
	signal, category, confidence := classifyEventToSignal(ev)
	if signal != "NETWORK_QUEUE_ANOMALY" {
		t.Fatalf("expected NETWORK_QUEUE_ANOMALY, got %s", signal)
	}
	if category != "NETWORK" {
		t.Fatalf("expected NETWORK category, got %s", category)
	}
	if confidence < 0.64 {
		t.Fatalf("expected confidence about 0.65, got %.2f", confidence)
	}
}

func TestClassifySignal_FalcoExecveSuspicious(t *testing.T) {
	// Falco may not provide PROCESS_SNAPSHOT_DIFF capability; rely on syscall+target heuristics.
	signal, mitre, score := classifySignal("execve", "/bin/sh -c id", "", "falco", nil, context.Background(), "")
	if signal != "SUSPICIOUS_EXEC_FROM_SNAPSHOT" {
		t.Fatalf("expected SUSPICIOUS_EXEC_FROM_SNAPSHOT, got %s", signal)
	}
	if mitre != "T1059" {
		t.Fatalf("expected mitre T1059, got %s", mitre)
	}
	if score != 45 {
		t.Fatalf("expected score 45, got %d", score)
	}
}

func TestClassifySignal_FalcoConnectNetwork(t *testing.T) {
	// Falco may provide network-ish evidence (ip:port, proto=..., dst=...); classify as network queue anomaly.
	signal, mitre, score := classifySignal("connect", "dst=8.8.8.8:53 proto=udp ratio=12.00", "", "falco", nil, context.Background(), "")
	if signal != "NETWORK_QUEUE_ANOMALY" {
		t.Fatalf("expected NETWORK_QUEUE_ANOMALY, got %s", signal)
	}
	if mitre != "T1046" {
		t.Fatalf("expected mitre T1046, got %s", mitre)
	}
	if score != 65 {
		t.Fatalf("expected score 65 (ratio>=12), got %d", score)
	}
}
