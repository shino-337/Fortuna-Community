package api

import (
	"net/http"
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
)

// GetPodAttackSteps returns attack steps for a specific pod
func GetPodAttackSteps(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := c.Param("uid")
		if podUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "pod_uid is required"})
			return
		}

		clusterID, ok := middleware.ResolvedPodClusterID(c)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "resolved pod cluster is required"})
			return
		}
		var steps []models.PodAttackStep
		if err := db.Where("cluster_id = ? AND pod_uid = ?", clusterID, podUID).
			Order("created_at DESC").
			Find(&steps).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch attack steps"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"podUid": podUID,
			"steps":  steps,
			"count":  len(steps),
		})
	}
}

// GetAttackStepsSummary returns summary of attack steps across active pods only.
func GetAttackStepsSummary(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := resolveRiskGovernanceScope(db, c)
		if !ok { return }
		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		defer cancel()
		queryDB := db.WithContext(ctx)
		type Summary struct {
			StepID     string `json:"stepId"`
			Category   string `json:"category"`
			Count      int64  `json:"count"`
			AvgConfidence float64 `json:"avgConfidence"`
		}

		summaries := make([]Summary, 0)
		query := queryDB.Model(&models.PodAttackStep{}).
			Joins("JOIN pods p ON p.cluster_id = pod_attack_steps.cluster_id AND p.uid = pod_attack_steps.pod_uid AND p.deleted_at IS NULL")
		query = scope.apply(query, "p.cluster_id")
		if err := query.
			Select("pod_attack_steps.step_id, pod_attack_steps.category, COUNT(*) as count, AVG(pod_attack_steps.confidence) as avg_confidence").
			Group("pod_attack_steps.step_id, pod_attack_steps.category").
			Order("count DESC, pod_attack_steps.step_id ASC, pod_attack_steps.category ASC").
			Scan(&summaries).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch attack steps"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"summary": summaries,
			"count":   len(summaries),
		})
	}
}
