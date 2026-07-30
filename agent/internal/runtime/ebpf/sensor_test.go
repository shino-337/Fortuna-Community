package ebpf

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	received := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var batch []runtime.Event
		_ = json.NewDecoder(r.Body).Decode(&batch)
		received += len(batch)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewSensor("connect", srv.URL, "node-2", 50*time.Millisecond, 5, false)
	ctx, cancel := context.WithCancel(context.Background())
	go s.flushLoop(ctx)
	s.enqueueEvent(runtime.Event{Signal: "EBPF_CONNECT_EVENT", Syscall: "connect", Timestamp: time.Now().Unix()})
	time.Sleep(120 * time.Millisecond)
	cancel()
	if received < 1 {
		t.Fatalf("expected flushed events, got %d", received)
	}
}
