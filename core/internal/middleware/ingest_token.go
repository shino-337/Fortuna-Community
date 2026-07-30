package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

const ingestTokenHeader = "X-Fortuna-Ingest-Token"

// RequireIngestToken enforces a shared secret on agent/runtime HTTP ingest routes.
// Empty tokens fail closed unless FORTUNA_ALLOW_UNAUTHED_INGEST=1 is explicitly set for local development.
func RequireIngestToken(expected string) gin.HandlerFunc {
	expected = strings.TrimSpace(expected)
	if expected == "" && allowUnauthedIngest() {
		return func(c *gin.Context) { c.Next() }
	}
	if expected == "" {
		return func(c *gin.Context) {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "ingest token not configured; set FORTUNA_INGEST_TOKEN",
				"code":  "ingest_token_unconfigured",
			})
		}
	}
	want := []byte(expected)
	return func(c *gin.Context) {
		if tokenFromIngestRequest(c, want) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "ingest authentication required",
		})
	}
}

func allowUnauthedIngest() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("FORTUNA_ALLOW_UNAUTHED_INGEST")))
	return v == "1" || v == "true"
}

func tokenFromIngestRequest(c *gin.Context, want []byte) bool {
	h := strings.TrimSpace(c.GetHeader(ingestTokenHeader))
	if constantTimeEqualBytes([]byte(h), want) {
		return true
	}
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		tok := strings.TrimSpace(auth[7:])
		if constantTimeEqualBytes([]byte(tok), want) {
			return true
		}
	}
	return false
}

func constantTimeEqualBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare(a, b) == 1
}
