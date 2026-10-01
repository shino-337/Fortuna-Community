package runtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fortuna/agent/internal/corehttp"
	"github.com/fortuna/api/collection"
)

func appendFalcoTestRecords(t *testing.T, path string, uids ...string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, uid := range uids {
		line := fmt.Sprintf(`{"time":1234567890,"rule":"isolation","priority":"Warning","output_fields":{"k8s.pod.uid":%q,"k8s.ns.name":"ns","evt.type":"execve"}}`+"\n", uid)
		if _, err = f.WriteString(line); err != nil {
			t.Fatal(err)
		}
	}
}

func newDurableFalcoTestReader(t *testing.T, path, statePath, url, cluster string) *FalcoReader {
	t.Helper()
	r := NewFalcoReader(path, time.Second, url, "node", nil)
	r.SetDeliveryState(statePath, cluster, "node-agent")
	t.Cleanup(r.CloseDeliveryState)
	return r
}

func TestFalcoDurableMixedBatchRestartAndRecovery(t *testing.T) {
	acceptStale := false
	received := map[string]Event{}
	effects := map[string]int{}
	var coverage collection.RuntimeCoverage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/api/v2/runtime/coverage" {
			_ = json.NewDecoder(req.Body).Decode(&coverage)
			return
		}
		if req.URL.Path != "/api/v2/runtime/events" {
			t.Errorf("protocol downgrade: %s", req.URL.Path)
			w.WriteHeader(400)
			return
		}
		var events []Event
		_ = json.NewDecoder(req.Body).Decode(&events)
		for _, event := range events {
			if event.Pod["uid"] == "stale" && !acceptStale {
				w.WriteHeader(403)
				_, _ = w.Write([]byte(`{"code":"runtime_ownership_mismatch"}`))
				return
			}
		}
		for _, event := range events {
			if _, exists := received[event.SourceRecordID]; !exists {
				effects[event.Pod["uid"].(string)]++
			}
			received[event.SourceRecordID] = event
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "falco.jsonl")
	statePath := filepath.Join(dir, "state.json")
	appendFalcoTestRecords(t, path)
	r := newDurableFalcoTestReader(t, path, statePath, srv.URL, "cluster-a")
	r.readAndSend(t.Context())
	appendFalcoTestRecords(t, path, "good-a", "stale", "good-b")
	r.readAndSend(t.Context())
	if effects["good-a"] != 1 || effects["good-b"] != 1 || len(r.delivery.state.Pending) != 0 || len(r.delivery.state.Quarantine) != 1 {
		t.Fatalf("mixed batch blocked good events: effects=%v state=%+v", effects, r.delivery.state)
	}
	if coverage.Status != "failed" || !strings.Contains(coverage.Reason, "quarantined") {
		t.Fatalf("quarantined evidence cannot prove clean coverage: %+v", coverage)
	}
	staleID := r.delivery.state.Quarantine[0].SourceRecordID
	st, err := os.Stat(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0600 {
		t.Fatalf("outbox permissions=%v", st.Mode())
	}
	r.CloseDeliveryState()
	after := newDurableFalcoTestReader(t, path, statePath, srv.URL, "cluster-a")
	appendFalcoTestRecords(t, path, "fresh")
	after.readAndSend(t.Context())
	if effects["fresh"] != 1 || effects["good-a"] != 1 || after.delivery.state.Quarantine[0].SourceRecordID != staleID {
		t.Fatalf("restart lost identity or blocked fresh events: effects=%v", effects)
	}
	acceptStale = true
	next := after.delivery.state
	next.QuarantineRetryAt = time.Now().Add(-time.Minute)
	if err := after.delivery.save(next); err != nil {
		t.Fatal(err)
	}
	after.readAndSend(t.Context())
	if effects["stale"] != 1 || len(after.delivery.state.Quarantine) != 0 || received[staleID].SourceRecordID != staleID {
		t.Fatalf("quarantine recovery/replay failed: effects=%v", effects)
	}
}

func TestFalcoDurableRotationReplaysPendingBeforeReadingReplacement(t *testing.T) {
	accept := false
	var pending Event
	var received []Event
	var coverage collection.RuntimeCoverage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/api/v2/runtime/coverage" {
			_ = json.NewDecoder(req.Body).Decode(&coverage)
			return
		}
		var events []Event
		_ = json.NewDecoder(req.Body).Decode(&events)
		if !accept {
			pending = events[0]
			w.WriteHeader(429)
			return
		}
		received = append(received, events...)
	}))
	defer srv.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "falco.jsonl")
	statePath := filepath.Join(dir, "state.json")
	appendFalcoTestRecords(t, path)
	r := newDurableFalcoTestReader(t, path, statePath, srv.URL, "cluster-a")
	r.readAndSend(t.Context())
	appendFalcoTestRecords(t, path, "old-pod")
	r.readAndSend(t.Context())
	next := r.delivery.state
	next.RetryAt = time.Now().Add(-time.Minute)
	if err := r.delivery.save(next); err != nil {
		t.Fatal(err)
	}
	r.CloseDeliveryState()
	if err := os.Rename(path, path+".rotated"); err != nil {
		t.Fatal(err)
	}
	// Keep the old inode alive and make the replacement bigger than the cursor.
	appendFalcoTestRecords(t, path, "new-pod-with-a-longer-uid")
	accept = true
	after := newDurableFalcoTestReader(t, path, statePath, srv.URL, "cluster-a")
	after.readAndSend(t.Context())
	if len(received) != 2 || received[0].SourceRecordID != pending.SourceRecordID || received[0].IngestedAt != pending.IngestedAt || received[1].Pod["uid"] != "new-pod-with-a-longer-uid" || received[1].SourceRecordID == pending.SourceRecordID {
		t.Fatalf("rotation lost/reordered evidence: pending=%+v received=%+v", pending, received)
	}
	if coverage.Status != "failed" || !strings.Contains(coverage.Reason, "rotated") {
		t.Fatalf("rotation must break coverage: %+v", coverage)
	}
}

func TestFalcoDurablePersistenceFailureDoesNotSendOrAdvanceCursor(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/api/v2/runtime/events" {
			calls++
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "falco.jsonl")
	statePath := filepath.Join(dir, "state.json")
	appendFalcoTestRecords(t, path)
	r := newDurableFalcoTestReader(t, path, statePath, srv.URL, "cluster-a")
	r.readAndSend(t.Context())
	if err := os.Rename(statePath, statePath+".backup"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(statePath, 0700); err != nil {
		t.Fatal(err)
	}
	appendFalcoTestRecords(t, path, "retained")
	r.readAndSend(t.Context())
	if calls != 0 || r.offset != 0 || r.delivery.state.Offset != 0 {
		t.Fatalf("HTTP/cursor advanced before persistence: calls=%d offset=%d state=%+v", calls, r.offset, r.delivery.state)
	}
	if err := os.Remove(statePath); err != nil {
		t.Fatal(err)
	} // Empty test-fixture directory only.
	if err := os.Rename(statePath+".backup", statePath); err != nil {
		t.Fatal(err)
	}
	r.readAndSend(t.Context())
	if calls != 1 || r.offset == 0 || len(r.delivery.state.Pending) != 0 {
		t.Fatalf("source evidence did not recover after persistence resumed: calls=%d state=%+v", calls, r.delivery.state)
	}
}

func TestFalcoDurableLargeBacklogDrainsWithinCapacity(t *testing.T) {
	received := map[string]bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v2/runtime/events" {
			return
		}
		var events []Event
		_ = json.NewDecoder(req.Body).Decode(&events)
		if len(events) > falcoStateMaxEvents {
			t.Errorf("oversized outbox batch: %d", len(events))
		}
		for _, event := range events {
			if received[event.SourceRecordID] {
				t.Errorf("duplicate source record: %s", event.SourceRecordID)
			}
			received[event.SourceRecordID] = true
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "falco.jsonl")
	appendFalcoTestRecords(t, path)
	r := newDurableFalcoTestReader(t, path, filepath.Join(dir, "state.json"), srv.URL, "cluster-a")
	r.readAndSend(t.Context())
	uids := make([]string, falcoStateMaxEvents+5)
	for i := range uids {
		uids[i] = fmt.Sprint(i)
	}
	appendFalcoTestRecords(t, path, uids...)
	r.readAndSend(t.Context())
	if len(received) != falcoStateMaxEvents {
		t.Fatalf("first bounded read did not progress: %d", len(received))
	}
	r.readAndSend(t.Context())
	if len(received) != len(uids) || len(r.delivery.state.Pending) != 0 {
		t.Fatalf("backlog stalled/lost: %d", len(received))
	}
}

func TestFalcoDurableBackoffSurvivesRestartAndMissingSource(t *testing.T) {
	calls := 0
	var first Event
	accept := false
	var received Event
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/api/v2/runtime/coverage" {
			return
		}
		calls++
		var events []Event
		_ = json.NewDecoder(req.Body).Decode(&events)
		if !accept {
			first = events[0]
			w.Header().Set("Retry-After", "120")
			w.WriteHeader(429)
			return
		}
		received = events[0]
	}))
	defer srv.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "falco.jsonl")
	statePath := filepath.Join(dir, "state.json")
	appendFalcoTestRecords(t, path)
	r := newDurableFalcoTestReader(t, path, statePath, srv.URL, "cluster-a")
	r.readAndSend(t.Context())
	appendFalcoTestRecords(t, path, "pod")
	r.readAndSend(t.Context())
	if calls != 1 || len(r.delivery.state.Pending) != 1 {
		t.Fatalf("pending evidence lost: calls=%d state=%+v", calls, r.delivery.state)
	}
	r.CloseDeliveryState()
	accept = true
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	after := newDurableFalcoTestReader(t, path, statePath, srv.URL, "cluster-a")
	after.readAndSend(t.Context())
	if calls != 1 {
		t.Fatalf("persisted Retry-After ignored after restart: %d", calls)
	}
	next := after.delivery.state
	next.RetryAt = time.Now().Add(-time.Minute)
	if err := after.delivery.save(next); err != nil {
		t.Fatal(err)
	}
	after.readAndSend(t.Context())
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(received)
	if calls != 2 || string(a) != string(b) || len(after.delivery.state.Pending) != 0 {
		t.Fatalf("replayed payload changed/lost: first=%s received=%s calls=%d", a, b, calls)
	}
}

func TestFalcoDurableStateFailsClosedOnCorruptionBindingAndConcurrentWriter(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/runtime/events" {
			calls++
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "falco.jsonl")
	statePath := filepath.Join(dir, "state.json")
	appendFalcoTestRecords(t, path)
	r := newDurableFalcoTestReader(t, path, statePath, srv.URL, "cluster-a")
	r.readAndSend(t.Context())
	appendFalcoTestRecords(t, path, "pod")
	concurrent := newDurableFalcoTestReader(t, path, statePath, srv.URL, "cluster-a")
	concurrent.readAndSend(t.Context())
	concurrent.CloseDeliveryState()
	if calls != 0 {
		t.Fatal("concurrent writer ingested events")
	}
	r.CloseDeliveryState()
	foreign := newDurableFalcoTestReader(t, path, statePath, srv.URL, "cluster-b")
	foreign.readAndSend(t.Context())
	foreign.CloseDeliveryState()
	if calls != 0 {
		t.Fatal("foreign-principal state ingested events")
	}
	if err := os.WriteFile(statePath, []byte(`{"version":`), 0600); err != nil {
		t.Fatal(err)
	}
	corrupt := newDurableFalcoTestReader(t, path, statePath, srv.URL, "cluster-a")
	corrupt.readAndSend(t.Context())
	if calls != 0 {
		t.Fatal("corrupt state was ignored")
	}
}

func TestFalcoDurableIsolationBudgetAndCapacityKeepEvidence(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/runtime/coverage" {
			return
		}
		calls++
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"code":"runtime_ownership_mismatch"}`))
	}))
	defer srv.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "falco.jsonl")
	statePath := filepath.Join(dir, "state.json")
	appendFalcoTestRecords(t, path)
	r := newDurableFalcoTestReader(t, path, statePath, srv.URL, "cluster-a")
	r.readAndSend(t.Context())
	uids := make([]string, 200)
	for i := range uids {
		uids[i] = fmt.Sprint(i)
	}
	appendFalcoTestRecords(t, path, uids...)
	r.readAndSend(t.Context())
	if calls > corehttp.DeliveryRequestLimit || len(r.delivery.state.Pending)+len(r.delivery.state.Quarantine) != 200 {
		t.Fatalf("unbounded isolation or loss: calls=%d", calls)
	}
	seen := map[string]bool{}
	for _, events := range [][]Event{r.delivery.state.Pending, r.delivery.state.Quarantine} {
		for _, event := range events {
			if seen[event.SourceRecordID] {
				t.Fatal("duplicated source record")
			}
			seen[event.SourceRecordID] = true
		}
	}
	next := r.delivery.state
	next.Pending = make([]Event, falcoStateMaxEvents+1)
	if err := r.delivery.save(next); err == nil || !strings.Contains(err.Error(), "capacity") {
		t.Fatalf("capacity must fail closed: %v", err)
	}
	if len(r.delivery.state.Pending)+len(r.delivery.state.Quarantine) != 200 {
		t.Fatal("capacity failure discarded evidence")
	}
}
