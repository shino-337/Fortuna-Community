package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ksam/core/internal/risk"
	"github.com/ksam/core/pkg/models"
	"github.com/ksam/core/pkg/worker"
)

// GetInsights returns all insights with optional filtering
func GetInsights(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var insights []models.Insight
		query := db.Model(&models.Insight{})

		// Filter by type
		if insightType := c.Query("type"); insightType != "" {
			query = query.Where("type = ?", insightType)
		}

		// Filter by severity
		if severity := c.Query("severity"); severity != "" {
			query = query.Where("severity = ?", severity)
		}

		// Filter by cluster (from affected resources)
		if clusterID := c.Query("cluster"); clusterID != "" {
			query = query.Where("affected_resources::text LIKE ?", "%"+clusterID+"%")
		}

		// Pagination
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
		offset := (page - 1) * pageSize

		var total int64
		query.Count(&total)

		if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&insights).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"insights": insights,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		})
	}
}

// GetInsight returns a specific insight by ID
func GetInsight(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var insight models.Insight
		if err := db.First(&insight, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Insight not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, insight)
	}
}

// GetInsightsSummary returns summary statistics of insights
func GetInsightsSummary(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var summary struct {
			Total      int64 `json:"total"`
			Critical   int64 `json:"critical"`
			High       int64 `json:"high"`
			Medium     int64 `json:"medium"`
			Low        int64 `json:"low"`
			ByType     map[string]int64 `json:"byType"`
		}

		// Count total
		db.Model(&models.Insight{}).Count(&summary.Total)

		// Count by severity
		db.Model(&models.Insight{}).Where("severity = ?", "Critical").Count(&summary.Critical)
		db.Model(&models.Insight{}).Where("severity = ?", "High").Count(&summary.High)
		db.Model(&models.Insight{}).Where("severity = ?", "Medium").Count(&summary.Medium)
		db.Model(&models.Insight{}).Where("severity = ?", "Low").Count(&summary.Low)

		// Count by type
		summary.ByType = make(map[string]int64)
		var typeCounts []struct {
			Type  string
			Count int64
		}
		db.Model(&models.Insight{}).
			Select("type, COUNT(*) as count").
			Group("type").
			Scan(&typeCounts)

		for _, tc := range typeCounts {
			summary.ByType[tc.Type] = tc.Count
		}

		c.JSON(http.StatusOK, summary)
	}
}

// TriggerRiskEvaluation manually triggers risk evaluation
// This uses the internal/risk engine for database-based evaluation
func TriggerRiskEvaluation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		riskEngine := risk.NewRiskEngine(db)
		if err := riskEngine.EvaluateRBACRisks(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Risk evaluation triggered successfully",
		})
	}
}

// TriggerHistoricalRiskEvaluation triggers evaluation using the Risk Worker engine
// This processes historical data from database using the same engine as Risk Worker
func TriggerHistoricalRiskEvaluation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Use the Risk Worker's historical evaluator
		evaluator := worker.NewHistoricalRiskEvaluator(db)
		
		ctx := c.Request.Context()
		if err := evaluator.EvaluateAllResources(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Historical risk evaluation completed successfully",
		})
	}
}

// DeleteInsight deletes an insight by ID
func DeleteInsight(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := db.Delete(&models.Insight{}, "id = ?", id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Insight deleted successfully"})
	}
}


