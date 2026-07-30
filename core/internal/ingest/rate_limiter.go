package ingest

import (
	"context"
	"log"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter manages rate limiting for agents
type RateLimiter struct {
	limiters map[string]*rate.Limiter // agentID -> limiter
	mu       sync.RWMutex
	config   RateLimitConfig
}

// RateLimitConfig configures rate limiting behavior
type RateLimitConfig struct {
	// Per-agent limits
	RequestsPerSecond float64 // Requests per second per agent
	BurstSize         int     // Burst size per agent

	// Global limits
	GlobalRPS       float64 // Global requests per second
	GlobalBurstSize int     // Global burst size

	// Cleanup
	CleanupInterval time.Duration // Interval to clean up unused limiters
	IdleTimeout     time.Duration // Timeout before removing idle limiter
}

// DefaultRateLimitConfig returns default rate limit configuration
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerSecond: 100.0,  // 100 requests per second per agent
		BurstSize:         200,    // Allow burst of 200 requests
		GlobalRPS:         1000.0, // 1000 requests per second globally
		GlobalBurstSize:   2000,   // Global burst of 2000 requests
		CleanupInterval:   5 * time.Minute,
		IdleTimeout:       30 * time.Minute,
	}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		config:   config,
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// getLimiter gets or creates a limiter for an agent
func (rl *RateLimiter) getLimiter(agentID string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[agentID]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	// Create new limiter
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists := rl.limiters[agentID]; exists {
		return limiter
	}

	// Create new limiter for this agent
	limiter = rate.NewLimiter(rate.Limit(rl.config.RequestsPerSecond), rl.config.BurstSize)
	rl.limiters[agentID] = limiter

	log.Printf("[RateLimiter] Created limiter for agent: %s (RPS: %.2f, Burst: %d)",
		agentID, rl.config.RequestsPerSecond, rl.config.BurstSize)

	return limiter
}

// Allow checks if a request from an agent is allowed
func (rl *RateLimiter) Allow(agentID string) bool {
	limiter := rl.getLimiter(agentID)
	return limiter.Allow()
}

// Wait waits until a request from an agent is allowed
func (rl *RateLimiter) Wait(ctx context.Context, agentID string) error {
	limiter := rl.getLimiter(agentID)
	return limiter.Wait(ctx)
}

// Reserve reserves a token for an agent
func (rl *RateLimiter) Reserve(agentID string) *rate.Reservation {
	limiter := rl.getLimiter(agentID)
	return limiter.Reserve()
}

// cleanup removes idle limiters periodically
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		_ = time.Now() // Reserved for future use
		toDelete := make([]string, 0)

		// Note: We can't track last access time without modifying limiter
		// For now, we'll keep all limiters (they're lightweight)
		// In production, you might want to track last access time

		// Clean up if we have too many limiters (simple heuristic)
		if len(rl.limiters) > 1000 {
			log.Printf("[RateLimiter] Warning: Too many limiters (%d), consider cleanup", len(rl.limiters))
		}

		rl.mu.Unlock()

		if len(toDelete) > 0 {
			log.Printf("[RateLimiter] Cleaned up %d idle limiters", len(toDelete))
		}
	}
}

// GetStats returns rate limiter statistics
func (rl *RateLimiter) GetStats() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return map[string]interface{}{
		"active_agents":   len(rl.limiters),
		"rps_per_agent":   rl.config.RequestsPerSecond,
		"burst_per_agent": rl.config.BurstSize,
		"global_rps":      rl.config.GlobalRPS,
		"global_burst":    rl.config.GlobalBurstSize,
	}
}

// GlobalRateLimiter manages global rate limiting
type GlobalRateLimiter struct {
	limiter *rate.Limiter
	config  RateLimitConfig
}

// NewGlobalRateLimiter creates a new global rate limiter
func NewGlobalRateLimiter(config RateLimitConfig) *GlobalRateLimiter {
	return &GlobalRateLimiter{
		limiter: rate.NewLimiter(rate.Limit(config.GlobalRPS), config.GlobalBurstSize),
		config:  config,
	}
}

// Allow checks if a global request is allowed
func (grl *GlobalRateLimiter) Allow() bool {
	return grl.limiter.Allow()
}

// Wait waits until a global request is allowed
func (grl *GlobalRateLimiter) Wait(ctx context.Context) error {
	return grl.limiter.Wait(ctx)
}
