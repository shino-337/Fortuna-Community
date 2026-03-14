package api

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/api/policy"
	"github.com/fortuna/core/internal/api/risk"
	"github.com/fortuna/core/internal/config"
	"github.com/fortuna/core/internal/ingest"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/security"
)

// Force link analytics handlers to prevent dead code elimination
// This ensures handlers are included in binary even if only referenced indirectly
var (
	_ = risk.GetRiskTrendsAnalytics
	_ = risk.GetRiskComparison
	_ = risk.GetRiskCorrelation
	// Policy handlers
	_ = policy.NewPolicyHandler
)

func SetupRoutes(router *gin.Engine, db *gorm.DB, cfg *config.Config) {
	SetupRoutesWithCertManager(router, db, cfg, nil, nil)
}

// SetupRoutesWithCertManager sets up routes with certificate manager and optional per-cluster rate limiter (Finding #6).
func SetupRoutesWithCertManager(router *gin.Engine, db *gorm.DB, cfg *config.Config, certManager *security.CertManager, clusterLimiter *ingest.ClusterRateLimiter) {
	InitPodDetailEncryptionKey(cfg.PodDetailEncryptionKey)
	// Phase 2.1: in-memory cache for GET /risks and GET /insights/summary (TTL 60s)
	defaultRisksCache = NewMemoryRisksCache(60 * time.Second)
	log.Printf("[API] ========================================")
	log.Printf("[API] SetupRoutesWithCertManager CALLED")
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

	// Agent ingest routes (no auth) – register before v1 so /api/v1/agent/* is unambiguous
	agent := router.Group("/api/v1/agent")
	{
		agent.POST("/sync", SyncDataFromAgent(db, clusterLimiter))
		agent.POST("/pod-runtime-metrics", IngestPodRuntimeMetricsPayload(db))
		agent.POST("/pod-processes", IngestPodProcessesPayload(db))
		agent.POST("/pod-network-connections", IngestPodNetworkConnectionsPayload(db))
		agent.POST("/pod-events", IngestPodEventsPayload(db))
	}
	log.Printf("[API] Agent ingest routes registered: POST /api/v1/agent/sync, pod-runtime-metrics, pod-processes, pod-network-connections, pod-events")

	// Protected routes
	v1 := router.Group("/api/v1")
	if cfg.AuthEnabled {
		v1.Use(middleware.AuthMiddleware(db, cfg.JWTSecret))
	}
	{
		// Domain-architect routes (see ROUTE_MIGRATION_MAPPING.md)
		registerInventoryRoutes(v1, db, cfg)
		registerRuntimeRoutes(v1, db)
		registerRiskRoutes(v1, db)
		registerGraphRoutes(v1, db)
		registerAuditRoutes(v1, db)
		registerPolicyRoutes(v1, db)

		// Health / data integrity
		v1.GET("/health/dashboard-data-integrity", DashboardDataIntegrity(db))

		// Current user
		v1.GET("/me", GetCurrentUser())
		v1.POST("/change-password", ChangePassword(db))
		if cfg.AuthEnabled {
			v1.GET("/users", middleware.RequireAdmin(), GetUsers(db))
		} else {
			v1.GET("/users", GetUsers(db))
		}

		// Capability Metadata (Phase 2) — global metadata, keep at v1 root for now
		v1.GET("/capability-metadata", GetCapabilityMetadataList(db))
		v1.GET("/capability-metadata/:capabilityId", GetCapabilityMetadata(db))

		// Promotion Rules (Phase 2.3)
		v1.GET("/promotion-rules", GetPromotionRulesList(db))
		v1.GET("/promotion-rules/capability/:capabilityId", GetPromotionRulesByCapability(db))
		v1.GET("/promotion-rules/signal/:signalType", GetPromotionRulesBySignalType(db))

		// WebSockets
		v1.GET("/ws/pod/:uid", PodDetailWS())
		v1.GET("/ws/risks", RisksWS())

		// Dashboard summaries (aggregate)
		v1.GET("/dashboard/stats", GetDashboardStats(db))
		v1.GET("/dashboard/metrics/threat-velocity", GetThreatVelocity(db))

		// Cluster (infrastructure: info, nodes, certificates)
		cluster := v1.Group("/cluster")
		cluster.GET("/info", GetClusterInfo(db))
		cluster.GET("/:id/nodes", GetClusterNodes(db))
		cluster.GET("/certificates/rotation/history", GetCertificateRotationHistory(db))
		if certManager != nil {
			log.Printf("[API] Registering cluster certificate routes")
			certHandler := NewCertHandler(certManager)
			cluster.GET("/certificates/info", certHandler.GetCertificateInfo)
			cluster.POST("/certificates/rotate", certHandler.RotateCertificate)
		}

		// Metrics (Workers/Queue/API Latency removed – use Prometheus when needed)
		v1.GET("/metrics/system", GetSystemMetrics(db))
		v1.GET("/metrics/policy-evaluation-cost", GetPolicyEvaluationCost(db))
		v1.GET("/error-logs", GetErrorLogs(db))
		v1.GET("/agents/status", GetAgentStatus(db))

		// Dashboard support
		v1.GET("/resources", GetResources(db))
		v1.GET("/notifications", GetNotifications(db))
		v1.GET("/monitoring/agents", GetAgentStatus(db))
	}
}
