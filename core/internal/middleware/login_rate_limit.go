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

func limiterForLogin(ip string) *rate.Limiter {
	loginRLMu.Lock()
	defer loginRLMu.Unlock()
	l, ok := loginRLByIP[ip]
	if !ok {
		// Sustained ~5 attempts/minute per IP with small burst for legitimate retries.
		l = rate.NewLimiter(rate.Every(12*time.Second), 5)
		loginRLByIP[ip] = l
	}
	return l
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
