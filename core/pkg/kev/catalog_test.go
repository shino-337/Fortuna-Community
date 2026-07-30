package kev

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCatalog_Refresh(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"vulnerabilities": []map[string]string{
				{"cveID": "CVE-2024-6387"},
				{"cveID": "CVE-2023-44487"},
			},
		})
	}))
	defer ts.Close()

	c := &Catalog{
		set:    make(map[string]struct{}),
		url:    ts.URL,
		client: ts.Client(),
		ttl:    0,
	}
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.set) != 2 {
		t.Fatalf("set size %d", len(c.set))
	}
	if _, ok := c.set["CVE-2024-6387"]; !ok {
		t.Fatal("missing CVE")
	}
}

func TestCatalog_Refresh_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := &Catalog{
		set:    make(map[string]struct{}),
		url:    ts.URL,
		client: ts.Client(),
	}
	if err := c.Refresh(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
