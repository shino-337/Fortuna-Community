package migrations

import (
	"os"
	"testing"
)

func TestShouldRunSeedMigrations(t *testing.T) {
	key := "FORTUNA_ENABLE_SEED_DATA"
	restore := os.Getenv(key)
	defer func() {
		if restore == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, restore)
		}
	}()

	tests := []struct {
		env    string
		expect bool
	}{
		{"", false},
		{"0", false},
		{"false", false},
		{"False", false},
		{"no", false},
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"1", true},
		{"yes", true},
		{"on", true},
		{"  true  ", true},
	}
	for _, tt := range tests {
		os.Unsetenv(key)
		if tt.env != "" {
			os.Setenv(key, tt.env)
		}
		got := shouldRunSeedMigrations()
		if got != tt.expect {
			t.Errorf("FORTUNA_ENABLE_SEED_DATA=%q: shouldRunSeedMigrations() = %v, want %v", tt.env, got, tt.expect)
		}
	}
}
