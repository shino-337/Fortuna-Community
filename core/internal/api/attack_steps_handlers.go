package api

import (
	"net/http"

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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		type Summary struct {
			StepID     string `json:"stepId"`
			Category   string `json:"category"`
			Count      int64  `json:"count"`
			AvgConfidence float64 `json:"avgConfidence"`
		}

		var summaries []Summary
		if err := db.Model(&models.PodAttackStep{}).
			Where("pod_uid IN (SELECT uid FROM pods WHERE deleted_at IS NULL)").
			Select("step_id, category, COUNT(*) as count, AVG(confidence) as avg_confidence").
			Group("step_id, category").
			Order("count DESC").
			Scan(&summaries).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"summary": summaries,
			"count":   len(summaries),
		})
	}
}
