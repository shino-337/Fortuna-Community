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
		if r.URL.Path != "/api/v1/runtime/events" {
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
