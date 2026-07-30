package extractor

import (
	"sync"
	"time"
)

type SyftResultCache struct {
	mu        sync.RWMutex
	items     map[string]syftCacheEntry
	ttl       time.Duration
	maxItems  int
	now       func() time.Time
}

type syftCacheEntry struct {
	pkgs   []Package
	expiry time.Time
}

func NewSyftResultCache(ttl time.Duration, maxItems int) *SyftResultCache {
	if ttl <= 0 || maxItems <= 0 {
		return nil
	}
	return &SyftResultCache{
		items:    make(map[string]syftCacheEntry),
		ttl:      ttl,
		maxItems: maxItems,
		now:      time.Now,
	}
}

func (c *SyftResultCache) Get(key string) []Package {
	if c == nil || key == "" {
		return nil
	}
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil
	}
	if c.now().After(e.expiry) {
		c.mu.Lock()
		// re-check under write lock
		if e2, ok2 := c.items[key]; ok2 && c.now().After(e2.expiry) {
			delete(c.items, key)
		}
		c.mu.Unlock()
		return nil
	}
	return e.pkgs
}

func (c *SyftResultCache) Set(key string, pkgs []Package) {
	if c == nil || key == "" || len(pkgs) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	// Simple eviction: if we exceed capacity, drop an arbitrary entry.
	// (Good enough for SBOM agent use; avoids pulling in an LRU implementation.)
	if c.maxItems > 0 && len(c.items) >= c.maxItems {
		for k := range c.items {
			delete(c.items, k)
			break
		}
	}

	c.items[key] = syftCacheEntry{
		pkgs:   pkgs,
		expiry: c.now().Add(c.ttl),
	}
}

