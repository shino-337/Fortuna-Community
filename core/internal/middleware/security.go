package middleware

import (
	"net/http"
	"github.com/gin-gonic/gin"
)

// SecurityHeaders adds security headers to responses
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// CRITICAL: Skip ALL security headers for OPTIONS requests (CORS preflight)
		// OPTIONS requests should ONLY have CORS headers, nothing else
		// This must be checked FIRST before any header is set
		// CORS middleware already handles OPTIONS and aborts with 204
		// We should not set any headers for OPTIONS - just pass through
		if c.Request.Method == "OPTIONS" {
			// Do NOT set any headers, just pass through
			// CORS middleware will handle it
			c.Next()
			return
		}
		
		// Prevent clickjacking
		c.Header("X-Frame-Options", "DENY")
		
		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")
		
		// Enable XSS protection
		c.Header("X-XSS-Protection", "1; mode=block")
		
		// Strict Transport Security (HSTS) - only for HTTPS
		// Skip for localhost development
		if c.Request.TLS != nil {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		
		// Content Security Policy (very relaxed for development - allow all localhost)
		// Note: CSP connect-src should allow API calls from dashboard
		// For API server, we don't need strict CSP - this is for the dashboard
		// But we set it to allow connections from localhost
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self' http://localhost:* http://127.0.0.1:* ws://localhost:* ws://127.0.0.1:* http://localhost:8080 http://127.0.0.1:8080")
		
		// Referrer Policy
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// Permissions Policy
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		
		c.Next()
	}
}

// CORS middleware for cross-origin requests
// This must be applied FIRST before other middleware
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		
		// Always set CORS headers for all requests
		// In development, allow any localhost origin
		if origin != "" {
			// Allow localhost origins (any port)
			if len(origin) >= 11 && (origin[:11] == "http://local" || origin[:14] == "http://127.0.0.1") {
				c.Header("Access-Control-Allow-Origin", origin)
			} else {
				// Check specific allowed origins
				allowedOrigins := []string{
					"http://localhost:3000",
					"http://localhost:8080",
					"http://127.0.0.1:3000",
					"http://127.0.0.1:8080",
				}
				for _, allowedOrigin := range allowedOrigins {
					if origin == allowedOrigin {
						c.Header("Access-Control-Allow-Origin", origin)
						break
					}
				}
			}
		} else {
			// No origin header - allow all for same-origin or direct calls
			c.Header("Access-Control-Allow-Origin", "*")
		}
		
		// Set all CORS headers
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type, Authorization")
		c.Header("Access-Control-Max-Age", "3600")
		
		// Handle preflight requests (OPTIONS) - must return early
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		
		c.Next()
	}
}

// RateLimiting middleware delegates to the in-memory/IP-aware rate limiter implementation.
func RateLimiting() gin.HandlerFunc {
	return RateLimiterMiddleware(DefaultRateLimiterConfig())
}

