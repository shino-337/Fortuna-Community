package runtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/fortuna/api/collection"
)

func TestCoverageReporterRetriesImmutablePayloadBeforeNewWindow(t *testing.T) {
	var mu sync.Mutex
	var got []collection.RuntimeCoverage
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/runtime/coverage" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var c collection.RuntimeCoverage
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		got = append(got, c)
		calls++
		n := calls
		mu.Unlock()
		if n == 1 {
			http.Error(w, "response lost", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	session := "session-reporter-000001"
	r := NewCoverageReporter(srv.URL, "falco", collection.RuntimeSourceFalco, session)
	start := time.Now().UTC()
	r.Reset(start)

	err := r.Observe(start.Add(time.Second), CoverageStats{}, "")
	if err == nil {
		t.Fatal("expected first coverage POST failure")
	}
	// The next observation must not mutate the unacknowledged payload. It first
	// retries that exact report, then emits the later window separately.
	requireNoErr(t, r.Observe(start.Add(2*time.Second), CoverageStats{Emitted: 1, Delivered: 1}, ""))

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 3 {
		t.Fatalf("coverage calls=%d want 3", len(got))
	}
	if got[0].SessionID != session || got[1].SessionID != session || got[2].SessionID != session {
		t.Fatalf("coverage session changed across reporter windows: %+v", got)
	}
	if got[0].ID != got[1].ID {
		t.Fatalf("pending coverage ID changed across retry: %q != %q", got[0].ID, got[1].ID)
	}
	b0, _ := json.Marshal(got[0])
	b1, _ := json.Marshal(got[1])
	if string(b0) != string(b1) {
		t.Fatalf("pending coverage payload mutated across retry:\n%s\n%s", b0, b1)
	}
	if got[2].ID == got[1].ID {
		t.Fatal("new window reused prior coverage ID")
	}
	if !got[2].WindowStart.Equal(got[1].WindowEnd) {
		t.Fatalf("windows not adjacent: prior=%v next=%v", got[1].WindowEnd, got[2].WindowStart)
	}
}

func TestCoverageReporterMarksLossAndErrorsFailed(t *testing.T) {
	var got []collection.RuntimeCoverage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var c collection.RuntimeCoverage
		_ = json.NewDecoder(r.Body).Decode(&c)
		got = append(got, c)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := NewCoverageReporter(srv.URL, "runtime-file", collection.RuntimeSourceFile)
	start := time.Now().UTC()
	r.Reset(start)
	requireNoErr(t, r.Observe(start.Add(time.Second), CoverageStats{Invalid: 1}, "invalid JSON"))
	requireNoErr(t, r.Observe(start.Add(2*time.Second), CoverageStats{}, ""))

	if len(got) != 2 {
		t.Fatalf("coverage calls=%d", len(got))
	}
	if got[0].Status != "failed" || got[0].Invalid != 1 {
		t.Fatalf("loss window not failed: %+v", got[0])
	}
	if got[1].Status != "complete" || got[1].Emitted != 0 || got[1].Delivered != 0 {
		t.Fatalf("clean empty recovery not complete: %+v", got[1])
	}
}

func TestCoverageReporterCoalescesCleanWindowsByCadence(t *testing.T) {
	var got []collection.RuntimeCoverage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var c collection.RuntimeCoverage
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			t.Fatal(err)
		}
		got = append(got, c)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := NewCoverageReporter(srv.URL, "runtime-file", collection.RuntimeSourceFile)
	r.SetCadence(30 * time.Second)
	start := time.Now().UTC()
	r.Reset(start)

	for i := 1; i <= 5; i++ {
		requireNoErr(t, r.Observe(start.Add(time.Duration(i)*5*time.Second), CoverageStats{}, ""))
	}
	if len(got) != 0 {
		t.Fatalf("clean observations emitted before cadence: %d", len(got))
	}
	requireNoErr(t, r.Observe(start.Add(30*time.Second), CoverageStats{}, ""))
	if len(got) != 1 {
		t.Fatalf("coverage receipts=%d want 1", len(got))
	}
	if !got[0].WindowStart.Equal(start) || !got[0].WindowEnd.Equal(start.Add(30*time.Second)) {
		t.Fatalf("coalesced window mismatch: %+v", got[0])
	}
}

func TestCoverageReporterFailureBypassesCadence(t *testing.T) {
	var got []collection.RuntimeCoverage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var c collection.RuntimeCoverage
		_ = json.NewDecoder(r.Body).Decode(&c)
		got = append(got, c)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := NewCoverageReporter(srv.URL, "runtime-file", collection.RuntimeSourceFile)
	r.SetCadence(30 * time.Second)
	start := time.Now().UTC()
	r.Reset(start)

	requireNoErr(t, r.Observe(start.Add(5*time.Second), CoverageStats{}, ""))
	if len(got) != 0 {
		t.Fatal("clean pre-cadence observation should remain aggregated")
	}
	requireNoErr(t, r.Observe(start.Add(10*time.Second), CoverageStats{Errors: 1}, "runtime source failed"))
	if len(got) != 1 || got[0].Status != "failed" || got[0].Errors != 1 {
		t.Fatalf("failure did not bypass cadence: %+v", got)
	}
	if !got[0].WindowStart.Equal(start) || !got[0].WindowEnd.Equal(start.Add(10*time.Second)) {
		t.Fatalf("failed aggregate window mismatch: %+v", got[0])
	}
}

func TestCoverageReporterFlushForcesCleanBacklog(t *testing.T) {
	var got []collection.RuntimeCoverage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var c collection.RuntimeCoverage
		_ = json.NewDecoder(r.Body).Decode(&c)
		got = append(got, c)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := NewCoverageReporter(srv.URL, "runtime-file", collection.RuntimeSourceFile)
	r.SetCadence(time.Minute)
	start := time.Now().UTC()
	r.Reset(start)
	requireNoErr(t, r.Observe(start.Add(5*time.Second), CoverageStats{}, ""))
	requireNoErr(t, r.Flush())
	if len(got) != 1 || got[0].Status != "complete" {
		t.Fatalf("final flush did not persist clean backlog: %+v", got)
	}
}

func requireNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
