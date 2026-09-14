package api

import (
	"net/http"
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

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

		var steps []models.PodAttackStep
		if err := db.Where("pod_uid = ?", podUID).
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
		if err := queryDB.Model(&models.PodAttackStep{}).
			Where("pod_uid IN (?)", scope.podUIDs(queryDB, false)).
			Select("step_id, category, COUNT(*) as count, AVG(confidence) as avg_confidence").
			Group("step_id, category").
			Order("count DESC, step_id ASC, category ASC").
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
