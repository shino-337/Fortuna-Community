package api

import (
	"strconv"
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

// MemoryRisksCache is an in-memory TTL cache for GET /risks and GET /insights/summary.
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

// BuildRisksListCacheKey builds cache key for GET /risks from query params. withScores: 0 or 1; priorityLevel, resourceNamespace, insightType, scoreBin optional.
func BuildRisksListCacheKey(clusterID, status, severity, search, priorityLevel, resourceNamespace, insightType string, sinceMinutes, page, pageSize, withScores, scoreBin int) string {
	if clusterID == "" {
		clusterID = "_"
	}
	if status == "" {
		status = "active"
	}
	if severity == "" {
		severity = "_"
	}
	if search == "" {
		search = "_"
	}
	if priorityLevel == "" {
		priorityLevel = "_"
	}
	if resourceNamespace == "" {
		resourceNamespace = "_"
	}
	if insightType == "" {
		insightType = "_"
	}
	scoreSuffix := "0"
	if withScores != 0 {
		scoreSuffix = "1"
	}
	// scoreBin uses strconv.Itoa so 0 is "0" (itoa(0) is "1" for legacy page/since)
	return "risks:list:" + clusterID + ":" + resourceNamespace + ":" + insightType + ":" + itoa(sinceMinutes) + ":" + status + ":" + severity + ":" + search + ":" + priorityLevel + ":" + itoa(page) + ":" + itoa(pageSize) + ":" + scoreSuffix + ":" + strconv.Itoa(scoreBin)
}

func itoa(i int) string {
	if i <= 0 {
		return "1"
	}
	b := []byte{}
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// BuildInsightsSummaryCacheKey builds cache key for GET /insights/summary.
func BuildInsightsSummaryCacheKey(clusterID string, sinceMinutes int) string {
	if clusterID == "" {
		clusterID = "_"
	}
	return "insights:summary:" + clusterID + ":" + strconv.Itoa(sinceMinutes)
}

// BuildInsightsSummaryGlobalCacheKey builds cache key for GET /insights/summary/global.
func BuildInsightsSummaryGlobalCacheKey(sinceMinutes int) string {
	return "insights:summary:global:" + strconv.Itoa(sinceMinutes)
}

// BuildRiskHistogramCacheKey builds cache key for GET /risk/histogram (clusterId + sinceMinutes).
func BuildRiskHistogramCacheKey(clusterID string, sinceMinutes int) string {
	if clusterID == "" {
		clusterID = "_"
	}
	return "risk:histogram:" + clusterID + ":" + strconv.Itoa(sinceMinutes)
}

// --- unit test helpers (used by risks_cache_test.go) ---

func (c *MemoryRisksCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.store)
}
