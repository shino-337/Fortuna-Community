package risk

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ksam/core/pkg/models"
)

// TrendPoint represents a single point in a time series
type TrendPoint struct {
	Date     string  `json:"date"`
	AvgScore float64 `json:"avgScore"`
	Count    int64   `json:"count"`
	P0Count  int64   `json:"p0Count"`
	P1Count  int64   `json:"p1Count"`
	P2Count  int64   `json:"p2Count"`
	P3Count  int64   `json:"p3Count"`
	MaxScore float64 `json:"maxScore"`
	MinScore float64 `json:"minScore"`
}

// GetRiskTrendsAnalytics returns risk trends over different time periods
func GetRiskTrendsAnalytics(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		// Parse parameters
		period := c.DefaultQuery("period", "daily") // daily, weekly, monthly, yearly
		days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
		if days <= 0 || days > 365 {
			days = 30
		}

		clusterID := c.Query("cluster")
		namespace := c.Query("namespace")

		cutoffDate := time.Now().AddDate(0, 0, -days)

		// Build query
		query := db.WithContext(ctx).Model(&models.RiskScore{}).
			Where("calculated_at >= ?", cutoffDate)

		if clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}

		if namespace != "" {
			query = query.Where("namespace = ?", namespace)
		}

		// Fetch all scores
		var scores []models.RiskScore
		if err := query.Find(&scores).Error; err != nil {
			log.Printf("[GetRiskTrendsAnalytics] Error fetching scores: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch trends"})
			return
		}

		// Aggregate by period
		trendsMap := make(map[string]*TrendAggregator)

		for _, score := range scores {
			var dateKey string
			switch period {
			case "daily":
				dateKey = score.CalculatedAt.Format("2006-01-02")
			case "weekly":
				// Get week start (Monday)
				weekStart := score.CalculatedAt
				for weekStart.Weekday() != time.Monday {
					weekStart = weekStart.AddDate(0, 0, -1)
				}
				dateKey = weekStart.Format("2006-01-02")
			case "monthly":
				dateKey = score.CalculatedAt.Format("2006-01")
			case "yearly":
				dateKey = score.CalculatedAt.Format("2006")
			default:
				dateKey = score.CalculatedAt.Format("2006-01-02")
			}

			if _, exists := trendsMap[dateKey]; !exists {
				trendsMap[dateKey] = &TrendAggregator{
					scoreSum:      0,
					count:         0,
					CriticalCount: 0,
					HighCount:     0,
					MediumCount:   0,
					LowCount:      0,
					maxScore:      -1,
					minScore:      101,
				}
			}

			agg := trendsMap[dateKey]
			agg.scoreSum += score.TotalScore
			agg.count++

			if score.TotalScore > agg.maxScore {
				agg.maxScore = score.TotalScore
			}
			if score.TotalScore < agg.minScore {
				agg.minScore = score.TotalScore
			}

			// Count by priority
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

		// Convert to response format
		trends := make([]TrendPoint, 0, len(trendsMap))
		for date, agg := range trendsMap {
			avgScore := 0.0
			if agg.count > 0 {
				avgScore = agg.scoreSum / float64(agg.count)
			}

			maxScore := agg.maxScore
			minScore := agg.minScore
			if maxScore < 0 {
				maxScore = 0
			}
			if minScore > 100 {
				minScore = 0
			}

			trends = append(trends, TrendPoint{
				Date:     date,
				AvgScore: avgScore,
				Count:    int64(agg.count),
				P0Count:  agg.CriticalCount,
				P1Count:  agg.HighCount,
				P2Count:  agg.MediumCount,
				P3Count:  agg.LowCount,
				MaxScore: maxScore,
				MinScore: minScore,
			})
		}

		// Sort by date
		sortTrendsByDate(trends)

		c.JSON(http.StatusOK, gin.H{
			"trends":       trends,
			"period":       period,
			"days":         days,
			"total_points": len(trends),
			"cluster":      clusterID,
			"namespace":    namespace,
		})
	}
}

// ComparisonResult represents comparison between two periods
type ComparisonResult struct {
	CurrentPeriod  PeriodMetrics `json:"currentPeriod"`
	PreviousPeriod PeriodMetrics `json:"previousPeriod"`
	Change         float64       `json:"change"`    // Percentage change
	ChangeAbs      float64       `json:"changeAbs"` // Absolute change
	Trend          string        `json:"trend"`     // "up", "down", "stable"
}

// PeriodMetrics represents metrics for a time period
type PeriodMetrics struct {
	AvgScore    float64 `json:"avgScore"`
	Count       int64   `json:"count"`
	P0Count     int64   `json:"p0Count"`
	P1Count     int64   `json:"p1Count"`
	P2Count     int64   `json:"p2Count"`
	P3Count     int64   `json:"p3Count"`
	MaxScore    float64 `json:"maxScore"`
	MinScore    float64 `json:"minScore"`
	PeriodStart string  `json:"periodStart"`
	PeriodEnd   string  `json:"periodEnd"`
}

// GetRiskComparison compares risk metrics across time periods
func GetRiskComparison(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		compareType := c.DefaultQuery("compare", "week-over-week") // week-over-week, month-over-month, year-over-year
		clusterID := c.Query("cluster")

		now := time.Now()
		var currentStart, currentEnd, previousStart, previousEnd time.Time

		switch compareType {
		case "week-over-week":
			// Current week (Monday to Sunday)
			currentStart = getWeekStart(now)
			currentEnd = currentStart.AddDate(0, 0, 7)
			previousStart = currentStart.AddDate(0, 0, -7)
			previousEnd = currentStart
		case "month-over-month":
			// Current month
			currentStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
			currentEnd = currentStart.AddDate(0, 1, 0)
			previousStart = currentStart.AddDate(0, -1, 0)
			previousEnd = currentStart
		case "year-over-year":
			// Current year
			currentStart = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
			currentEnd = currentStart.AddDate(1, 0, 0)
			previousStart = currentStart.AddDate(-1, 0, 0)
			previousEnd = currentStart
		default:
			compareType = "week-over-week"
			currentStart = getWeekStart(now)
			currentEnd = currentStart.AddDate(0, 0, 7)
			previousStart = currentStart.AddDate(0, 0, -7)
			previousEnd = currentStart
		}

		// Get current period metrics
		currentMetrics := getPeriodMetrics(ctx, db, currentStart, currentEnd, clusterID)

		// Get previous period metrics
		previousMetrics := getPeriodMetrics(ctx, db, previousStart, previousEnd, clusterID)

		// Calculate change
		change := 0.0
		changeAbs := 0.0
		trend := "stable"

		if previousMetrics.AvgScore > 0 {
			change = ((currentMetrics.AvgScore - previousMetrics.AvgScore) / previousMetrics.AvgScore) * 100
			changeAbs = currentMetrics.AvgScore - previousMetrics.AvgScore
		} else if currentMetrics.AvgScore > 0 {
			change = 100
			changeAbs = currentMetrics.AvgScore
		}

		if change > 5 {
			trend = "up"
		} else if change < -5 {
			trend = "down"
		}

		currentMetrics.PeriodStart = currentStart.Format("2006-01-02")
		currentMetrics.PeriodEnd = currentEnd.Format("2006-01-02")
		previousMetrics.PeriodStart = previousStart.Format("2006-01-02")
		previousMetrics.PeriodEnd = previousEnd.Format("2006-01-02")

		c.JSON(http.StatusOK, ComparisonResult{
			CurrentPeriod:  currentMetrics,
			PreviousPeriod: previousMetrics,
			Change:         change,
			ChangeAbs:      changeAbs,
			Trend:          trend,
		})
	}
}

// CorrelationResult represents correlation analysis results
type CorrelationResult struct {
	Factor      string             `json:"factor"`
	Correlation float64            `json:"correlation"` // -1 to 1
	Strength    string             `json:"strength"`    // "strong", "moderate", "weak", "none"
	Insights    []string           `json:"insights"`
	DataPoints  []CorrelationPoint `json:"dataPoints"`
}

// CorrelationPoint represents a single data point for correlation
type CorrelationPoint struct {
	X float64 `json:"x"` // Factor value
	Y float64 `json:"y"` // Risk score
}

// GetRiskCorrelation analyzes correlations between risk and other factors
func GetRiskCorrelation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		factor := c.DefaultQuery("factor", "deployments") // deployments, cluster_age, namespace_activity
		_ = c.Query("cluster")                            // Reserved for future use

		// For now, return placeholder correlation
		// Full implementation would require additional data sources
		result := CorrelationResult{
			Factor:      factor,
			Correlation: 0.0,
			Strength:    "none",
			Insights:    []string{"Correlation analysis requires additional data sources"},
			DataPoints:  []CorrelationPoint{},
		}

		// TODO: Implement actual correlation calculation when data sources are available
		// This would require:
		// - Deployment history data
		// - Cluster creation dates
		// - Namespace activity metrics
		// - Team ownership data

		c.JSON(http.StatusOK, result)
	}
}

// Helper functions

type TrendAggregator struct {
	scoreSum      float64
	count         int
	CriticalCount int64
	HighCount     int64
	MediumCount   int64
	LowCount      int64
	maxScore      float64
	minScore      float64
}

func getWeekStart(t time.Time) time.Time {
	weekStart := t
	for weekStart.Weekday() != time.Monday {
		weekStart = weekStart.AddDate(0, 0, -1)
	}
	return time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, weekStart.Location())
}

func getPeriodMetrics(ctx context.Context, db *gorm.DB, start, end time.Time, clusterID string) PeriodMetrics {
	query := db.WithContext(ctx).Model(&models.RiskScore{}).
		Where("calculated_at >= ? AND calculated_at < ?", start, end)

	if clusterID != "" {
		query = query.Where("cluster_id = ?", clusterID)
	}

	var scores []models.RiskScore
	if err := query.Find(&scores).Error; err != nil {
		log.Printf("[getPeriodMetrics] Error: %v", err)
		return PeriodMetrics{}
	}

	if len(scores) == 0 {
		return PeriodMetrics{}
	}

	var sumScore float64
	var maxScore, minScore float64 = -1, 101
	var p0Count, p1Count, p2Count, p3Count int64

	for _, score := range scores {
		sumScore += score.TotalScore
		if score.TotalScore > maxScore {
			maxScore = score.TotalScore
		}
		if score.TotalScore < minScore {
			minScore = score.TotalScore
		}

		switch score.PriorityLevel {
		case "P0":
			p0Count++
		case "P1":
			p1Count++
		case "P2":
			p2Count++
		case "P3":
			p3Count++
		}
	}

	avgScore := sumScore / float64(len(scores))
	if maxScore < 0 {
		maxScore = 0
	}
	if minScore > 100 {
		minScore = 0
	}

	return PeriodMetrics{
		AvgScore: avgScore,
		Count:    int64(len(scores)),
		P0Count:  p0Count,
		P1Count:  p1Count,
		P2Count:  p2Count,
		P3Count:  p3Count,
		MaxScore: maxScore,
		MinScore: minScore,
	}
}

func sortTrendsByDate(trends []TrendPoint) {
	for i := 0; i < len(trends)-1; i++ {
		for j := i + 1; j < len(trends); j++ {
			if trends[i].Date > trends[j].Date {
				trends[i], trends[j] = trends[j], trends[i]
			}
		}
	}
}

// init function to force link handlers and prevent dead code elimination
func init() {
	// Force reference to handlers to ensure they're linked into binary
	// This prevents Go compiler from eliminating them as dead code
	_ = GetRiskTrendsAnalytics
	_ = GetRiskComparison
	_ = GetRiskCorrelation
}
