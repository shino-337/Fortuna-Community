package config

import (
	"encoding/base64"
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

func TestLoadRejectsAuthDisabledOutsideDevMode(t *testing.T) {
	t.Setenv("JWT_SECRET", strings.Repeat("a", 32))
	t.Setenv("AUTH_ENABLED", "false")
	t.Setenv("FORTUNA_DEV_MODE", "")

	_, err := Load("")
	if err == nil || !strings.Contains(err.Error(), "AUTH_ENABLED=false") {
		t.Fatalf("expected AUTH_ENABLED=false to be rejected outside dev mode, got %v", err)
	}
}

func TestLoadAllowsAuthDisabledInDevMode(t *testing.T) {
	t.Setenv("JWT_SECRET", strings.Repeat("a", 32))
	t.Setenv("AUTH_ENABLED", "false")
	t.Setenv("FORTUNA_DEV_MODE", "1")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.AuthEnabled {
		t.Fatal("expected auth to be disabled in dev mode")
	}
}

func TestParsePodDetailEncryptionKeys(t *testing.T) {
	valid := base64.StdEncoding.EncodeToString(make([]byte, 32))
	short := base64.StdEncoding.EncodeToString(make([]byte, 16))
	cases := []struct {
		name, current, previous string
		wantKeys                int
		wantErr                 bool
	}{
		{name: "unset", wantKeys: 0},
		{name: "current only", current: valid, wantKeys: 1},
		{name: "with previous", current: valid, previous: valid + ", " + valid, wantKeys: 3},
		{name: "not base64", current: "not-a-key!", wantErr: true},
		{name: "wrong length", current: short, wantErr: true},
		{name: "bad previous", current: valid, previous: short, wantErr: true},
		{name: "previous without current", previous: valid, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			keys, err := ParsePodDetailEncryptionKeys(tc.current, tc.previous)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if len(keys) != tc.wantKeys {
				t.Fatalf("keys=%d want %d", len(keys), tc.wantKeys)
			}
		})
	}
}

func TestLoadRejectsInvalidPodDetailEncryptionKey(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("POD_DETAIL_ENCRYPTION_KEY", "too-short")
	if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "POD_DETAIL_ENCRYPTION_KEY") {
		t.Fatalf("expected key error, got %v", err)
	}
}

func TestLoadRejectsUnauthedIngestOutsideDevMode(t *testing.T) {
	t.Setenv("JWT_SECRET", strings.Repeat("a", 32))
	t.Setenv("FORTUNA_ALLOW_UNAUTHED_INGEST", "1")
	t.Setenv("FORTUNA_DEV_MODE", "")

	_, err := Load("")
	if err == nil || !strings.Contains(err.Error(), "FORTUNA_ALLOW_UNAUTHED_INGEST") {
		t.Fatalf("expected FORTUNA_ALLOW_UNAUTHED_INGEST to be rejected outside dev mode, got %v", err)
	}

	t.Setenv("FORTUNA_DEV_MODE", "1")
	if _, err := Load(""); err != nil {
		t.Fatalf("dev mode should allow unauthenticated ingest, got %v", err)
	}
}
