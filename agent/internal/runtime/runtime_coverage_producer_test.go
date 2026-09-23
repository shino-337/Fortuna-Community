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
