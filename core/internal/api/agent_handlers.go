package api

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ksam/core/internal/service"
	"gorm.io/gorm"
)

// SyncDataFromAgent handles data sync from agent via HTTP (temporary until gRPC is fully implemented)
func SyncDataFromAgent(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ClusterID string                 `json:"clusterId" binding:"required"`
			Data      map[string]interface{} `json:"data" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		rawDelta, hasDelta := req.Data["isDeltaSync"]
		rawFull, hasFull := req.Data["isFullSync"]
		log.Printf("[AgentAPI] cluster=%s hasDelta=%v value=%v (type=%T), hasFull=%v value=%v (type=%T)",
			req.ClusterID, hasDelta, rawDelta, rawDelta, hasFull, rawFull, rawFull)

		agentService := service.NewAgentService(db)
		if err := agentService.SyncData(req.ClusterID, req.Data); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Data synced successfully",
		})
	}
}
