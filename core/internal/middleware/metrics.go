package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/fortuna/core/internal/metrics"
)

// MetricsMiddleware records HTTP metrics
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method
		endpoint := c.FullPath()

		// Record metrics
		metrics.HTTPRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(method, endpoint).Observe(duration)
	}
}

