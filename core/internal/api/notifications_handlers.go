package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetNotifications returns notifications list (stub for now).
// No backend table; returns empty. Dashboard should treat as "no data" not fake data.
// See GET /health/dashboard-data-integrity for endpoint inventory.
func GetNotifications() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"_dataSource":   "stub",
			"notifications": []map[string]interface{}{},
			"total":         0,
		})
	}
}
