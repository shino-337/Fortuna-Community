package runtime

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestRuntimeReaderRetainsOffsetUntilIngestSucceeds(t *testing.T) {
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
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "events.jsonl")
	line := `{"pod":{"uid":"pod-a","namespace":"default"},"syscall":"execve","target":"/bin/sh","timestamp":1,"confidence":1}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0600); err != nil {
		t.Fatal(err)
	}

	r := NewReader(path, 0, srv.URL)
	r.readAndSend()
	if r.offset != 0 {
		t.Fatalf("failed ingest advanced offset to %d", r.offset)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("first attempt calls=%d", got)
	}

	r.readAndSend()
	if r.offset == 0 {
		t.Fatal("successful retry did not advance offset")
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("retry calls=%d", got)
	}
	if r.sentEvents != 1 || r.failedEvents != 1 {
		t.Fatalf("unexpected counters sent=%d failed=%d", r.sentEvents, r.failedEvents)
	}
}

func TestRuntimeReaderAdvancesPastInvalidOnlyInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte("not-json\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r := NewReader(path, 0, "http://127.0.0.1:1")
	r.readAndSend()
	if r.offset == 0 {
		t.Fatal("invalid-only input should advance to avoid infinite retry")
	}
	if r.invalidLines != 1 {
		t.Fatal(fmt.Sprintf("invalid lines=%d", r.invalidLines))
	}
}
