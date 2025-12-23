package risk

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// PriorityStats represents statistics for a priority level
type PriorityStats struct {
	Count      int64   `json:"count"`
	AvgScore   float64 `json:"avgScore"`
	Percentage float64 `json:"percentage"`
	MaxScore   float64 `json:"maxScore"`
	MinScore   float64 `json:"minScore"`
}

// GetPriorityStatistics returns statistics for all priority levels
func GetPriorityStatistics(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		// Get total count for percentage calculation
		var totalCount int64
		if err := db.WithContext(ctx).Model(&models.RiskScore{}).Count(&totalCount).Error; err != nil {
			log.Printf("[GetPriorityStatistics] Error counting total: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch statistics"})
			return
		}

		// Get statistics by priority level
		type PriorityResult struct {
			PriorityLevel string  `gorm:"column:priority_level"`
			Count         int64   `gorm:"column:count"`
			AvgScore      float64 `gorm:"column:avg_score"`
			MaxScore      float64 `gorm:"column:max_score"`
			MinScore      float64 `gorm:"column:min_score"`
		}

		var results []PriorityResult
		err := db.WithContext(ctx).
			Model(&models.RiskScore{}).
			Select("priority_level, COUNT(*) as count, AVG(total_score) as avg_score, MAX(total_score) as max_score, MIN(total_score) as min_score").
			Group("priority_level").
			Find(&results).Error

		if err != nil {
			log.Printf("[GetPriorityStatistics] Error fetching statistics: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch statistics"})
			return
		}

		// Build response map
		priorities := make(map[string]PriorityStats)
		highestPriority := "P3" // Default to lowest

		priorityOrder := map[string]int{
			"P0": 1,
			"P1": 2,
			"P2": 3,
			"P3": 4,
		}

		for _, result := range results {
			percentage := 0.0
			if totalCount > 0 {
				percentage = float64(result.Count) * 100.0 / float64(totalCount)
			}

			priorities[result.PriorityLevel] = PriorityStats{
				Count:      result.Count,
				AvgScore:   result.AvgScore,
				Percentage: percentage,
				MaxScore:   result.MaxScore,
				MinScore:   result.MinScore,
			}

			// Track highest priority (lowest number)
			if priorityOrder[result.PriorityLevel] < priorityOrder[highestPriority] {
				highestPriority = result.PriorityLevel
			}
		}

		// Ensure all priority levels are present (even if count is 0)
		for _, p := range []string{"P0", "P1", "P2", "P3"} {
			if _, exists := priorities[p]; !exists {
				priorities[p] = PriorityStats{
					Count:      0,
					AvgScore:   0,
					Percentage: 0,
					MaxScore:   0,
					MinScore:   0,
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"priorities":      priorities,
			"total":           totalCount,
			"highestPriority": highestPriority,
		})
	}
}

// GetTopRisks returns top N risks, optionally filtered by priority
func GetTopRisks(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		// Parse limit (default 10, max 100)
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
		if limit <= 0 {
			limit = 10
		}
		if limit > 100 {
			limit = 100
		}

		// Optional priority filter
		priority := c.Query("priority")

		// Optional cluster filter
		clusterID := c.Query("cluster")

		// Optional namespace filter
		namespace := c.Query("namespace")

		// Build query
		query := db.WithContext(ctx).Model(&models.RiskScore{})

		if priority != "" {
			query = query.Where("priority_level = ?", priority)
		}

		if clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}

		if namespace != "" {
			query = query.Where("namespace = ?", namespace)
		}

		// Fetch top risks ordered by score descending
		var risks []models.RiskScore
		err := query.Order("total_score DESC").Limit(limit).Find(&risks).Error

		if err != nil {
			log.Printf("[GetTopRisks] Error fetching top risks: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch top risks"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"risks":     risks,
			"limit":     limit,
			"priority":  priority,
			"cluster":   clusterID,
			"namespace": namespace,
		})
	}
}

// GroupedRisk represents a group of risks
type GroupedRisk struct {
	Key      string             `json:"key"` // Cluster ID, namespace, or resource type
	Count    int64              `json:"count"`
	AvgScore float64            `json:"avgScore"`
	MaxScore float64            `json:"maxScore"`
	MinScore float64            `json:"minScore"`
	Risks    []models.RiskScore `json:"risks,omitempty"` // Optional: include actual risks
}

// GetGroupedRisks returns risks grouped by cluster, namespace, type, or priority
func GetGroupedRisks(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		defer cancel()

		// Parse groupBy parameter (cluster, namespace, type, priority)
		groupBy := c.DefaultQuery("by", "cluster")
		if !contains([]string{"cluster", "namespace", "type", "priority"}, groupBy) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid 'by' parameter. Must be: cluster, namespace, type, or priority"})
			return
		}

		// Optional priority filter
		priority := c.Query("priority")

		// Optional cluster filter (when grouping by namespace or type)
		clusterID := c.Query("cluster")

		// Optional namespace filter (when grouping by type)
		namespace := c.Query("namespace")

		// Optional: include actual risks in response
		includeRisks := c.Query("includeRisks") == "true"

		// Build base query
		query := db.WithContext(ctx).Model(&models.RiskScore{})

		if priority != "" {
			query = query.Where("priority_level = ?", priority)
		}

		if clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}

		if namespace != "" {
			query = query.Where("namespace = ?", namespace)
		}

		// Group by selected dimension
		var groupColumn string
		switch groupBy {
		case "cluster":
			groupColumn = "cluster_id"
		case "namespace":
			groupColumn = "namespace"
		case "type":
			groupColumn = "resource_type"
		case "priority":
			groupColumn = "priority_level"
		}

		// Fetch grouped statistics
		type GroupResult struct {
			Key      string  `gorm:"column:key"`
			Count    int64   `gorm:"column:count"`
			AvgScore float64 `gorm:"column:avg_score"`
			MaxScore float64 `gorm:"column:max_score"`
			MinScore float64 `gorm:"column:min_score"`
		}

		var results []GroupResult
		err := query.
			Select(groupColumn + " as key, COUNT(*) as count, AVG(total_score) as avg_score, MAX(total_score) as max_score, MIN(total_score) as min_score").
			Group(groupColumn).
			Order("avg_score DESC").
			Find(&results).Error

		if err != nil {
			log.Printf("[GetGroupedRisks] Error fetching grouped risks: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch grouped risks"})
			return
		}

		// Build response
		grouped := make(map[string]GroupedRisk)
		for _, result := range results {
			gr := GroupedRisk{
				Key:      result.Key,
				Count:    result.Count,
				AvgScore: result.AvgScore,
				MaxScore: result.MaxScore,
				MinScore: result.MinScore,
			}

			// Optionally include actual risks
			if includeRisks {
				var risks []models.RiskScore
				riskQuery := db.WithContext(ctx).Model(&models.RiskScore{})

				// Apply same filters
				if priority != "" {
					riskQuery = riskQuery.Where("priority_level = ?", priority)
				}
				if clusterID != "" {
					riskQuery = riskQuery.Where("cluster_id = ?", clusterID)
				}
				if namespace != "" {
					riskQuery = riskQuery.Where("namespace = ?", namespace)
				}

				// Filter by group key
				switch groupBy {
				case "cluster":
					riskQuery = riskQuery.Where("cluster_id = ?", result.Key)
				case "namespace":
					riskQuery = riskQuery.Where("namespace = ?", result.Key)
				case "type":
					riskQuery = riskQuery.Where("resource_type = ?", result.Key)
				case "priority":
					riskQuery = riskQuery.Where("priority_level = ?", result.Key)
				}

				if err := riskQuery.Order("total_score DESC").Limit(10).Find(&risks).Error; err == nil {
					gr.Risks = risks
				}
			}

			grouped[result.Key] = gr
		}

		c.JSON(http.StatusOK, gin.H{
			"grouped":      grouped,
			"groupBy":      groupBy,
			"priority":     priority,
			"cluster":      clusterID,
			"namespace":    namespace,
			"includeRisks": includeRisks,
		})
	}
}

// contains checks if a string slice contains a value
func contains(slice []string, value string) bool {
	for _, s := range slice {
		if s == value {
			return true
		}
	}
	return false
}
