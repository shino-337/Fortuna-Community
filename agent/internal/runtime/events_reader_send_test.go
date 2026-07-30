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
	if atomic.LoadUint64(&r.v1Fallback) != 0 {
		t.Fatalf("expected v1Fallback=0, got %d", atomic.LoadUint64(&r.v1Fallback))
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

func TestReaderSend_V1Fallback_MetricsUpdated(t *testing.T) {
	var v1Calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/runtime/events" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Path == "/api/v1/runtime/events" {
			atomic.AddInt32(&v1Calls, 1)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	r := NewReader("/tmp/unused", time.Second, ts.URL)
	err := r.send([]Event{{
		Pod:       map[string]interface{}{"uid": "pod-2"},
		Syscall:   "connect",
		Target:    "1.1.1.1:443",
		Timestamp: time.Now().Unix(),
	}})
	if err != nil {
		t.Fatalf("send error: %v", err)
	}
	if atomic.LoadInt32(&v1Calls) != 1 {
		t.Fatalf("expected exactly 1 v1 call, got %d", atomic.LoadInt32(&v1Calls))
	}
	if atomic.LoadUint64(&r.v1Fallback) != 1 {
		t.Fatalf("expected v1Fallback=1, got %d", atomic.LoadUint64(&r.v1Fallback))
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
