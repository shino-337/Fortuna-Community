package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// loginRL limits POST /api/v1/auth/login per client IP (stricter than global API rate limit).
var (
	loginRLMu   sync.Mutex
	loginRLByIP = make(map[string]*rate.Limiter)
)

// loginRLMaxEntries bounds the per-IP map so a flood of distinct source
// addresses cannot grow it without limit.
const loginRLMaxEntries = 10000

func limiterForLogin(ip string) *rate.Limiter {
	loginRLMu.Lock()
	defer loginRLMu.Unlock()
	l, ok := loginRLByIP[ip]
	if !ok {
		if len(loginRLByIP) >= loginRLMaxEntries {
			pruneIdleLoginLimiters()
		}
		// Sustained ~5 attempts/minute per IP with small burst for legitimate retries.
		l = rate.NewLimiter(rate.Every(12*time.Second), 5)
		loginRLByIP[ip] = l
	}
	return l
}

// pruneIdleLoginLimiters drops limiters whose bucket has fully refilled; such a
// limiter behaves exactly like a new one, so removing it loses no state. Entries
// still being throttled are kept, so the map only exceeds the bound while that
// many IPs attempted a login within the last minute. Caller holds loginRLMu.
func pruneIdleLoginLimiters() {
	for ip, l := range loginRLByIP {
		if l.Tokens() >= float64(l.Burst()) {
			delete(loginRLByIP, ip)
		}
	}
}

// LoginRateLimit applies a dedicated per-IP limit before password verification.
func LoginRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if ip == "" {
			ip = "unknown"
		}
		if !limiterForLogin(ip).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many login attempts; try again later",
			})
			return
		}
		c.Next()
	}
}
