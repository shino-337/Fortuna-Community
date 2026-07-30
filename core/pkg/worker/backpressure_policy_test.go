package worker

import (
	"os"
	"testing"
)

// TestBackpressurePolicyFromEnv verifies that FORTUNA_BACKPRESSURE_POLICY is
// correctly parsed and that unknown values fall back to PolicyRetry.
func TestBackpressurePolicyFromEnv(t *testing.T) {
	tests := []struct {
		envValue string
		want     BackpressurePolicy
	}{
		{"drop", PolicyDrop},
		{"DROP", PolicyDrop},
		{"retry", PolicyRetry},
		{"defer", PolicyDefer},
		{"block", PolicyBlock},
		{"", PolicyRetry},
		{"unknown", PolicyRetry},
		{"  retry  ", PolicyRetry},
	}

	for _, tc := range tests {
		t.Run("env="+tc.envValue, func(t *testing.T) {
			os.Setenv("FORTUNA_BACKPRESSURE_POLICY", tc.envValue)
			defer os.Unsetenv("FORTUNA_BACKPRESSURE_POLICY")
			got := BackpressurePolicyFromEnv()
			if got != tc.want {
				t.Errorf("BackpressurePolicyFromEnv(%q) = %q, want %q", tc.envValue, got, tc.want)
			}
		})
	}
}

// TestWorkerLoadTracker_TryAcquireRelease verifies basic slot accounting.
func TestWorkerLoadTracker_TryAcquireRelease(t *testing.T) {
	tracker := NewWorkerLoadTracker("test-worker", 2)

	if !tracker.TryAcquire() {
		t.Fatal("first acquire should succeed")
	}
	if !tracker.TryAcquire() {
		t.Fatal("second acquire should succeed")
	}
	if tracker.TryAcquire() {
		t.Fatal("third acquire should fail (at capacity)")
	}
	if !tracker.IsBackpressured() {
		t.Fatal("should be backpressured at capacity")
	}
	tracker.Release()
	if tracker.IsBackpressured() {
		// After release load = 1, max = 2, so not at capacity anymore
		t.Fatal("should not be backpressured after release")
	}
	if tracker.GetCurrentLoad() != 1 {
		t.Errorf("expected load=1, got %d", tracker.GetCurrentLoad())
	}
	tracker.Release()
	if tracker.GetCurrentLoad() != 0 {
		t.Errorf("expected load=0, got %d", tracker.GetCurrentLoad())
	}
}

// TestWorkerLoadTracker_SetPolicy verifies that SetPolicy updates the policy.
func TestWorkerLoadTracker_SetPolicy(t *testing.T) {
	tracker := NewWorkerLoadTracker("test-worker", 10)
	tracker.SetPolicy(PolicyDrop)
	if tracker.policy != PolicyDrop {
		t.Errorf("expected policy=drop, got %s", tracker.policy)
	}
	tracker.SetPolicy(PolicyBlock)
	if tracker.policy != PolicyBlock {
		t.Errorf("expected policy=block, got %s", tracker.policy)
	}
}

// TestDefaultBackpressureConfig verifies defaults.
func TestDefaultBackpressureConfig(t *testing.T) {
	os.Unsetenv("FORTUNA_BACKPRESSURE_POLICY")
	cfg := DefaultBackpressureConfig()
	if cfg.MaxConcurrent != 100 {
		t.Errorf("expected MaxConcurrent=100, got %d", cfg.MaxConcurrent)
	}
	if cfg.Threshold != 0.8 {
		t.Errorf("expected Threshold=0.8, got %f", cfg.Threshold)
	}
	if cfg.Policy != PolicyRetry {
		t.Errorf("expected default policy=retry, got %s", cfg.Policy)
	}
}
