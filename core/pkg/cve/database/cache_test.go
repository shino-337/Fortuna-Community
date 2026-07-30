package database

import (
	"fmt"
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

func TestCVECache_Clear(t *testing.T) {
	cache := NewCVECache()
	cache.Set("k", []*cve.CVE{{ID: "CVE-1"}})
	if _, ok := cache.Get("k"); !ok {
		t.Fatal("expected hit before Clear")
	}
	cache.Clear()
	if _, ok := cache.Get("k"); ok {
		t.Fatal("expected miss after Clear")
	}
}

func TestCVECache_MaxEntriesEviction(t *testing.T) {
	c := &CVECache{
		cache:      make(map[string]cveCacheEntry),
		ttl:        time.Hour,
		maxEntries: 3,
	}
	for i := 1; i <= 4; i++ {
		c.Set(fmt.Sprintf("k%d", i), []*cve.CVE{{ID: fmt.Sprintf("CVE-%d", i)}})
	}
	if len(c.cache) != 3 {
		t.Fatalf("expected exactly 3 entries after eviction, got %d keys %v", len(c.cache), keysOf(c.cache))
	}
	if _, ok := c.Get("k4"); !ok {
		t.Fatal("expected most recent key k4 to remain")
	}
}

func keysOf(m map[string]cveCacheEntry) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

