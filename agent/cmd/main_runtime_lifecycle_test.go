package main

import (
	"testing"

	"github.com/fortuna/agent/internal/config"
	"github.com/fortuna/api/collection"
)

func TestRuntimeProducerDeclarationsAreCompleteAndFailClosed(t *testing.T) {
	cfg := &config.Config{
		RuntimeEventsEnabled: true,
		FalcoEventsEnabled: false,
		EBPFEnabled: true,
		EBPFMode: "exec",
	}
	got := runtimeProducerDeclarations(cfg)
	if len(got) != collection.RuntimeProducerRegistrySize() {
		t.Fatalf("producer declarations=%d want %d", len(got), collection.RuntimeProducerRegistrySize())
	}
	seen := map[string]collection.RuntimeProducerDeclaration{}
	for _, p := range got {
		seen[p.ProducerID] = p
	}
	file := seen["runtime-file"]
	if !file.Enabled || file.Authoritative {
		t.Fatalf("enabled file producer must remain non-authoritative without upstream health proof: %+v", file)
	}
	falco := seen["falco"]
	if falco.Enabled || falco.Authoritative {
		t.Fatalf("disabled Falco producer trusted: %+v", falco)
	}
	ebpf := seen["ebpf-exec"]
	if !ebpf.Enabled || ebpf.Authoritative {
		t.Fatalf("built-in eBPF must be enabled but non-authoritative: %+v", ebpf)
	}
	for _, id := range []string{"ebpf-connect", "ebpf-all"} {
		if seen[id].Enabled || seen[id].Authoritative {
			t.Fatalf("unselected eBPF producer %s was enabled: %+v", id, seen[id])
		}
	}
}


func TestRuntimeProducerDeclarationsNeverInferAuthorityFromEnablement(t *testing.T) {
	cfg := &config.Config{
		RuntimeEventsEnabled: true,
		FalcoEventsEnabled: true,
		EBPFEnabled: true,
		EBPFMode: "all",
	}
	for _, p := range runtimeProducerDeclarations(cfg) {
		if p.Authoritative {
			t.Fatalf("current runtime producer %s inferred authority from enablement: %+v", p.ProducerID, p)
		}
	}
}
