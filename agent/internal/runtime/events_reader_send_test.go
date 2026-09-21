package runtime

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestReaderSend_V2Success_EnrichesCanonicalFieldsAndMetrics(t *testing.T) {
	var got []Event
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/runtime/events" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	r := NewReader("/tmp/unused", time.Second, ts.URL)
	err := r.send([]Event{{
		Pod:             map[string]interface{}{"uid": "pod-1"},
		Syscall:         "openat",
		Target:          "/tmp/x",
		Timestamp:       time.Now().Unix(),
		ResolutionState: "partial",
	}})
	if err != nil {
		t.Fatalf("send error: %v", err)
	}
	if atomic.LoadUint64(&r.v2Success) != 1 {
		t.Fatalf("expected v2Success=1, got %d", atomic.LoadUint64(&r.v2Success))
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	if got[0].EventID == "" || got[0].ObservedAt == "" || got[0].IngestedAt == "" || got[0].PayloadHash == "" {
		t.Fatalf("canonical fields not enriched: %+v", got[0])
	}
	if got[0].ResolutionState != "partial" {
		t.Fatalf("resolution_state mismatch, got %q", got[0].ResolutionState)
	}
}

func TestNormalizeResolutionState(t *testing.T) {
	cases := map[string]string{
		"resolved":   "resolved",
		"partial":    "partial",
		"unresolved": "unresolved",
		"INVALID":    "unresolved",
		"":           "unresolved",
	}
	for in, want := range cases {
		if got := normalizeResolutionState(in); got != want {
			t.Fatalf("normalizeResolutionState(%q)=%q want %q", in, got, want)
		}
	}
}

// A 404, server failure, or authorization failure must never downgrade protocol.
// Returning the error lets the existing retry queues retain the batch.
func TestRuntimeSendersNeverDowngrade(t *testing.T) {
	for _, status := range []int{404, 401, 403, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			for _, kind := range []string{"file", "falco"} {
				t.Run(kind, func(t *testing.T) {
					var calls int32
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						atomic.AddInt32(&calls, 1)
						if req.URL.Path != "/api/v2/runtime/events" {
							t.Errorf("protocol downgrade: %s", req.URL.Path)
						}
						w.WriteHeader(status)
					}))
					defer srv.Close()
					events := []Event{{Pod: map[string]interface{}{"uid": "p"}, Syscall: "execve", Confidence: 0.9}}
					var err error
					if kind == "file" {
						err = NewReader("/unused", time.Second, srv.URL).send(events)
					} else {
						err = NewFalcoReader("/unused", time.Second, srv.URL, "node", nil).send(events)
					}
					if err == nil || atomic.LoadInt32(&calls) != 1 {
						t.Fatalf("want one failed v2 request, calls=%d err=%v", calls, err)
					}
				})
			}
		})
	}
}
