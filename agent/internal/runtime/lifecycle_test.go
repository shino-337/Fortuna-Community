package runtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fortuna/api/collection"
)

func TestProducerLifecycleReporterRunningAndStopping(t *testing.T) {
	var got []collection.RuntimeProducerManifest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/runtime/producers" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var m collection.RuntimeProducerManifest
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			t.Fatal(err)
		}
		got = append(got, m)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	producers := []collection.RuntimeProducerDeclaration{
		{ProducerID: "runtime-file", SourceKind: collection.RuntimeSourceFile, Enabled: true, Authoritative: true},
		{ProducerID: "falco", SourceKind: collection.RuntimeSourceFalco},
		{ProducerID: "ebpf-exec", SourceKind: collection.RuntimeSourceEBPF, Enabled: true},
		{ProducerID: "ebpf-connect", SourceKind: collection.RuntimeSourceEBPF},
		{ProducerID: "ebpf-all", SourceKind: collection.RuntimeSourceEBPF},
	}
	session := "session-agent-00000001"
	started := time.Now().UTC().Add(-time.Second)
	r := NewProducerLifecycleReporter(srv.URL, session, started, producers)

	if err := r.Report(collection.RuntimeAgentRunning); err != nil {
		t.Fatal(err)
	}
	if err := r.Stop(); err != nil {
		t.Fatal(err)
	}
	// A late ticker-style running report cannot reopen a stopped lifecycle.
	if err := r.Report(collection.RuntimeAgentRunning); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("manifests=%d want 2", len(got))
	}
	if got[0].SessionID != session || got[0].AgentState != collection.RuntimeAgentRunning {
		t.Fatalf("unexpected running manifest: %+v", got[0])
	}
	if err := got[0].Validate(time.Now().UTC()); err != nil {
		t.Fatalf("running manifest invalid: %v", err)
	}
	if got[1].SessionID != session || got[1].AgentState != collection.RuntimeAgentStopping {
		t.Fatalf("unexpected stopping manifest: %+v", got[1])
	}
	for _, p := range got[1].Producers {
		if p.Enabled || p.Authoritative {
			t.Fatalf("stopping manifest left producer enabled: %+v", p)
		}
	}
	if err := got[1].Validate(time.Now().UTC()); err != nil {
		t.Fatalf("stopping manifest invalid: %v", err)
	}
}
