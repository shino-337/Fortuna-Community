package corehttp

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTokenFile(t *testing.T, token string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(token+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func newRequest(t *testing.T, path string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "http://core"+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func TestAgentRoutePrefersScopedTokenFileOverLegacyToken(t *testing.T) {
	scoped := strings.Repeat("s", 32)
	legacy := strings.Repeat("l", 32)
	t.Setenv(agentTokenFileEnv, writeTokenFile(t, scoped))
	t.Setenv("FORTUNA_INGEST_TOKEN", legacy)

	req := newRequest(t, "/api/v1/agent/sync")
	ApplyOptionalAuthorization(req)
	if got := req.Header.Get("X-Fortuna-Ingest-Token"); got != scoped {
		t.Fatalf("scoped route token=%q want scoped token", got)
	}
}

func TestConfiguredScopedTokenFileFailsClosedWithoutLegacyFallback(t *testing.T) {
	legacy := strings.Repeat("l", 32)
	t.Setenv(agentTokenFileEnv, filepath.Join(t.TempDir(), "missing"))
	t.Setenv("FORTUNA_INGEST_TOKEN", legacy)

	req := newRequest(t, "/api/v1/agent/pod-events")
	ApplyOptionalAuthorization(req)
	if got := req.Header.Get("X-Fortuna-Ingest-Token"); got != invalidScopedAgentToken {
		t.Fatalf("missing scoped token source did not force fail-closed header: %q", got)
	}
	if got := req.Header.Get("X-Fortuna-Ingest-Token"); got == legacy {
		t.Fatal("missing scoped token source fell back to legacy token")
	}
}

func TestInvalidScopedTokenFileFailsClosedWithoutLegacyFallback(t *testing.T) {
	t.Setenv(agentTokenFileEnv, writeTokenFile(t, "too-short"))
	legacy := strings.Repeat("l", 32)
	t.Setenv("FORTUNA_INGEST_TOKEN", legacy)

	req := newRequest(t, "/api/v1/agent/pod-processes")
	ApplyOptionalAuthorization(req)
	if got := req.Header.Get("X-Fortuna-Ingest-Token"); got != invalidScopedAgentToken {
		t.Fatalf("invalid scoped token source did not force fail-closed header: %q", got)
	}
	if got := req.Header.Get("X-Fortuna-Ingest-Token"); got == legacy {
		t.Fatal("invalid scoped token source fell back to legacy token")
	}
}

func TestScopedTokenFailureBlocksBearerAndStaleHeaderFallback(t *testing.T) {
	t.Setenv(agentTokenFileEnv, filepath.Join(t.TempDir(), "missing"))
	t.Setenv("FORTUNA_INGEST_TOKEN", strings.Repeat("l", 32))
	t.Setenv("FORTUNA_CORE_HTTP_AUTHORIZATION", "Bearer "+strings.Repeat("p", 32))

	req := newRequest(t, "/api/v1/agent/sync")
	req.Header.Set("X-Fortuna-Ingest-Token", strings.Repeat("stale", 8))
	ApplyOptionalAuthorization(req)
	if got := req.Header.Get("X-Fortuna-Ingest-Token"); got != invalidScopedAgentToken {
		t.Fatalf("scoped failure left a usable X header: %q", got)
	}
	if got := req.Header.Get("Authorization"); got == "" {
		t.Fatal("proxy Authorization should remain present for the upstream proxy")
	}
	// Core's scoped middleware evaluates X-Fortuna-Ingest-Token first. The
	// deliberately too-short sentinel therefore prevents fallback to Bearer.
	if len(req.Header.Get("X-Fortuna-Ingest-Token")) >= 32 {
		t.Fatal("fail-closed sentinel could be accepted by the Core token-length gate")
	}
}

func TestRuntimeRoutesRemainOnLegacyTokenDuringC2Migration(t *testing.T) {
	t.Setenv(agentTokenFileEnv, writeTokenFile(t, strings.Repeat("s", 32)))
	legacy := strings.Repeat("l", 32)
	t.Setenv("FORTUNA_INGEST_TOKEN", legacy)

	for _, path := range []string{"/api/v1/runtime/events", "/api/v2/runtime/events"} {
		req := newRequest(t, path)
		ApplyOptionalAuthorization(req)
		if got := req.Header.Get("X-Fortuna-Ingest-Token"); got != legacy {
			t.Fatalf("runtime route %s token=%q want legacy token", path, got)
		}
	}
}

func TestScopedTokenFileIsRereadForRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	oldToken := strings.Repeat("a", 32)
	newToken := strings.Repeat("b", 32)
	if err := os.WriteFile(path, []byte(oldToken), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(agentTokenFileEnv, path)
	t.Setenv("FORTUNA_INGEST_TOKEN", strings.Repeat("l", 32))

	first := newRequest(t, "/api/v1/agent/sync")
	ApplyOptionalAuthorization(first)
	if got := first.Header.Get("X-Fortuna-Ingest-Token"); got != oldToken {
		t.Fatalf("initial token=%q", got)
	}

	tmp := path + ".new"
	if err := os.WriteFile(tmp, []byte(newToken+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
	second := newRequest(t, "/api/v1/agent/sync")
	ApplyOptionalAuthorization(second)
	if got := second.Header.Get("X-Fortuna-Ingest-Token"); got != newToken {
		t.Fatalf("rotated token=%q want new token", got)
	}
}

func TestReverseProxyAuthorizationRemainsIndependent(t *testing.T) {
	t.Setenv("FORTUNA_CORE_HTTP_AUTHORIZATION", "Bearer proxy-token")
	t.Setenv("FORTUNA_INGEST_TOKEN", strings.Repeat("l", 32))

	req := newRequest(t, "/api/v1/runtime/events")
	ApplyOptionalAuthorization(req)
	if got := req.Header.Get("Authorization"); got != "Bearer proxy-token" {
		t.Fatalf("proxy authorization=%q", got)
	}
}
