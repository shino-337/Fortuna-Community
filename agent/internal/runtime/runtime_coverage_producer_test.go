package runtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/fortuna/api/collection"
)

func TestRuntimeFileCoverageEmptyAndInvalid(t *testing.T) {
	for _, tc := range []string{"empty", "invalid"} {
		t.Run(tc, func(t *testing.T) {
			var got []collection.RuntimeCoverage
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v2/runtime/coverage":
					var c collection.RuntimeCoverage
					if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
						t.Fatal(err)
					}
					got = append(got, c)
					w.WriteHeader(http.StatusOK)
				case "/api/v2/runtime/events":
					w.WriteHeader(http.StatusOK)
				default:
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
			}))
			defer srv.Close()

			path := filepath.Join(t.TempDir(), "runtime.jsonl")
			content := []byte{}
			if tc == "invalid" {
				content = []byte("not-json\n")
			}
			if err := os.WriteFile(path, content, 0600); err != nil {
				t.Fatal(err)
			}
			r := NewReader(path, 0, srv.URL)
			r.readAndSend()

			if len(got) != 1 {
				t.Fatalf("coverage reports=%d", len(got))
			}
			if tc == "empty" {
				if got[0].Status != "complete" || got[0].Emitted != 0 || got[0].Delivered != 0 {
					t.Fatalf("empty runtime window not complete: %+v", got[0])
				}
			} else {
				if got[0].Status != "failed" || got[0].Invalid != 1 {
					t.Fatalf("invalid runtime input not failed coverage: %+v", got[0])
				}
			}
		})
	}
}

func TestFalcoCoverageRejectsUnresolvedEventAsDrop(t *testing.T) {
	var got []collection.RuntimeCoverage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/runtime/coverage":
			var c collection.RuntimeCoverage
			if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
				t.Fatal(err)
			}
			got = append(got, c)
			w.WriteHeader(http.StatusOK)
		case "/api/v2/runtime/events":
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "falco.jsonl")
	if err := os.WriteFile(path, []byte{}, 0600); err != nil {
		t.Fatal(err)
	}
	r := NewFalcoReader(path, 0, srv.URL, "node-a", nil)
	// Establish a clean tail cursor first.
	r.readAndSend(t.Context())
	got = nil

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(`{"rule":"no-pod","priority":"Warning","output_fields":{"evt.type":"execve"}}` + "\n")
	if err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	_ = f.Close()

	r.readAndSend(t.Context())
	if len(got) != 1 {
		t.Fatalf("coverage reports=%d", len(got))
	}
	if got[0].Status != "failed" || got[0].Dropped != 1 {
		t.Fatalf("unresolved Falco event did not break coverage: %+v", got[0])
	}
}


func TestFalcoCoverageDetectsFileReplacementAndProcessesNewFile(t *testing.T) {
	var coverage []collection.RuntimeCoverage
	var delivered int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/runtime/coverage":
			var report collection.RuntimeCoverage
			if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
				t.Fatal(err)
			}
			coverage = append(coverage, report)
			w.WriteHeader(http.StatusOK)
		case "/api/v2/runtime/events":
			var events []Event
			if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
				t.Fatal(err)
			}
			delivered += len(events)
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "falco.jsonl")
	old := `{"rule":"historical","priority":"Warning","output_fields":{"k8s.pod.uid":"old","evt.type":"execve"}}` + "\n"
	if err := os.WriteFile(path, []byte(old), 0600); err != nil {
		t.Fatal(err)
	}

	r := NewFalcoReader(path, 0, srv.URL, "node-a", nil)
	// Initial non-empty file is intentionally tailed from EOF.
	r.readAndSend(t.Context())
	if delivered != 0 {
		t.Fatalf("historical Falco backlog should not be delivered: %d", delivered)
	}

	replacement := filepath.Join(dir, "falco.new")
	fresh := `{"rule":"fresh","priority":"Warning","output_fields":{"k8s.pod.uid":"new","evt.type":"execve"}}` + "\n"
	if err := os.WriteFile(replacement, []byte(fresh), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}

	r.readAndSend(t.Context())
	if delivered != 1 {
		t.Fatalf("replacement Falco file event was skipped: delivered=%d", delivered)
	}
	if len(coverage) != 1 || coverage[0].Status != "failed" || coverage[0].Errors == 0 {
		t.Fatalf("file replacement must break clean continuity: %+v", coverage)
	}
}


func TestRuntimeFileCoverageRetainsPartialRecord(t *testing.T) {
	var coverage []collection.RuntimeCoverage
	delivered := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/runtime/coverage":
			var report collection.RuntimeCoverage
			if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
				t.Fatal(err)
			}
			coverage = append(coverage, report)
			w.WriteHeader(http.StatusOK)
		case "/api/v2/runtime/events":
			var events []Event
			if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
				t.Fatal(err)
			}
			delivered += len(events)
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "runtime.jsonl")
	prefix := `{"pod":{"uid":"pod-a"},"syscall":"execve","target":"/bin/sh"`
	suffix := `,"timestamp":1,"confidence":1}` + "\n"
	if err := os.WriteFile(path, []byte(prefix), 0600); err != nil {
		t.Fatal(err)
	}

	r := NewReader(path, 0, srv.URL)
	r.readAndSend()
	if delivered != 0 || r.invalidLines != 0 || len(r.lineBuf) == 0 {
		t.Fatalf("partial record was consumed or misclassified: delivered=%d invalid=%d buf=%q", delivered, r.invalidLines, string(r.lineBuf))
	}
	if len(coverage) != 1 || coverage[0].Status != "failed" || coverage[0].Errors == 0 {
		t.Fatalf("partial record did not break coverage: %+v", coverage)
	}

	fh, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fh.WriteString(suffix); err != nil {
		_ = fh.Close()
		t.Fatal(err)
	}
	_ = fh.Close()

	r.readAndSend()
	if delivered != 1 || r.invalidLines != 0 || len(r.lineBuf) != 0 {
		t.Fatalf("completed record was not delivered exactly once: delivered=%d invalid=%d buf=%q", delivered, r.invalidLines, string(r.lineBuf))
	}
	if len(coverage) != 2 || coverage[1].Status != "complete" {
		t.Fatalf("completed record did not recover coverage: %+v", coverage)
	}
}

func TestRuntimeFileCoverageDetectsFileReplacement(t *testing.T) {
	var coverage []collection.RuntimeCoverage
	delivered := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/runtime/coverage":
			var report collection.RuntimeCoverage
			if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
				t.Fatal(err)
			}
			coverage = append(coverage, report)
			w.WriteHeader(http.StatusOK)
		case "/api/v2/runtime/events":
			var events []Event
			if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
				t.Fatal(err)
			}
			delivered += len(events)
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.jsonl")
	first := `{"pod":{"uid":"old"},"syscall":"execve","target":"x","timestamp":1,"confidence":1}` + "\n"
	if err := os.WriteFile(path, []byte(first), 0600); err != nil {
		t.Fatal(err)
	}
	r := NewReader(path, 0, srv.URL)
	r.readAndSend()
	if delivered != 1 {
		t.Fatalf("initial event delivery=%d", delivered)
	}

	replacement := filepath.Join(dir, "runtime.new")
	second := `{"pod":{"uid":"new"},"syscall":"execve","target":"/a/much/longer/target/to/exceed/the/old/cursor","timestamp":2,"confidence":1}` + "\n"
	if err := os.WriteFile(replacement, []byte(second), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}

	r.readAndSend()
	if delivered != 2 {
		t.Fatalf("replacement file event was skipped: delivered=%d", delivered)
	}
	if len(coverage) != 2 || coverage[1].Status != "failed" || coverage[1].Errors == 0 {
		t.Fatalf("runtime file replacement must break clean continuity: %+v", coverage)
	}
}


func TestRuntimeFilePartialRecordSurvivesReaderRestart(t *testing.T) {
	var coverage []collection.RuntimeCoverage
	delivered := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/runtime/coverage":
			var report collection.RuntimeCoverage
			if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
				t.Fatal(err)
			}
			coverage = append(coverage, report)
			w.WriteHeader(http.StatusOK)
		case "/api/v2/runtime/events":
			var events []Event
			if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
				t.Fatal(err)
			}
			delivered += len(events)
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "runtime.jsonl")
	prefix := `{"pod":{"uid":"pod-restart"},"syscall":"execve","target":"/bin/sh"`
	suffix := `,"timestamp":7,"confidence":1}` + "\n"
	if err := os.WriteFile(path, []byte(prefix), 0600); err != nil {
		t.Fatal(err)
	}

	beforeRestart := NewReader(path, 0, srv.URL)
	beforeRestart.readAndSend()
	if delivered != 0 || len(beforeRestart.lineBuf) == 0 {
		t.Fatalf("initial partial record not retained: delivered=%d buf=%q", delivered, string(beforeRestart.lineBuf))
	}
	if len(coverage) != 1 || coverage[0].Status != "failed" || coverage[0].Errors == 0 {
		t.Fatalf("partial record must fail the pre-restart window: %+v", coverage)
	}

	// Simulate process restart: Reader cursor and lineBuf are intentionally
	// in-memory only. The new Reader starts from offset zero, so it re-observes
	// the prefix instead of silently skipping bytes that were only buffered by
	// the prior process.
	afterRestart := NewReader(path, 0, srv.URL)
	afterRestart.readAndSend()
	if delivered != 0 || len(afterRestart.lineBuf) == 0 {
		t.Fatalf("restart silently lost partial prefix: delivered=%d buf=%q", delivered, string(afterRestart.lineBuf))
	}
	if len(coverage) != 2 || coverage[1].Status != "failed" || coverage[1].Errors == 0 {
		t.Fatalf("restart must retain a failed evidence boundary for pending partial data: %+v", coverage)
	}

	fh, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fh.WriteString(suffix); err != nil {
		_ = fh.Close()
		t.Fatal(err)
	}
	_ = fh.Close()

	afterRestart.readAndSend()
	if delivered != 1 || len(afterRestart.lineBuf) != 0 {
		t.Fatalf("partial record disappeared across restart: delivered=%d buf=%q", delivered, string(afterRestart.lineBuf))
	}
	if len(coverage) != 3 || coverage[2].Status != "complete" {
		t.Fatalf("completed post-restart record did not recover observation: %+v", coverage)
	}
}
