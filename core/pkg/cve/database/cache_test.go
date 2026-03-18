package database

import (
	"testing"
	"time"

	"github.com/fortuna/core/pkg/cve"
)

func TestCVECache_HitAndExpiry(t *testing.T) {
	cache := NewCVECache()

	key := "go:stdlib:*"
	now := time.Now()
	val := []*cve.CVE{{ID: "GO-STDLIB-TEST", Published: now}}

	// Store and get immediately: should hit
	cache.Set(key, val)
	got, ok := cache.Get(key)
	if !ok {
		t.Fatal("expected cache hit immediately after Set")
	}
	if len(got) != 1 || got[0].ID != "GO-STDLIB-TEST" {
		t.Fatalf("unexpected cached value: %+v", got)
	}

	// Force expiry by manipulating internal ttl to negative and re-setting
	cache.ttl = -1 * time.Second
	cache.Set(key, val)
	if _, ok := cache.Get(key); !ok {
		// ok: entry is expired and removed
		return
	}
	t.Fatal("expected cache entry to be expired after negative TTL")
}

