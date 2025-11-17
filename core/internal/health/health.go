package health

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// HealthStatus represents health check status
type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Checks    map[string]string `json:"checks"`
}

// HealthCheck performs health checks
func HealthCheck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := HealthStatus{
			Status:    "healthy",
			Timestamp: time.Now(),
			Checks:    make(map[string]string),
		}

		// Check database
		sqlDB, err := db.DB()
		if err != nil {
			status.Status = "unhealthy"
			status.Checks["database"] = "error: " + err.Error()
			c.JSON(http.StatusServiceUnavailable, status)
			return
		}

		// Ping database
		if err := sqlDB.Ping(); err != nil {
			status.Status = "unhealthy"
			status.Checks["database"] = "error: " + err.Error()
			c.JSON(http.StatusServiceUnavailable, status)
			return
		}
		status.Checks["database"] = "ok"

		c.JSON(http.StatusOK, status)
	}
}

// ReadinessCheck performs readiness checks
func ReadinessCheck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := HealthStatus{
			Status:    "ready",
			Timestamp: time.Now(),
			Checks:    make(map[string]string),
		}

		// Check database connection
		sqlDB, err := db.DB()
		if err != nil {
			status.Status = "not_ready"
			status.Checks["database"] = "error: " + err.Error()
			c.JSON(http.StatusServiceUnavailable, status)
			return
		}

		if err := sqlDB.Ping(); err != nil {
			status.Status = "not_ready"
			status.Checks["database"] = "error: " + err.Error()
			c.JSON(http.StatusServiceUnavailable, status)
			return
		}

		status.Checks["database"] = "ok"
		c.JSON(http.StatusOK, status)
	}
}

// LivenessCheck performs liveness checks
func LivenessCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "alive",
			"timestamp": time.Now(),
		})
	}
}

