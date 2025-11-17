package api

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"

	"github.com/ksam/core/internal/config"
	"github.com/ksam/core/internal/middleware"
)

func SetupRoutes(router *gin.Engine, db *gorm.DB, cfg *config.Config) {
	// Prometheus metrics endpoint (no auth required)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Public auth routes
	auth := router.Group("/api/v1/auth")
	{
		auth.POST("/login", Login(db, cfg.JWTSecret, cfg.TokenExpirationHours))
		if cfg.AuthEnabled {
			auth.POST("/register", middleware.AuthMiddleware(db, cfg.JWTSecret), middleware.RequireAdmin(), Register(db, cfg.JWTSecret))
		} else {
			auth.POST("/register", Register(db, cfg.JWTSecret))
		}
	}

	// Protected routes
	v1 := router.Group("/api/v1")
	if cfg.AuthEnabled {
		v1.Use(middleware.AuthMiddleware(db, cfg.JWTSecret))
	}
	{
		// Current user
		v1.GET("/me", GetCurrentUser())
		v1.POST("/change-password", ChangePassword(db))

		// Clusters
		v1.GET("/clusters", GetClusters(db))
		v1.GET("/clusters/:id", GetCluster(db))

		// ServiceAccounts
		v1.GET("/serviceaccounts", GetServiceAccounts(db))
		v1.GET("/serviceaccounts/:id", GetServiceAccount(db))
		v1.GET("/serviceaccounts/:id/permissions", GetServiceAccountPermissions(db))
		v1.PUT("/serviceaccounts/:id", UpdateServiceAccount(db))
		v1.DELETE("/serviceaccounts/:id", DeleteServiceAccount(db))
		
		// Bulk operations (admin only)
		v1.POST("/serviceaccounts/bulk/disable", middleware.RequireAdmin(), BulkDisableServiceAccounts(db))
		v1.POST("/serviceaccounts/bulk/delete", middleware.RequireAdmin(), BulkDeleteServiceAccounts(db))
		v1.POST("/serviceaccounts/disable-inactive", middleware.RequireAdmin(), DisableInactiveServiceAccounts(db))

		// Graph
		v1.GET("/graph", GetGraph(db))

		// Audit
		v1.GET("/audit", GetAuditLogs(db))
		v1.GET("/audit/reports", GetAuditReports(db))
	}

	// Agent endpoints (no auth required for now, can add token-based auth later)
	agent := router.Group("/api/v1/agent")
	{
		agent.POST("/sync", SyncDataFromAgent(db))
	}
}

