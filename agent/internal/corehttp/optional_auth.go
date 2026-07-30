package corehttp

import (
	"net/http"
	"os"
	"strings"
)

// ApplyOptionalAuthorization sets headers used by Core HTTP ingest when the corresponding env vars are set:
//   - FORTUNA_CORE_HTTP_AUTHORIZATION → Authorization (e.g. Bearer …) for reverse proxies
//   - FORTUNA_INGEST_TOKEN → X-Fortuna-Ingest-Token when Core is configured with the same FORTUNA_INGEST_TOKEN
func ApplyOptionalAuthorization(req *http.Request) {
	if req == nil {
		return
	}
	v := strings.TrimSpace(os.Getenv("FORTUNA_CORE_HTTP_AUTHORIZATION"))
	if v != "" {
		req.Header.Set("Authorization", v)
	}
	if tok := strings.TrimSpace(os.Getenv("FORTUNA_INGEST_TOKEN")); tok != "" {
		req.Header.Set("X-Fortuna-Ingest-Token", tok)
	}
}
