package api

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"

	"github.com/ksam/core/internal/config"
	"github.com/ksam/core/internal/middleware"
	"github.com/ksam/core/pkg/security"
)

func SetupRoutes(router *gin.Engine, db *gorm.DB, cfg *config.Config) {
	SetupRoutesWithCertManager(router, db, cfg, nil)
}

// SetupRoutesWithCertManager sets up routes with certificate manager
func SetupRoutesWithCertManager(router *gin.Engine, db *gorm.DB, cfg *config.Config, certManager *security.CertManager) {
	log.Printf("[API] ========================================")
	log.Printf("[API] Setting up routes with CertManager")
	log.Printf("[API]   CertManager == nil: %v", certManager == nil)
	log.Printf("[API]   TLS_ENABLED: %v", cfg.TLSEnabled)
	log.Printf("[API] ========================================")

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
		v1.GET("/clusters/stats", GetClustersStats(db))
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

		// Deployments
		v1.GET("/deployments", GetDeployments(db))
		v1.GET("/deployments/:id", GetDeployment(db))
		v1.GET("/replicasets", GetReplicaSets(db))
		v1.GET("/replicasets/:id", GetReplicaSet(db))

		// Graph
		v1.GET("/graph", GetGraph(db))
		v1.GET("/graph/blast-radius/:id", GetBlastRadius(db))
		v1.GET("/graph/shortest-path", GetShortestPath(db))
		v1.GET("/graph/accessible/:id", GetAccessibleResources(db))
		v1.POST("/graph/query", ExecuteGraphQuery(db))

		// Audit
		v1.GET("/audit", GetAuditLogs(db))
		v1.GET("/audit/reports", GetAuditReports(db))

		// Insights
		v1.GET("/insights", GetInsights(db))
		v1.GET("/insights/summary", GetInsightsSummary(db))
		v1.GET("/insights/:id", GetInsight(db))
		v1.DELETE("/insights/:id", DeleteInsight(db))
		v1.POST("/insights/evaluate", TriggerRiskEvaluation(db))                      // Uses internal/risk engine
		v1.POST("/insights/evaluate/historical", TriggerHistoricalRiskEvaluation(db)) // Uses Risk Worker engine

		// Certificate management (if cert manager available)
		if certManager != nil {
			log.Printf("[API] ========================================")
			log.Printf("[API] Registering certificate management routes")
			log.Printf("[API]   CertManager: %v", certManager != nil)
			certHandler := NewCertHandler(certManager)
			certs := v1.Group("/certificates")
			{
				certs.GET("/info", certHandler.GetCertificateInfo)
				certs.POST("/rotate", certHandler.RotateCertificate)
			}
			log.Printf("[API] ✅ Certificate routes registered:")
			log.Printf("[API]   GET  /api/v1/certificates/info")
			log.Printf("[API]   POST /api/v1/certificates/rotate")
			log.Printf("[API] ========================================")
		} else {
			log.Printf("[API] ⚠️  CertManager is nil - certificate routes NOT registered")
			log.Printf("[API]   TLS_ENABLED: %v", cfg.TLSEnabled)
		}
	}

	// Agent endpoints (no auth required for now, can add token-based auth later)
	agent := router.Group("/api/v1/agent")
	{
		agent.POST("/sync", SyncDataFromAgent(db))
	}
}
