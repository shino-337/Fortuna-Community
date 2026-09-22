package runtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fortuna/api/collection"
)

func TestCoverageReporterRetainsPendingWindowUntilCoreACK(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "agent.token")
	if err := os.WriteFile(tokenFile, []byte("scoped-token-0123456789abcdef0123456789"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FORTUNA_AGENT_TOKEN_FILE", tokenFile)

	var calls int32
	var receipts []collection.RuntimeCoverage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/runtime/coverage" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("X-Fortuna-Ingest-Token") != "scoped-token-0123456789abcdef0123456789" {
			t.Fatalf("missing scoped token")
		}
		var receipt collection.RuntimeCoverage
		if err := json.NewDecoder(r.Body).Decode(&receipt); err != nil {
			t.Fatalf("decode receipt: %v", err)
		}
		receipts = append(receipts, receipt)
		if atomic.AddInt32(&calls, 1) == 1 {
			http.Error(w, "lost ack", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	reporter := NewCoverageReporter(srv.URL, "runtime-file", "file", &http.Client{Timeout: time.Second})
	if reporter == nil {
		t.Fatal("expected scoped coverage reporter")
	}
	firstEnd := time.Now().UTC()
	err := reporter.Report(t.Context(), CoverageObservation{End: firstEnd, Emitted: 1, Delivered: 1})
	if err == nil {
		t.Fatal("first coverage POST should fail")
	}
	secondEnd := firstEnd.Add(time.Second)
	if err := reporter.Report(t.Context(), CoverageObservation{End: secondEnd}); err != nil {
		t.Fatalf("second coverage POST: %v", err)
	}
	if len(receipts) != 2 {
		t.Fatalf("receipts=%d", len(receipts))
	}
	if receipts[1].Emitted != 1 || receipts[1].Delivered != 1 {
		t.Fatalf("pending counters were lost: %+v", receipts[1])
	}
	if !receipts[1].WindowStart.Equal(receipts[0].WindowStart) {
		t.Fatalf("pending window start changed after failed coverage POST")
	}
	if !receipts[1].WindowEnd.Equal(secondEnd.Truncate(time.Microsecond)) {
		t.Fatalf("second receipt did not extend pending window: %+v", receipts[1])
	}
}

func TestCoverageReporterKeepsFailureUntilFailureReceiptACK(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "agent.token")
	if err := os.WriteFile(tokenFile, []byte("scoped-token-0123456789abcdef0123456789"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FORTUNA_AGENT_TOKEN_FILE", tokenFile)

	var calls int32
	var last collection.RuntimeCoverage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&last)
		if atomic.AddInt32(&calls, 1) == 1 {
			http.Error(w, "no ack", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	reporter := NewCoverageReporter(srv.URL, "falco", "falco", &http.Client{Timeout: time.Second})
	end := time.Now().UTC()
	_ = reporter.Report(t.Context(), CoverageObservation{
		End: end, Emitted: 1, Delivered: 0, Failed: true, Reason: "delivery_failed_retained",
	})
	if err := reporter.Report(t.Context(), CoverageObservation{
		End: end.Add(time.Second), Emitted: 1, Delivered: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if last.Status != "failed" || last.Delivered == last.Emitted {
		t.Fatalf("failed pending observation was erased before ACK: %+v", last)
	}
}

func TestCoverageReporterDisabledWithoutScopedCredential(t *testing.T) {
	t.Setenv("FORTUNA_AGENT_TOKEN_FILE", "")
	if got := NewCoverageReporter("http://core", "file", "file", nil); got != nil {
		t.Fatal("legacy shared-token mode must not emit verified runtime coverage")
	}
}

func TestRuntimeCoverageIDDeterministic(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	c := collection.RuntimeCoverage{
		Version: 1, ProducerID: "p", SourceKind: "file", Status: "complete",
		WindowStart: now.Add(-time.Second), WindowEnd: now,
	}
	if a, b := runtimeCoverageID(c), runtimeCoverageID(c); a != b || len(a) != 64 {
		t.Fatalf("coverage id not deterministic: %q %q", a, b)
	}
}
