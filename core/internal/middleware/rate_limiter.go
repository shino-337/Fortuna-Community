package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiterConfig holds rate limiting configuration
type RateLimiterConfig struct {
	RequestsPerSecond float64
	BurstSize         int
	Enabled           bool
}

// DefaultRateLimiterConfig returns default rate limiter configuration
func DefaultRateLimiterConfig() *RateLimiterConfig {
	return &RateLimiterConfig{
		RequestsPerSecond: 100.0, // 100 requests per second
		BurstSize:         200,    // Allow burst of 200 requests
		Enabled:           true,
	}
}

// rateLimiterStore stores rate limiters per IP/client
type rateLimiterStore struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	config   *RateLimiterConfig
	cleanup  *time.Ticker
}

var (
	globalStore *rateLimiterStore
	once        sync.Once
)

// getRateLimiterStore returns the global rate limiter store
func getRateLimiterStore(config *RateLimiterConfig) *rateLimiterStore {
	once.Do(func() {
		globalStore = &rateLimiterStore{
			limiters: make(map[string]*rate.Limiter),
			config:   config,
			cleanup:  time.NewTicker(5 * time.Minute), // Cleanup every 5 minutes
		}
		go globalStore.cleanupExpired()
	})
	return globalStore
}

// cleanupExpired removes old limiters to prevent memory leaks
func (rls *rateLimiterStore) cleanupExpired() {
	for range rls.cleanup.C {
		rls.mu.Lock()
		// Simple cleanup: if we have too many limiters, remove some
		// In production, use LRU cache or time-based expiration
		if len(rls.limiters) > 10000 {
			// Remove half of the limiters (simple strategy)
			count := 0
			for key := range rls.limiters {
				if count >= len(rls.limiters)/2 {
					break
				}
				delete(rls.limiters, key)
				count++
			}
		}
		rls.mu.Unlock()
	}
}

// getLimiter returns or creates a rate limiter for the given key
func (rls *rateLimiterStore) getLimiter(key string) *rate.Limiter {
	rls.mu.RLock()
	limiter, exists := rls.limiters[key]
	rls.mu.RUnlock()

	if exists {
		return limiter
	}

	// Create new limiter
	rls.mu.Lock()
	defer rls.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists := rls.limiters[key]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(
		rate.Limit(rls.config.RequestsPerSecond),
		rls.config.BurstSize,
	)
	rls.limiters[key] = limiter

	return limiter
}

// getClientKey extracts a unique key for rate limiting (IP address or user ID)
func getClientKey(c *gin.Context) string {
	// Try to get user ID from context (if authenticated)
	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(string); ok && id != "" {
			return fmt.Sprintf("user:%s", id)
		}
	}

	// Fallback to IP address
	ip := c.ClientIP()
	if ip == "" {
		ip = c.RemoteIP()
	}
	if ip == "" {
		ip = "unknown"
	}

	return fmt.Sprintf("ip:%s", ip)
}

// RateLimiterMiddleware creates a rate limiting middleware
func RateLimiterMiddleware(config *RateLimiterConfig) gin.HandlerFunc {
	if config == nil {
		config = DefaultRateLimiterConfig()
	}

	if !config.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	store := getRateLimiterStore(config)

	return func(c *gin.Context) {
		key := getClientKey(c)
		limiter := store.getLimiter(key)

		// Check if request is allowed
		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded",
				"message": fmt.Sprintf("Too many requests. Limit: %.0f req/s, Burst: %d", config.RequestsPerSecond, config.BurstSize),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// PerEndpointRateLimiter creates rate limiters for specific endpoints
type PerEndpointRateLimiter struct {
	endpoints map[string]*RateLimiterConfig
	defaultConfig *RateLimiterConfig
}

// NewPerEndpointRateLimiter creates a new per-endpoint rate limiter
func NewPerEndpointRateLimiter(defaultConfig *RateLimiterConfig) *PerEndpointRateLimiter {
	if defaultConfig == nil {
		defaultConfig = DefaultRateLimiterConfig()
	}

	return &PerEndpointRateLimiter{
		endpoints:     make(map[string]*RateLimiterConfig),
		defaultConfig: defaultConfig,
	}
}

// SetEndpointLimit sets rate limit for a specific endpoint
func (p *PerEndpointRateLimiter) SetEndpointLimit(path string, config *RateLimiterConfig) {
	p.endpoints[path] = config
}

// Middleware returns the rate limiter middleware
func (p *PerEndpointRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.FullPath()
		config := p.defaultConfig

		// Check if there's a specific limit for this endpoint
		if endpointConfig, exists := p.endpoints[path]; exists {
			config = endpointConfig
		}

		if !config.Enabled {
			c.Next()
			return
		}

		key := fmt.Sprintf("%s:%s", path, getClientKey(c))
		store := getRateLimiterStore(config)
		limiter := store.getLimiter(key)

		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded",
				"message": fmt.Sprintf("Too many requests to %s. Limit: %.0f req/s", path, config.RequestsPerSecond),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

