package corehttp

import (
	"net/http"
	"os"
	"strings"
)

// ApplyOptionalAuthorization sets the Authorization header when
// FORTUNA_CORE_HTTP_AUTHORIZATION is non-empty (full value, e.g. "Bearer <token>").
// Core agent ingest routes are unauthenticated by default; use this when Core sits
// behind a reverse proxy that requires a shared secret or service token.
func ApplyOptionalAuthorization(req *http.Request) {
	if req == nil {
		return
	}
	v := strings.TrimSpace(os.Getenv("FORTUNA_CORE_HTTP_AUTHORIZATION"))
	if v != "" {
		req.Header.Set("Authorization", v)
	}
}
