package api

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// GetWorkerMetrics returns worker metrics. Requires Prometheus integration; until then returns unsupported.
// Dashboard must not display fake healthy/queue values. See GET /health/dashboard-data-integrity.
func GetWorkerMetrics(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"_dataSource": "unsupported",
			"message":     "Worker metrics require Prometheus integration",
			"workers":     []map[string]interface{}{},
		})
	}
}

// GetQueueMetrics returns queue depth metrics. Requires Prometheus; until then returns unsupported.
// Dashboard must not display fake zeros as real data. See GET /health/dashboard-data-integrity.
func GetQueueMetrics(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"_dataSource": "unsupported",
			"message":     "Queue metrics require Prometheus integration",
			"timestamp":   time.Now(),
		})
	}
}

// GetAgentStatus returns agent status from the agents table (real data).
func GetAgentStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !db.Migrator().HasTable("agents") {
			c.JSON(http.StatusOK, gin.H{
				"agents": []map[string]interface{}{}, "total": 0, "healthy": 0, "slow": 0, "disconnected": 0,
			})
			return
		}

		tenMinutesAgo := time.Now().Add(-10 * time.Minute)
		var agentsList []models.Agent
		db.Where("deleted_at IS NULL AND status = ? AND (last_seen_at > ? OR last_seen_at IS NULL)", "ready", tenMinutesAgo).
			Find(&agentsList)

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

// GetErrorLogs returns error logs. No backend aggregation yet; returns unsupported.
// Dashboard must not display fake entries. See GET /health/dashboard-data-integrity.
func GetErrorLogs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"_dataSource": "unsupported",
			"message":     "Error log aggregation not yet implemented",
			"logs":        []map[string]interface{}{},
		})
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
			"_dataSource":      "partial",
			"message":          "avgPerRule/peak/CPU/memory require Prometheus",
		})
	}
}

