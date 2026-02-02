package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// DashboardDataIntegrityResponse is the response for GET /health/dashboard-data-integrity.
// It allows operators to verify that dashboard data is traceable and identifies stubs/placeholders.
type DashboardDataIntegrityResponse struct {
	Timestamp   time.Time              `json:"timestamp"`
	CrossChecks CrossChecks            `json:"crossChecks"`
	Alerts      []string               `json:"alerts"`
	Endpoints   []EndpointDataSource   `json:"endpoints"`
}

type CrossChecks struct {
	ActiveAgentsCount    int64 `json:"activeAgentsCount"`
	DashboardAgentsCount int64 `json:"dashboardAgentsCount"` // same source as dashboard stats
	ClustersCount        int64 `json:"clustersCount"`        // active clusters (recent sync)
	PodsCount            int64 `json:"podsCount"`
	InsightsCount        int64 `json:"insightsCount"`
	CriticalInsights     int64 `json:"criticalInsights"`
}

type EndpointDataSource struct {
	Path     string `json:"path"`
	Source   string `json:"source"`   // "real" | "stub" | "placeholder"
	Agent    string `json:"agent"`    // which agent produces data, or ""
	Note     string `json:"note,omitempty"`
}

// DashboardDataIntegrity returns cross-checks and endpoint inventory for dashboard data integrity.
// GET /health/dashboard-data-integrity
func DashboardDataIntegrity(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		resp := DashboardDataIntegrityResponse{
			Timestamp: time.Now(),
			Alerts:    []string{},
			Endpoints: endpointInventory(),
		}

		// Cross-checks: active agents vs dashboard-visible data
		if db.Migrator().HasTable("agents") {
			tenMin := time.Now().Add(-10 * time.Minute)
			db.Model(&models.Agent{}).
				Where("deleted_at IS NULL AND status = ? AND (last_seen_at > ? OR last_seen_at IS NULL)", "ready", tenMin).
				Count(&resp.CrossChecks.ActiveAgentsCount)
			resp.CrossChecks.DashboardAgentsCount = resp.CrossChecks.ActiveAgentsCount
		}

		if db.Migrator().HasTable("clusters") {
			cutoff := time.Now().Add(-7 * 24 * time.Hour)
			db.Table("clusters").Where("last_sync >= ?", cutoff).Count(&resp.CrossChecks.ClustersCount)
		}
		db.Table("pods").Where("deleted_at IS NULL").Count(&resp.CrossChecks.PodsCount)
		db.Model(&models.Insight{}).Where("insight_type = ? AND deleted_at IS NULL", "vulnerability").Count(&resp.CrossChecks.InsightsCount)
		db.Model(&models.Insight{}).
			Where("insight_type = ? AND LOWER(severity) = ? AND deleted_at IS NULL", "vulnerability", "critical").
			Count(&resp.CrossChecks.CriticalInsights)

		// Alerts: data exists but no agents
		if resp.CrossChecks.InsightsCount > 0 && resp.CrossChecks.ActiveAgentsCount == 0 {
			resp.Alerts = append(resp.Alerts, "data_exists_no_agents: insights exist but no active agents in last 10m")
		}
		if resp.CrossChecks.PodsCount > 0 && resp.CrossChecks.ActiveAgentsCount == 0 {
			resp.Alerts = append(resp.Alerts, "data_exists_no_agents: pods exist but no active agents in last 10m")
		}

		c.JSON(http.StatusOK, resp)
	}
}

func endpointInventory() []EndpointDataSource {
	return []EndpointDataSource{
		{Path: "/api/v1/dashboard/stats", Source: "real", Agent: "agent (sync)", Note: "clusters/pods/agents/insights from DB"},
		{Path: "/api/v1/clusters", Source: "real", Agent: "agent (sync)", Note: "clusters from DB, filtered by last_sync"},
		{Path: "/api/v1/agents/status", Source: "real", Agent: "agent (heartbeat)", Note: "agents table"},
		{Path: "/api/v1/risks", Source: "real", Agent: "core (insights)", Note: "insights table"},
		{Path: "/api/v1/sbom", Source: "real", Agent: "agent (SBOM)", Note: "sbom from agent"},
		{Path: "/api/v1/resources", Source: "real", Agent: "agent (sync)", Note: "resources from sync"},
		{Path: "/api/v1/rules", Source: "real", Agent: "core", Note: "rules from DB"},
		{Path: "/api/v1/audit", Source: "real", Agent: "core", Note: "audit_logs table"},
		{Path: "/api/v1/reports", Source: "real", Agent: "core", Note: "aggregated from audit_logs"},
		{Path: "/api/v1/metrics/system", Source: "real", Agent: "core", Note: "DB aggregates"},
		{Path: "/api/v1/dashboard/metrics/threat-velocity", Source: "real", Agent: "core", Note: "insights by severity/date"},
		{Path: "/api/v1/pod-capabilities/summary/*", Source: "real", Agent: "agent (PCE)", Note: "pod_capabilities"},
		{Path: "/api/v1/attack-paths/graph", Source: "real", Agent: "core", Note: "graph from DB"},
		{Path: "/api/v1/notifications", Source: "stub", Agent: "", Note: "no backend table; returns empty"},
		{Path: "/api/v1/metrics/workers", Source: "placeholder", Agent: "", Note: "requires Prometheus; returns unsupported"},
		{Path: "/api/v1/metrics/queue", Source: "placeholder", Agent: "", Note: "requires Prometheus; returns unsupported"},
		{Path: "/api/v1/error-logs", Source: "stub", Agent: "", Note: "no backend; returns unsupported"},
		{Path: "/api/v1/certificates/info", Source: "real", Agent: "core", Note: "from CertManager when TLS enabled"},
		{Path: "/api/v1/certificates/rotation/history", Source: "stub", Agent: "", Note: "not implemented; 404"},
		{Path: "/api/v1/users", Source: "stub", Agent: "", Note: "no route; 404"},
	}
}
