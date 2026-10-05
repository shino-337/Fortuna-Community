package middleware

import (
	"net/http/httptest"
	"testing"
)

func TestWebSocketProtocolToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/ws/risks?token=from-url", nil)
	req.Header.Set("Sec-WebSocket-Protocol", "fortuna.v1, fortuna.bearer.aaa.bbb.ccc")
	if got := webSocketProtocolToken(req); got != "aaa.bbb.ccc" {
		t.Fatalf("token from subprotocol: %q", got)
	}
	for _, header := range []string{"", "fortuna.v1", "fortuna.bearer.", "bearer.aaa"} {
		req.Header.Set("Sec-WebSocket-Protocol", header)
		if got := webSocketProtocolToken(req); got != "" {
			t.Fatalf("%q: unexpected token %q", header, got)
		}
	}
}

func TestIsWebSocketUpgrade(t *testing.T) {
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "keep-alive, Upgrade")
	if !isWebSocketUpgrade(req) {
		t.Fatal("expected websocket upgrade request")
	}

	plain := httptest.NewRequest("GET", "/api/v1/me", nil)
	if isWebSocketUpgrade(plain) {
		t.Fatal("plain HTTP request must not be treated as websocket upgrade")
	}
}
