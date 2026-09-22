package runtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"accepted": 1, "processed": 0, "replayed": 0})
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


func TestRuntimeReaderBuffersPartialRecordUntilNewline(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"accepted": 1})
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "events.jsonl")
	prefix := `{"pod":{"uid":"pod-a","namespace":"default"},"syscall":"execve","target":"/bin/sh"`
	if err := os.WriteFile(path, []byte(prefix), 0600); err != nil {
		t.Fatal(err)
	}
	r := NewReader(path, time.Second, srv.URL)
	r.readAndSend()
	if r.invalidLines != 0 || atomic.LoadInt32(&calls) != 0 {
		t.Fatalf("partial line was treated as an event: invalid=%d calls=%d", r.invalidLines, calls)
	}
	if string(r.lineBuf) != prefix {
		t.Fatalf("partial line not retained: %q", string(r.lineBuf))
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`,"timestamp":1}` + "\n")
	_ = f.Close()
	r.readAndSend()
	if atomic.LoadInt32(&calls) != 1 || r.sentEvents != 1 || len(r.lineBuf) != 0 {
		t.Fatalf("completed partial record was not delivered once: calls=%d sent=%d buf=%q", calls, r.sentEvents, string(r.lineBuf))
	}
}

func TestRuntimeReaderDetectsInodeRotationAndReadsNewFileFromStart(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var batch []Event
		_ = json.NewDecoder(r.Body).Decode(&batch)
		atomic.AddInt32(&calls, int32(len(batch)))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"accepted": len(batch)})
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	first := `{"pod":{"uid":"pod-a"},"syscall":"read","target":"a","timestamp":1}` + "\n"
	requireWrite := func(p, v string) {
		t.Helper()
		if err := os.WriteFile(p, []byte(v), 0600); err != nil { t.Fatal(err) }
	}
	requireWrite(path, first)
	r := NewReader(path, time.Second, srv.URL)
	r.readAndSend()
	if atomic.LoadInt32(&calls) != 1 { t.Fatalf("first file calls=%d", calls) }

	if err := os.Rename(path, path+".1"); err != nil { t.Fatal(err) }
	second := `{"pod":{"uid":"pod-a"},"syscall":"read","target":"b","timestamp":2}` + "\n"
	requireWrite(path, second)
	r.readAndSend()
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("rotated file was not read from beginning: calls=%d", calls)
	}
}
