package risk

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
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
		scope, ok := resolveAnalyticsScope(db, c)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		// Parse parameters
		period := c.DefaultQuery("period", "daily") // daily, weekly, monthly, yearly
		days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
		if days <= 0 || days > 365 {
			days = 30
		}

		clusterID := scope.clusterID
		namespace := c.Query("namespace")

		cutoffDate := time.Now().UTC().AddDate(0, 0, -days)

		// Build query
		query := scope.apply(db.WithContext(ctx).Model(&models.RiskScore{}), "cluster_id").
			Where("calculated_at >= ?", cutoffDate)

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
			score.CalculatedAt = score.CalculatedAt.UTC()
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
		scope, ok := resolveAnalyticsScope(db, c)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		compareType := c.DefaultQuery("compare", "week-over-week") // week-over-week, month-over-month, year-over-year

		now := time.Now().UTC()
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
		currentMetrics, err := getPeriodMetrics(ctx, db, currentStart, currentEnd, scope)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch comparison"})
			return
		}

		// Get previous period metrics
		previousMetrics, err := getPeriodMetrics(ctx, db, previousStart, previousEnd, scope)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch comparison"})
			return
		}

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

// GetRiskCorrelation analyzes correlations between risk scores and CVE severity.
// Supported factors: cve_count, cve_severity, namespace, node.
func GetRiskCorrelation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := resolveAnalyticsScope(db, c)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		factor := c.DefaultQuery("factor", "cve_count")

		var result CorrelationResult
		var err error

		switch factor {
		case "cve_count":
			result, err = correlateCVECountVsRisk(ctx, db, scope)
		case "cve_severity":
			result, err = correlateCVESeverityVsRisk(ctx, db, scope)
		default:
			result, err = correlateCVECountVsRisk(ctx, db, scope)
			result.Factor = factor
		}

		if err != nil {
			log.Printf("[RiskCorrelation] query error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch correlation"})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

type nsRiskRow struct {
	Namespace string
	AvgScore  float64
	CVECount  int64
	CritCount int64
}

func correlateCVECountVsRisk(ctx context.Context, db *gorm.DB, scope analyticsScope) (CorrelationResult, error) {
	result := CorrelationResult{Factor: "cve_count", Insights: []string{}}

	rows, err := queryNamespaceCorrelation(ctx, db, scope, false)
	if err != nil {
		return result, err
	}

	if len(rows) < 3 {
		result.Strength = "insufficient_data"
		result.Insights = append(result.Insights, "Need at least 3 namespaces with CVE matches for correlation")
		return result, nil
	}

	xs := make([]float64, len(rows))
	ys := make([]float64, len(rows))
	for i, r := range rows {
		xs[i] = float64(r.CVECount)
		ys[i] = r.AvgScore
		result.DataPoints = append(result.DataPoints, CorrelationPoint{X: xs[i], Y: ys[i]})
	}

	result.Correlation = pearsonCorrelation(xs, ys)
	result.Strength = correlationStrength(result.Correlation)

	if result.Correlation > 0.5 {
		result.Insights = append(result.Insights,
			fmt.Sprintf("Strong positive correlation (r=%.2f): namespaces with more CVEs tend to have higher risk scores", result.Correlation))
	} else if result.Correlation < -0.3 {
		result.Insights = append(result.Insights,
			fmt.Sprintf("Negative correlation (r=%.2f): namespaces with more CVEs have lower risk scores (may indicate better patching)", result.Correlation))
	} else {
		result.Insights = append(result.Insights,
			fmt.Sprintf("Weak correlation (r=%.2f): CVE count alone is not a strong predictor of risk score", result.Correlation))
	}

	maxCVENs := ""
	maxCVE := int64(0)
	for _, r := range rows {
		if r.CVECount > maxCVE {
			maxCVE = r.CVECount
			maxCVENs = r.Namespace
		}
	}
	if maxCVENs != "" {
		result.Insights = append(result.Insights,
			fmt.Sprintf("Most exposed namespace: %s (%d unique CVEs)", maxCVENs, maxCVE))
	}

	return result, nil
}

func correlateCVESeverityVsRisk(ctx context.Context, db *gorm.DB, scope analyticsScope) (CorrelationResult, error) {
	result := CorrelationResult{Factor: "cve_severity", Insights: []string{}}

	rows, err := queryNamespaceCorrelation(ctx, db, scope, true)
	if err != nil {
		return result, err
	}

	if len(rows) < 3 {
		result.Strength = "insufficient_data"
		result.Insights = append(result.Insights, "Need at least 3 namespaces with critical CVEs for severity correlation")
		return result, nil
	}

	xs := make([]float64, len(rows))
	ys := make([]float64, len(rows))
	for i, r := range rows {
		xs[i] = float64(r.CritCount)
		ys[i] = r.AvgScore
		result.DataPoints = append(result.DataPoints, CorrelationPoint{X: xs[i], Y: ys[i]})
	}

	result.Correlation = pearsonCorrelation(xs, ys)
	result.Strength = correlationStrength(result.Correlation)

	result.Insights = append(result.Insights,
		fmt.Sprintf("Correlation between critical CVE count and risk score: r=%.2f (%s)", result.Correlation, result.Strength))

	totalCrit := int64(0)
	for _, r := range rows {
		totalCrit += r.CritCount
	}
	result.Insights = append(result.Insights,
		fmt.Sprintf("Total critical CVEs across %d namespaces: %d", len(rows), totalCrit))

	return result, nil
}

func pearsonCorrelation(xs, ys []float64) float64 {
	n := float64(len(xs))
	if n < 2 {
		return 0
	}

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for i := range xs {
		sumX += xs[i]
		sumY += ys[i]
		sumXY += xs[i] * ys[i]
		sumX2 += xs[i] * xs[i]
		sumY2 += ys[i] * ys[i]
	}

	num := n*sumXY - sumX*sumY
	den := math.Sqrt((n*sumX2 - sumX*sumX) * (n*sumY2 - sumY*sumY))
	if den == 0 {
		return 0
	}
	r := num / den
	return math.Round(r*100) / 100
}

func correlationStrength(r float64) string {
	abs := math.Abs(r)
	switch {
	case abs >= 0.7:
		return "strong"
	case abs >= 0.4:
		return "moderate"
	case abs >= 0.2:
		return "weak"
	default:
		return "none"
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

func getPeriodMetrics(ctx context.Context, db *gorm.DB, start, end time.Time, scope analyticsScope) (PeriodMetrics, error) {
	query := scope.apply(db.WithContext(ctx).Model(&models.RiskScore{}), "cluster_id").
		Where("calculated_at >= ? AND calculated_at < ?", start, end)

	var scores []models.RiskScore
	if err := query.Find(&scores).Error; err != nil {
		log.Printf("[getPeriodMetrics] Error: %v", err)
		return PeriodMetrics{}, err
	}

	if len(scores) == 0 {
		return PeriodMetrics{}, nil
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
	}, nil
}

func sortTrendsByDate(trends []TrendPoint) {
	sort.Slice(trends, func(i, j int) bool { return trends[i].Date < trends[j].Date })
}

// Aggregate scores before joining CVEs so SBOM multiplicity cannot weight scores.
// Pod ownership keeps identically named namespaces in different clusters separate.
func queryNamespaceCorrelation(ctx context.Context, db *gorm.DB, scope analyticsScope, critical bool) ([]nsRiskRow, error) {
	db = db.WithContext(ctx)
	scores := scope.apply(db.Model(&models.RiskScore{}), "cluster_id").
		Select("cluster_id, namespace, AVG(total_score) AS avg_score").
		Where("namespace != ''").Group("cluster_id, namespace")
	q := db.Table("(?) AS rs", scores).
		Select("rs.namespace, rs.avg_score, COUNT(DISTINCT cm.cve_id) AS cve_count, COUNT(DISTINCT CASE WHEN cm.severity = 'CRITICAL' THEN cm.cve_id END) AS crit_count").
		Joins("JOIN pods p ON p.cluster_id = rs.cluster_id AND p.namespace = rs.namespace").
		Joins("JOIN sboms s ON s.pod_uid = p.uid AND s.deleted_at IS NULL").
		Joins("JOIN cve_matches cm ON cm.sbom_id = s.id AND cm.deleted_at IS NULL").
		Group("rs.cluster_id, rs.namespace, rs.avg_score")
	if critical {
		q = q.Having("COUNT(DISTINCT CASE WHEN cm.severity = 'CRITICAL' THEN cm.cve_id END) > 0").Order("crit_count DESC")
	} else {
		q = q.Having("COUNT(DISTINCT cm.cve_id) > 0").Order("cve_count DESC")
	}
	var rows []nsRiskRow
	err := q.Limit(100).Scan(&rows).Error
	return rows, err
}

// init function to force link handlers and prevent dead code elimination
func init() {
	// Force reference to handlers to ensure they're linked into binary
	// This prevents Go compiler from eliminating them as dead code
	_ = GetRiskTrendsAnalytics
	_ = GetRiskComparison
	_ = GetRiskCorrelation
}
