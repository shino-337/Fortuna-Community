package api

import (
	"strings"
	"sync"
	"time"
)

// defaultRisksCache is set by SetupRoutesWithCertManager; used by GetInsightsListCached and GetInsightsSummaryCached.
var defaultRisksCache RisksCache

var risksCacheTTL = 60 * time.Second

// RisksCache is a TTL cache for risk list and insights summary responses (in-memory when Redis not configured).
type RisksCache interface {
	Get(key string) ([]byte, bool)
	Set(key string, value []byte, ttl time.Duration)
	// ClearByPrefix removes all keys that have the given prefix (e.g. "risks:list:" or "insights:summary:").
	ClearByPrefix(prefix string)
}

type cacheEntry struct {
	body []byte
	exp  time.Time
}

// MemoryRisksCache is an in-memory TTL cache for GET /risk/insights (list) and GET /risk/insights/summary.
type MemoryRisksCache struct {
	mu    sync.RWMutex
	store map[string]cacheEntry
	ttl   time.Duration
}

// NewMemoryRisksCache creates an in-memory cache with default TTL (e.g. 60s).
func NewMemoryRisksCache(defaultTTL time.Duration) *MemoryRisksCache {
	if defaultTTL <= 0 {
		defaultTTL = 60 * time.Second
	}
	c := &MemoryRisksCache{store: make(map[string]cacheEntry), ttl: defaultTTL}
	go c.cleanup()
	return c
}

func (c *MemoryRisksCache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	e, ok := c.store[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.exp) {
		return nil, false
	}
	return e.body, true
}

func (c *MemoryRisksCache) Set(key string, value []byte, ttl time.Duration) {
	if ttl <= 0 {
		ttl = c.ttl
	}
	c.mu.Lock()
	c.store[key] = cacheEntry{body: value, exp: time.Now().Add(ttl)}
	c.mu.Unlock()
}

// ClearByPrefix removes all cache entries whose key has the given prefix.
func (c *MemoryRisksCache) ClearByPrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.store {
		if strings.HasPrefix(k, prefix) {
			delete(c.store, k)
		}
	}
}

func (c *MemoryRisksCache) cleanup() {
	tick := time.NewTicker(2 * time.Minute)
	defer tick.Stop()
	for range tick.C {
		c.mu.Lock()
		now := time.Now()
		for k, e := range c.store {
			if now.After(e.exp) {
				delete(c.store, k)
			}
		}
		c.mu.Unlock()
	}
}

// BuildRisksListCacheKey builds cache key for GET /risks from query params. withScores: 0 or 1; finalLevel, resourceNamespace, insightType, scoreBin optional. view: instance|group|_.
func BuildRisksListCacheKey(clusterID, status, severity, search, finalLevel, resourceNamespace, insightType string, sinceMinutes, page, pageSize, withScores, scoreBin int, view string) string {
	if status == "" {
		status = "active"
	}
	return encodedCacheKey("risks:list:", []interface{}{clusterID, status, severity, search, finalLevel, resourceNamespace, insightType, sinceMinutes, page, pageSize, withScores, scoreBin, strings.ToLower(strings.TrimSpace(view))})
}

// BuildInsightsSummaryCacheKey builds cache key for GET /insights/summary.
func BuildInsightsSummaryCacheKey(clusterID string, sinceMinutes int) string {
	return encodedCacheKey("insights:summary:", []interface{}{"selection", clusterID, sinceMinutes})
}
func BuildInsightsSummaryGlobalCacheKey(sinceMinutes int) string {
	return encodedCacheKey("insights:summary:", []interface{}{"global", sinceMinutes})
}
func BuildRiskHistogramCacheKey(clusterID string, sinceMinutes int) string {
	return encodedCacheKey("risk:histogram:", []interface{}{clusterID, sinceMinutes})
}

// --- unit test helpers (used by risks_cache_test.go) ---

func (c *MemoryRisksCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.store)
}
