package middleware

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// DefaultMaxRequestBodyBytes bounds every HTTP request body. Agent inventory
// syncs are the largest payloads; operators with very large clusters can raise
// the limit with FORTUNA_MAX_REQUEST_BODY_BYTES.
const DefaultMaxRequestBodyBytes int64 = 64 << 20

// TrustedProxiesFromEnv returns the proxy IPs/CIDRs whose X-Forwarded-For and
// X-Real-IP headers may be used for c.ClientIP(). Unset means no proxy is
// trusted and the TCP peer address is used. Gin otherwise trusts every proxy,
// which lets any client spoof its IP and bypass per-IP rate limits.
func TrustedProxiesFromEnv() []string {
	var out []string
	for _, p := range strings.Split(os.Getenv("FORTUNA_TRUSTED_PROXIES"), ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// MaxRequestBodyBytesFromEnv returns the configured body limit or the default.
func MaxRequestBodyBytesFromEnv() int64 {
	v := strings.TrimSpace(os.Getenv("FORTUNA_MAX_REQUEST_BODY_BYTES"))
	if v == "" {
		return DefaultMaxRequestBodyBytes
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return DefaultMaxRequestBodyBytes
	}
	return n
}

// MaxRequestBody caps the request body so a single client cannot exhaust Core
// memory with an unbounded JSON payload. Handlers see a read error once the
// limit is exceeded.
func MaxRequestBody(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > limit {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": "request body too large",
				"code":  "request_body_too_large",
			})
			return
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}

// sensitiveQueryParams are redacted from access logs. Browser WebSocket clients
// may authenticate with ?token=<JWT>, which must never be written to logs.
var sensitiveQueryParams = []string{"token", "access_token"}

// RedactQuery replaces sensitive query parameter values in a request URI.
func RedactQuery(rawPath string) string {
	path, rawQuery, found := strings.Cut(rawPath, "?")
	if !found || rawQuery == "" {
		return rawPath
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return path + "?[unparseable query redacted]"
	}
	redacted := false
	for _, key := range sensitiveQueryParams {
		if _, ok := values[key]; ok {
			values.Set(key, "REDACTED")
			redacted = true
		}
	}
	if !redacted {
		return rawPath
	}
	return path + "?" + values.Encode()
}

// RedactingLogger is gin.Logger with sensitive query values removed.
func RedactingLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(p gin.LogFormatterParams) string {
		latency := p.Latency
		if latency > time.Minute {
			latency = latency.Truncate(time.Second)
		}
		return fmt.Sprintf("[GIN] %v | %3d | %13v | %15s | %-7s %#v\n%s",
			p.TimeStamp.Format("2006/01/02 - 15:04:05"),
			p.StatusCode,
			latency,
			p.ClientIP,
			p.Method,
			RedactQuery(p.Path),
			p.ErrorMessage,
		)
	})
}
