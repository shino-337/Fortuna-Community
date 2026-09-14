package api

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/api/policy"
	"github.com/fortuna/core/internal/api/risk"
	"github.com/fortuna/core/internal/config"
	"github.com/fortuna/core/internal/ingest"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/malware"
	"github.com/fortuna/core/pkg/security"
)

// Force link analytics handlers to prevent dead code elimination
// This ensures handlers are included in binary even if only referenced indirectly
var (
	_ = risk.GetRiskTrendsAnalytics
	_ = risk.GetRiskComparison
	_ = risk.GetRiskCorrelation
	_ = risk.GetSupplyChainCorrelation
	_ = risk.GetRuntimeCVECorrelation
	// Policy handlers
	_ = policy.NewPolicyHandler
)

func SetupRoutes(router *gin.Engine, db *gorm.DB, cfg *config.Config) {
	SetupRoutesWithCertManager(router, db, cfg, nil, nil, nil)
}

// SetupRoutesWithCertManager sets up routes with certificate manager and optional per-cluster rate limiter (Finding #6).
// If publishSBOMCreated is non-nil, registers POST /api/v1/internal/trigger-cve-match to re-publish sbom.created for a given sbom_id.
func SetupRoutesWithCertManager(router *gin.Engine, db *gorm.DB, cfg *config.Config, certManager *security.CertManager, clusterLimiter *ingest.ClusterRateLimiter, publishSBOMCreated PublishSBOMCreatedFunc) {
	InitPodDetailEncryptionKey(cfg.PodDetailEncryptionKey)
	// Phase 2.1: in-memory cache for GET /risk/insights (list) and GET /risk/insights/summary (TTL 60s)
	defaultRisksCache = NewMemoryRisksCache(60 * time.Second)
	log.Printf("[API] ========================================")
	log.Printf("[API] SetupRoutesWithCertManager CALLED")
	log.Printf("[API] Setting up routes with CertManager")
	log.Printf("[API]   CertManager == nil: %v", certManager == nil)
	log.Printf("[API]   TLS_ENABLED: %v", cfg.TLSEnabled)
	log.Printf("[API] ========================================")

	p := func(perm authorization.Permission) gin.HandlerFunc {
		return middleware.RequirePermission(db, perm)
	}

	// Public auth routes
	auth := router.Group("/api/v1/auth")
	{
		auth.POST("/login", middleware.LoginRateLimit(), Login(db, cfg.JWTSecret, cfg.TokenExpirationHours))
		if cfg.AuthEnabled {
			auth.POST("/register", middleware.AuthMiddleware(db, cfg.JWTSecret), p(authorization.PermissionAuthRegister), Register(db, cfg.JWTSecret))
		} else {
			auth.POST("/register", Register(db, cfg.JWTSecret))
		}
	}

	ingestAuth := middleware.RequireIngestToken(cfg.IngestToken)

	// Agent ingest routes (no JWT) require FORTUNA_INGEST_TOKEN unless explicit local-dev unauth ingest is enabled.
	agent := router.Group("/api/v1/agent")
	agent.Use(ingestAuth)
	{
		agent.POST("/sync", SyncDataFromAgent(db, clusterLimiter))
		agent.POST("/pod-runtime-metrics", IngestPodRuntimeMetricsPayload(db))
		agent.POST("/pod-processes", IngestPodProcessesPayload(db))
		agent.POST("/pod-network-connections", IngestPodNetworkConnectionsPayload(db))
		agent.POST("/pod-events", IngestPodEventsPayload(db))
	}
	log.Printf("[API] Agent ingest routes registered: POST /api/v1/agent/sync, pod-runtime-metrics, pod-processes, pod-network-connections, pod-events")

	// Runtime ingest routes (no JWT): sensors/agents publish runtime events here.
	runtimeIngest := router.Group("/api/v1/runtime")
	runtimeIngest.Use(ingestAuth)
	{
		runtimeIngest.POST("/events", PostRuntimeEvents(db))
	}
	log.Printf("[API] Runtime ingest routes registered: POST /api/v1/runtime/events")

	runtimeIngestV2 := router.Group("/api/v2/runtime")
	runtimeIngestV2.Use(ingestAuth)
	{
		runtimeIngestV2.POST("/events", PostRuntimeEventsV2(db))
	}
	log.Printf("[API] Runtime ingest routes registered: POST /api/v2/runtime/events")

	// Protected routes — every handler is wrapped with explicit permission middleware (deny-by-default when auth enabled).
	v1 := router.Group("/api/v1")
	if cfg.AuthEnabled {
		v1.Use(middleware.AuthMiddleware(db, cfg.JWTSecret))
	} else {
		v1.Use(middleware.DevelopmentPrincipal())
	}
	{
		registerInventoryRoutes(v1, db, cfg)
		registerRuntimeRoutes(v1, db)
		registerRiskRoutes(v1, db)
		registerGraphRoutes(v1, db)
		registerAuditRoutes(v1, db)
		registerInvestigationRoutes(v1, db, p)
		registerPolicyRoutes(v1, db)

		malwareMgr := malware.NewManager(db)
		registerMalwareRoutes(v1, db, malwareMgr)

		v1.GET("/health/dashboard-data-integrity", p(authorization.PermissionObservabilityMetricsRead), DashboardDataIntegrity(db))
		v1.GET("/debug/technique-overlay", p(authorization.PermissionSystemDebug), GetTechniqueOverlayDigest)

		v1.GET("/me", p(authorization.PermissionAuthSession), GetCurrentUser())
		v1.POST("/change-password", p(authorization.PermissionAuthPasswordChange), ChangePassword(db))
		v1.GET("/users", p(authorization.PermissionUsersRead), GetUsers(db))
		v1.PATCH("/users/:id", middleware.RequireAnyPermission(db, authorization.PermissionUsersUpdate, authorization.PermissionUsersRoleAssign), PatchUser(db))
		v1.DELETE("/users/:id", p(authorization.PermissionUsersDelete), DeleteUser(db))

		v1.GET("/rbac/permission-catalog", p(authorization.PermissionSystemAuditRead), GetRBACPermissionCatalog)

		v1.GET("/sessions", p(authorization.PermissionSessionsRead), ListUserSessions(db))
		v1.DELETE("/sessions/:id", p(authorization.PermissionSessionsRevoke), RevokeUserSession(db))
		v1.DELETE("/sessions/revoke-all", middleware.RequireAnyPermission(db, authorization.PermissionSessionsRevoke, authorization.PermissionSessionsRevokeAll), RevokeAllUserSessions(db))

		v1.GET("/governance/security-activity", p(authorization.PermissionSystemAuditRead), ListSecurityActivity(db))
		v1.GET("/governance/investigation-events", p(authorization.PermissionSystemAuditRead), ListInvestigationEvents(db))
		v1.GET("/governance/permission-explorer", p(authorization.PermissionSystemAuditRead), GetGovernancePermissionExplorer())
		v1.GET("/governance/access-review", p(authorization.PermissionSystemAuditRead), GetGovernanceAccessReview(db))
		v1.GET("/governance/correlation-signals", p(authorization.PermissionSystemAuditRead), GetGovernanceCorrelationSignals(db))
		v1.GET("/governance/emergency-access", p(authorization.PermissionSystemAuditRead), EmergencyAccessPlaceholder)

		v1.GET("/capability-metadata", p(authorization.PermissionInventoryRead), GetCapabilityMetadataList(db))
		v1.GET("/capability-metadata/:capabilityId", p(authorization.PermissionInventoryRead), GetCapabilityMetadata(db))

		v1.GET("/promotion-rules", p(authorization.PermissionRulesRead), GetPromotionRulesList(db))
		v1.GET("/promotion-rules/capability/:capabilityId", p(authorization.PermissionRulesRead), GetPromotionRulesByCapability(db))
		v1.GET("/promotion-rules/signal/:signalType", p(authorization.PermissionRulesRead), GetPromotionRulesBySignalType(db))

		v1.GET("/ws/pod/:uid", p(authorization.PermissionInventoryRead), middleware.RequirePodUIDClusterScope(db, "uid"), PodDetailWS(db))
		v1.GET("/ws/risks", p(authorization.PermissionFindingsRead), RisksWS(db))

		v1.GET("/dashboard/stats", p(authorization.PermissionFindingsRead), GetDashboardStats(db))
		v1.GET("/dashboard/metrics/threat-velocity", p(authorization.PermissionFindingsRead), GetThreatVelocity(db))

		cluster := v1.Group("/cluster")
		cluster.Use(middleware.RequireClusterScope(db, "id"))
		cluster.GET("/info", p(authorization.PermissionInventoryRead), GetClusterInfo(db))
		cluster.GET("/:id/nodes", p(authorization.PermissionInventoryRead), GetClusterNodes(db))
		cluster.GET("/certificates/rotation/history", p(authorization.PermissionInventoryRead), GetCertificateRotationHistory(db))
		if certManager != nil {
			log.Printf("[API] Registering cluster certificate routes")
			certHandler := NewCertHandler(certManager)
			cluster.GET("/certificates/info", p(authorization.PermissionInventoryRead), certHandler.GetCertificateInfo)
			cluster.POST("/certificates/rotate", p(authorization.PermissionClusterCertificatesRotate), RotateCertificateHandler(db, certManager))
		}

		v1.GET("/metrics/system", p(authorization.PermissionObservabilityMetricsRead), GetSystemMetrics(db))
		v1.GET("/metrics/policy-evaluation-cost", p(authorization.PermissionObservabilityMetricsRead), GetPolicyEvaluationCost(db))
		v1.GET("/metrics/workers", p(authorization.PermissionObservabilityMetricsRead), GetWorkerStatus(db))
		v1.GET("/error-logs", p(authorization.PermissionObservabilityLogsRead), GetErrorLogs(db))
		v1.GET("/agents/status", p(authorization.PermissionObservabilityAgentsRead), GetAgentStatus(db))

		v1.GET("/resources", p(authorization.PermissionInventoryRead), GetResources(db))
		v1.GET("/resources/:kind/:uid", p(authorization.PermissionInventoryRead), GetResourceDetail(db))
		v1.POST("/bulk/serviceaccounts/disable", p(authorization.PermissionInventoryBulk), BulkDisableServiceAccounts(db))
		v1.DELETE("/bulk/serviceaccounts/delete", p(authorization.PermissionInventoryBulk), BulkDeleteServiceAccounts(db))
		v1.GET("/notifications", p(authorization.PermissionObservabilityMetricsRead), GetNotifications(db))
		v1.PATCH("/notifications/:id/read", p(authorization.PermissionObservabilityMetricsRead), MarkNotificationRead(db))
		v1.POST("/notifications/read-all", p(authorization.PermissionObservabilityMetricsRead), MarkAllNotificationsRead(db))
		v1.GET("/monitoring/agents", p(authorization.PermissionObservabilityAgentsRead), GetAgentStatus(db))
		v1.GET("/monitoring/pipeline-health", p(authorization.PermissionObservabilityMetricsRead), GetPipelineHealth(db))

		if publishSBOMCreated != nil {
			v1.POST("/internal/trigger-cve-match", p(authorization.PermissionInternalCVETrigger), TriggerCVEMatch(db, publishSBOMCreated))
		}
	}

	v2 := router.Group("/api/v2")
	if cfg.AuthEnabled {
		v2.Use(middleware.AuthMiddleware(db, cfg.JWTSecret))
	} else {
		v2.Use(middleware.DevelopmentPrincipal())
	}
	{
		registerRuntimeV2Routes(v2, db)
	}

	if strings.TrimSpace(os.Getenv("FORTUNA_SKIP_ROUTE_SECURITY_VERIFY")) != "true" {
		opts := RouteVerifyOptions{
			CertRoutesRegistered:    certManager != nil,
			CVEMatchRouteRegistered: publishSBOMCreated != nil,
		}
		if err := VerifyFortunaRouteSecurityContract(router, opts); err != nil {
			log.Fatalf("[API] route security contract verification failed: %v", err)
		}
		if err := WriteRouteSecurityReport(opts); err != nil {
			log.Fatalf("[API] route security report: %v", err)
		}
	} else {
		log.Printf("[API] WARNING: FORTUNA_SKIP_ROUTE_SECURITY_VERIFY=true — route security contract checks skipped")
	}
}
