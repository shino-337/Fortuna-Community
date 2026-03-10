package api

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/api/policy"
	"github.com/fortuna/core/internal/api/risk"
	"github.com/fortuna/core/internal/config"
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
	SetupRoutesWithCertManager(router, db, cfg, nil)
}

// SetupRoutesWithCertManager sets up routes with certificate manager
func SetupRoutesWithCertManager(router *gin.Engine, db *gorm.DB, cfg *config.Config, certManager *security.CertManager) {
	InitPodDetailEncryptionKey(cfg.PodDetailEncryptionKey)
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

	// Protected routes
	v1 := router.Group("/api/v1")
	if cfg.AuthEnabled {
		v1.Use(middleware.AuthMiddleware(db, cfg.JWTSecret))
	}
	{
		// Risk Scores (MVP2) - Register FIRST to avoid route conflicts
		// Gin matches routes in order, so /risk must come before ANY parameter routes
		// CRITICAL: Use fmt.Fprintf to ensure logs appear immediately
		fmt.Fprintf(os.Stdout, "[API] ========================================\n")
		fmt.Fprintf(os.Stdout, "[API] Registering Risk API routes (FIRST)...\n")
		log.Printf("[API] ========================================")
		log.Printf("[API] Registering Risk API routes (FIRST)...")
		log.Printf("[API] Registering Risk API routes (FIRST)...")

		// Always register routes - if handlers are nil, Gin will panic which we'll catch
		// CRITICAL: Register more specific routes FIRST to avoid conflicts
		// MVP2 Phase 1.3: Risk Analytics APIs (register FIRST - more specific)
		fmt.Fprintf(os.Stdout, "[API] Registering Analytics routes...\n")
		log.Printf("[API] Registering Analytics routes...")

		// Register analytics routes directly (no nil check to ensure they're linked)
		v1.GET("/risk/analytics/trends", risk.GetRiskTrendsAnalytics(db))
		v1.GET("/risk/analytics/comparison", risk.GetRiskComparison(db))
		v1.GET("/risk/analytics/correlation", risk.GetRiskCorrelation(db))

		log.Printf("[API] Analytics routes registered")

		// MVP2 Phase 1.2: Risk Prioritization APIs
		v1.GET("/risk/priorities", risk.GetPriorityStatistics(db))
		v1.GET("/risk/top", risk.GetTopRisks(db))
		v1.GET("/risk/grouped", risk.GetGroupedRisks(db))

		// Basic risk routes (register AFTER specific routes)
		v1.GET("/risk/scores", risk.GetRiskScores(db))
		v1.GET("/risk/scores/:uid", risk.GetRiskScore(db))
		v1.POST("/risk/scores/:uid/calculate", risk.CalculateRiskScore(db))
		v1.GET("/risk/trends", risk.GetRiskTrends(db))
		fmt.Fprintf(os.Stdout, "[API] Risk API routes registered successfully\n")
		log.Printf("[API] Risk API routes registered successfully")
		fmt.Fprintf(os.Stdout, "[API] ========================================\n")
		log.Printf("[API] ========================================")

		// Health / data integrity (same handler as root /health/dashboard-data-integrity, for /api/v1 callers)
		v1.GET("/health/dashboard-data-integrity", DashboardDataIntegrity(db))

		// Current user
		v1.GET("/me", GetCurrentUser())
		v1.POST("/change-password", ChangePassword(db))
		// Users list (admin only)
		if cfg.AuthEnabled {
			v1.GET("/users", middleware.RequireAdmin(), GetUsers(db))
		} else {
			v1.GET("/users", GetUsers(db))
		}

		// Clusters
		v1.GET("/clusters", GetClusters(db))
		v1.GET("/clusters/stats", GetClustersStats(db))
		v1.GET("/clusters/:id/overview", GetClusterOverview(db))
		v1.GET("/clusters/:id/inventory", GetClusterInventory(db))
		v1.GET("/clusters/:id/agents", GetClusterAgents(db))
		v1.GET("/clusters/:id/security-summary", GetClusterSecuritySummary(db))
		v1.GET("/clusters/:id/nodes/:nodeName", GetClusterNode(db))
		v1.GET("/clusters/:id", GetCluster(db))

		// ServiceAccounts
		v1.GET("/serviceaccounts", GetServiceAccounts(db))
		v1.GET("/serviceaccounts/by-uid/:uid", GetServiceAccountByUID(db))
		v1.GET("/serviceaccounts/:id", GetServiceAccount(db))
		v1.GET("/serviceaccounts/:id/permissions", GetServiceAccountPermissions(db))
		v1.PUT("/serviceaccounts/:id", UpdateServiceAccount(db))
		v1.DELETE("/serviceaccounts/:id", DeleteServiceAccount(db))

		// Bulk operations (admin only)
		v1.POST("/serviceaccounts/bulk/disable", middleware.RequireAdmin(), BulkDisableServiceAccounts(db))
		v1.POST("/serviceaccounts/bulk/delete", middleware.RequireAdmin(), BulkDeleteServiceAccounts(db))
		v1.POST("/serviceaccounts/disable-inactive", middleware.RequireAdmin(), DisableInactiveServiceAccounts(db))

		// Attack Steps (Phase 2) - Register BEFORE /pods/:id to avoid route conflict
		v1.GET("/attack-steps/pods/:podUid", GetPodAttackSteps(db))
		v1.GET("/attack-steps/summary", GetAttackStepsSummary(db))

		// Capability Metadata (Phase 2)
		v1.GET("/capability-metadata", GetCapabilityMetadataList(db))
		v1.GET("/capability-metadata/:capabilityId", GetCapabilityMetadata(db))

		// Promotion Rules (Phase 2.3)
		v1.GET("/promotion-rules", GetPromotionRulesList(db))
		v1.GET("/promotion-rules/capability/:capabilityId", GetPromotionRulesByCapability(db))
		v1.GET("/promotion-rules/signal/:signalType", GetPromotionRulesBySignalType(db))

		// Runtime Signals (Phase 2.3)
		v1.GET("/runtime-signals", GetRuntimeSignalsList(db))
		v1.GET("/runtime-signals/pods/:podUid", GetRuntimeSignalsByPod(db))

		// Pods
		v1.GET("/pods", GetPods(db))
		// Pod Detail WebSocket (Phase 5.1) – push on ingest; register before other pod routes
		v1.GET("/ws/pod/:uid", PodDetailWS())
		// Pod Detail by-uid (register before /pods/:id so "by-uid" is not captured as id)
		v1.GET("/pods/by-uid/:uid/runtime-metrics", GetPodRuntimeMetricsByUID(db))
		v1.GET("/pods/by-uid/:uid/processes", GetPodProcessesByUID(db))
		v1.GET("/pods/by-uid/:uid/network-connections", GetPodNetworkConnectionsByUID(db))
		v1.GET("/pods/by-uid/:uid/events", GetPodEventsByUID(db))
		v1.GET("/pods/by-uid/:uid/spec", GetPodSpecYAMLByUID(db))
		v1.GET("/pods/by-uid/:uid", GetPodByUID(db))
		// Pod Detail by id (numeric)
		v1.GET("/pods/:id/runtime-metrics", GetPodRuntimeMetrics(db))
		v1.GET("/pods/:id/processes", GetPodProcesses(db))
		v1.GET("/pods/:id/network-connections", GetPodNetworkConnections(db))
		v1.GET("/pods/:id/events", GetPodEvents(db))
		v1.GET("/pods/:id/spec", GetPodSpecYAML(db))
		v1.GET("/pods/:id", GetPod(db))
		v1.GET("/pods/:id/capabilities", GetPodCapabilities(db))
		v1.POST("/runtime-events", PostRuntimeEvents(db))
		v1.GET("/pod-capabilities", GetPodCapabilitiesList(db))
		v1.GET("/pod-capabilities/summary", GetPodCapabilitiesSummary(db))
		v1.GET("/pod-capabilities/summary/cluster", GetPodCapabilitiesSummaryByCluster(db))
		v1.GET("/pod-capabilities/summary/capability", GetPodCapabilitiesSummaryByCapability(db))
		v1.GET("/pod-capabilities/summary/namespace", GetPodCapabilitiesSummaryByNamespace(db))
		v1.GET("/pod-capabilities/summary/severity", GetPodCapabilitiesSummaryBySeverity(db))
		v1.GET("/pod-capabilities/trends", GetPodCapabilitiesTrend(db))

		// Runtime Risk Profiles
		v1.GET("/runtime-risk/pods/:podUid", GetPodRiskProfile(db))
		v1.GET("/runtime-risk/pods/:podUid/events", GetPodRuntimeEvents(db))
		v1.GET("/runtime-risk/summary", GetRuntimeRiskSummary(db))
		v1.GET("/runtime-risk/top", GetTopRuntimeRisks(db))

		// SBOM Analysis dashboards
		v1.GET("/sbom", GetSBOMList(db))
		v1.GET("/sbom/:podId", GetSBOMDetail(db))

		// Dashboard summaries
		v1.GET("/dashboard/stats", GetDashboardStats(db))
		v1.GET("/dashboard/metrics/threat-velocity", GetThreatVelocity(db))

		// RisK management
		v1.GET("/risks", GetInsightsList(db))
		v1.PATCH("/risks/:riskId", UpdateInsightStatus(db))
		v1.GET("/risks/pods/:podUid/report", GetPodRiskReport(db))

		// Attack path visualization
		v1.GET("/attack-paths/graph", AttackPathsGraph(db))

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
		// Advanced graph queries
		v1.GET("/graph/attack-paths/:uid", GetAttackPaths(db))
		v1.GET("/graph/permissions/:uid", GetServiceAccountPermissionsGraph(db))
		v1.GET("/graph/risky-pods", GetRiskyPods(db))

		// Audit
		v1.GET("/audit", GetAuditLogs(db))
		v1.GET("/audit/reports", GetAuditReports(db))
		v1.GET("/audit-logs", GetAuditLogs(db))
		v1.GET("/reports", GetAuditReports(db))

		// Insights
		v1.GET("/insights", GetInsights(db))
		v1.GET("/insights/summary", GetInsightsSummary(db))
		v1.GET("/insights/:id", GetInsight(db))
		v1.DELETE("/insights/:id", DeleteInsight(db))
		v1.POST("/insights/evaluate", TriggerRiskEvaluation(db))                      // Uses HistoricalRiskEvaluator (pkg/riskengine)
		v1.POST("/insights/evaluate/historical", TriggerHistoricalRiskEvaluation(db)) // Uses Risk Worker engine
		v1.POST("/insights/:id/acknowledge", AcknowledgeInsight(db))
		v1.POST("/insights/:id/resolve", ResolveInsight(db))
		v1.POST("/insights/:id/dismiss", DismissInsight(db))

		// Rules Management
		v1.GET("/rules", GetRules(db))
		v1.GET("/rules/:id", GetRule(db))
		v1.POST("/rules", CreateRule(db))
		v1.PUT("/rules/:id", UpdateRule(db))
		v1.DELETE("/rules/:id", DeleteRule(db))
		v1.POST("/rules/:id/test", TestRule(db))
		v1.POST("/rules/reload", ReloadRules(db))
		v1.GET("/rules/:id/metrics", GetRuleMetrics(db))
		v1.GET("/rules/:id/matches", GetRuleMatches(db))

		// Policy Management (MVP2 Phase 2)
		log.Printf("[API] Registering Policy API routes...")
		policyHandler := policy.NewPolicyHandler(db)
		policies := v1.Group("/policies")
		{
			// Policy Templates
			policies.GET("/templates", policyHandler.ListTemplates)
			policies.GET("/templates/:templateId", policyHandler.GetTemplate)
			policies.POST("/templates", policyHandler.CreateTemplate)
			policies.PUT("/templates/:templateId/:version", policyHandler.UpdateTemplate)
			policies.DELETE("/templates/:templateId/:version", policyHandler.DeleteTemplate)

			// Policy Instances
			policies.GET("/instances", policyHandler.ListInstances)
			policies.GET("/instances/:instanceName", policyHandler.GetInstance)
			policies.POST("/instances", policyHandler.CreateInstance)
			policies.PUT("/instances/:instanceName", policyHandler.UpdateInstance)
			policies.DELETE("/instances/:instanceName", policyHandler.DeleteInstance)
		}
		log.Printf("[API] ✅ Policy API routes registered")

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

		// Certificate rotation history (empty until rotation_history table; real API, no mock)
		v1.GET("/certificates/rotation/history", GetCertificateRotationHistory(db))

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

	// Agent endpoints (no auth required for now, can add token-based auth later)
	agent := router.Group("/api/v1/agent")
	{
		agent.POST("/sync", SyncDataFromAgent(db))
		// Pod Detail ingest (metrics, processes, network, events)
		agent.POST("/pod-runtime-metrics", IngestPodRuntimeMetricsPayload(db))
		agent.POST("/pod-processes", IngestPodProcessesPayload(db))
		agent.POST("/pod-network-connections", IngestPodNetworkConnectionsPayload(db))
		agent.POST("/pod-events", IngestPodEventsPayload(db))
	}
}
