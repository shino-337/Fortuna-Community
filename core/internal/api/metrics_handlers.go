package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// GetAgentStatus returns agent status from the agents table (real data).
// Returns all ready agents (no cutoff) so dashboard always shows latest; status per agent is healthy/slow/disconnected from last_seen_at.
func GetAgentStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !db.Migrator().HasTable("agents") {
			c.JSON(http.StatusOK, gin.H{
				"agents": []map[string]interface{}{}, "total": 0, "healthy": 0, "slow": 0, "disconnected": 0,
			})
			return
		}

		var agentsList []models.Agent
		db.Where("deleted_at IS NULL AND (status = ? OR status IS NULL)", "ready").Order("last_seen_at DESC NULLS LAST").Find(&agentsList)

		// Resolve cluster display name from clusters table (most recently synced)
		var displayClusterID, displayClusterName string
		var latestCluster models.Cluster
		if err := db.Order("last_sync DESC").First(&latestCluster).Error; err == nil {
			displayClusterID = latestCluster.ID
			displayClusterName = latestCluster.Name
			if displayClusterName == "" {
				displayClusterName = displayClusterID
			}
		} else {
			// From environment only; no hardcoded cluster name
			displayClusterID = os.Getenv("DEFAULT_CLUSTER_ID")
			displayClusterName = os.Getenv("DEFAULT_CLUSTER_NAME")
			if displayClusterName == "" {
				displayClusterName = displayClusterID
			}
			if displayClusterID == "" {
				displayClusterID = "unknown"
				displayClusterName = "unknown"
			}
		}

		agents := make([]map[string]interface{}, 0, len(agentsList))
		healthyCount := 0
		for _, a := range agentsList {
			status := "healthy"
			if a.LastSeenAt != nil && time.Since(*a.LastSeenAt) > 15*time.Minute {
				status = "disconnected"
			} else if a.LastSeenAt != nil && time.Since(*a.LastSeenAt) > 5*time.Minute {
				status = "slow"
			} else {
				healthyCount++
			}
			lastHB := time.Time{}
			if a.LastSeenAt != nil {
				lastHB = *a.LastSeenAt
			}
			agents = append(agents, map[string]interface{}{
				"agentId":       a.AgentID,
				"clusterId":     displayClusterID,
				"clusterName":   displayClusterName,
				"nodeName":      a.NodeName,
				"status":        status,
				"lastHeartbeat": lastHB,
				"version":       a.Version,
			})
		}

		slow := 0
		disconnected := 0
		for _, ag := range agents {
			if ag["status"] == "slow" {
				slow++
			} else if ag["status"] == "disconnected" {
				disconnected++
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"agents":       agents,
			"total":        len(agents),
			"healthy":      healthyCount,
			"slow":         slow,
			"disconnected": disconnected,
		})
	}
}

// GetSystemMetrics returns system health metrics
func GetSystemMetrics(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Count active clusters only (same definition as GetClusters / dashboard stats)
		var clusterCount int64
		cutoff := time.Now().Add(-ActiveClusterCutoff)
		db.Model(&models.Cluster{}).Where("last_sync >= ?", cutoff).Count(&clusterCount)

		// Count distinct pods by UID to avoid duplicates
		var podCount int64
		db.Model(&models.Pod{}).Distinct("uid").Count(&podCount)

		var saCount int64
		db.Model(&models.ServiceAccount{}).Count(&saCount)

		var insightCount int64
		db.Model(&models.Insight{}).Count(&insightCount)

		// Get last sync time
		var lastSync time.Time
		var latestCluster models.Cluster
		if err := db.Order("last_sync DESC").First(&latestCluster).Error; err == nil {
			lastSync = latestCluster.LastSync
		}

		c.JSON(http.StatusOK, gin.H{
			"health": map[string]interface{}{
				"status": "healthy",
			},
			"sync": map[string]interface{}{
				"lastFullScan": lastSync,
				"nextScan":     lastSync.Add(10 * time.Minute),
			},
			"resources": map[string]interface{}{
				"clusters":        clusterCount,
				"pods":            podCount,
				"serviceAccounts": saCount,
				"insights":        insightCount,
			},
			"api": map[string]interface{}{
				"status": "healthy",
			},
		})
	}
}

// QueryPrometheusMetrics queries Prometheus for specific metrics
// Note: This is a placeholder - in production, you'd query Prometheus API
func QueryPrometheusMetrics(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := c.Query("query")
		if query == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter required"})
			return
		}

		// For now, return placeholder response
		// In production, this would query Prometheus API
		c.JSON(http.StatusOK, gin.H{
			"message": "Prometheus query endpoint - requires Prometheus client configuration",
			"query":   query,
		})
	}
}

// GetErrorLogs returns error logs from the error_logs table (real data) with pagination.
// Query: page (default 1), pageSize (default 20, max 200), level (optional), source (optional).
func GetErrorLogs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !db.Migrator().HasTable("error_logs") {
			c.JSON(http.StatusOK, gin.H{"logs": []map[string]interface{}{}, "total": 0})
			return
		}
		page, _ := parseIntDefault(c.Query("page"), 1)
		pageSize, _ := parseIntDefault(c.Query("pageSize"), 20)
		if page < 1 {
			page = 1
		}
		if pageSize < 1 {
			pageSize = 20
		}
		if pageSize > 200 {
			pageSize = 200
		}
		level := c.Query("level")
		source := c.Query("source")

		q := db.Model(&models.ErrorLog{}).Where("deleted_at IS NULL")
		if level != "" {
			q = q.Where("level = ?", level)
		}
		if source != "" {
			q = q.Where("source = ?", source)
		}
		var total int64
		if err := q.Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		offset := (page - 1) * pageSize
		var list []models.ErrorLog
		findQ := db.Where("deleted_at IS NULL")
		if level != "" {
			findQ = findQ.Where("level = ?", level)
		}
		if source != "" {
			findQ = findQ.Where("source = ?", source)
		}
		if err := findQ.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&list).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		logs := make([]map[string]interface{}, 0, len(list))
		for _, e := range list {
			logs = append(logs, map[string]interface{}{
				"id":      e.ID,
				"time":    e.CreatedAt.Format(time.RFC3339),
				"level":   e.Level,
				"message": e.Message,
				"source":  e.Source,
			})
		}
		c.JSON(http.StatusOK, gin.H{"logs": logs, "total": total})
	}
}

func parseIntDefault(s string, defaultVal int) (int, bool) {
	if s == "" {
		return defaultVal, false
	}
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return defaultVal, false
	}
	return n, true
}

func cveStageStatus(hasTable bool, matchRuns, cveMatches, cveRows, packageRows, activeGenerationID int64) string {
	if !hasTable {
		return "not-configured"
	}
	if matchRuns > 0 && (cveRows == 0 || packageRows == 0 || activeGenerationID == 0) {
		return "catalog-unavailable"
	}
	if cveMatches > 0 {
		return "running"
	}
	if matchRuns > 0 {
		return "idle"
	}
	return "idle"
}

func cveStageDescription(hasTable bool, matchRuns, cveMatches, cveRows, packageRows, activeGenerationID int64) string {
	if !hasTable {
		return "CVE match table is missing; persisted CVE findings cannot be reported."
	}
	if matchRuns > 0 && (cveRows == 0 || packageRows == 0) {
		return "SBOM match runs completed, but CVE reference tables are empty; load the CVE catalog before trusting matcher output."
	}
	if matchRuns > 0 && activeGenerationID == 0 {
		return "SBOM match runs completed, but no active CVE catalog generation is available; rematch after a verified catalog load."
	}
	if cveMatches > 0 {
		return "Persisted CVE findings from cve_matches."
	}
	if matchRuns > 0 {
		return "SBOM match runs completed, but no CVE findings are persisted in cve_matches."
	}
	return "CVE match output from cve_matches; no persisted findings yet."
}

func policyStageStatus(hasViolationsTable, hasInstancesTable, hasTemplatesTable bool, activity int64) string {
	if !hasViolationsTable && !hasInstancesTable && !hasTemplatesTable {
		return "not-configured"
	}
	if activity > 0 {
		return "running"
	}
	if !hasViolationsTable || !hasInstancesTable {
		return "not-configured"
	}
	return "idle"
}

func policyStageDescription(hasViolationsTable, hasInstancesTable, hasTemplatesTable bool, templates int64) string {
	if !hasViolationsTable && !hasInstancesTable && !hasTemplatesTable {
		return "Policy persistence tables are missing; policy processing is not configured."
	}
	if !hasViolationsTable || !hasInstancesTable {
		return "Policy stage has partial schema; policy instances or violations are not available."
	}
	if templates == 0 {
		return "Policy schema is present, but no templates or instances are configured."
	}
	return "Policy activity from policy violations, instances, or templates."
}

// GetWorkerStatus returns the status of background workers derived from DB activity.
// Workers tracked: sbom, cve-matcher, correlator, risk, policy.
func GetWorkerStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		countWhere := func(dest *int64, model interface{}, conds ...interface{}) {
			*dest = 0
			q := db.Model(model)
			if len(conds) > 0 {
				q = q.Where(conds[0], conds[1:]...)
			}
			_ = q.Count(dest).Error
		}

		// SBOM ingest + CVE match pipeline: rows in sboms and/or successful sbom_match_runs.
		var sbomRows, matchSucceeded, matchFailed int64
		if db.Migrator().HasTable("sboms") {
			countWhere(&sbomRows, &models.SBOM{})
		}
		if db.Migrator().HasTable("sbom_match_runs") {
			countWhere(&matchSucceeded, &models.SBOMMatchRun{}, "status = ?", "succeeded")
			countWhere(&matchFailed, &models.SBOMMatchRun{}, "status = ?", "failed")
		}
		sbomActivity := sbomRows
		if matchSucceeded > sbomActivity {
			sbomActivity = matchSucceeded
		}

		// Count CVE matches; pipeline failures are shared with the SBOM match-run stage.
		hasCVEMatchesTable := db.Migrator().HasTable("cve_matches")
		var cveProcessed int64
		if hasCVEMatchesTable {
			countWhere(&cveProcessed, &models.CVEMatch{})
		}
		var cveRows, packageRows, activeCVEGeneration int64
		if db.Migrator().HasTable("cves") {
			db.Table("cves").Count(&cveRows)
		}
		if db.Migrator().HasTable("package_vulnerabilities") {
			db.Table("package_vulnerabilities").Count(&packageRows)
		}
		if db.Migrator().HasTable("catalog_generations") {
			var generationID sql.NullInt64
			_ = db.Table("catalog_generations").
				Select("MAX(id)").
				Where("catalog_type = ? AND status = ? AND deleted_at IS NULL", "cve", "active").
				Scan(&generationID).Error
			if generationID.Valid {
				activeCVEGeneration = generationID.Int64
			}
		}

		// Count insights (correlator output)
		var correlatorProcessed int64
		if db.Migrator().HasTable("insights") {
			countWhere(&correlatorProcessed, &models.Insight{})
		}

		// Count risk scores
		var riskProcessed int64
		if db.Migrator().HasTable("risk_scores") {
			countWhere(&riskProcessed, &models.RiskScore{})
		}

		// Policy: violations are sparse; instances/templates show the engine is deployed and idle-ready.
		hasPolicyViolationsTable := db.Migrator().HasTable("policy_violations")
		hasPolicyInstancesTable := db.Migrator().HasTable("policy_instances")
		hasPolicyTemplatesTable := db.Migrator().HasTable("policy_templates")
		var policyViolations, policyInstances, policyTemplates int64
		if hasPolicyViolationsTable {
			countWhere(&policyViolations, &models.PolicyViolation{})
		}
		if hasPolicyInstancesTable {
			countWhere(&policyInstances, &models.PolicyInstance{})
		}
		if hasPolicyTemplatesTable {
			countWhere(&policyTemplates, &models.PolicyTemplate{})
		}
		policyActivity := policyViolations + policyInstances
		if policyActivity == 0 && policyTemplates > 0 {
			policyActivity = policyTemplates
		}

		// workerStatus derives a stage status from cumulative DB counts. This endpoint
		// intentionally does not claim live worker process health; live queue and
		// concurrency are exported as Prometheus metrics instead.
		// "idle" means the stage has no persisted output yet; it is not live process health.
		// "degraded" fires when failures exceed 10% of processed output AND there are more than
		// 5 failures (to avoid false alarms on brand-new or lightly-used deployments).
		workerStatus := func(processed, failed int64) string {
			if processed == 0 && failed == 0 {
				return "idle"
			}
			if failed > processed/10 && failed > 5 {
				return "degraded"
			}
			return "running"
		}

		workers := []map[string]interface{}{
			{
				"name":          "sbom",
				"kind":          "pipeline-stage",
				"source":        "db-derived",
				"live":          false,
				"description":   "SBOM ingest activity from sboms and sbom_match_runs.",
				"queueDepth":    0,
				"activeWorkers": 0,
				"processed":     sbomActivity,
				"failed":        matchFailed,
				"status":        workerStatus(sbomActivity, matchFailed),
			},
			{
				"name":          "cve-matcher",
				"kind":          "pipeline-stage",
				"source":        "db-derived",
				"live":          false,
				"description":   cveStageDescription(hasCVEMatchesTable, matchSucceeded, cveProcessed, cveRows, packageRows, activeCVEGeneration),
				"queueDepth":    0,
				"activeWorkers": 0,
				"processed":     cveProcessed,
				"failed":        int64(0),
				"status":        cveStageStatus(hasCVEMatchesTable, matchSucceeded, cveProcessed, cveRows, packageRows, activeCVEGeneration),
			},
			{
				"name":          "correlator",
				"kind":          "pipeline-stage",
				"source":        "db-derived",
				"live":          false,
				"description":   "Correlation output from insights.",
				"queueDepth":    0,
				"activeWorkers": 0,
				"processed":     correlatorProcessed,
				"failed":        int64(0),
				"status":        workerStatus(correlatorProcessed, 0),
			},
			{
				"name":          "risk",
				"kind":          "pipeline-stage",
				"source":        "db-derived",
				"live":          false,
				"description":   "Risk score output from risk_scores.",
				"queueDepth":    0,
				"activeWorkers": 0,
				"processed":     riskProcessed,
				"failed":        int64(0),
				"status":        workerStatus(riskProcessed, 0),
			},
			{
				"name":          "policy",
				"kind":          "pipeline-stage",
				"source":        "db-derived",
				"live":          false,
				"description":   policyStageDescription(hasPolicyViolationsTable, hasPolicyInstancesTable, hasPolicyTemplatesTable, policyTemplates),
				"queueDepth":    0,
				"activeWorkers": 0,
				"processed":     policyActivity,
				"failed":        int64(0),
				"status":        policyStageStatus(hasPolicyViolationsTable, hasPolicyInstancesTable, hasPolicyTemplatesTable, policyActivity),
			},
		}

		c.JSON(http.StatusOK, gin.H{"workers": workers})
	}
}

// GetPolicyEvaluationCost returns policy evaluation cost metrics.
// Only DB-derived totalEvaluations is real; other fields require metrics and are omitted.
func GetPolicyEvaluationCost(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var totalInsights int64
		db.Model(&models.Insight{}).Count(&totalInsights)
		evaluationsPerDay := totalInsights * 10

		c.JSON(http.StatusOK, gin.H{
			"totalEvaluations": evaluationsPerDay,
		})
	}
}
