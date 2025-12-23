package risk

import (
	"context"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/risk"
)

// GetRiskScores returns all risk scores with optional filtering
func GetRiskScores(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var scores []models.RiskScore
		query := db.Model(&models.RiskScore{})

		// Filter by cluster
		if clusterID := c.Query("cluster"); clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}

		// Filter by namespace
		if namespace := c.Query("namespace"); namespace != "" {
			query = query.Where("namespace = ?", namespace)
		}

		// Filter by resource type
		if resourceType := c.Query("type"); resourceType != "" {
			query = query.Where("resource_type = ?", resourceType)
		}

		// Filter by priority level (supports multiple: priority=P0,P1)
		if priority := c.Query("priority"); priority != "" {
			priorities := strings.Split(priority, ",")
			if len(priorities) == 1 {
				query = query.Where("priority_level = ?", priority)
			} else {
				query = query.Where("priority_level IN ?", priorities)
			}
		}

		// Filter by minimum score
		if minScore := c.Query("minScore"); minScore != "" {
			if score, err := strconv.ParseFloat(minScore, 64); err == nil {
				query = query.Where("total_score >= ?", score)
			}
		}

		// Filter by maximum score
		if maxScore := c.Query("maxScore"); maxScore != "" {
			if score, err := strconv.ParseFloat(maxScore, 64); err == nil {
				query = query.Where("total_score <= ?", score)
			}
		}

		// Pagination
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
		offset := (page - 1) * pageSize

		var total int64
		query.Count(&total)

		// Sorting (default: total_score DESC)
		sortBy := c.DefaultQuery("sortBy", "score")
		switch sortBy {
		case "score":
			query = query.Order("total_score DESC")
		case "priority":
			// Order by priority level (P0 first, then P1, P2, P3)
			query = query.Order("CASE priority_level WHEN 'P0' THEN 1 WHEN 'P1' THEN 2 WHEN 'P2' THEN 3 WHEN 'P3' THEN 4 END, total_score DESC")
		case "name":
			query = query.Order("resource_name ASC, total_score DESC")
		case "namespace":
			query = query.Order("namespace ASC, total_score DESC")
		case "calculated":
			query = query.Order("calculated_at DESC")
		default:
			query = query.Order("total_score DESC")
		}

		if err := query.Offset(offset).Limit(pageSize).Find(&scores).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"scores":   scores,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
			"sortBy":   sortBy,
		})
	}
}

// GetRiskScore returns a specific risk score by resource UID
func GetRiskScore(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uid")
		clusterID := c.DefaultQuery("cluster", "")

		var score models.RiskScore
		query := db.Where("resource_uid = ?", uid)
		if clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}

		if err := query.First(&score).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Risk score not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, score)
	}
}

// CalculateRiskScore calculates and saves risk score for a resource using V2 scorer
func CalculateRiskScore(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uid")

		// Use V2 scorer
		scorer := risk.NewScorer(db)
		score, err := scorer.CalculateScore(c.Request.Context(), uid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Save to database
		if err := scorer.SaveScore(c.Request.Context(), score); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, score)
	}
}

// GetRiskTrends returns risk trends over time
// ✅ FIXED: Uses GORM Query Builder instead of Raw() to avoid SELECT clause stripping
func GetRiskTrends(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// ✅ Step 1: Validate input
		days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
		if days <= 0 || days > 365 {
			days = 30
		}

		clusterID := c.Query("cluster")
		if clusterID != "" && len(clusterID) > 255 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster ID"})
			return
		}

		log.Printf("[GetRiskTrends] Fetching trends for last %d days, cluster=%s", days, clusterID)

		// ✅ Step 2: Fetch data using GORM Query Builder (NOT Raw!)
		cutoffDate := time.Now().AddDate(0, 0, -days)

		// Set query timeout
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		// Build query using GORM Query Builder (consistent with GetRiskScores)
		// Use time.Time directly - GORM handles it correctly
		query := db.WithContext(ctx).
			Model(&models.RiskScore{}).
			Where("calculated_at >= ?", cutoffDate)

		// Optional cluster filter
		if clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}

		// Fetch all scores
		var scores []models.RiskScore
		err := query.Find(&scores).Error
		if err != nil {
			log.Printf("[GetRiskTrends] Error fetching scores: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to fetch risk trends",
			})
			return
		}

		log.Printf("[GetRiskTrends] Fetched %d risk scores", len(scores))

		// ✅ Step 3: Aggregate by date in Go
		type TrendAggregator struct {
			scoreSum      float64
			count         int
			CriticalCount int64
			HighCount     int64
			MediumCount   int64
			LowCount      int64
		}

		trendsMap := make(map[string]*TrendAggregator)

		for _, score := range scores {
			date := score.CalculatedAt.Format("2006-01-02")

			if _, exists := trendsMap[date]; !exists {
				trendsMap[date] = &TrendAggregator{
					scoreSum:      0,
					count:         0,
					CriticalCount: 0,
					HighCount:     0,
					MediumCount:   0,
					LowCount:      0,
				}
			}

			agg := trendsMap[date]
			agg.scoreSum += score.TotalScore
			agg.count++

			// Count by priority level
			switch score.PriorityLevel {
			case "P0":
				agg.CriticalCount++
			case "P1":
				agg.HighCount++
			case "P2":
				agg.MediumCount++
			case "P3":
				agg.LowCount++
			}
		}

		// ✅ Step 4: Convert to response format
		type TrendPoint struct {
			Date          string  `json:"date"`
			AvgScore      float64 `json:"avgScore"`
			CriticalCount int64   `json:"criticalCount"`
			HighCount     int64   `json:"highCount"`
			MediumCount   int64   `json:"mediumCount"`
			LowCount      int64   `json:"lowCount"`
		}

		trends := make([]TrendPoint, 0, len(trendsMap))
		for date, agg := range trendsMap {
			avgScore := 0.0
			if agg.count > 0 {
				avgScore = agg.scoreSum / float64(agg.count)
			}

			trends = append(trends, TrendPoint{
				Date:          date,
				AvgScore:      avgScore,
				CriticalCount: agg.CriticalCount,
				HighCount:     agg.HighCount,
				MediumCount:   agg.MediumCount,
				LowCount:      agg.LowCount,
			})
		}

		// ✅ Step 5: Sort by date
		sort.Slice(trends, func(i, j int) bool {
			return trends[i].Date < trends[j].Date
		})

		log.Printf("[GetRiskTrends] Successfully aggregated %d trend data points", len(trends))

		c.JSON(http.StatusOK, gin.H{
			"trends":      trends,
			"period_days": days,
			"total_days":  len(trends),
		})
	}
}
