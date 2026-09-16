package corehttp

import (
	"net/http"
	"os"
	"strings"
)

const (
	agentTokenFileEnv       = "FORTUNA_AGENT_TOKEN_FILE"
	invalidScopedAgentToken = "invalid"
)

// ApplyOptionalAuthorization sets headers used by Core HTTP ingest.
//
// Migration contract:
//   - FORTUNA_CORE_HTTP_AUTHORIZATION is independent reverse-proxy Authorization.
//   - /api/v1/agent/* prefers FORTUNA_AGENT_TOKEN_FILE when configured. The file
//     is reread for every request so atomic replacement rotates credentials without
//     restarting the agent. If the configured file is missing/invalid, auth fails
//     closed and never falls back to the legacy shared token or a Bearer header.
//   - Other HTTP ingest paths (notably /api/v1|v2/runtime/* during C2 migration)
//     continue to use FORTUNA_INGEST_TOKEN until their scoped-auth cutover.
func ApplyOptionalAuthorization(req *http.Request) {
	if req == nil {
		return
	}
	if v := strings.TrimSpace(os.Getenv("FORTUNA_CORE_HTTP_AUTHORIZATION")); v != "" {
		req.Header.Set("Authorization", v)
	}

	if isScopedAgentHTTPRoute(req) {
		if tokenFile := strings.TrimSpace(os.Getenv(agentTokenFileEnv)); tokenFile != "" {
			if tok := readScopedAgentToken(tokenFile); tok != "" {
				req.Header.Set("X-Fortuna-Ingest-Token", tok)
			} else {
				// Core prefers X-Fortuna-Ingest-Token before Authorization: Bearer.
				// Set a deliberately too-short value so a missing/invalid scoped source
				// cannot fall through to a proxy Bearer credential or a stale header.
				req.Header.Set("X-Fortuna-Ingest-Token", invalidScopedAgentToken)
			}
			return // configured scoped source is authoritative; never fall back
		}
	}

	if tok := strings.TrimSpace(os.Getenv("FORTUNA_INGEST_TOKEN")); tok != "" {
		req.Header.Set("X-Fortuna-Ingest-Token", tok)
	}
}

func isScopedAgentHTTPRoute(req *http.Request) bool {
	return req != nil && req.URL != nil && strings.HasPrefix(req.URL.Path, "/api/v1/agent/")
}

func readScopedAgentToken(path string) string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) > 8192 {
		return ""
	}
	tok := strings.TrimSpace(string(data))
	// Keep the transport-side bounds aligned with core/pkg/agentidentity. This is
	// not an entropy check; provisioning is responsible for cryptographic randomness.
	if len(tok) < 32 || len(tok) > 4096 {
		return ""
	}
	return tok
}
