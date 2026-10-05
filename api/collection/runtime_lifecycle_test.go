package collection

import (
	"testing"
	"time"
)

func validRuntimeManifest(now time.Time) RuntimeProducerManifest {
	producers := []RuntimeProducerDeclaration{
		{ProducerID: "runtime-file", SourceKind: RuntimeSourceFile, Enabled: true, Authoritative: false},
		{ProducerID: "falco", SourceKind: RuntimeSourceFalco},
		{ProducerID: "ebpf-exec", SourceKind: RuntimeSourceEBPF, Enabled: true},
		{ProducerID: "ebpf-connect", SourceKind: RuntimeSourceEBPF},
		{ProducerID: "ebpf-all", SourceKind: RuntimeSourceEBPF},
	}
	SortRuntimeProducerDeclarations(producers)
	return RuntimeProducerManifest{
		Version:          RuntimeProducerManifestVersion,
		SessionID:        "session-contract-000001",
		SessionStartedAt: now.Add(-time.Second),
		ReportedAt:       now,
		AgentState:       RuntimeAgentRunning,
		Producers:        producers,
	}
}

func TestRuntimeProducerManifestRequiresCompleteFailClosedRegistry(t *testing.T) {
	now := time.Now().UTC()
	valid := validRuntimeManifest(now)
	if err := valid.Validate(now); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}

	missing := valid
	missing.Producers = append([]RuntimeProducerDeclaration(nil), valid.Producers[:len(valid.Producers)-1]...)
	if err := missing.Validate(now); err == nil {
		t.Fatal("missing producer declaration was accepted")
	}

	duplicate := valid
	duplicate.Producers = append([]RuntimeProducerDeclaration(nil), valid.Producers...)
	duplicate.Producers[1] = duplicate.Producers[0]
	if err := duplicate.Validate(now); err == nil {
		t.Fatal("duplicate producer declaration was accepted")
	}

	selfAssertedAuthority := valid
	selfAssertedAuthority.Producers = append([]RuntimeProducerDeclaration(nil), valid.Producers...)
	for i := range selfAssertedAuthority.Producers {
		if selfAssertedAuthority.Producers[i].ProducerID == "runtime-file" {
			selfAssertedAuthority.Producers[i].Authoritative = true
		}
	}
	if err := selfAssertedAuthority.Validate(now); err == nil {
		t.Fatal("manifest v1 accepted self-asserted runtime authority")
	}

	stopping := valid
	stopping.AgentState = RuntimeAgentStopping
	if err := stopping.Validate(now); err == nil {
		t.Fatal("stopping manifest with enabled producer was accepted")
	}
}

func TestRuntimeCoverageRequiresExecutionSession(t *testing.T) {
	now := time.Now().UTC()
	c := RuntimeCoverage{
		Version:     RuntimeCoverageVersion,
		ID:          "coverage-contract-00001",
		ProducerID:  "falco",
		SourceKind:  RuntimeSourceFalco,
		Status:      "complete",
		WindowStart: now.Add(-time.Second),
		WindowEnd:   now,
	}
	if err := c.Validate(now); err == nil {
		t.Fatal("coverage without session was accepted")
	}
	c.SessionID = "session-contract-000001"
	if err := c.Validate(now); err != nil {
		t.Fatalf("session-bound coverage rejected: %v", err)
	}
}
