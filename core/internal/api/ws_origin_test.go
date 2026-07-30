package api

import (
	"net/http/httptest"
	"testing"
)

func TestWsAllowedOrigin(t *testing.T) {
	t.Setenv("FORTUNA_WS_ALLOWED_ORIGINS", "https://dashboard.example.com,http://192.168.56.100:30956")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://dashboard.example.com")
	if !wsAllowedOrigin(req) {
		t.Fatal("explicit allowlist should match")
	}
	reqNodePort := httptest.NewRequest("GET", "/", nil)
	reqNodePort.Header.Set("Origin", "http://192.168.56.100:30956")
	if !wsAllowedOrigin(reqNodePort) {
		t.Fatal("explicit NodePort dashboard origin should match")
	}
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("Origin", "https://evil.example")
	if wsAllowedOrigin(req2) {
		t.Fatal("unknown origin should reject")
	}
	req3 := httptest.NewRequest("GET", "/", nil)
	if !wsAllowedOrigin(req3) {
		t.Fatal("missing Origin should allow")
	}
}
