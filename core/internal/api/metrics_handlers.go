package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// GetAgentStatus returns persisted Agent identity/version plus heartbeat-derived
// liveness. Data availability is separate from per-Agent liveness: schema/query
// failure is never represented as a successful empty Agent set.
func GetAgentStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		db := db.WithContext(c.Request.Context())
		if !db.Migrator().HasTable("agents") {
			respondSchemaUnavailable(c, "agent_status_schema_unavailable", "Agent status is unavailable; agents table is missing")
			return
		}

		var agentsList []models.Agent
		if err := db.Where("deleted_at IS NULL AND (status = ? OR status IS NULL)", "ready").
			Order("last_seen_at DESC NULLS LAST").Find(&agentsList).Error; err != nil {
			respondDataUnavailable(c, "agent_status_query_failed", "Agent status could not be loaded")
			return
		}

		clusterIDs := make([]string, 0, len(agentsList))
		seenClusters := make(map[string]struct{}, len(agentsList))
		for _, agent := range agentsList {
			if agent.ClusterID == "" {
				continue
			}
			if _, ok := seenClusters[agent.ClusterID]; ok {
				continue
			}
			seenClusters[agent.ClusterID] = struct{}{}
			clusterIDs = append(clusterIDs, agent.ClusterID)
		}
		clusterNames := make(map[string]string, len(clusterIDs))
		if len(clusterIDs) > 0 {
			var clusters []models.Cluster
			if err := db.Where("id IN ?", clusterIDs).Find(&clusters).Error; err != nil {
				respondDataUnavailable(c, "agent_cluster_query_failed", "Agent cluster metadata could not be loaded")
				return
			}
			for _, cluster := range clusters {
				name := cluster.Name
				if name == "" {
					name = cluster.ID
				}
				clusterNames[cluster.ID] = name
			}
		}

		agents := make([]map[string]interface{}, 0, len(agentsList))
		healthyCount, slowCount, disconnectedCount := 0, 0, 0
		now := time.Now()
		for _, a := range agentsList {
			status := "disconnected"
			if a.LastSeenAt != nil {
				age := now.Sub(*a.LastSeenAt)
				switch {
				case age > 15*time.Minute:
					status = "disconnected"
				case age > 5*time.Minute:
					status = "slow"
				default:
					status = "healthy"
				}
			}
			switch status {
			case "healthy":
				healthyCount++
			case "slow":
				slowCount++
			default:
				disconnectedCount++
			}
			clusterName := clusterNames[a.ClusterID]
			if clusterName == "" {
				clusterName = a.ClusterID
			}
			var lastHeartbeat interface{}
			if a.LastSeenAt != nil {
				lastHeartbeat = *a.LastSeenAt
			}
			agents = append(agents, map[string]interface{}{
				"agentId":       a.AgentID,
				"clusterId":     a.ClusterID,
				"clusterName":   clusterName,
				"nodeName":      a.NodeName,
				"status":        status,
				"lastHeartbeat": lastHeartbeat,
				"version":       a.Version,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"dataStatus":   "available",
			"healthBasis":  "lastSeenAt",
			"agents":       agents,
			"total":        len(agents),
			"healthy":      healthyCount,
			"slow":         slowCount,
			"disconnected": disconnectedCount,
		})
	}
}

// GetSystemMetrics returns DB-derived system metrics. Query failures are
// availability failures and must not be projected as healthy zero counters.
func GetSystemMetrics(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		db := db.WithContext(c.Request.Context())
		if !requireAvailabilityTables(c, db, "system_metrics_schema_unavailable",
			"System metrics require the core inventory/risk schema",
			"clusters", "pods", "service_accounts", "insights") {
			return
		}
		fail := func(err error, code, message string) bool {
			if err == nil {
				return false
			}
			respondDataUnavailable(c, code, message)
			return true
		}

		var clusterCount int64
		cutoff := time.Now().Add(-ActiveClusterCutoff)
		if fail(db.Model(&models.Cluster{}).Where("last_sync >= ?", cutoff).Count(&clusterCount).Error,
			"system_metrics_clusters_unavailable", "Cluster metrics could not be loaded") {
			return
		}

		var podCount int64
		if fail(db.Raw(`
			SELECT COUNT(*) FROM (
				SELECT cluster_id, uid
				FROM pods
				WHERE deleted_at IS NULL
				GROUP BY cluster_id, uid
			) scoped_pods
		`).Scan(&podCount).Error,
			"system_metrics_pods_unavailable", "Pod metrics could not be loaded") {
			return
		}

		var saCount int64
		if fail(db.Model(&models.ServiceAccount{}).Count(&saCount).Error,
			"system_metrics_service_accounts_unavailable", "ServiceAccount metrics could not be loaded") {
			return
		}

		var insightCount int64
		if fail(db.Model(&models.Insight{}).Count(&insightCount).Error,
			"system_metrics_insights_unavailable", "Insight metrics could not be loaded") {
			return
		}

		var lastSync time.Time
		var latestCluster models.Cluster
		err := db.Order("last_sync DESC").First(&latestCluster).Error
		if err == nil {
			lastSync = latestCluster.LastSync
		} else if err != gorm.ErrRecordNotFound {
			respondDataUnavailable(c, "system_metrics_sync_unavailable", "Last sync state could not be loaded")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"dataStatus": "available",
			"health": map[string]interface{}{
				"status": "healthy",
				"source": "database",
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
				"status": "serving",
			},
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
		db := db.WithContext(c.Request.Context())
		if !requireAvailabilityTables(c, db, "worker_metrics_schema_unavailable",
			"Worker metrics require SBOM, match-run, Insight and risk-score schemas",
			"sboms", "sbom_match_runs", "insights", "risk_scores") {
			return
		}
		var queryErr error
		captureErr := func(err error) {
			if err != nil && queryErr == nil {
				queryErr = err
			}
		}
		countWhere := func(dest *int64, model interface{}, conds ...interface{}) {
			*dest = 0
			if queryErr != nil {
				return
			}
			q := db.Model(model)
			if len(conds) > 0 {
				q = q.Where(conds[0], conds[1:]...)
			}
			captureErr(q.Count(dest).Error)
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
			captureErr(db.Table("cves").Count(&cveRows).Error)
		}
		if db.Migrator().HasTable("package_vulnerabilities") {
			captureErr(db.Table("package_vulnerabilities").Count(&packageRows).Error)
		}
		if db.Migrator().HasTable("catalog_generations") {
			var generationID sql.NullInt64
			captureErr(db.Table("catalog_generations").
				Select("MAX(id)").
				Where("catalog_type = ? AND status = ? AND deleted_at IS NULL", "cve", "active").
				Scan(&generationID).Error)
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

		if queryErr != nil {
			respondDataUnavailable(c, "worker_metrics_query_failed", "Worker metrics could not be loaded")
			return
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

		c.JSON(http.StatusOK, gin.H{"dataStatus": "available", "workers": workers})
	}
}
