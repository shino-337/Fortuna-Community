package scheduler

import (
	"os"
	"testing"
)

func TestGetResolvedRetentionDays(t *testing.T) {
	// Restore env after test
	defer func() {
		_ = os.Unsetenv("INSIGHTS_RESOLVED_RETENTION_DAYS")
	}()

	t.Run("default when unset", func(t *testing.T) {
		_ = os.Unsetenv("INSIGHTS_RESOLVED_RETENTION_DAYS")
		got := getResolvedRetentionDays()
		if got != defaultResolvedRetentionDays {
			t.Errorf("getResolvedRetentionDays() = %d, want %d", got, defaultResolvedRetentionDays)
		}
	})

	t.Run("from env valid", func(t *testing.T) {
		os.Setenv("INSIGHTS_RESOLVED_RETENTION_DAYS", "14")
		defer os.Unsetenv("INSIGHTS_RESOLVED_RETENTION_DAYS")
		got := getResolvedRetentionDays()
		if got != 14 {
			t.Errorf("getResolvedRetentionDays() = %d, want 14", got)
		}
	})

	t.Run("from env invalid falls back to default", func(t *testing.T) {
		os.Setenv("INSIGHTS_RESOLVED_RETENTION_DAYS", "x")
		defer os.Unsetenv("INSIGHTS_RESOLVED_RETENTION_DAYS")
		got := getResolvedRetentionDays()
		if got != defaultResolvedRetentionDays {
			t.Errorf("getResolvedRetentionDays() = %d, want default %d", got, defaultResolvedRetentionDays)
		}
	})

	t.Run("zero or negative ignored", func(t *testing.T) {
		os.Setenv("INSIGHTS_RESOLVED_RETENTION_DAYS", "0")
		defer os.Unsetenv("INSIGHTS_RESOLVED_RETENTION_DAYS")
		got := getResolvedRetentionDays()
		if got != defaultResolvedRetentionDays {
			t.Errorf("getResolvedRetentionDays() with 0 = %d, want default %d", got, defaultResolvedRetentionDays)
		}
	})
}

func TestGetActiveRetentionDays(t *testing.T) {
	defer func() {
		_ = os.Unsetenv("INSIGHTS_ACTIVE_RETENTION_DAYS")
	}()

	t.Run("default when unset", func(t *testing.T) {
		_ = os.Unsetenv("INSIGHTS_ACTIVE_RETENTION_DAYS")
		got := getActiveRetentionDays()
		if got != defaultActiveRetentionDays {
			t.Errorf("getActiveRetentionDays() = %d, want %d", got, defaultActiveRetentionDays)
		}
	})

	t.Run("from env valid", func(t *testing.T) {
		os.Setenv("INSIGHTS_ACTIVE_RETENTION_DAYS", "365")
		defer os.Unsetenv("INSIGHTS_ACTIVE_RETENTION_DAYS")
		got := getActiveRetentionDays()
		if got != 365 {
			t.Errorf("getActiveRetentionDays() = %d, want 365", got)
		}
	})
}
