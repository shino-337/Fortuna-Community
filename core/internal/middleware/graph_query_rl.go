package middleware

import (
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// graphQueryRL is a dedicated limiter for POST /graph/query (per principal + IP).
var (
	graphQueryMu    sync.Mutex
	graphQueryByKey = make(map[string]*rate.Limiter)
)

func limiterForGraphQuery(key string) *rate.Limiter {
	graphQueryMu.Lock()
	defer graphQueryMu.Unlock()
	l, ok := graphQueryByKey[key]
	if !ok {
		l = rate.NewLimiter(5, 20) // 5 req/s sustained, burst 20
		graphQueryByKey[key] = l
	}
	return l
}

func graphQueryRateKey(c *gin.Context) string {
	ip := c.ClientIP()
	if ip == "" {
		ip = "unknown"
	}
	if uid, ok := c.Get("userID"); ok {
		if id, ok2 := uid.(uint); ok2 && id > 0 {
			return "u:" + strconv.FormatUint(uint64(id), 10) + "|" + ip
		}
	}
	return "ip:" + ip
}

// GraphQueryRateLimit applies a conservative per-principal/IP limit before expensive graph execution.
func GraphQueryRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := graphQueryRateKey(c)
		if !limiterForGraphQuery(key).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "graph query rate limit exceeded"})
			return
		}
		c.Next()
	}
}
