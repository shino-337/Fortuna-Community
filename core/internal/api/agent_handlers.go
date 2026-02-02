package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/fortuna/core/internal/service"
	"github.com/fortuna/core/pkg/capability"
	"github.com/fortuna/core/pkg/worker"
	"gorm.io/gorm"
)

// ClusterPayload is the agent → core contract (SSOT).
type ClusterPayload struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Source       string `json:"source"`        // "auto" | "env"
	K8sVersion   string `json:"k8s_version,omitempty"`
	Distribution string `json:"distribution,omitempty"`
}

// SyncDataFromAgent handles data sync from agent via HTTP.
// Accepts cluster object (id, name, source, k8s_version, distribution) or legacy clusterId/clusterName.
func SyncDataFromAgent(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ClusterID   string                 `json:"clusterId"`
			ClusterName string                 `json:"clusterName"`
			Cluster     *ClusterPayload        `json:"cluster"`
			Data        map[string]interface{} `json:"data" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		clusterID := req.ClusterID
		clusterName := req.ClusterName
		source := ""
		k8sVersion := ""
		distribution := ""
		if req.Cluster != nil {
			clusterID = req.Cluster.ID
			clusterName = req.Cluster.Name
			source = req.Cluster.Source
			k8sVersion = req.Cluster.K8sVersion
			distribution = req.Cluster.Distribution
		}
		if clusterID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "clusterId or cluster.id required"})
			return
		}
		if clusterName == "" {
			clusterName = clusterID
		}

		_, hasDelta := req.Data["isDeltaSync"]
		_, hasFull := req.Data["isFullSync"]
		log.Printf("[AgentAPI] cluster=%s name=%s source=%s hasDelta=%v hasFull=%v",
			clusterID, clusterName, source, hasDelta, hasFull)

		agentService := service.NewAgentService(db)
		if err := agentService.SyncData(clusterID, clusterName, source, k8sVersion, distribution, req.Data); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if isFull, ok := req.Data["isFullSync"].(bool); ok && isFull {
			go func(cid string) {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()
				log.Printf("[AgentAPI] Triggering historical risk evaluation for cluster=%s", cid)
				evaluator := worker.NewHistoricalRiskEvaluator(db)
				if err := evaluator.EvaluateAllResources(ctx); err != nil {
					log.Printf("[AgentAPI] Risk evaluation failed: %v", err)
				}
			}(clusterID)

			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()
				log.Printf("[AgentAPI] Triggering PodCapabilityEngine evaluation")
				if err := capability.EvaluateAllPods(ctx, db); err != nil {
					log.Printf("[AgentAPI] PCE evaluation failed: %v", err)
				}
			}()
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Data synced successfully",
		})
	}
}
