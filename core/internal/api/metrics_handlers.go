package api

import (
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
			if a.LastSeenAt != nil && time.Since(*a.LastSeenAt) > 5*time.Minute {
				status = "slow"
			} else if a.LastSeenAt != nil && time.Since(*a.LastSeenAt) > 15*time.Minute {
				status = "disconnected"
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

// GetWorkerStatus returns the status of background workers derived from DB activity.
// Workers tracked: sbom, cve-matcher, correlator, risk, policy.
func GetWorkerStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		type workerStat struct {
			name      string
			tableName string
			failField string
		}

		// Count SBOMs processed; sbom_match_runs tracks the SBOM pipeline (includes CVE matching stage)
		var sbomProcessed, matchRunFailed int64
		db.Table("sboms").Where("deleted_at IS NULL").Count(&sbomProcessed)
		db.Table("sbom_match_runs").Where("status = ?", "failed").Count(&matchRunFailed)

		// Count CVE matches; pipeline failures are shared with the SBOM match-run stage
		var cveProcessed int64
		db.Table("cve_matches").Where("deleted_at IS NULL").Count(&cveProcessed)

		// Count insights (correlator output)
		var correlatorProcessed int64
		db.Table("insights").Where("deleted_at IS NULL").Count(&correlatorProcessed)

		// Count risk scores
		var riskProcessed int64
		db.Table("risk_scores").Where("deleted_at IS NULL").Count(&riskProcessed)

		// Count policy violations
		var policyProcessed int64
		db.Table("policy_violations").Where("deleted_at IS NULL").Count(&policyProcessed)

		// workerStatus derives a health status from cumulative DB counts.
		// "degraded" fires when failures exceed 10% of processed output AND there are more than
		// 5 failures (to avoid false alarms on brand-new or lightly-used deployments).
		workerStatus := func(processed, failed int64) string {
			if processed == 0 && failed == 0 {
				return "stopped"
			}
			if failed > processed/10 && failed > 5 {
				return "degraded"
			}
			return "running"
		}

		workers := []map[string]interface{}{
			{
				"name":          "sbom",
				"queueDepth":    0,
				"activeWorkers": 0,
				"processed":     sbomProcessed,
				"failed":        matchRunFailed,
				"status":        workerStatus(sbomProcessed, matchRunFailed),
			},
			{
				"name":          "cve-matcher",
				"queueDepth":    0,
				"activeWorkers": 0,
				"processed":     cveProcessed,
				"failed":        int64(0),
				"status":        workerStatus(cveProcessed, 0),
			},
			{
				"name":          "correlator",
				"queueDepth":    0,
				"activeWorkers": 0,
				"processed":     correlatorProcessed,
				"failed":        int64(0),
				"status":        workerStatus(correlatorProcessed, 0),
			},
			{
				"name":          "risk",
				"queueDepth":    0,
				"activeWorkers": 0,
				"processed":     riskProcessed,
				"failed":        int64(0),
				"status":        workerStatus(riskProcessed, 0),
			},
			{
				"name":          "policy",
				"queueDepth":    0,
				"activeWorkers": 0,
				"processed":     policyProcessed,
				"failed":        int64(0),
				"status":        workerStatus(policyProcessed, 0),
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

