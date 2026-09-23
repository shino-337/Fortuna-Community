package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func respondDataUnavailableWithRetry(c *gin.Context, code, message string, retryable bool) {
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"status":    "unavailable",
		"code":      code,
		"error":     message,
		"retryable": retryable,
	})
}

// respondDataUnavailable is for transient backing-store/query availability failures.
func respondDataUnavailable(c *gin.Context, code, message string) {
	respondDataUnavailableWithRetry(c, code, message, true)
}

// respondSchemaUnavailable is for deployment/schema prerequisites that require
// migration/operator action rather than client retry.
func respondSchemaUnavailable(c *gin.Context, code, message string) {
	respondDataUnavailableWithRetry(c, code, message, false)
}
