package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func respondDataUnavailable(c *gin.Context, code, message string) {
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"status":    "unavailable",
		"code":      code,
		"error":     message,
		"retryable": true,
	})
}
