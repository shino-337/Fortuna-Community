package middleware

import (
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// Contract tests for DefaultRateLimiterConfig + golang.org/x/time/rate (Sprint 1 B2 acceptance: burst / sustained behavior).

func TestDefaultRateLimiterConfig_BurstExhaustion(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	l := rate.NewLimiter(rate.Limit(cfg.RequestsPerSecond), cfg.BurstSize)
	for i := 0; i < cfg.BurstSize; i++ {
		if !l.Allow() {
			t.Fatalf("request %d within burst should be allowed", i)
		}
	}
	if l.Allow() {
		t.Fatal("expected rejection immediately after burst is exhausted")
	}
}

func TestDefaultRateLimiterConfig_SustainedRefill(t *testing.T) {
	// Low RPS so a short sleep reliably adds tokens without flaky CI.
	const rps = 20
	const burst = 3
	l := rate.NewLimiter(rate.Limit(rps), burst)
	for i := 0; i < burst; i++ {
		_ = l.Allow()
	}
	if l.Allow() {
		t.Fatal("burst should be empty")
	}
	// ~2 tokens at 20/s over 150ms
	time.Sleep(150 * time.Millisecond)
	allowed := 0
	for i := 0; i < 5; i++ {
		if l.Allow() {
			allowed++
		}
	}
	if allowed < 1 {
		t.Fatalf("expected at least 1 token after refill window, got %d", allowed)
	}
}
