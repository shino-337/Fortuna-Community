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
	if err := r.Stop(); err != nil {
		t.Fatal(err)
	}
	// A repeated Stop and a late ticker-style running report cannot reopen or
	// move the acknowledged stopping lifecycle.
	
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


func TestProducerLifecycleStopSerializesAfterInflightHeartbeat(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	states := make(chan string, 2)
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var m collection.RuntimeProducerManifest
		if err := json.NewDecoder(req.Body).Decode(&m); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		calls++
		if calls == 1 {
			close(entered)
			<-release
		}
		states <- m.AgentState
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	producers := []collection.RuntimeProducerDeclaration{
		{ProducerID: "runtime-file", SourceKind: collection.RuntimeSourceFile, Enabled: true, Authoritative: true},
		{ProducerID: "falco", SourceKind: collection.RuntimeSourceFalco},
		{ProducerID: "ebpf-exec", SourceKind: collection.RuntimeSourceEBPF},
		{ProducerID: "ebpf-connect", SourceKind: collection.RuntimeSourceEBPF},
		{ProducerID: "ebpf-all", SourceKind: collection.RuntimeSourceEBPF},
	}
	r := NewProducerLifecycleReporter(srv.URL, "session-ordering-000001", time.Now().UTC().Add(-time.Second), producers)

	runDone := make(chan error, 1)
	go func() { runDone <- r.Report(collection.RuntimeAgentRunning) }()
	<-entered

	stopDone := make(chan error, 1)
	go func() { stopDone <- r.Stop() }()

	select {
	case state := <-states:
		t.Fatalf("lifecycle request completed before releasing in-flight running report: %s", state)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	if err := <-runDone; err != nil {
		t.Fatal(err)
	}
	if err := <-stopDone; err != nil {
		t.Fatal(err)
	}
	first := <-states
	second := <-states
	if first != collection.RuntimeAgentRunning || second != collection.RuntimeAgentStopping {
		t.Fatalf("lifecycle order=%q,%q want running,stopping", first, second)
	}
}
