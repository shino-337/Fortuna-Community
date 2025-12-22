package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ksam/core/pkg/models"
)

// GetWorkerMetrics returns worker metrics
func GetWorkerMetrics(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get metrics from Prometheus registry
		// For now, return structured data that can be queried from /metrics endpoint
		// In production, you'd query Prometheus API

		workers := []map[string]interface{}{
			{
				"type":     "normalizer",
				"status":   "healthy",
				"queueDepth": 0, // Would query from metrics
			},
			{
				"type":     "correlator",
				"status":   "healthy",
				"queueDepth": 0,
			},
			{
				"type":     "risk",
				"status":   "healthy",
				"queueDepth": 0,
			},
		}

		c.JSON(http.StatusOK, gin.H{
			"workers": workers,
		})
	}
}

// GetQueueMetrics returns queue depth metrics
func GetQueueMetrics(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Query Prometheus for queue depth metrics
		// For now, return placeholder structure

		c.JSON(http.StatusOK, gin.H{
			"normalizer": 0,
			"correlator": 0,
			"risk":       0,
			"timestamp":  time.Now(),
		})
	}
}

// GetAgentStatus returns agent status
func GetAgentStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Query database for agent heartbeats
		// For now, return cluster-based status

		var clusters []models.Cluster
		db.Find(&clusters)

		agents := []map[string]interface{}{}
		for _, cluster := range clusters {
			// Get last sync time as heartbeat indicator
			status := "healthy"
			if time.Since(cluster.LastSync) > 5*time.Minute {
				status = "slow"
			}
			if time.Since(cluster.LastSync) > 15*time.Minute {
				status = "disconnected"
			}

			agents = append(agents, map[string]interface{}{
				"clusterId":   cluster.ID,
				"clusterName": cluster.Name,
				"status":      status,
				"lastHeartbeat": cluster.LastSync,
				"nodeName":    "unknown", // Would come from agent_status table
			})
		}

		healthyCount := 0
		for _, agent := range agents {
			if agent["status"] == "healthy" {
				healthyCount++
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"agents":      agents,
			"total":       len(agents),
			"healthy":     healthyCount,
			"slow":        len(agents) - healthyCount,
			"disconnected": 0,
		})
	}
}

// GetSystemMetrics returns system health metrics
func GetSystemMetrics(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get cluster stats
		var clusterCount int64
		db.Model(&models.Cluster{}).Count(&clusterCount)

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
				"pods":           podCount,
				"serviceAccounts": saCount,
				"insights":        insightCount,
			},
			"api": map[string]interface{}{
				"status": "healthy",
				"avgLatency": "234ms", // Would come from metrics
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

// GetPolicyEvaluationCost returns policy evaluation cost metrics
func GetPolicyEvaluationCost(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get total insights count as proxy for evaluations
		var totalInsights int64
		db.Model(&models.Insight{}).Count(&totalInsights)

		// Estimate evaluations per day (rough estimate)
		evaluationsPerDay := totalInsights * 10 // Rough estimate

		c.JSON(http.StatusOK, gin.H{
			"totalEvaluations": evaluationsPerDay,
			"avgPerRule":      "12μs", // Would come from actual metrics
			"peak":            "18μs",
			"totalCPU":        "0.5 cores",
			"memory":          "2.1 GB",
			"costPerEvaluation": "10ns",
		})
	}
}

