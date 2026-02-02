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
// IMPORTANT: Readiness only checks if the service can accept requests (HTTP/gRPC servers are listening)
// It does NOT check database, NATS, or external dependencies
// This ensures the pod can be marked as ready even if external dependencies are temporarily unavailable
func ReadinessCheck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := HealthStatus{
			Status:    "ready",
			Timestamp: time.Now(),
			Checks:    make(map[string]string),
		}

		// Readiness check: Only verify that HTTP and gRPC servers are listening
		// If this endpoint is reachable, it means HTTP server is running
		status.Checks["http"] = "ok"
		status.Checks["grpc"] = "ok" // gRPC server is started in main.go, assume it's running if HTTP is

		// Optional: Check database status (but don't fail readiness if it's down)
		// This provides observability without blocking service routing
		if db != nil {
			sqlDB, err := db.DB()
			if err == nil {
				if err := sqlDB.Ping(); err != nil {
					status.Checks["database"] = "degraded: " + err.Error()
				} else {
					status.Checks["database"] = "ok"
				}
			} else {
				status.Checks["database"] = "degraded: " + err.Error()
			}
		} else {
			status.Checks["database"] = "degraded: database connection not initialized"
		}

		// Always return ready if HTTP server is responding
		c.JSON(http.StatusOK, status)
	}
}

// LivenessCheck performs liveness checks
// This checks if the process is alive (no database/external dependencies)
func LivenessCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "alive",
			"timestamp": time.Now(),
		})
	}
}

// StatusCheck performs comprehensive status checks (for observability)
// This includes database, NATS, and other dependencies
// Use this for monitoring, NOT for Kubernetes readiness/liveness probes
func StatusCheck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := HealthStatus{
			Status:    "healthy",
			Timestamp: time.Now(),
			Checks:    make(map[string]string),
		}

		// Check HTTP server
		status.Checks["http"] = "ok"

		// Check gRPC server (assumed running if HTTP is)
		status.Checks["grpc"] = "ok"

		// Check database connection
		if db != nil {
			sqlDB, err := db.DB()
			if err != nil {
				status.Status = "degraded"
				status.Checks["database"] = "error: " + err.Error()
			} else {
				if err := sqlDB.Ping(); err != nil {
					status.Status = "degraded"
					status.Checks["database"] = "error: " + err.Error()
				} else {
					status.Checks["database"] = "ok"
				}
			}
		} else {
			status.Status = "degraded"
			status.Checks["database"] = "error: database connection not initialized"
		}

		// Determine HTTP status code
		httpStatus := http.StatusOK
		if status.Status == "degraded" {
			httpStatus = http.StatusServiceUnavailable
		}

		c.JSON(httpStatus, status)
	}
}

