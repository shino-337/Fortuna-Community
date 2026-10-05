package risk

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/api/listlimit"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/risk"
)

type riskScoreSortMode string

const (
	sortScore      riskScoreSortMode = "score"
	sortPriority   riskScoreSortMode = "priority"
	sortName       riskScoreSortMode = "name"
	sortNamespace  riskScoreSortMode = "namespace"
	sortCalculated riskScoreSortMode = "calculated"
)

func scorerRank(version string) int {
	if strings.EqualFold(strings.TrimSpace(version), "v3") {
		return 1
	}
	return 0
}

func scoreKey(s models.RiskScore) [3]string {
	return [3]string{s.ResourceType, s.ResourceUID, s.ClusterID}
}

func includeLegacyRiskRootFields() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("FORTUNA_UI_RISK_LEGACY_ROOT_FIELDS")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

// collapsePreferredScores selects one authoritative v3 score per resource.
func collapsePreferredScores(scores []models.RiskScore) []models.RiskScore {
	best := make(map[[3]string]models.RiskScore, len(scores))
	for _, s := range scores {
		if scorerRank(s.ScorerVersion) == 0 {
			continue
		}
		k := scoreKey(s)
		cur, ok := best[k]
		if !ok {
			best[k] = s
			continue
		}
		curRank := scorerRank(cur.ScorerVersion)
		newRank := scorerRank(s.ScorerVersion)
		// Deterministic tie-break: same CalculatedAt from concurrent writes must not flip winner between requests.
		betterTime := s.CalculatedAt.After(cur.CalculatedAt)
		sameTime := s.CalculatedAt.Equal(cur.CalculatedAt)
		if newRank > curRank || (newRank == curRank && betterTime) || (newRank == curRank && sameTime && s.ID > cur.ID) {
			best[k] = s
		}
	}
	out := make([]models.RiskScore, 0, len(best))
	for _, s := range best {
		out = append(out, s)
	}
	return out
}

func normalizeSortMode(v string) riskScoreSortMode {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case string(sortPriority):
		return sortPriority
	case string(sortName):
		return sortName
	case string(sortNamespace):
		return sortNamespace
	case string(sortCalculated):
		return sortCalculated
	default:
		return sortScore
	}
}

const (
	riskScoresDefaultPageSize = 50
	riskScoresMaxPageSize     = 500
)

// riskScoresPageOffset returns the row offset for page, or false when the page is past the end.
// The quotient is checked before multiplying to avoid integer overflow on huge page numbers.
func riskScoresPageOffset(total int64, page, pageSize int) (int, bool) {
	if total == 0 || int64(page-1) > (total-1)/int64(pageSize) {
		return 0, false
	}
	return (page - 1) * pageSize, true
}

// riskScoreLevelBands are the [lo, hi) total_score bands of risk.DeriveFinalLevelFromScore.
var riskScoreLevelBands = map[string]string{
	"critical": "total_score >= 70",
	"high":     "total_score >= 40 AND total_score < 70",
	"medium":   "total_score >= 20 AND total_score < 40",
	"low":      "total_score < 20",
}

// applyRiskScoreFinalLevelFilter keeps rows whose unified level is want; an unknown level matches nothing.
func applyRiskScoreFinalLevelFilter(q *gorm.DB, want string) *gorm.DB {
	cond, ok := riskScoreLevelBands[strings.ToLower(strings.TrimSpace(want))]
	if !ok {
		return q.Where("1 = 0")
	}
	return q.Where(cond)
}

// riskScoresOrderSQL orders GET /risk/scores. Ties break on id ascending. Name and namespace compare
// case-insensitively in byte order (COLLATE "C" on PostgreSQL, SQLite's default BINARY collation).
// Within one case-insensitive value, rows with the exact same spelling sort by score, and different
// spellings ("Beta" vs "beta") keep the order of their lowest id, as the former in-memory sort did.
func riskScoresOrderSQL(db *gorm.DB, mode riskScoreSortMode) string {
	text := func(col string) string {
		lower := "LOWER(" + col + ")"
		if db.Dialector.Name() == "postgres" {
			lower += ` COLLATE "C"`
		}
		return lower + " ASC, MIN(id) OVER (PARTITION BY " + col + ")"
	}
	switch mode {
	case sortPriority:
		return "CASE WHEN total_score >= 70 THEN 0 WHEN total_score >= 40 THEN 1 WHEN total_score >= 20 THEN 2 ELSE 3 END ASC, total_score DESC, id ASC"
	case sortName:
		return text("resource_name") + " ASC, total_score DESC, id ASC"
	case sortNamespace:
		return text("namespace") + " ASC, total_score DESC, id ASC"
	case sortCalculated:
		return "calculated_at DESC, id ASC"
	default:
		return "total_score DESC, id ASC"
	}
}

// GetRiskScores returns all risk scores with optional filtering
func GetRiskScores(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, authorized := resolveAnalyticsScope(db, c)
		if !authorized {
			return
		}
		// Authoritative store: only unified V3 rows (legacy v1/v2 soft-deleted in migration 120).
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		query := scope.currentScores(db.WithContext(ctx))

		// Filter by namespace
		if namespace := c.Query("namespace"); namespace != "" {
			query = query.Where("namespace = ?", namespace)
		}

		// Filter by resource type
		if resourceType := c.Query("type"); resourceType != "" {
			query = query.Where("resource_type = ?", resourceType)
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

		// Pagination: pageSize uses the shared list limit rules (default 50, max 500).
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		if page < 1 {
			page = 1
		}
		pageSize := listlimit.ParseParam(c, "pageSize", riskScoresDefaultPageSize, riskScoresMaxPageSize)

		// currentScores already yields one authoritative v3 row per resource, so the
		// finalLevel filter, count, sort and page all run in the database.
		if fl := strings.TrimSpace(c.Query("finalLevel")); fl != "" {
			query = applyRiskScoreFinalLevelFilter(query, fl)
		}
		var total int64
		if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch risk scores"})
			return
		}
		sortMode := normalizeSortMode(c.DefaultQuery("sortBy", "score"))
		paged := []models.RiskScore{}
		if offset, ok := riskScoresPageOffset(total, page, pageSize); ok {
			if err := query.Order(riskScoresOrderSQL(db, sortMode)).Offset(offset).Limit(pageSize).Find(&paged).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch risk scores"})
				return
			}
		}

		serialized := make([]gin.H, 0, len(paged))
		for _, s := range paged {
			row := gin.H{
				"id":               s.ID,
				"resourceType":     s.ResourceType,
				"resourceUid":      s.ResourceUID,
				"resourceName":     s.ResourceName,
				"namespace":        s.Namespace,
				"clusterId":        s.ClusterID,
				"totalScore":       s.TotalScore,
				"baseScore":        s.BaseScore,
				"severityWeight":   s.SeverityWeight,
				"impactMultiplier": s.ImpactMultiplier,
				"timeDecay":        s.TimeDecay,
				"factors":          s.Factors,
				"insightsCount":    s.InsightsCount,
				"highestSeverity":  s.HighestSeverity,
				"calculatedAt":     s.CalculatedAt,
				"createdAt":        s.CreatedAt,
				"updatedAt":        s.UpdatedAt,
				"deletedAt":        s.DeletedAt,
				"scorerVersion":    s.ScorerVersion,
				"risk": gin.H{
					"score": s.TotalScore,
					"level": risk.DeriveFinalLevelFromScore(s.TotalScore),
				},
				"final_score": s.TotalScore,
				"final_level": risk.DeriveFinalLevelFromScore(s.TotalScore),
				"meta": gin.H{
					"last_updated_at": s.CalculatedAt.UTC().Format(time.RFC3339),
				},
			}
			if bd := risk.ParseBreakdownFromFactorsJSON(s.Factors); len(bd) > 0 {
				row["breakdown"] = bd
				row["drivers"] = bd
			}
			if dim := risk.ParseDimensionScoresV3FromFactorsJSON(s.Factors); dim != nil {
				row["dimensions"] = dim
			}
			if includeLegacyRiskRootFields() {
				row["legacy"] = gin.H{
					"highest_severity": s.HighestSeverity,
					"priority_level":   s.PriorityLevel,
				}
			}
			serialized = append(serialized, row)
		}

		c.JSON(http.StatusOK, gin.H{
			"scores":   serialized,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
			"sortBy":   string(sortMode),
		})
	}
}

// GetRiskScore returns a specific risk score by resource UID
func GetRiskScore(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uid")
		clusterID := c.DefaultQuery("cluster", "")

		var score models.RiskScore
		query := db.Where("resource_uid = ? AND LOWER(TRIM(COALESCE(scorer_version, ''))) = ?", uid, "v3")
		if clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}

		var candidates []models.RiskScore
		if err := query.Order("calculated_at DESC, id DESC").Find(&candidates).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Risk score not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if len(candidates) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Risk score not found"})
			return
		}

		preferred := collapsePreferredScores(candidates)
		if len(preferred) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Risk score not found"})
			return
		}
		score = preferred[0]

		// Canonical unified fields (additive): final_level, breakdown[], severity_hint from stored aggregates.
		raw, err := json.Marshal(score)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(raw, &payload); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		payload["final_level"] = risk.DeriveFinalLevelFromScore(score.TotalScore)
		if sev := strings.TrimSpace(score.HighestSeverity); sev != "" {
			payload["severity_hint"] = strings.ToLower(sev)
		}
		if bd := risk.ParseBreakdownFromFactorsJSON(score.Factors); len(bd) > 0 {
			payload["breakdown"] = bd
			payload["drivers"] = bd
		}
		payload["final_score"] = score.TotalScore
		payload["risk"] = gin.H{
			"score": score.TotalScore,
			"level": payload["final_level"],
		}
		payload["meta"] = gin.H{
			"last_updated_at": score.CalculatedAt.UTC().Format(time.RFC3339),
		}
		delete(payload, "priorityLevel")
		delete(payload, "priority_level")
		if includeLegacyRiskRootFields() {
			payload["legacy"] = gin.H{
				"highest_severity": score.HighestSeverity,
				"priority_level":   score.PriorityLevel,
			}
		}

		c.JSON(http.StatusOK, payload)
	}
}

// CalculateRiskScore calculates and persists the unified V3 risk score for a resource.
// Query param mode must be v3 or omitted (default). V2 / both are rejected — V2 DB writes removed.
func CalculateRiskScore(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uid")
		mode := strings.ToLower(strings.TrimSpace(c.DefaultQuery("mode", "v3")))
		if mode == "" {
			mode = "v3"
		}
		if mode != "v3" {
			log.Printf("[CalculateRiskScore] rejected deprecated mode=%q resource_uid=%s", mode, uid)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "only mode=v3 is supported; V2 risk score persistence has been removed. Prefer GET /risk/scores/:uid (v3-preferred read).",
			})
			return
		}

		ctx := c.Request.Context()
		scorerV3 := risk.NewUnifiedScorerV3(db)
		scoreV3, err := scorerV3.CalculateScoreV3(ctx, uid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := scorerV3.SaveScoreV3(ctx, scoreV3); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"mode": "v3", "v3": scoreV3})
	}
}

// SyncRiskScores recalculates and saves V3 risk_scores for all resources that have active/acknowledged insights.
// Runs in background; returns 202 Accepted. Query mode must be v3 or omitted (default); V2/both rejected.
func SyncRiskScores(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, authorized := resolveAnalyticsScope(db, c)
		if !authorized {
			return
		}
		mode := strings.ToLower(strings.TrimSpace(c.DefaultQuery("mode", "v3")))
		if mode == "" {
			mode = "v3"
		}
		if mode != "v3" {
			log.Printf("[SyncRiskScores] rejected deprecated mode=%q", mode)
			c.JSON(http.StatusBadRequest, gin.H{"error": "only mode=v3 is supported; V2 sync writes have been removed"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		filtered, err := scope.syncUIDs(db.WithContext(ctx))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to select resources for sync"})
			return
		}
		count := len(filtered)
		if count == 0 {
			c.JSON(http.StatusOK, gin.H{"message": "No resources with active insights to sync", "resources": 0})
			return
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()
			syncSelectedRiskScores(ctx, db, filtered)
		}()
		c.JSON(http.StatusAccepted, gin.H{
			"message":   fmt.Sprintf("Risk score sync started (mode=%s)", mode),
			"resources": count,
			"mode":      mode,
		})
	}
}

// GetRiskTrends returns risk trends over time
// ✅ FIXED: Uses GORM Query Builder instead of Raw() to avoid SELECT clause stripping
func GetRiskTrends(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, authorized := resolveAnalyticsScope(db, c)
		if !authorized {
			return
		}
		// ✅ Step 1: Validate input
		days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
		if days <= 0 || days > 365 {
			days = 30
		}

		clusterID := scope.clusterID
		if clusterID != "" && len(clusterID) > 255 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster ID"})
			return
		}

		log.Printf("[GetRiskTrends] Fetching trends for last %d days, cluster=%s", days, clusterID)

		// ✅ Step 2: Fetch data using GORM Query Builder (NOT Raw!)
		cutoffDate := time.Now().UTC().AddDate(0, 0, -days)

		// Set query timeout
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		// Build query using GORM Query Builder (consistent with GetRiskScores)
		// Use time.Time directly - GORM handles it correctly
		query := scope.apply(db.WithContext(ctx).Model(&models.RiskScore{}), "cluster_id").
			Where("calculated_at >= ?", cutoffDate)

		// Optional cluster filter
		if clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}

		// Aggregate per UTC day in the database; only one row per day is returned.
		type TrendPoint struct {
			Date          string  `json:"date"`
			AvgScore      float64 `json:"avgScore"`
			CriticalCount int64   `json:"criticalCount"`
			HighCount     int64   `json:"highCount"`
			MediumCount   int64   `json:"mediumCount"`
			LowCount      int64   `json:"lowCount"`
		}
		trends := make([]TrendPoint, 0)
		bucket, err := riskScoreDayBucketSQL(db)
		if err == nil {
			// Level bands match risk.DeriveFinalLevelFromScore.
			err = query.Select(bucket + ` AS date, AVG(COALESCE(total_score, 0)) AS avg_score,
 SUM(CASE WHEN COALESCE(total_score, 0) >= 70 THEN 1 ELSE 0 END) AS critical_count,
 SUM(CASE WHEN COALESCE(total_score, 0) >= 40 AND COALESCE(total_score, 0) < 70 THEN 1 ELSE 0 END) AS high_count,
 SUM(CASE WHEN COALESCE(total_score, 0) >= 20 AND COALESCE(total_score, 0) < 40 THEN 1 ELSE 0 END) AS medium_count,
 SUM(CASE WHEN COALESCE(total_score, 0) < 20 THEN 1 ELSE 0 END) AS low_count`).
				Group(bucket).Order(bucket + " ASC").Scan(&trends).Error
		}
		if err != nil {
			log.Printf("[GetRiskTrends] Error aggregating scores: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to fetch risk trends",
			})
			return
		}

		log.Printf("[GetRiskTrends] Successfully aggregated %d trend data points", len(trends))

		c.JSON(http.StatusOK, gin.H{
			"trends":      trends,
			"period_days": days,
			"total_days":  len(trends),
		})
	}
}

func syncSelectedRiskScores(ctx context.Context, db *gorm.DB, uids []string) {
	scorer := risk.NewUnifiedScorerV3(db)
	for _, uid := range uids {
		if ctx.Err() != nil {
			return
		}
		score, err := scorer.CalculateScoreV3(ctx, uid)
		if err == nil {
			err = scorer.SaveScoreV3(ctx, score)
		}
		if err != nil {
			log.Printf("[SyncRiskScores] resource_uid=%s failed: %v", uid, err)
		}
	}
}
