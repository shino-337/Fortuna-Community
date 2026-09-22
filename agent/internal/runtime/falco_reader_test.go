package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fortuna/api/collection"
)

func TestFalcoReader_ToRuntimeEvent_MinimalMapping(t *testing.T) {
	r := NewFalcoReader("/tmp/falco.jsonl", 0, "http://core", "node-1", nil)
	fe := falcoEvent{
		Rule:     "Terminal shell in container",
		Priority: "Warning",
		Tags:     []string{"mitre_technique=T1059"},
		OutputFields: map[string]interface{}{
			"k8s.pod.uid":   "11111111-1111-1111-1111-111111111111",
			"k8s.ns.name":   "default",
			"k8s.pod.name":  "demo",
			"k8s.node.name": "node-1",
			"evt.type":      "execve",
			"proc.cmdline":  "/bin/sh -c id",
		},
	}
	ev, ok := r.toRuntimeEvent(context.Background(), &fe)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if ev.Runtime != "falco" || ev.Syscall != "execve" {
		t.Fatalf("unexpected ev: %+v", ev)
	}
	if ev.MitreTechnique != "T1059" {
		t.Fatalf("mitre: %q", ev.MitreTechnique)
	}
	if ev.Severity != "medium" {
		t.Fatalf("severity: %q", ev.Severity)
	}
	if ev.Confidence <= 0 {
		t.Fatalf("confidence must be positive for core v2 ingest: %v", ev.Confidence)
	}
	if ev.Pod["uid"] != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("pod uid: %v", ev.Pod["uid"])
	}
	b, _ := json.Marshal(ev)
	if len(b) == 0 {
		t.Fatalf("marshal empty")
	}
}

func TestExtractMitreTechnique_FallbackPlainTag(t *testing.T) {
	got := extractMitreTechnique([]string{"something", "T1611"})
	if got != "T1611" {
		t.Fatalf("got %q", got)
	}
}

func TestParseFalcoJSONLines_singleAndConcatenated(t *testing.T) {
	t.Helper()
	one := []byte(`{"rule":"x","priority":"Notice","output_fields":{"k8s.pod.uid":"u"}}`)
	got, err := parseFalcoJSONLines(one)
	if err != nil || len(got) != 1 || got[0].Rule != "x" {
		t.Fatalf("single: got %+v err %v", got, err)
	}
	two := []byte(`{"rule":"a","priority":"Notice"}{"rule":"b","priority":"Notice"}`)
	got2, err2 := parseFalcoJSONLines(two)
	if err2 != nil || len(got2) != 2 || got2[0].Rule != "a" || got2[1].Rule != "b" {
		t.Fatalf("concat: got %+v err %v", got2, err2)
	}
}

func TestFalcoReader_ToRuntimeEvent_FallbackSyscallWhenEvtTypeMissing(t *testing.T) {
	r := NewFalcoReader("/tmp/falco.jsonl", time.Second, "http://core", "node-1", nil)
	fe := falcoEvent{
		Rule:     "Write below etc",
		Priority: "Notice",
		OutputFields: map[string]interface{}{
			"k8s.pod.uid":  "22222222-2222-2222-2222-222222222222",
			"k8s.ns.name":  "default",
			"k8s.pod.name": "demo",
			"fd.name":      "/etc/foo",
		},
	}
	ev, ok := r.toRuntimeEvent(context.Background(), &fe)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if ev.Syscall != "falco.alert" {
		t.Fatalf("syscall: %q", ev.Syscall)
	}
}

func TestFalcoReader_ToRuntimeEvent_PreservesResolutionStateFromFields(t *testing.T) {
	r := NewFalcoReader("/tmp/falco.jsonl", time.Second, "http://core", "node-1", nil)
	fe := falcoEvent{
		Rule: "Some Falco Rule",
		OutputFields: map[string]interface{}{
			"k8s.pod.uid":              "33333333-3333-3333-3333-333333333333",
			"k8s.ns.name":              "default",
			"k8s.pod.name":             "demo",
			"evt.type":                 "openat",
			"fortuna.resolution_state": "resolved",
		},
	}
	ev, ok := r.toRuntimeEvent(context.Background(), &fe)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if ev.ResolutionState != "resolved" {
		t.Fatalf("resolution_state: %q", ev.ResolutionState)
	}
}

func TestFalcoReader_ToRuntimeEvent_PreservesResolutionStateFromTags(t *testing.T) {
	r := NewFalcoReader("/tmp/falco.jsonl", time.Second, "http://core", "node-1", nil)
	fe := falcoEvent{
		Rule: "Some Falco Rule",
		Tags: []string{"runtime", "resolution_state=partial"},
		OutputFields: map[string]interface{}{
			"k8s.pod.uid":  "44444444-4444-4444-4444-444444444444",
			"k8s.ns.name":  "default",
			"k8s.pod.name": "demo",
			"evt.type":     "openat",
		},
	}
	ev, ok := r.toRuntimeEvent(context.Background(), &fe)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if ev.ResolutionState != "partial" {
		t.Fatalf("resolution_state: %q", ev.ResolutionState)
	}
}

func TestFalcoReaderRetainsCursorAndPartialLineUntilIngestSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/runtime/events" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"accepted": 1, "processed": 0, "replayed": 0})
	}))
	defer srv.Close()

	prefix := `{"rule":"retry","priority":"Warning","output_fields":{"k8s.pod.uid":"pod-a"`
	suffix := `,"k8s.ns.name":"default","evt.type":"execve"}}` + "\n"
	path := filepath.Join(t.TempDir(), "falco.jsonl")
	if err := os.WriteFile(path, []byte(prefix), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(suffix); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	r := NewFalcoReader(path, time.Second, srv.URL, "node-1", nil)
	r.offset = int64(len(prefix))
	r.lineBuf = []byte(prefix)
	startOffset := r.offset

	r.readAndSend(context.Background())
	if r.offset != startOffset {
		t.Fatalf("failed ingest advanced offset: got=%d want=%d", r.offset, startOffset)
	}
	if string(r.lineBuf) != prefix {
		t.Fatalf("failed ingest did not restore partial prefix: %q", string(r.lineBuf))
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("first attempt calls=%d", got)
	}

	r.readAndSend(context.Background())
	if r.offset != int64(len(prefix)+len(suffix)) {
		t.Fatalf("successful retry offset=%d want=%d", r.offset, len(prefix)+len(suffix))
	}
	if len(r.lineBuf) != 0 {
		t.Fatalf("successful retry left partial buffer: %q", string(r.lineBuf))
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("retry calls=%d", got)
	}
	if r.sentEvents != 1 || r.failedEvents != 1 {
		t.Fatalf("unexpected counters sent=%d failed=%d", r.sentEvents, r.failedEvents)
	}
}


func TestFalcoRuntimeEventIdentityStableAcrossRetry(t *testing.T) {
	r := NewFalcoReader("/tmp/falco.jsonl", time.Second, "http://core", "node-1", nil)
	fe := falcoEvent{
		Time: "2026-09-22T08:00:00.123456789Z",
		Rule: "Stable retry", Priority: "Warning",
		OutputFields: map[string]interface{}{
			"k8s.pod.uid": "pod-stable", "k8s.ns.name": "default",
			"k8s.pod.name": "demo", "evt.type": "execve", "proc.cmdline": "/bin/sh",
		},
	}
	first, ok := r.toRuntimeEvent(context.Background(), &fe)
	if !ok {
		t.Fatal("first Falco conversion failed")
	}
	time.Sleep(10 * time.Millisecond)
	second, ok := r.toRuntimeEvent(context.Background(), &fe)
	if !ok {
		t.Fatal("second Falco conversion failed")
	}
	if first.EventID != second.EventID || first.ObservedAt != second.ObservedAt || first.Timestamp != second.Timestamp {
		t.Fatalf("Falco retry identity changed: first=%+v second=%+v", first, second)
	}
	if first.IngestedAt != "" || second.IngestedAt != "" {
		t.Fatal("Falco producer must leave ingested_at to Core")
	}
}


func TestFalcoUnresolvedPodIsReportedAsCoverageDrop(t *testing.T) {
	token := strings.Repeat("s", 32)
	tokenFile := filepath.Join(t.TempDir(), "agent.token")
	if err := os.WriteFile(tokenFile, []byte(token), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FORTUNA_AGENT_TOKEN_FILE", tokenFile)

	var receipts []collection.RuntimeCoverage
	var eventCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/runtime/coverage":
			var receipt collection.RuntimeCoverage
			if err := json.NewDecoder(r.Body).Decode(&receipt); err != nil { t.Fatal(err) }
			receipts = append(receipts, receipt)
			w.WriteHeader(http.StatusOK)
		case "/api/v2/runtime/events":
			atomic.AddInt32(&eventCalls, 1)
			t.Fatal("unresolved Falco event must not be sent as owned runtime event")
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "falco.jsonl")
	if err := os.WriteFile(path, nil, 0600); err != nil { t.Fatal(err) }
	r := NewFalcoReader(path, time.Second, srv.URL, "node-a", nil)
	r.readAndSend(context.Background()) // establish initial file identity/coverage

	line := `{"time":"2026-09-22T08:00:00Z","rule":"unresolved","priority":"Warning","output_fields":{"evt.type":"execve","proc.cmdline":"/bin/sh"}}` + "\n"
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil { t.Fatal(err) }
	_, _ = file.WriteString(line)
	_ = file.Close()

	r.readAndSend(context.Background())
	if atomic.LoadUint64(&r.droppedEvents) != 1 {
		t.Fatalf("expected unresolved Falco alert to count as dropped, got %d", atomic.LoadUint64(&r.droppedEvents))
	}
	if atomic.LoadInt32(&eventCalls) != 0 {
		t.Fatalf("unexpected runtime event calls: %d", eventCalls)
	}
	found := false
	for _, receipt := range receipts {
		if receipt.Dropped == 1 && receipt.Status == "failed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no failed coverage receipt captured for unresolved Falco alert: %+v", receipts)
	}
}
