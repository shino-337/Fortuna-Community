package ebpf

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fortuna/agent/internal/runtime"
)

func TestNormalizeEBPFMode(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"exec", "exec"},
		{"connect", "connect"},
		{"all", "all"},
		{"", "exec"},
		{"weird", "exec"},
		{"  CONNECT ", "connect"},
	}
	for _, c := range cases {
		got := normalizeEBPFMode(c.in)
		if got != c.want {
			t.Fatalf("normalizeEBPFMode(%q): got %q want %q", c.in, got, c.want)
		}
	}
}

func TestSendBatchToCoreRuntimeEvents(t *testing.T) {
	var got []runtime.Event
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/runtime/events" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewSensor("exec", srv.URL, "node-1", 100*time.Millisecond, 10, false)
	err := s.send([]runtime.Event{{Signal: "EBPF_EXEC_EVENT", Syscall: "execve", Timestamp: time.Now().Unix()}})
	if err != nil {
		t.Fatalf("send() error: %v", err)
	}
	if len(got) != 1 || got[0].Signal != "EBPF_EXEC_EVENT" {
		t.Fatalf("unexpected payload: %+v", got)
	}
	if got[0].EventID == "" || got[0].SourceRecordID == "" || got[0].ObservedAt == "" || got[0].IngestedAt == "" || got[0].PayloadHash == "" {
		t.Fatalf("eBPF did not emit canonical v2 metadata: %+v", got[0])
	}
}

func TestEventIncludesCapabilityField(t *testing.T) {
	ev := runtime.Event{
		Signal:     "EBPF_EXEC_EVENT",
		Syscall:    "execve",
		Capability: "EBPF_EXEC_TRACE",
		Timestamp:  time.Now().Unix(),
	}
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	if string(b) == "" || !contains(string(b), "\"capability\":\"EBPF_EXEC_TRACE\"") {
		t.Fatalf("expected capability field in json, got: %s", string(b))
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func TestFlushLoopDrainsQueuedEvents(t *testing.T) {
	var received int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var batch []runtime.Event
		_ = json.NewDecoder(r.Body).Decode(&batch)
		atomic.AddInt32(&received, int32(len(batch)))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewSensor("connect", srv.URL, "node-2", 50*time.Millisecond, 5, false)
	ctx, cancel := context.WithCancel(context.Background())
	go s.flushLoop(ctx)
	s.enqueueEvent(runtime.Event{Signal: "EBPF_CONNECT_EVENT", Syscall: "connect", Timestamp: time.Now().Unix()})
	time.Sleep(120 * time.Millisecond)
	cancel()
	if atomic.LoadInt32(&received) < 1 {
		t.Fatalf("expected flushed events, got %d", atomic.LoadInt32(&received))
	}
}

func TestFlushLoopRetriesFailedBatchWithoutDroppingIt(t *testing.T) {
	var calls int32
	attempts := make(chan []runtime.Event, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var batch []runtime.Event
		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
			t.Errorf("decode batch: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		attempts <- batch
		if atomic.AddInt32(&calls, 1) == 1 {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewSensor("exec", srv.URL, "node-retry", 20*time.Millisecond, 4, false)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.flushLoop(ctx)
		close(done)
	}()

	want := runtime.Event{Signal: "EBPF_RETRY", Syscall: "execve", Timestamp: time.Now().Unix()}
	s.enqueueEvent(want)

	readAttempt := func(label string) []runtime.Event {
		t.Helper()
		select {
		case batch := <-attempts:
			return batch
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for %s attempt", label)
			return nil
		}
	}
	first := readAttempt("first")
	second := readAttempt("retry")
	if len(first) != 1 || len(second) != 1 || first[0].Signal != want.Signal || second[0].Signal != want.Signal {
		t.Fatalf("failed batch was not retried intact: first=%+v second=%+v", first, second)
	}
	if first[0].SourceRecordID == "" || first[0].SourceRecordID != second[0].SourceRecordID {
		t.Fatalf("retry changed source-record identity: first=%q second=%q", first[0].SourceRecordID, second[0].SourceRecordID)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("flush loop did not stop")
	}
	if got := atomic.LoadUint64(&s.droppedEvents); got != 0 {
		t.Fatalf("transient send failure counted as dropped after successful retry: %d", got)
	}
}

func TestFlushLoopAccountsRetainedBatchOnShutdownFailure(t *testing.T) {
	attempted := make(chan struct{}, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var batch []runtime.Event
		_ = json.NewDecoder(r.Body).Decode(&batch)
		attempted <- struct{}{}
		http.Error(w, "still unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := NewSensor("exec", srv.URL, "node-stop", 20*time.Millisecond, 4, false)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.flushLoop(ctx)
		close(done)
	}()
	s.enqueueEvent(runtime.Event{Signal: "EBPF_STOP", Syscall: "execve", Timestamp: time.Now().Unix()})

	select {
	case <-attempted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for failed send")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("flush loop did not stop")
	}
	if got := atomic.LoadUint64(&s.droppedEvents); got != 1 {
		t.Fatalf("shutdown loss must be explicitly accounted: dropped=%d", got)
	}
}


func TestCoverageReportsPendingDeliveryAsFailed(t *testing.T) {
	var got runtimeCoverageEnvelope
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/runtime/coverage" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewSensor("exec", srv.URL, "node-a", time.Second, 4, false)
	atomic.StoreUint64(&s.coverageEmitted, 1)
	atomic.StoreUint64(&s.coverageErrors, 1)
	atomic.StoreUint32(&s.deliveryPending, 1)
	s.reportCoverage()

	if got.Status != "failed" || got.Errors == 0 {
		t.Fatalf("pending delivery did not fail coverage: %+v", got)
	}
}

type runtimeCoverageEnvelope struct {
	Status string `json:"status"`
	Errors uint64 `json:"errors"`
}


func TestCoverageSnapshotDoesNotSplitInflightDelivery(t *testing.T) {
	eventEntered := make(chan struct{})
	releaseEvent := make(chan struct{})
	coverageGot := make(chan runtimeCoverageWindow, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/runtime/events":
			close(eventEntered)
			<-releaseEvent
			w.WriteHeader(http.StatusOK)
		case "/api/v2/runtime/coverage":
			var got runtimeCoverageWindow
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode coverage: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			coverageGot <- got
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	s := NewSensor("exec", srv.URL, "node-a", time.Second, 4, false)
	sendDone := make(chan error, 1)
	go func() {
		sendDone <- s.send([]runtime.Event{{Signal: "EBPF_EXEC", Syscall: "execve", Timestamp: time.Now().Unix()}})
	}()

	select {
	case <-eventEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event delivery")
	}

	reportDone := make(chan struct{})
	go func() {
		s.reportCoverage()
		close(reportDone)
	}()

	select {
	case <-reportDone:
		t.Fatal("coverage snapshot completed while event delivery was still in flight")
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseEvent)
	if err := <-sendDone; err != nil {
		t.Fatal(err)
	}
	select {
	case <-reportDone:
	case <-time.After(2 * time.Second):
		t.Fatal("coverage snapshot did not complete")
	}

	select {
	case got := <-coverageGot:
		if got.Status != "failed" || got.Emitted != 1 || got.Delivered != 1 || got.Errors == 0 {
			t.Fatalf("delivery accounting split or no-op sensor claimed clean coverage: %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("coverage report missing")
	}
}

type runtimeCoverageWindow struct {
	Status    string `json:"status"`
	Emitted   uint64 `json:"emitted"`
	Delivered uint64 `json:"delivered"`
	Errors    uint64 `json:"errors"`
}


func TestEBPFCoverageNeverClaimsCompleteWhileSensorIsNoop(t *testing.T) {
	var got runtimeCoverageWindow
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/runtime/coverage" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewSensor("exec", srv.URL, "node-a", time.Second, 4, false)
	s.reportCoverage()

	if got.Status != "failed" || got.Errors == 0 {
		t.Fatalf("no-op eBPF sensor claimed authoritative coverage: %+v", got)
	}
}
