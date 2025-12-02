package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"gorm.io/gorm"
)

// ErrorType classifies errors for retry logic
type ErrorType int

const (
	// ErrorTypeRetryable indicates the error is transient and should be retried
	ErrorTypeRetryable ErrorType = iota
	// ErrorTypeNonRetryable indicates the error is permanent and should not be retried
	ErrorTypeNonRetryable
	// ErrorTypeFatal indicates the error is fatal and should crash the worker
	ErrorTypeFatal
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxAttempts  int           // Maximum number of retry attempts
	InitialDelay time.Duration // Initial delay before first retry
	MaxDelay     time.Duration // Maximum delay between retries
	Multiplier   float64       // Exponential backoff multiplier
	Jitter       bool          // Whether to add jitter to delays
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 1 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
}

// ClassifyError classifies an error to determine if it should be retried
func ClassifyError(err error) ErrorType {
	if err == nil {
		return ErrorTypeNonRetryable // No error, no retry needed
	}

	errStr := err.Error()

	// Database connection errors - retryable
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Record not found is usually non-retryable (data issue)
		return ErrorTypeNonRetryable
	}

	// Check for database connection errors
	if errors.Is(err, gorm.ErrInvalidDB) ||
		errors.Is(err, gorm.ErrInvalidTransaction) ||
		errors.Is(err, gorm.ErrNotImplemented) ||
		errors.Is(err, gorm.ErrMissingWhereClause) {
		// These are usually retryable
		return ErrorTypeRetryable
	}

	// Network/timeout errors - retryable
	if containsAny(errStr, []string{
		"connection refused",
		"connection reset",
		"timeout",
		"network",
		"temporary failure",
		"deadline exceeded",
		"context deadline exceeded",
	}) {
		return ErrorTypeRetryable
	}

	// Database errors - retryable
	if containsAny(errStr, []string{
		"database is locked",
		"too many connections",
		"connection pool",
		"server closed",
		"broken pipe",
	}) {
		return ErrorTypeRetryable
	}

	// Data validation errors - non-retryable
	if containsAny(errStr, []string{
		"invalid data format",
		"schema validation",
		"json unmarshal",
		"invalid json",
		"malformed",
	}) {
		return ErrorTypeNonRetryable
	}

	// Fatal errors - should crash worker
	if containsAny(errStr, []string{
		"out of memory",
		"panic",
		"fatal",
	}) {
		return ErrorTypeFatal
	}

	// NATS connection errors - retryable
	if containsAny(errStr, []string{
		"nats connection",
		"stream not found",
		"consumer not found",
	}) {
		return ErrorTypeRetryable
	}

	// Default to retryable for unknown errors (safer)
	return ErrorTypeRetryable
}

// containsAny checks if string contains any of the substrings
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// RetryWithBackoff retries a function with exponential backoff
func RetryWithBackoff(ctx context.Context, config RetryConfig, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Execute the function
		err := fn()
		if err == nil {
			// Success
			if attempt > 0 {
				log.Printf("[Retry] Succeeded after %d attempts", attempt+1)
			}
			return nil
		}

		lastErr = err

		// Classify error
		errorType := ClassifyError(err)
		if errorType == ErrorTypeNonRetryable {
			log.Printf("[Retry] Non-retryable error after %d attempts: %v", attempt+1, err)
			return fmt.Errorf("non-retryable error: %w", err)
		}
		if errorType == ErrorTypeFatal {
			log.Printf("[Retry] Fatal error: %v", err)
			return fmt.Errorf("fatal error: %w", err)
		}

		// Don't retry on last attempt
		if attempt == config.MaxAttempts-1 {
			log.Printf("[Retry] Max attempts (%d) reached, giving up: %v", config.MaxAttempts, err)
			return fmt.Errorf("max retry attempts exceeded: %w", err)
		}

		// Calculate delay with exponential backoff
		delay := time.Duration(float64(config.InitialDelay) * math.Pow(config.Multiplier, float64(attempt)))
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}

		// Add jitter if enabled
		if config.Jitter {
			// Add ±10% jitter
			jitter := time.Duration(float64(delay) * 0.1)
			delay = delay + time.Duration((float64(jitter) * (0.5 - 0.1*float64(attempt%2)))) // Simple jitter
		}

		log.Printf("[Retry] Attempt %d/%d failed: %v. Retrying in %v...", attempt+1, config.MaxAttempts, err, delay)

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return fmt.Errorf("retry exhausted: %w", lastErr)
}

// RetryableError wraps an error to mark it as retryable
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	return fmt.Sprintf("retryable error: %v", e.Err)
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// NonRetryableError wraps an error to mark it as non-retryable
type NonRetryableError struct {
	Err error
}

func (e *NonRetryableError) Error() string {
	return fmt.Sprintf("non-retryable error: %v", e.Err)
}

func (e *NonRetryableError) Unwrap() error {
	return e.Err
}


