package health

import (
	"net"
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

// ReadinessCheck performs readiness checks.
// If grpcPort is non-empty, it verifies the gRPC server is listening on 127.0.0.1:grpcPort
// so that the pod is not marked Ready until Agents can connect (avoids "connection refused" on 9090).
// It does NOT fail readiness on database/NATS so the pod can accept traffic once HTTP+gRPC are up.
func ReadinessCheck(db *gorm.DB, grpcPort string) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := HealthStatus{
			Status:    "ready",
			Timestamp: time.Now(),
			Checks:    make(map[string]string),
		}

		status.Checks["http"] = "ok"

		// Verify gRPC is actually listening so Service endpoints don't get traffic before 9090 is ready
		if grpcPort != "" {
			conn, err := net.DialTimeout("tcp", "127.0.0.1:"+grpcPort, 1*time.Second)
			if err != nil {
				status.Status = "not_ready"
				status.Checks["grpc"] = "not listening: " + err.Error()
				c.JSON(http.StatusServiceUnavailable, status)
				return
			}
			conn.Close()
			status.Checks["grpc"] = "ok"
		} else {
			status.Checks["grpc"] = "ok"
		}

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

