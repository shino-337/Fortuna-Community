// Package ingest provides per-cluster rate limiting for sync and SBOM ingest (Finding #6).
package ingest

import (
	"log"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ClusterLimitConfig holds per-cluster rate limits for sync and SBOM.
type ClusterLimitConfig struct {
	SyncRPS   float64
	SyncBurst int
	SBOMRPS   float64
	SBOMBurst int
	Enabled   bool
}

// DefaultClusterLimitConfig returns defaults for per-cluster limits.
func DefaultClusterLimitConfig() ClusterLimitConfig {
	return ClusterLimitConfig{
		SyncRPS:   10.0,  // 10 sync requests per second per cluster
		SyncBurst: 20,    // burst 20
		SBOMRPS:   50.0,  // 50 SBOM findings per second per cluster
		SBOMBurst: 100,   // burst 100
		Enabled:   true,
	}
}

// ClusterRateLimiter limits requests per cluster_id (sync and SBOM separately).
type ClusterRateLimiter struct {
	mu       sync.RWMutex
	syncMap  map[string]*rate.Limiter
	sbomMap  map[string]*rate.Limiter
	config   ClusterLimitConfig
	cleanup  *time.Ticker
}

// NewClusterRateLimiter creates a per-cluster rate limiter.
func NewClusterRateLimiter(config ClusterLimitConfig) *ClusterRateLimiter {
	rl := &ClusterRateLimiter{
		syncMap: make(map[string]*rate.Limiter),
		sbomMap: make(map[string]*rate.Limiter),
		config:  config,
		cleanup: time.NewTicker(10 * time.Minute),
	}
	go rl.cleanupIdle()
	return rl
}

func (c *ClusterRateLimiter) getSyncLimiter(clusterID string) *rate.Limiter {
	if clusterID == "" {
		clusterID = "unknown"
	}
	c.mu.RLock()
	l, ok := c.syncMap[clusterID]
	c.mu.RUnlock()
	if ok {
		return l
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if l, ok = c.syncMap[clusterID]; ok {
		return l
	}
	l = rate.NewLimiter(rate.Limit(c.config.SyncRPS), c.config.SyncBurst)
	c.syncMap[clusterID] = l
	return l
}

func (c *ClusterRateLimiter) getSBOMLimiter(clusterID string) *rate.Limiter {
	if clusterID == "" {
		clusterID = "unknown"
	}
	c.mu.RLock()
	l, ok := c.sbomMap[clusterID]
	c.mu.RUnlock()
	if ok {
		return l
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if l, ok = c.sbomMap[clusterID]; ok {
		return l
	}
	l = rate.NewLimiter(rate.Limit(c.config.SBOMRPS), c.config.SBOMBurst)
	c.sbomMap[clusterID] = l
	return l
}

// AllowSync returns true if sync request for clusterID is allowed.
func (c *ClusterRateLimiter) AllowSync(clusterID string) bool {
	if !c.config.Enabled {
		return true
	}
	return c.getSyncLimiter(clusterID).Allow()
}

// AllowSBOM returns true if SBOM request for clusterID is allowed.
func (c *ClusterRateLimiter) AllowSBOM(clusterID string) bool {
	if !c.config.Enabled {
		return true
	}
	return c.getSBOMLimiter(clusterID).Allow()
}

func (c *ClusterRateLimiter) cleanupIdle() {
	for range c.cleanup.C {
		c.mu.Lock()
		if len(c.syncMap)+len(c.sbomMap) > 2000 {
			log.Printf("[ClusterRateLimiter] Many limiters (sync=%d sbom=%d); consider tuning", len(c.syncMap), len(c.sbomMap))
		}
		c.mu.Unlock()
	}
}
