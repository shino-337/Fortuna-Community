package middleware

import (
	"net/http/httptest"
	"testing"
)

func TestAllowAuthQueryTokenDefaultsDisabled(t *testing.T) {
	t.Setenv("FORTUNA_ALLOW_AUTH_QUERY_TOKEN", "")
	if allowAuthQueryToken() {
		t.Fatal("query-token auth should be disabled by default")
	}
}

func TestAllowAuthQueryTokenRequiresExplicitOptIn(t *testing.T) {
	t.Setenv("FORTUNA_ALLOW_AUTH_QUERY_TOKEN", "true")
	if !allowAuthQueryToken() {
		t.Fatal("query-token auth should be enabled when explicitly configured")
	}
}

func TestIsWebSocketUpgrade(t *testing.T) {
	req := httptest.NewRequest("GET", "/ws?token=abc", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "keep-alive, Upgrade")
	if !isWebSocketUpgrade(req) {
		t.Fatal("expected websocket upgrade request")
	}

	plain := httptest.NewRequest("GET", "/api/v1/me?token=abc", nil)
	if isWebSocketUpgrade(plain) {
		t.Fatal("plain HTTP request must not be treated as websocket upgrade")
	}
}
