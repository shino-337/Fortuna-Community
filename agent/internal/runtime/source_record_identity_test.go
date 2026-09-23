package runtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeFileSourceRecordIdentitySurvivesRestartAndSeparatesIdenticalRecords(t *testing.T) {
	var batches [][]Event
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/runtime/events":
			var events []Event
			if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
				t.Fatal(err)
			}
			batches = append(batches, events)
			w.WriteHeader(http.StatusOK)
		case "/api/v2/runtime/coverage":
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "events.jsonl")
	line := `{"pod":{"uid":"pod-a","namespace":"default"},"syscall":"execve","target":"/bin/sh","timestamp":1700000000,"confidence":0.9}` + "\n"
	if err := os.WriteFile(path, []byte(line+line), 0600); err != nil {
		t.Fatal(err)
	}

	first := NewReader(path, 0, srv.URL)
	first.readAndSend()
	if len(batches) != 1 || len(batches[0]) != 2 {
		t.Fatalf("first delivery batches=%d events=%v", len(batches), batches)
	}
	a, b := batches[0][0], batches[0][1]
	if a.EventID == "" || a.EventID != b.EventID {
		t.Fatalf("test requires legacy semantic EventID collision: a=%q b=%q", a.EventID, b.EventID)
	}
	if a.SourceRecordID == "" || b.SourceRecordID == "" || a.SourceRecordID == b.SourceRecordID {
		t.Fatalf("identical same-second records collapsed source identity: a=%q b=%q", a.SourceRecordID, b.SourceRecordID)
	}

	// Simulate Agent restart. The generic reader intentionally has no durable
	// cursor and replays complete records, but physical record identities must be
	// reconstructed identically so Core can dedupe them.
	second := NewReader(path, 0, srv.URL)
	second.readAndSend()
	if len(batches) != 2 || len(batches[1]) != 2 {
		t.Fatalf("restart delivery batches=%d events=%v", len(batches), batches)
	}
	if batches[1][0].SourceRecordID != a.SourceRecordID || batches[1][1].SourceRecordID != b.SourceRecordID {
		t.Fatalf("restart changed physical identities: first=%q,%q second=%q,%q",
			a.SourceRecordID, b.SourceRecordID,
			batches[1][0].SourceRecordID, batches[1][1].SourceRecordID)
	}
}
