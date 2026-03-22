package epss

import (
	"context"
	"fmt"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestClient_LookupMany_ConcurrencyLimit(t *testing.T) {
	var inflight atomic.Int32
	var peak atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := inflight.Add(1)
		for {
			old := peak.Load()
			if cur <= old {
				break
			}
			if peak.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		inflight.Add(-1)
		cve := r.URL.Query().Get("cve")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"OK","data":[{"cve":"` + cve + `","epss":"0.1","percentile":"0.2"}]}`))
	}))
	defer ts.Close()

	c := NewClient()
	c.baseURL = ts.URL
	c.cache = make(map[string]cacheEntry)
	c.ttl = time.Hour

	const wantConc = 2
	ids := make([]string, 6)
	for i := range ids {
		ids[i] = fmt.Sprintf("CVE-2024-%d", 2000+i)
	}

	out := c.LookupMany(context.Background(), ids, wantConc)
	if len(out) != 6 {
		t.Fatalf("expected 6 results, got %d", len(out))
	}
	if p := peak.Load(); p > int32(wantConc) {
		t.Fatalf("peak concurrent requests %d > limit %d", p, wantConc)
	}
}

func TestClient_LookupMany_DedupesCVEs(t *testing.T) {
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		cve := r.URL.Query().Get("cve")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"OK","data":[{"cve":"` + cve + `","epss":"0.5","percentile":"0.9"}]}`))
	}))
	defer ts.Close()

	c := NewClient()
	c.baseURL = ts.URL
	c.cache = make(map[string]cacheEntry)
	c.ttl = time.Hour

	out := c.LookupMany(context.Background(), []string{"CVE-2024-1", "cve-2024-1", "CVE-2024-1"}, 4)
	if len(out) != 1 {
		t.Fatalf("expected 1 unique, got %d %+v", len(out), out)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 HTTP call, got %d", calls.Load())
	}
}
