package retry

import (
	"context"
	"fmt"
	"math"
	"time"
)

// Config holds retry configuration
type Config struct {
	MaxRetries  int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
}

// DefaultConfig returns default retry configuration
func DefaultConfig() *Config {
	return &Config{
		MaxRetries:  5,
		InitialWait: 1 * time.Second,
		MaxWait:     30 * time.Second,
		Multiplier:  2.0,
	}
}

// Retry executes a function with exponential backoff retry logic
func Retry(ctx context.Context, cfg *Config, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute function
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't wait after last attempt
		if attempt < cfg.MaxRetries {
			wait := calculateWait(cfg, attempt)
			
			// Wait with context cancellation support
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
				// Continue to next attempt
			}
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// calculateWait calculates wait time with exponential backoff
func calculateWait(cfg *Config, attempt int) time.Duration {
	wait := float64(cfg.InitialWait) * math.Pow(cfg.Multiplier, float64(attempt))
	
	if wait > float64(cfg.MaxWait) {
		wait = float64(cfg.MaxWait)
	}

	return time.Duration(wait)
}

// WithRetry wraps a function with retry logic
func WithRetry(cfg *Config, fn func() error) error {
	return Retry(context.Background(), cfg, fn)
}

