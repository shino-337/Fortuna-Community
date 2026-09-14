package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/api/risk"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
)

// registerRiskRoutes registers /api/v1/risk/* (analytics, scores, rules, insights, pod report, attack-steps, runtime-risk).
// Pod-scoped routes use :uid (Kubernetes pod UID).
func registerRiskRoutes(api *gin.RouterGroup, db *gorm.DB) {
	p := func(perm authorization.Permission) gin.HandlerFunc {
		return middleware.RequirePermission(db, perm)
	}

	api.GET("/risk/analytics/trends", p(authorization.PermissionFindingsRead), risk.GetRiskTrendsAnalytics(db))
	api.GET("/risk/analytics/comparison", p(authorization.PermissionFindingsRead), risk.GetRiskComparison(db))
	api.GET("/risk/analytics/correlation", p(authorization.PermissionFindingsRead), risk.GetRiskCorrelation(db))
	api.GET("/risk/analytics/supply-chain", p(authorization.PermissionFindingsRead), risk.GetSupplyChainCorrelation(db))
	api.GET("/risk/analytics/runtime-cve", p(authorization.PermissionFindingsRead), risk.GetRuntimeCVECorrelation(db))
	api.GET("/risk/priorities", p(authorization.PermissionFindingsRead), risk.GetPriorityStatistics(db))
	api.GET("/risk/top", p(authorization.PermissionFindingsRead), risk.GetTopRisks(db))
	api.GET("/risk/grouped", p(authorization.PermissionFindingsRead), risk.GetGroupedRisks(db))
	api.GET("/risk/scores", p(authorization.PermissionFindingsRead), risk.GetRiskScores(db))
	api.POST("/risk/scores/sync", p(authorization.PermissionRiskEvaluate), risk.SyncRiskScores(db))
	api.GET("/risk/scores/:uid", p(authorization.PermissionFindingsRead), middleware.RequirePodUIDClusterScope(db, "uid"), risk.GetRiskScore(db))
	api.POST("/risk/scores/:uid/calculate", p(authorization.PermissionRiskEvaluate), middleware.RequirePodUIDClusterScope(db, "uid"), risk.CalculateRiskScore(db))
	api.GET("/risk/trends", p(authorization.PermissionFindingsRead), risk.GetRiskTrends(db))

	api.GET("/risk/rules", p(authorization.PermissionRulesRead), GetRiskRulesList(db))
	api.GET("/risk/rules/export", p(authorization.PermissionRulesExport), ExportRiskRulesYAML(db))
	api.POST("/risk/rules/validate", p(authorization.PermissionRulesRead), ValidateRiskRule())
	api.POST("/risk/rules/import", p(authorization.PermissionRulesImport), ImportRiskRule(db))
	api.GET("/risk/rules/:id", p(authorization.PermissionRulesRead), GetRiskRuleByID(db))
	api.POST("/risk/rules", p(authorization.PermissionRulesWrite), CreateRiskRule(db))
	api.PUT("/risk/rules/:id", p(authorization.PermissionRulesWrite), UpdateRiskRule(db))
	api.DELETE("/risk/rules/:id", p(authorization.PermissionRulesDelete), DeleteRiskRule(db))

	api.GET("/risk/histogram", p(authorization.PermissionFindingsRead), GetRiskHistogram(db))

	api.GET("/risk/insights/export", p(authorization.PermissionExportFindings), ExportRisksCSV(db))
	api.GET("/risk/insights/summary/by-cluster", p(authorization.PermissionFindingsRead), GetInsightsSummaryByCluster(db))
	api.GET("/risk/insights/summary/global", p(authorization.PermissionFindingsRead), GetInsightsSummaryGlobalCached(db))
	api.GET("/risk/insights/summary", p(authorization.PermissionFindingsRead), GetInsightsSummaryCached(db))
	api.POST("/risk/insights/evaluate/historical", p(authorization.PermissionRiskEvaluate), TriggerHistoricalRiskEvaluation(db))
	api.POST("/risk/insights/evaluate", p(authorization.PermissionRiskEvaluate), TriggerRiskEvaluation(db))
	api.GET("/risk/insights", p(authorization.PermissionFindingsRead), GetInsightsListCached(db))
	api.GET("/risk/insights/:id", p(authorization.PermissionFindingsRead), GetInsight(db))
	api.GET("/risk/insights/:id/context", p(authorization.PermissionFindingsRead), GetInsightContext(db))
	api.DELETE("/risk/insights/:id", p(authorization.PermissionFindingsDelete), DeleteInsight(db))
	api.POST("/risk/insights/bulk", p(authorization.PermissionFindingsBulk), BulkInsightsAction(db))
	api.POST("/risk/insights/:id/acknowledge", p(authorization.PermissionFindingsAck), AcknowledgeInsight(db))
	api.POST("/risk/insights/:id/resolve", p(authorization.PermissionFindingsResolve), ResolveInsight(db))
	api.POST("/risk/insights/:id/dismiss", p(authorization.PermissionFindingsDismiss), DismissInsight(db))
	api.PATCH("/risk/insights/:id", middleware.RequireAnyPermission(db, authorization.PermissionFindingsAck, authorization.PermissionFindingsDismiss, authorization.PermissionFindingsResolve, authorization.PermissionFindingsReopen), UpdateInsightStatus(db))

	api.POST("/risk/exceptions", p(authorization.PermissionFindingsExceptionCreate), CreateException(db))
	api.GET("/risk/exceptions", p(authorization.PermissionFindingsRead), ListExceptions(db))
	api.DELETE("/risk/exceptions/:id", p(authorization.PermissionFindingsExceptionDelete), DeleteException(db))

	riskPods := api.Group("/risk/pods")
	riskPods.Use(middleware.RequirePodUIDClusterScope(db, "uid"))
	riskPods.GET("/:uid/report", p(authorization.PermissionFindingsRead), GetPodRiskReport(db))
	riskPods.GET("/:uid/attack-steps", p(authorization.PermissionFindingsRead), GetPodAttackSteps(db))
	api.GET("/risk/attack-steps/summary", p(authorization.PermissionFindingsRead), GetAttackStepsSummary(db))

	riskPods.GET("/:uid/runtime", p(authorization.PermissionRuntimeRead), GetPodRiskProfile(db))
	riskPods.GET("/:uid/runtime/events", p(authorization.PermissionRuntimeRead), GetPodRuntimeEvents(db))
	api.GET("/risk/runtime/summary", p(authorization.PermissionRuntimeRead), GetRuntimeRiskSummary(db))
	api.GET("/risk/runtime/top", p(authorization.PermissionRuntimeRead), GetTopRuntimeRisks(db))
}
