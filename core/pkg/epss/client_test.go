package epss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_Lookup(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cve") != "CVE-2024-6387" {
			http.Error(w, "bad cve", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"OK","data":[{"cve":"CVE-2024-6387","epss":"0.5","percentile":"0.9"}]}`))
	}))
	defer ts.Close()

	c := NewClient()
	c.baseURL = ts.URL
	c.cache = make(map[string]cacheEntry)
	c.ttl = time.Hour

	e, p, ok := c.Lookup(context.Background(), "CVE-2024-6387")
	if !ok {
		t.Fatal("expected ok")
	}
	if e != 0.5 || p != 0.9 {
		t.Fatalf("got epss=%v perc=%v", e, p)
	}

	// cache hit path
	e2, p2, ok2 := c.Lookup(context.Background(), "CVE-2024-6387")
	if !ok2 || e2 != e || p2 != p {
		t.Fatalf("cache miss or mismatch: ok=%v e=%v p=%v", ok2, e2, p2)
	}
}

func TestEvidenceJSON(t *testing.T) {
	s := EvidenceJSON(0.12, 0.88)
	if s == "" || !strings.Contains(s, "0.12") {
		t.Fatalf("unexpected json: %q", s)
	}
}
