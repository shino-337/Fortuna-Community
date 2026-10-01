package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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

func requireAvailabilityTables(c *gin.Context, db *gorm.DB, code, message string, tables ...string) bool {
	for _, table := range tables {
		if !db.Migrator().HasTable(table) {
			// HasTable only returns a bool and also reports false when its catalog
			// query cannot reach the database. Confirm connectivity before calling
			// the absence a non-retryable migration problem.
			if err := db.Exec("SELECT 1").Error; err != nil {
				respondDataUnavailable(c, strings.TrimSuffix(code, "_schema_unavailable")+"_query_failed", message)
				return false
			}
			respondSchemaUnavailable(c, code, message)
			return false
		}
	}
	return true
}
