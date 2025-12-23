package api

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/worker"
)

// GetInsights returns all insights with optional filtering
func GetInsights(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var insights []models.Insight
		query := db.Model(&models.Insight{})
		// GORM automatically filters soft-deleted records (deleted_at IS NULL)

		// Default: only show active insights (unless status filter is explicitly set)
		statusParam := c.Query("status")
		if statusParam == "all" {
			// Show all insights (no status filter) - explicitly requested
			log.Printf("[GetInsights] Status=all: returning all insights")
		} else if statusParam != "" {
			// Filter by specific status
			query = query.Where("status = ?", statusParam)
			log.Printf("[GetInsights] Status=%s: filtering by status", statusParam)
		} else {
			// Default to active insights only when no status parameter provided
			// Use explicit Where clause to ensure it's applied
			query = query.Where("status = ?", "active")
			log.Printf("[GetInsights] No status param: defaulting to active (applying WHERE status = 'active')")
		}

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
			Total    int64            `json:"total"`
			Critical int64            `json:"critical"`
			High     int64            `json:"high"`
			Medium   int64            `json:"medium"`
			Low      int64            `json:"low"`
			ByType   map[string]int64 `json:"byType"`
		}

		// Count total - only active insights (not soft-deleted, status = 'active')
		// GORM automatically filters soft-deleted records (deleted_at IS NULL)
		query := db.Model(&models.Insight{}).Where("status = ? OR status IS NULL", "active")
		query.Count(&summary.Total)

		// Count by severity (case-insensitive using Raw SQL for PostgreSQL)
		// Use a single GROUP BY query to get all counts at once
		var severityCounts []struct {
			Severity string `gorm:"column:severity"`
			Count    int64  `gorm:"column:count"`
		}
		// Only count active insights (not soft-deleted, status = 'active')
		db.Raw(`
			SELECT LOWER(severity) as severity, COUNT(*) as count 
			FROM insights 
			WHERE deleted_at IS NULL AND (status = 'active' OR status IS NULL)
			GROUP BY LOWER(severity)
		`).Scan(&severityCounts)

		// Map results to summary
		for _, sc := range severityCounts {
			switch sc.Severity {
			case "critical":
				summary.Critical = sc.Count
			case "high":
				summary.High = sc.Count
			case "medium":
				summary.Medium = sc.Count
			case "low":
				summary.Low = sc.Count
			}
		}

		// Count by type
		summary.ByType = make(map[string]int64)
		var typeCounts []struct {
			Type  string
			Count int64
		}
		// Only count active insights
		db.Model(&models.Insight{}).
			Where("deleted_at IS NULL AND (status = 'active' OR status IS NULL)").
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
// This uses the HistoricalRiskEvaluator for database-based evaluation
func TriggerRiskEvaluation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		evaluator := worker.NewHistoricalRiskEvaluator(db)
		if err := evaluator.EvaluateAllResources(ctx); err != nil {
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
		ctx := c.Request.Context()

		// Step 1: Re-evaluate all resources (creates/updates insights for existing risks)
		evaluator := worker.NewHistoricalRiskEvaluator(db)
		if err := evaluator.EvaluateAllResources(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Step 2: Auto-resolve insights where risks no longer exist
		statusUpdater := worker.NewInsightStatusUpdater(db)
		if err := statusUpdater.UpdateStatusForResolvedRisks(ctx); err != nil {
			// Log error but don't fail the request - evaluation was successful
			log.Printf("[TriggerHistoricalRiskEvaluation] Error updating insight statuses: %v", err)
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

// AcknowledgeInsight acknowledges an insight (keeps status as 'active' but marks as acknowledged)
func AcknowledgeInsight(db *gorm.DB) gin.HandlerFunc {
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

		// Update insight (acknowledgment doesn't change status, just updates timestamp)
		now := time.Now()
		updates := map[string]interface{}{
			"updated_at": now,
		}

		if err := db.Model(&insight).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":      "Insight acknowledged successfully",
			"acknowledged": true,
			"status":       insight.Status,
		})
	}
}

// ResolveInsight resolves an insight
func ResolveInsight(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var request struct {
			Resolution string `json:"resolution"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var insight models.Insight
		if err := db.First(&insight, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Insight not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Update insight status to 'resolved'
		insight.Status = "resolved"
		insight.RecommendedAction = request.Resolution
		insight.UpdatedAt = time.Now()

		if err := db.Save(&insight).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":  "Insight resolved successfully",
			"resolved": true,
			"status":   insight.Status,
		})
	}
}

// DismissInsight dismisses an insight (marks as dismissed)
func DismissInsight(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var request struct {
			Reason string `json:"reason"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			// Reason is optional, continue without it
		}

		var insight models.Insight
		if err := db.First(&insight, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Insight not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Update insight status to 'dismissed'
		insight.Status = "dismissed"
		insight.UpdatedAt = time.Now()

		if err := db.Save(&insight).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":   "Insight dismissed successfully",
			"dismissed": true,
			"status":    insight.Status,
		})
	}
}
