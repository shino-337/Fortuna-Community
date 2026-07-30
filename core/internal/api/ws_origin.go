package api

import (
	"net/http"
	"os"
	"strings"
)

// wsAllowedOrigin is used for Gorilla WebSocket CheckOrigin (OWASP: do not allow arbitrary origins).
// - Requests with no Origin header are allowed (non-browser clients, same-origin tools).
// - Origins listed in FORTUNA_WS_ALLOWED_ORIGINS (comma-separated) are allowed.
// - http(s)://localhost:* and http(s)://127.0.0.1:* are allowed for local dashboard dev.
func wsAllowedOrigin(r *http.Request) bool {
	o := strings.TrimSpace(r.Header.Get("Origin"))
	if o == "" {
		return true
	}
	for _, p := range strings.Split(os.Getenv("FORTUNA_WS_ALLOWED_ORIGINS"), ",") {
		if s := strings.TrimSpace(p); s != "" && strings.EqualFold(s, o) {
			return true
		}
	}
	lo := strings.ToLower(o)
	if strings.HasPrefix(lo, "http://localhost:") || strings.HasPrefix(lo, "http://127.0.0.1:") {
		return true
	}
	if strings.HasPrefix(lo, "https://localhost:") || strings.HasPrefix(lo, "https://127.0.0.1:") {
		return true
	}
	return false
}
