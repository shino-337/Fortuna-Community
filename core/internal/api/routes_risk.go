package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/api/risk"
)

// registerRiskRoutes registers /api/v1/risk/* (analytics, scores, rules, insights, pod report, attack-steps, runtime-risk).
// Pod-scoped routes use :uid (Kubernetes pod UID).
func registerRiskRoutes(api *gin.RouterGroup, db *gorm.DB) {
	// Risk analytics (register first – more specific)
	api.GET("/risk/analytics/trends", risk.GetRiskTrendsAnalytics(db))
	api.GET("/risk/analytics/comparison", risk.GetRiskComparison(db))
	api.GET("/risk/analytics/correlation", risk.GetRiskCorrelation(db))
	api.GET("/risk/analytics/supply-chain", risk.GetSupplyChainCorrelation(db))
	api.GET("/risk/analytics/runtime-cve", risk.GetRuntimeCVECorrelation(db))
	api.GET("/risk/priorities", risk.GetPriorityStatistics(db))
	api.GET("/risk/top", risk.GetTopRisks(db))
	api.GET("/risk/grouped", risk.GetGroupedRisks(db))
	api.GET("/risk/scores", risk.GetRiskScores(db))
	api.POST("/risk/scores/sync", risk.SyncRiskScores(db))
	api.GET("/risk/scores/:uid", risk.GetRiskScore(db))
	api.POST("/risk/scores/:uid/calculate", risk.CalculateRiskScore(db))
	api.GET("/risk/trends", risk.GetRiskTrends(db))

	// Risk rules CRUD and YAML import/export (specific routes before :id)
	api.GET("/risk/rules", GetRiskRulesList(db))
	api.GET("/risk/rules/export", ExportRiskRulesYAML(db))
	api.POST("/risk/rules/validate", ValidateRiskRule())
	api.POST("/risk/rules/import", ImportRiskRule(db))
	api.GET("/risk/rules/:id", GetRiskRuleByID(db))
	api.POST("/risk/rules", CreateRiskRule(db))
	api.PUT("/risk/rules/:id", UpdateRiskRule(db))
	api.DELETE("/risk/rules/:id", DeleteRiskRule(db))

	// Risk histogram (score distribution for Risk Center chart)
	api.GET("/risk/histogram", GetRiskHistogram(db))

	// Insights (risks) — single source of truth
	api.GET("/risk/insights/export", ExportRisksCSV(db))
	api.GET("/risk/insights/summary/by-cluster", GetInsightsSummaryByCluster(db))
	api.GET("/risk/insights/summary/global", GetInsightsSummaryGlobalCached(db))
	api.GET("/risk/insights/summary", GetInsightsSummaryCached(db))
	api.POST("/risk/insights/evaluate/historical", TriggerHistoricalRiskEvaluation(db))
	api.POST("/risk/insights/evaluate", TriggerRiskEvaluation(db))
	api.GET("/risk/insights", GetInsightsListCached(db))
	api.GET("/risk/insights/:id", GetInsight(db))
	api.GET("/risk/insights/:id/context", GetInsightContext(db))
	api.DELETE("/risk/insights/:id", DeleteInsight(db))
	api.POST("/risk/insights/bulk", BulkInsightsAction(db))
	api.POST("/risk/insights/:id/acknowledge", AcknowledgeInsight(db))
	api.POST("/risk/insights/:id/resolve", ResolveInsight(db))
	api.POST("/risk/insights/:id/dismiss", DismissInsight(db))
	api.PATCH("/risk/insights/:id", UpdateInsightStatus(db))

	// Exception policies (RP-5: false-positive / dismiss suppression)
	api.POST("/risk/exceptions", CreateException(db))
	api.GET("/risk/exceptions", ListExceptions(db))
	api.DELETE("/risk/exceptions/:id", DeleteException(db))

	// Pod-scoped risk
	api.GET("/risk/pods/:uid/report", GetPodRiskReport(db))
	api.GET("/risk/pods/:uid/attack-steps", GetPodAttackSteps(db))
	api.GET("/risk/attack-steps/summary", GetAttackStepsSummary(db))

	// Runtime risk (pod UID)
	api.GET("/risk/pods/:uid/runtime", GetPodRiskProfile(db))
	api.GET("/risk/pods/:uid/runtime/events", GetPodRuntimeEvents(db))
	api.GET("/risk/runtime/summary", GetRuntimeRiskSummary(db))
	api.GET("/risk/runtime/top", GetTopRuntimeRisks(db))
}
