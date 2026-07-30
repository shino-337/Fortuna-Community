package risk

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

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

func scoreKey(s models.RiskScore) string {
	return s.ResourceType + "|" + s.ResourceUID + "|" + s.ClusterID
}

func includeLegacyRiskRootFields() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("FORTUNA_UI_RISK_LEGACY_ROOT_FIELDS")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

// collapsePreferredScores selects one authoritative v3 score per resource.
func collapsePreferredScores(scores []models.RiskScore) []models.RiskScore {
	best := make(map[string]models.RiskScore, len(scores))
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

func unifiedLevelSortRank(total float64) int {
	switch risk.DeriveFinalLevelFromScore(total) {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}

func filterRiskScoresByFinalLevel(in []models.RiskScore, want string) []models.RiskScore {
	want = strings.ToLower(strings.TrimSpace(want))
	if want == "" {
		return in
	}
	out := make([]models.RiskScore, 0, len(in))
	for _, s := range in {
		if strings.ToLower(risk.DeriveFinalLevelFromScore(s.TotalScore)) == want {
			out = append(out, s)
		}
	}
	return out
}

func sortCollapsedScores(scores []models.RiskScore, mode riskScoreSortMode) {
	sort.Slice(scores, func(i, j int) bool {
		a, b := scores[i], scores[j]
		switch mode {
		case sortPriority:
			pa, pb := unifiedLevelSortRank(a.TotalScore), unifiedLevelSortRank(b.TotalScore)
			if pa != pb {
				return pa < pb
			}
			return a.TotalScore > b.TotalScore
		case sortName:
			if a.ResourceName != b.ResourceName {
				return strings.ToLower(a.ResourceName) < strings.ToLower(b.ResourceName)
			}
			return a.TotalScore > b.TotalScore
		case sortNamespace:
			if a.Namespace != b.Namespace {
				return strings.ToLower(a.Namespace) < strings.ToLower(b.Namespace)
			}
			return a.TotalScore > b.TotalScore
		case sortCalculated:
			return a.CalculatedAt.After(b.CalculatedAt)
		case sortScore:
			fallthrough
		default:
			return a.TotalScore > b.TotalScore
		}
	})
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

func paginateScores(scores []models.RiskScore, page, pageSize int) []models.RiskScore {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize
	if offset >= len(scores) {
		return []models.RiskScore{}
	}
	end := offset + pageSize
	if end > len(scores) {
		end = len(scores)
	}
	return scores[offset:end]
}

// GetRiskScores returns all risk scores with optional filtering
func GetRiskScores(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var rawScores []models.RiskScore
		// Authoritative store: only unified V3 rows (legacy v1/v2 soft-deleted in migration 120).
		query := db.Model(&models.RiskScore{}).Where("LOWER(TRIM(COALESCE(scorer_version, ''))) = ?", "v3")

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
		// Load candidate rows then collapse to one authoritative score per resource
		// v3-only authoritative selection.
		if err := query.Find(&rawScores).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		collapsed := collapsePreferredScores(rawScores)
		if fl := strings.TrimSpace(c.Query("finalLevel")); fl != "" {
			collapsed = filterRiskScoresByFinalLevel(collapsed, fl)
		}

		sortMode := normalizeSortMode(c.DefaultQuery("sortBy", "score"))
		sortCollapsedScores(collapsed, sortMode)
		paged := paginateScores(collapsed, page, pageSize)

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
			"total":    len(collapsed),
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
		mode := strings.ToLower(strings.TrimSpace(c.DefaultQuery("mode", "v3")))
		if mode == "" {
			mode = "v3"
		}
		if mode != "v3" {
			log.Printf("[SyncRiskScores] rejected deprecated mode=%q", mode)
			c.JSON(http.StatusBadRequest, gin.H{"error": "only mode=v3 is supported; V2 sync writes have been removed"})
			return
		}

		var uids []string
		err := db.Model(&models.Insight{}).
			Where("status IN (?) AND deleted_at IS NULL", []string{"active", "acknowledged"}).
			Distinct("resource_uid").
			Pluck("resource_uid", &uids).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// Filter empty UIDs
		filtered := make([]string, 0, len(uids))
		for _, u := range uids {
			if strings.TrimSpace(u) != "" {
				filtered = append(filtered, u)
			}
		}
		count := len(filtered)
		if count == 0 {
			c.JSON(http.StatusOK, gin.H{"message": "No resources with active insights to sync", "resources": 0})
			return
		}
		go func() {
			ctx := context.Background()
			ok, fail, total, err := risk.BackfillV3RiskScoresFromActiveInsights(ctx, db)
			if err != nil {
				log.Printf("[SyncRiskScores] failed: %v", err)
				return
			}
			log.Printf("[SyncRiskScores] Completed sync total=%d ok=%d fail=%d (mode=%s)", total, ok, fail, mode)
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

			// Count by unified ADR level (derived from total_score)
			switch risk.DeriveFinalLevelFromScore(score.TotalScore) {
			case "critical":
				agg.CriticalCount++
			case "high":
				agg.HighCount++
			case "medium":
				agg.MediumCount++
			case "low":
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
