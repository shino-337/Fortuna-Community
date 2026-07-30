package config

import (
	"strings"
	"testing"
)

func TestLoadRequiresJWTSecretOutsideDevMode(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	t.Setenv("FORTUNA_JWT_SECRET", "")
	t.Setenv("FORTUNA_DEV_MODE", "")

	_, err := Load("")
	if err == nil {
		t.Fatal("expected Load to fail when JWT secret is unset outside dev mode")
	}
	if !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("expected JWT secret error, got %v", err)
	}
}

func TestLoadAcceptsFortunaJWTSecretAlias(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	t.Setenv("FORTUNA_JWT_SECRET", strings.Repeat("a", 32))
	t.Setenv("FORTUNA_DEV_MODE", "")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.JWTSecret != strings.Repeat("a", 32) {
		t.Fatalf("unexpected JWT secret %q", cfg.JWTSecret)
	}
}

func TestLoadGeneratesEphemeralJWTSecretInDevMode(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	t.Setenv("FORTUNA_JWT_SECRET", "")
	t.Setenv("FORTUNA_DEV_MODE", "1")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed in dev mode: %v", err)
	}
	if len(cfg.JWTSecret) < 32 {
		t.Fatalf("expected generated dev JWT secret, got length %d", len(cfg.JWTSecret))
	}
}
