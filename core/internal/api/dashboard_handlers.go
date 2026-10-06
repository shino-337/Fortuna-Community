package api

import (
	"encoding/csv"
	"encoding/json"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/metrics"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/risk"
	"github.com/fortuna/core/pkg/securityaudit"
)

type DashboardStatsDTO struct {
	DataStatus       string `json:"dataStatus,omitempty"`
	TotalClusters    int64  `json:"totalClusters"`
	ActiveAgents     int64  `json:"activeAgents"`
	RunningPods      int64  `json:"runningPods"`
	TotalRisks       int64  `json:"totalRisks"`
	CriticalRisks    int64  `json:"criticalRisks"`
	Resolved24h      int64  `json:"resolved24h"`           // Insights resolved in last 24h
	AffectedPodCount int64  `json:"affectedPodCount"`      // Distinct pods with at least one active insight (Affected Workloads)
	ClusterName      string `json:"clusterName,omitempty"` // When clusterId filter is set: display name from K8s (via agent sync)
}

type ThreatVelocityPoint struct {
	Date          string `json:"date"`
	CriticalCount int64  `json:"critical"`
	HighCount     int64  `json:"high"`
	MediumCount   int64  `json:"medium"`
	LowCount      int64  `json:"low"`
	// UnscoredCount counts findings whose resource has no risk score yet, so they have no risk level.
	UnscoredCount int64 `json:"unscored"`
}

// GetThreatVelocity returns daily counts of insights grouped by risk level (the score band of the
// resource's preferred risk score, as in the insights summary), never by rule severity.
// Query param days: 1–30 (default 7). Query param clusterId: optional.
// Query param byType: "vulnerability" (default) = CVE + supply_chain_malware insights; "all" = every insight_type (RBAC, capability, etc.).
// Pod filter: only count Pod insights when pod exists (deleted_at IS NULL).
func GetThreatVelocity(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		days := 7
		if d := c.Query("days"); d != "" {
			if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 30 {
				days = parsed
			}
		}
		byType := strings.ToLower(strings.TrimSpace(c.DefaultQuery("byType", "vulnerability")))
		clusterID := strings.TrimSpace(c.Query("clusterId"))
		if clusterID != "" {
			clusterID = NormalizeClusterID(db, clusterID)
		}

		filter, ok := aggregateScope(db, c)
		if !ok {
			return
		}
		start := time.Now().UTC().AddDate(0, 0, -days+1).Truncate(24 * time.Hour)

		var rows []struct {
			Date  time.Time
			Level string
			Count int64
		}

		// Pod filter: same as insights/summary – only count Pod insights when pod still exists
		var baseQuery *gorm.DB
		if byType == "all" {
			baseQuery = db.Model(&models.Insight{}).
				Where("detected_at >= ? AND deleted_at IS NULL", start).
				Where("(resource_type != 'Pod' OR EXISTS (SELECT 1 FROM pods p WHERE p.cluster_id = insights.cluster_id AND p.uid = insights.resource_uid AND p.deleted_at IS NULL))")
		} else {
			// Default "vulnerability" mode: CVE findings plus supply-chain malware (same operational slice as Risk Findings / SBOM threats).
			baseQuery = db.Model(&models.Insight{}).
				Where("insight_type IN ? AND detected_at >= ? AND deleted_at IS NULL", []string{"vulnerability", "supply_chain_malware"}, start).
				Where("(resource_type != 'Pod' OR EXISTS (SELECT 1 FROM pods p WHERE p.cluster_id = insights.cluster_id AND p.uid = insights.resource_uid AND p.deleted_at IS NULL))")
		}
		if clusterID != "" {
			baseQuery = baseQuery.Where("insights.cluster_id = ?", clusterID)
		}
		baseQuery = scopedAggregateQuery(db, baseQuery, filter, "insights.resource_uid")
		level := "CASE WHEN pref.total_score IS NULL THEN 'unscored' WHEN pref.total_score >= 70 THEN 'critical' WHEN pref.total_score >= 40 THEN 'high' WHEN pref.total_score >= 20 THEN 'medium' ELSE 'low' END"
		day := "date_trunc('day', insights.detected_at)"
		if err := baseQuery.
			Joins("LEFT JOIN " + preferredRiskScoreSubquerySQL + " AS pref ON pref.cluster_id = insights.cluster_id AND pref.resource_uid = insights.resource_uid").
			Select(day + " as date, " + level + " as level, COUNT(*) as count").
			Group(day + ", " + level).
			Order(day).
			Scan(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load threat velocity"})
			return
		}

		points := map[string]*ThreatVelocityPoint{}
		for i := 0; i < days; i++ {
			d := start.AddDate(0, 0, i)
			key := d.Format("2006-01-02")
			points[key] = &ThreatVelocityPoint{Date: key}
		}

		for _, row := range rows {
			key := row.Date.Format("2006-01-02")
			point, ok := points[key]
			if !ok {
				point = &ThreatVelocityPoint{Date: key}
				points[key] = point
			}
			switch row.Level {
			case "unscored":
				point.UnscoredCount = row.Count
			case "critical":
				point.CriticalCount = row.Count
			case "high":
				point.HighCount = row.Count
			case "medium":
				point.MediumCount = row.Count
			case "low":
				point.LowCount = row.Count
			}
		}

		result := make([]ThreatVelocityPoint, 0, len(points))
		for i := 0; i < days; i++ {
			d := start.AddDate(0, 0, i)
			key := d.Format("2006-01-02")
			result = append(result, *points[key])
		}

		c.JSON(http.StatusOK, gin.H{"trend": result})
	}
}

type RiskFilter struct {
	Severity          string   `form:"severity"`
	Status            string   `form:"status"`
	Search            string   `form:"search"`
	Type              string   `form:"type"`
	ClusterID         string   `form:"clusterId"`
	ScopedClusterIDs  []string `form:"-"`
	ResourceNamespace string   `form:"resourceNamespace"` // namespace filter (Phase 1)
	// SinceMinutes: when > 0, only insights with detected_at >= now - sinceMinutes.
	// Not applied when Type is vulnerability or supply_chain_malware (SBOM-derived; detected_at is first-seen, not recurring).
	SinceMinutes int    `form:"sinceMinutes"`
	WithScores   int    `form:"withScores"` // when != 0: include totalScore, final_level, breakdown from preferred risk_scores
	FinalLevel   string `form:"finalLevel"` // low|medium|high|critical — filter by ADR bands on preferred total_score
	ScoreBin     int    `form:"scoreBin"`   // when 0,10,...,90: filter to findings whose resource score is in [scoreBin, scoreBin+10) (histogram click)
	View         string `form:"view"`       // instance (default) | group — grouped findings by type + CVE/title key
	// Sort and Order pick the server-side ordering (see normalizeInsightsListSort). Unknown values are ignored.
	Sort  string `form:"sort"`
	Order string `form:"order"`
	// Assignee is "me" (the caller's findings) or "none" (unassigned). AssigneeUserID is resolved from the session.
	Assignee       string `form:"assignee"`
	AssigneeUserID uint   `form:"-"`
}

// applyRiskFilterAssignee validates the assignee filter and resolves "me" to the caller. It answers 400
// for any other value, so a typo does not silently return every finding.
func applyRiskFilterAssignee(c *gin.Context, filter *RiskFilter) bool {
	filter.Assignee = strings.ToLower(strings.TrimSpace(filter.Assignee))
	switch filter.Assignee {
	case "":
		return true
	case "none":
		return true
	case "me":
		id, ok := requestUserID(c)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "assignee=me needs a signed-in user"})
			return false
		}
		filter.AssigneeUserID = id
		return true
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "assignee must be me or none"})
		return false
	}
}

func applyRiskFilterClusterScope(db *gorm.DB, c *gin.Context, filter *RiskFilter) bool {
	if filter == nil {
		return true
	}
	clusterID := strings.TrimSpace(filter.ClusterID)
	if clusterID != "" {
		clusterID = NormalizeClusterID(db, clusterID)
		filter.ClusterID = clusterID
		if _, restricted := middleware.ScopedClusterIDs(c); restricted && !middleware.ClusterAllowed(c, clusterID) {
			middleware.AbortClusterScopeDenied(db, c, clusterID)
			return false
		}
		return true
	}
	if clusterIDs, restricted := middleware.ScopedClusterIDs(c); restricted {
		filter.ScopedClusterIDs = clusterIDs
	}
	return true
}

// riskInsightsListTimeWindowApplies is false for SBOM-derived insight types: their detected_at stays at first
// match, so a short sinceMinutes (dashboard time window) would hide active findings incorrectly.
func riskInsightsListTimeWindowApplies(insightType string) bool {
	switch strings.TrimSpace(insightType) {
	case "vulnerability", "supply_chain_malware":
		return false
	default:
		return true
	}
}

// InsightWithScore extends Insight with optional risk score fields (from risk_scores join).
type InsightWithScore struct {
	models.Insight
	TotalScore          *float64            `json:"totalScore,omitempty"`
	ExploitabilityScore *float64            `json:"exploitabilityScore,omitempty"`
	BusinessImpactScore *float64            `json:"businessImpactScore,omitempty"`
	TimeDecay           *float64            `json:"timeDecay,omitempty"`
	SeverityHint        string              `json:"severity_hint,omitempty"`
	FinalScore          *float64            `json:"final_score,omitempty"`
	FinalLevel          string              `json:"final_level,omitempty"`
	Breakdown           []RiskBreakdownItem `json:"breakdown,omitempty"`
	Risk                *RiskProjection     `json:"risk,omitempty"`
	Drivers             []RiskBreakdownItem `json:"drivers,omitempty"`
	Meta                *RiskMetaProjection `json:"meta,omitempty"`
}

// RiskBreakdownItem is the canonical explainability row for risk API consumers (alias of pkg/risk type).
type RiskBreakdownItem = risk.CanonicalBreakdownItem

type RiskProjection struct {
	Score float64 `json:"score"`
	Level string  `json:"level"`
}

type RiskMetaProjection struct {
	LastUpdatedAt string `json:"last_updated_at,omitempty"`
}

func scorerPreferenceRank(version string) int {
	if strings.EqualFold(strings.TrimSpace(version), "v3") {
		return 1
	}
	return 0
}

// insightsListApplyFilters applies the same WHERE clauses as GET /risk/insights list (Risk Center).
func insightsListApplyFilters(query *gorm.DB, db *gorm.DB, filter RiskFilter, statusFilter string) *gorm.DB {
	// Qualify columns with insights. so the same scope is safe when combined with JOINs (e.g. grouped list + pref scores).
	if strings.TrimSpace(filter.ClusterID) != "" {
		clusterID := NormalizeClusterID(db, strings.TrimSpace(filter.ClusterID))
		query = query.Where(
			"insights.resource_type = ? AND insights.resource_uid IN (SELECT uid FROM pods WHERE cluster_id = ? AND deleted_at IS NULL)",
			"Pod", clusterID,
		)
	} else if len(filter.ScopedClusterIDs) > 0 {
		query = query.Where(
			"insights.resource_type = ? AND insights.resource_uid IN (SELECT uid FROM pods WHERE cluster_id IN ? AND deleted_at IS NULL)",
			"Pod", filter.ScopedClusterIDs,
		)
	} else {
		query = query.Where("(insights.resource_type != 'Pod' OR insights.resource_uid IN (SELECT uid FROM pods WHERE deleted_at IS NULL))")
	}
	if filter.Severity != "" {
		query = query.Where("LOWER(insights.severity) = ?", filter.Severity)
	}
	switch statusFilter {
	case "", "all":
	case "open":
		// Everything still on someone's plate: needs triage or in review.
		query = query.Where("insights.status IN ?", []string{"active", "acknowledged"})
	default:
		query = query.Where("insights.status = ?", statusFilter)
	}
	if filter.Type != "" {
		query = query.Where("insights.insight_type = ?", filter.Type)
	}
	if strings.TrimSpace(filter.ResourceNamespace) != "" {
		query = query.Where("insights.resource_namespace = ?", strings.TrimSpace(filter.ResourceNamespace))
	}
	if filter.Search != "" {
		search := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where(
			"LOWER(insights.title) LIKE ? OR LOWER(insights.description) LIKE ? OR LOWER(insights.resource_name) LIKE ? OR LOWER(insights.cve_id) LIKE ? OR LOWER(insights.affected_component) LIKE ?",
			search, search, search, search, search,
		)
	}
	if filter.SinceMinutes > 0 && riskInsightsListTimeWindowApplies(filter.Type) {
		since := time.Now().Add(-time.Duration(filter.SinceMinutes) * time.Minute)
		query = query.Where("insights.detected_at >= ?", since)
	}
	switch filter.Assignee {
	case "me":
		if filter.AssigneeUserID == 0 {
			query = query.Where("1 = 0")
		} else {
			query = query.Where("insights.assignee_user_id = ?", filter.AssigneeUserID)
		}
	case "none":
		query = query.Where("insights.assignee_user_id IS NULL")
	}
	query = applyInsightsListFinalLevelFilter(query, filter.FinalLevel)
	return query
}

// applyInsightsListFinalLevelFilter keeps findings whose preferred risk score maps to the given ADR final level.
func applyInsightsListFinalLevelFilter(query *gorm.DB, finalLevel string) *gorm.DB {
	fl := strings.ToLower(strings.TrimSpace(finalLevel))
	var lo, hi float64
	switch fl {
	case "critical":
		lo, hi = 70, 101
	case "high":
		lo, hi = 40, 70
	case "medium":
		lo, hi = 20, 40
	case "low":
		lo, hi = 0, 20
	default:
		return query
	}
	return query.Where(`EXISTS (
		SELECT 1 FROM `+preferredRiskScoreSubquerySQL+` AS s
		WHERE s.cluster_id = insights.cluster_id
		  AND s.resource_uid = insights.resource_uid
		  AND s.total_score >= ? AND s.total_score < ?
	)`, lo, hi)
}

// preferredRiskScoreSubquerySQL returns one authoritative total_score per (cluster, resource):
// the latest V3 row, else the latest row of any scorer version (see preferredRiskScoreOrderSQL).
const preferredRiskScoreSubquerySQL = `(SELECT z.cluster_id, z.resource_uid, z.total_score FROM (
	SELECT rs.cluster_id AS cluster_id, rs.resource_uid AS resource_uid, rs.total_score AS total_score,
		ROW_NUMBER() OVER (
			PARTITION BY rs.cluster_id, rs.resource_uid
			ORDER BY CASE LOWER(TRIM(COALESCE(rs.scorer_version, ''))) WHEN 'v3' THEN 1 ELSE 0 END DESC,
				rs.calculated_at DESC, rs.id DESC
		) AS rn
	FROM risk_scores rs WHERE rs.deleted_at IS NULL
) z WHERE z.rn = 1)`

// applyInsightsListScoreBinFilter restricts rows to resources whose preferred risk score lies in [scoreBin, scoreBin+10).
func applyInsightsListScoreBinFilter(query *gorm.DB, hasScoreBin bool, scoreBin int) *gorm.DB {
	if !hasScoreBin {
		return query
	}
	if scoreBin < 0 || scoreBin > 90 || scoreBin%10 != 0 {
		return query
	}
	hi := float64(scoreBin + 10)
	lo := float64(scoreBin)
	return query.Where(`EXISTS (
		SELECT 1 FROM `+preferredRiskScoreSubquerySQL+` AS s
		WHERE s.cluster_id = insights.cluster_id
		  AND s.resource_uid = insights.resource_uid
		  AND s.total_score >= ? AND s.total_score < ?
	)`, lo, hi)
}

func severityStringFromMaxRank(rank int) string {
	switch rank {
	case 4:
		return "critical"
	case 3:
		return "high"
	case 2:
		return "medium"
	case 1:
		return "low"
	default:
		return "medium"
	}
}

// getInsightsGroupListData returns paginated groups (insight_type + CVE or normalized title) with member counts and max preferred score.
func getInsightsGroupListData(db *gorm.DB, filter RiskFilter, page, pageSize int, hasScoreBin bool) (gin.H, error) {
	statusFilter := strings.TrimSpace(filter.Status)
	if statusFilter == "" {
		statusFilter = "active"
	}
	base := db.Model(&models.Insight{})
	base = insightsListApplyFilters(base, db, filter, statusFilter)
	base = applyInsightsListScoreBinFilter(base, hasScoreBin, filter.ScoreBin)

	joinPreferred := `LEFT JOIN ` + preferredRiskScoreSubquerySQL + ` AS pref ON pref.cluster_id = insights.cluster_id AND pref.resource_uid = insights.resource_uid`
	groupKeyExpr := `COALESCE(NULLIF(TRIM(LOWER(insights.cve_id)), ''), LOWER(TRIM(insights.title)))`
	sevRankExpr := `MAX(CASE LOWER(insights.severity) WHEN 'critical' THEN 4 WHEN 'high' THEN 3 WHEN 'medium' THEN 2 WHEN 'low' THEN 1 ELSE 0 END)`

	subGrouped := base.Session(&gorm.Session{}).
		Joins(joinPreferred).
		Select("insights.insight_type, " + groupKeyExpr + " AS group_key").
		Group("insights.insight_type, " + groupKeyExpr)

	var total int64
	if err := db.Table("(?) AS grouped", subGrouped).Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	type groupScanRow struct {
		InsightType     string   `gorm:"column:insight_type"`
		GroupKey        string   `gorm:"column:group_key"`
		MemberCount     int64    `gorm:"column:member_count"`
		MaxScore        *float64 `gorm:"column:max_score"`
		MaxSevRank      int      `gorm:"column:max_sev_rank"`
		SampleInsightID uint     `gorm:"column:sample_insight_id"`
		DisplayTitle    string   `gorm:"column:display_title"`
	}
	var rows []groupScanRow
	err := base.Session(&gorm.Session{}).
		Joins(joinPreferred).
		Select(`insights.insight_type AS insight_type, ` + groupKeyExpr + ` AS group_key,
			COUNT(*) AS member_count,
			MAX(pref.total_score) AS max_score,
			` + sevRankExpr + ` AS max_sev_rank,
			MIN(insights.id) AS sample_insight_id,
			MAX(insights.title) AS display_title`).
		Group(`insights.insight_type, ` + groupKeyExpr).
		Order(insightsGroupListOrder(normalizeInsightsListSort(filter.Sort, filter.Order))).
		Offset(offset).Limit(pageSize).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	groups := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		h := gin.H{
			"group_key":         r.GroupKey,
			"insight_type":      r.InsightType,
			"title":             r.DisplayTitle,
			"member_count":      r.MemberCount,
			"sample_insight_id": r.SampleInsightID,
			"max_severity":      severityStringFromMaxRank(r.MaxSevRank),
		}
		if r.MaxScore != nil {
			s := *r.MaxScore
			h["max_score"] = s
			h["final_level"] = risk.DeriveFinalLevelFromScore(s)
		}
		groups = append(groups, h)
	}

	return gin.H{
		"view":     "group",
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"groups":   groups,
		"insights": []InsightWithScore{},
	}, nil
}

// getInsightsListData runs the same query as GetInsightsList and returns the response map (for caching).
// hasScoreBin indicates whether scoreBin was explicitly provided as a query parameter (to distinguish from default zero value).
func getInsightsListData(db *gorm.DB, filter RiskFilter, page, pageSize int, hasScoreBin bool) (gin.H, error) {
	start := time.Now()
	var listLen int
	defer func() {
		metrics.RiskEvaluationDuration.Observe(time.Since(start).Seconds())
		metrics.InsightsBatchSize.Observe(float64(listLen))
	}()

	view := strings.ToLower(strings.TrimSpace(filter.View))
	if view == "group" {
		out, err := getInsightsGroupListData(db, filter, page, pageSize, hasScoreBin)
		if err != nil {
			return nil, err
		}
		if raw, ok := out["groups"]; ok {
			switch g := raw.(type) {
			case []gin.H:
				listLen = len(g)
			case []interface{}:
				listLen = len(g)
			}
		}
		return out, nil
	}

	statusFilter := strings.TrimSpace(filter.Status)
	if statusFilter == "" {
		statusFilter = "active"
	}
	query := db.Model(&models.Insight{})
	query = insightsListApplyFilters(query, db, filter, statusFilter)
	query = applyInsightsListScoreBinFilter(query, hasScoreBin, filter.ScoreBin)

	var total int64
	query.Count(&total)
	offset := (page - 1) * pageSize
	var insights []models.Insight
	sortKey, sortOrder := normalizeInsightsListSort(filter.Sort, filter.Order)
	query = applyInsightsListOrder(query, sortKey, sortOrder)
	query.Offset(offset).Limit(pageSize).Find(&insights)

	if filter.WithScores == 0 || len(insights) == 0 {
		listLen = len(insights)
		return gin.H{
			"view":     "instance",
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
			"insights": insights,
		}, nil
	}
	listLen = len(insights)

	// Phase 3.1: left-join risk_scores by resource_uid (batch lookup)
	uids := make([]string, 0, len(insights))
	seen := make(map[string]struct{})
	for _, i := range insights {
		if i.ResourceUID != "" {
			if _, ok := seen[i.ResourceUID]; !ok {
				seen[i.ResourceUID] = struct{}{}
				uids = append(uids, i.ResourceUID)
			}
		}
	}
	var scores []models.RiskScore
	if len(uids) > 0 {
		db.Model(&models.RiskScore{}).Where("resource_uid IN ?", uids).Find(&scores)
	}
	scoreByUID := make(map[string]*models.RiskScore)
	for i := range scores {
		cur := &scores[i]
		existing, ok := scoreByUID[cur.ResourceUID]
		if !ok {
			scoreByUID[cur.ResourceUID] = cur
			continue
		}
		existingRank := scorerPreferenceRank(existing.ScorerVersion)
		curRank := scorerPreferenceRank(cur.ScorerVersion)
		if curRank > existingRank || (curRank == existingRank && cur.CalculatedAt.After(existing.CalculatedAt)) {
			scoreByUID[cur.ResourceUID] = cur
		}
	}
	withScores := make([]InsightWithScore, len(insights))
	for i := range insights {
		withScores[i] = InsightWithScore{
			Insight:      insights[i],
			SeverityHint: insights[i].Severity,
		}
		if s := scoreByUID[insights[i].ResourceUID]; s != nil {
			withScores[i].TotalScore = &s.TotalScore
			withScores[i].ExploitabilityScore = &s.ExploitabilityScore
			withScores[i].BusinessImpactScore = &s.BusinessImpactScore
			withScores[i].TimeDecay = &s.TimeDecay
			withScores[i].FinalScore = &s.TotalScore
			withScores[i].FinalLevel = risk.DeriveFinalLevelFromScore(s.TotalScore)
			withScores[i].Breakdown = risk.ParseBreakdownFromFactorsJSON(s.Factors)
			withScores[i].Risk = &RiskProjection{
				Score: s.TotalScore,
				Level: withScores[i].FinalLevel,
			}
			withScores[i].Drivers = withScores[i].Breakdown
			withScores[i].Meta = &RiskMetaProjection{
				LastUpdatedAt: s.CalculatedAt.UTC().Format(time.RFC3339),
			}
		}
	}
	listLen = len(insights)
	return gin.H{
		"view":     "instance",
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"insights": withScores,
	}, nil
}

// GetInsightsList is reused for /risks (with filters).
func GetInsightsList(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var filter RiskFilter
		if err := c.ShouldBindQuery(&filter); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if !applyRiskFilterClusterScope(db, c, &filter) {
			return
		}
		if !applyRiskFilterAssignee(c, &filter) {
			return
		}
		hasScoreBin := strings.TrimSpace(c.Query("scoreBin")) != ""
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
		if page < 1 {
			page = 1
		}
		if pageSize < 1 || pageSize > 100 {
			pageSize = 20
		}
		result, err := getInsightsListData(db, filter, page, pageSize, hasScoreBin)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// GetInsightsListCached wraps GetInsightsList with in-memory cache (TTL 60s). Use when defaultRisksCache is set.
func GetInsightsListCached(db *gorm.DB) gin.HandlerFunc {
	inner := GetInsightsList(db)
	return func(c *gin.Context) {
		if defaultRisksCache == nil {
			inner(c)
			return
		}
		var filter RiskFilter
		if err := c.ShouldBindQuery(&filter); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters"})
			return
		}
		if !applyRiskFilterClusterScope(db, c, &filter) {
			return
		}
		if !applyRiskFilterAssignee(c, &filter) {
			return
		}
		hasScoreBin := strings.TrimSpace(c.Query("scoreBin")) != ""
		statusFilter := strings.TrimSpace(filter.Status)
		if statusFilter == "" {
			statusFilter = "active"
		}
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
		if page < 1 {
			page = 1
		}
		if pageSize < 1 || pageSize > 100 {
			pageSize = 20
		}
		clusterID := strings.TrimSpace(filter.ClusterID)
		if clusterID != "" {
			clusterID = NormalizeClusterID(db, clusterID)
		} else if len(filter.ScopedClusterIDs) > 0 {
			clusterID = "scope:" + strings.Join(filter.ScopedClusterIDs, ",")
		}
		sortKey, sortOrder := normalizeInsightsListSort(filter.Sort, filter.Order)
		key := BuildRisksListCacheKey(clusterID, statusFilter, filter.Severity, filter.Search, strings.TrimSpace(filter.FinalLevel), strings.TrimSpace(filter.ResourceNamespace), strings.TrimSpace(filter.Type), filter.SinceMinutes, page, pageSize, filter.WithScores, filter.ScoreBin, strings.ToLower(strings.TrimSpace(filter.View)), sortKey, sortOrder)
		key = authorizationCacheKey(c, key) + ":bin=" + strconv.FormatBool(hasScoreBin)
		// "me" differs per caller, so the resolved user id is part of the key.
		key += ":assignee=" + filter.Assignee + ":" + strconv.FormatUint(uint64(filter.AssigneeUserID), 10)
		if b, ok := defaultRisksCache.Get(key); ok {
			c.Data(http.StatusOK, "application/json", b)
			return
		}
		result, err := getInsightsListData(db, filter, page, pageSize, hasScoreBin)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		b, _ := json.Marshal(result)
		defaultRisksCache.Set(key, b, risksCacheTTL)
		c.Data(http.StatusOK, "application/json", b)
	}
}

// riskHistogramBin is one bin for GET /risk/histogram (score distribution).
type riskHistogramBin struct {
	Bin           int `json:"bin"`
	Count         int `json:"count"`
	CriticalCount int `json:"critical_count"`
	HighCount     int `json:"high_count"`
	MediumCount   int `json:"medium_count"`
	LowCount      int `json:"low_count"`
}

// GetRiskHistogram returns score distribution (bins 0–100) for Risk Center histogram chart.
// Query: clusterId, sinceMinutes (default 30), withScores=1. Cached 30s; invalidated on insights update.

func GetRiskHistogram(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID := strings.TrimSpace(c.Query("clusterId"))
		if clusterID != "" {
			clusterID = NormalizeClusterID(db, clusterID)
		}
		filter, allowed := aggregateScope(db, c)
		if !allowed {
			return
		}
		sinceMinutes, _ := strconv.Atoi(c.DefaultQuery("sinceMinutes", "30"))
		if sinceMinutes < 0 {
			sinceMinutes = 30
		}
		key := authorizationCacheKey(c, BuildRiskHistogramCacheKey(clusterID, sinceMinutes))
		if defaultRisksCache != nil {
			if b, ok := defaultRisksCache.Get(key); ok {
				c.Data(http.StatusOK, "application/json", b)
				return
			}
		}

		// Base filter: insights joined to risk_scores, active only, optional cluster + since
		joinWhere := "i.deleted_at IS NULL AND (i.status IN ('active', 'acknowledged') OR i.status IS NULL)"
		args := []interface{}{}
		if clusterID != "" {
			joinWhere += " AND i.resource_type = 'Pod' AND i.resource_uid IN (SELECT uid FROM pods WHERE cluster_id = ? AND deleted_at IS NULL)"
			args = append(args, clusterID)
		} else {
			joinWhere += " AND (i.resource_type != 'Pod' OR i.resource_uid IN (SELECT uid FROM pods WHERE deleted_at IS NULL))"
		}
		if len(filter.ScopedClusterIDs) > 0 {
			joinWhere += " AND i.resource_uid IN (SELECT uid FROM pods WHERE cluster_id IN ? AND deleted_at IS NULL)"
			args = append(args, filter.ScopedClusterIDs)
		}
		if sinceMinutes > 0 {
			joinWhere += " AND i.detected_at >= ?"
			args = append(args, time.Now().Add(-time.Duration(sinceMinutes)*time.Minute))
		}

		// Bins: FLOOR(rs.total_score/10)*10, count and severity breakdown
		binQuery := `SELECT (CASE WHEN rs.total_score >= 100 THEN 90 ELSE FLOOR(rs.total_score / 10) * 10 END) AS bin,
  COUNT(*) AS count,
  COALESCE(SUM(CASE WHEN LOWER(i.severity) = 'critical' THEN 1 ELSE 0 END), 0) AS critical_count,
  COALESCE(SUM(CASE WHEN LOWER(i.severity) = 'high' THEN 1 ELSE 0 END), 0) AS high_count,
  COALESCE(SUM(CASE WHEN LOWER(i.severity) = 'medium' THEN 1 ELSE 0 END), 0) AS medium_count,
  COALESCE(SUM(CASE WHEN LOWER(i.severity) = 'low' THEN 1 ELSE 0 END), 0) AS low_count
FROM insights i
INNER JOIN ` + preferredRiskScoreSubquerySQL + ` AS rs ON rs.cluster_id = i.cluster_id AND rs.resource_uid = i.resource_uid
WHERE ` + joinWhere + `
GROUP BY CASE WHEN rs.total_score >= 100 THEN 90 ELSE FLOOR(rs.total_score / 10) * 10 END
ORDER BY bin`
		var bins []riskHistogramBin
		if err := db.Raw(binQuery, args...).Scan(&bins).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Totals and average score, P0 count
		totQuery := `SELECT COUNT(*) AS total_findings,
  COALESCE(AVG(rs.total_score), 0) AS average_score,
  COALESCE(SUM(CASE WHEN rs.total_score >= 70 THEN 1 ELSE 0 END), 0) AS p0_count
FROM insights i
INNER JOIN ` + preferredRiskScoreSubquerySQL + ` AS rs ON rs.cluster_id = i.cluster_id AND rs.resource_uid = i.resource_uid
WHERE ` + joinWhere
		var totalFindings int
		var averageScore float64
		var p0Count int
		if err := db.Raw(totQuery, args...).Row().Scan(&totalFindings, &averageScore, &p0Count); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Fill all 10 bins 0,10,...,90 (frontend expects fixed bins)
		binMap := make(map[int]riskHistogramBin)
		for _, b := range bins {
			binMap[b.Bin] = b
		}
		outBins := make([]gin.H, 0, 10)
		for b := 0; b <= 90; b += 10 {
			row := binMap[b]
			percent := 0.0
			if totalFindings > 0 && row.Count > 0 {
				percent = float64(row.Count) / float64(totalFindings) * 100
			}
			outBins = append(outBins, gin.H{
				"bin":               b,
				"count":             row.Count,
				"percent":           roundPercent(percent),
				"severityBreakdown": gin.H{"critical": row.CriticalCount, "high": row.HighCount, "medium": row.MediumCount, "low": row.LowCount},
				"critical_count":    row.CriticalCount,
				"high_count":        row.HighCount,
				"medium_count":      row.MediumCount,
				"low_count":         row.LowCount,
			})
		}

		resp := gin.H{
			"bins":          outBins,
			"totalFindings": totalFindings,
			"averageScore":  roundPercent(averageScore),
			"p0Count":       p0Count,
		}
		b, _ := json.Marshal(resp)
		if defaultRisksCache != nil {
			defaultRisksCache.Set(key, b, 30*time.Second)
		}
		c.Data(http.StatusOK, "application/json", b)
	}
}

func roundPercent(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

const maxRisksExportLimit = 10000

// csvSafeCell stops spreadsheet apps from evaluating a cell as a formula.
// Finding titles and resource names come from cluster objects, so a pod named
// "=HYPERLINK(...)" would otherwise run when an analyst opens the export.
func csvSafeCell(v string) string {
	if v == "" {
		return v
	}
	switch v[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + v
	}
	return v
}

// writeRisksExportHTML writes print-optimized HTML table for "Print to PDF" (Phase 3.3).
// exportRiskLevel returns the finding's risk level and score as the Risk Center shows
// them, or empty strings when its resource has no score yet.
func exportRiskLevel(scores map[string]float64, i models.Insight) (string, string) {
	score, ok := scores[riskScoreKey(i.ClusterID, i.ResourceUID)]
	if !ok {
		return "", ""
	}
	return risk.DeriveFinalLevelFromScore(score), strconv.FormatFloat(score, 'f', 0, 64)
}

func writeRisksExportHTML(w http.ResponseWriter, insights []models.Insight, scores map[string]float64) {
	w.Write([]byte(`<!DOCTYPE html><html><head><meta charset="utf-8"/><title>Risks Export</title>`))
	w.Write([]byte(`<style>body{font-family:sans-serif;margin:1rem;} table{border-collapse:collapse;width:100%;} th,td{border:1px solid #333;padding:6px;text-align:left;} th{background:#444;color:#fff;} @media print{body{margin:0;}}</style></head><body>`))
	w.Write([]byte(`<h1>Risks Export</h1><p>Generated at ` + time.Now().Format(time.RFC3339) + ` — ` + strconv.Itoa(len(insights)) + ` findings. Use browser Print → Save as PDF.</p><table><thead><tr>`))
	headers := []string{"ID", "Title", "Risk level", "Risk score", "Rule severity", "Status", "Type", "Resource", "Namespace", "Finding ref", "Detected At"}
	for _, h := range headers {
		w.Write([]byte("<th>" + html.EscapeString(h) + "</th>"))
	}
	w.Write([]byte("</tr></thead><tbody>"))
	for _, i := range insights {
		level, score := exportRiskLevel(scores, i)
		w.Write([]byte("<tr><td>" + strconv.FormatUint(uint64(i.ID), 10) + "</td><td>" + html.EscapeString(i.Title) + "</td><td>" + html.EscapeString(level) + "</td><td>" + html.EscapeString(score) + "</td><td>" + html.EscapeString(i.Severity) + "</td><td>" + html.EscapeString(i.Status) + "</td><td>" + html.EscapeString(i.InsightType) + "</td><td>" + html.EscapeString(i.ResourceName) + "</td><td>" + html.EscapeString(i.ResourceNamespace) + "</td><td>" + html.EscapeString(i.CVEID) + "</td><td>" + i.DetectedAt.Format(time.RFC3339) + "</td></tr>"))
	}
	w.Write([]byte("</tbody></table></body></html>"))
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// ExportRisksCSV returns risks (insights) as CSV or PDF (print-optimized HTML) with same filters as GetInsightsList.
// Query params: clusterId, severity, status, search, type, sinceMinutes, assignee=me|none, format=csv|pdf (default csv).
func ExportRisksCSV(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var filter RiskFilter
		if err := c.ShouldBindQuery(&filter); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// Exports leave the platform, so they get the same user cluster scope and
		// the same filters as the Risk Center list they are exported from.
		if !applyRiskFilterClusterScope(db, c, &filter) {
			return
		}
		if !applyRiskFilterAssignee(c, &filter) {
			return
		}
		format := strings.ToLower(strings.TrimSpace(c.Query("format")))
		hasScoreBin := strings.TrimSpace(c.Query("scoreBin")) != ""
		statusFilter := strings.TrimSpace(filter.Status)
		if statusFilter == "" {
			statusFilter = "active"
		}

		query := insightsListApplyFilters(db.Model(&models.Insight{}), db, filter, statusFilter)
		// The list scopes through pod UIDs, which are unique only inside a
		// cluster; an export also pins the finding's own cluster.
		if scopedIDs, restricted := middleware.ScopedClusterIDs(c); restricted {
			query = query.Where("insights.cluster_id IN ?", scopedIDs)
		}
		// Histogram bin filter: score in [scoreBin, scoreBin+10) (e.g. scoreBin=10 → 10–19)
		// Only apply when scoreBin query param is explicitly provided.
		// Same preferred score per (cluster, resource) as the list.
		query = applyInsightsListScoreBinFilter(query, hasScoreBin, filter.ScoreBin)

		if format == "pdf" {
			// PDF: load up to limit for HTML (single response)
			var insights []models.Insight
			query.Order("detected_at DESC").Limit(maxRisksExportLimit).Find(&insights)
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.Header("Content-Disposition", `attachment; filename="risks-export.html"`)
			writeRisksExportHTML(c.Writer, insights, preferredScoresForInsights(db, insights))
			ev := securityaudit.FromRequest(
				c,
				authorization.ToStrings(middleware.GrantedPermissions(c)),
				c.GetString(middleware.CtxJWTSessionID),
				"export_findings",
				"export",
				"insights",
				"success",
				"medium",
				"jwt",
				nil,
				map[string]any{"format": "pdf", "row_count": len(insights)},
				map[string]any{"filters": filter},
				nil,
			)
			securityaudit.Append(db, &ev)
			return
		}

		// CSV: stream in chunks (Phase 3 export streaming)
		exportStart := time.Now()
		var totalExported int
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", `attachment; filename="risks-export.csv"`)
		csvW := csv.NewWriter(c.Writer)
		_ = csvW.Write([]string{
			"id", "title", "description", "severity", "status", "insight_type", "resource_type",
			"resource_name", "resource_namespace", "resource_uid", "finding_reference", "detected_at", "created_at", "updated_at",
			// Appended so existing column positions stay; severity above is the rule severity.
			"risk_level", "risk_score",
		})
		csvW.Flush()
		if err := csvW.Error(); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		const exportChunkSize = 500
		for offset := 0; offset < maxRisksExportLimit; offset += exportChunkSize {
			var chunk []models.Insight
			if err := query.Order("detected_at DESC").Limit(exportChunkSize).Offset(offset).Find(&chunk).Error; err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			if len(chunk) == 0 {
				break
			}
			totalExported += len(chunk)
			scores := preferredScoresForInsights(db, chunk)
			for _, i := range chunk {
				level, score := exportRiskLevel(scores, i)
				_ = csvW.Write([]string{
					strconv.FormatUint(uint64(i.ID), 10),
					csvSafeCell(i.Title),
					csvSafeCell(i.Description),
					csvSafeCell(i.Severity),
					csvSafeCell(i.Status),
					csvSafeCell(i.InsightType),
					csvSafeCell(i.ResourceType),
					csvSafeCell(i.ResourceName),
					csvSafeCell(i.ResourceNamespace),
					csvSafeCell(i.ResourceUID),
					csvSafeCell(i.CVEID),
					i.DetectedAt.Format(time.RFC3339),
					i.CreatedAt.Format(time.RFC3339),
					i.UpdatedAt.Format(time.RFC3339),
					level,
					score,
				})
			}
			csvW.Flush()
			if err := csvW.Error(); err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			if len(chunk) < exportChunkSize {
				break
			}
		}
		metrics.RiskEvaluationDuration.Observe(time.Since(exportStart).Seconds())
		metrics.InsightsBatchSize.Observe(float64(totalExported))
		ev := securityaudit.FromRequest(
			c,
			authorization.ToStrings(middleware.GrantedPermissions(c)),
			c.GetString(middleware.CtxJWTSessionID),
			"export_findings",
			"export",
			"insights",
			"success",
			"medium",
			"jwt",
			nil,
			map[string]any{"format": "csv", "row_count": totalExported},
			map[string]any{"filters": filter},
			nil,
		)
		securityaudit.Append(db, &ev)
	}
}

// UpdateInsightStatus updates status (acknowledged/resolved).
func UpdateInsightStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var payload struct {
			Status string `json:"status" binding:"required"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// "active" reopens a finding; the route already admits findings.reopen
		// holders, so the handler must accept the transition they are granted.
		if payload.Status != "acknowledged" && payload.Status != "resolved" && payload.Status != "active" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status must be acknowledged, resolved or active"})
			return
		}

		needed := authorization.PermissionFindingsAck
		switch payload.Status {
		case "resolved":
			needed = authorization.PermissionFindingsResolve
		case "active":
			needed = authorization.PermissionFindingsReopen
		}
		if !authorization.HasPermission(middleware.GrantedPermissions(c), needed) {
			c.JSON(http.StatusForbidden, gin.H{"error": "permission denied"})
			return
		}

		var before models.Insight
		if err := db.First(&before, "id = ?", id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "insight not found"})
			return
		}
		if !requireInsightClusterScope(db, c, before) {
			return
		}
		if payload.Status == "acknowledged" && (before.Status == "resolved" || before.Status == "dismissed") {
			c.JSON(http.StatusConflict, gin.H{"error": "closed findings cannot be acknowledged"})
			return
		}
		if payload.Status == "active" && before.Status != "resolved" && before.Status != "dismissed" && before.Status != "acknowledged" {
			c.JSON(http.StatusConflict, gin.H{"error": "only resolved, dismissed or acknowledged findings can be reopened"})
			return
		}
		updates := map[string]interface{}{"status": payload.Status, "updated_at": time.Now()}
		switch payload.Status {
		case "resolved":
			updates["resolved_at"] = time.Now()
		case "active":
			updates["resolved_at"] = nil
		}
		if err := db.Model(&models.Insight{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		scheduleUnifiedScoreRecalculation(db, before)
		appendInsightGovernanceEvent(db, c, securityaudit.ActionFindingsPatch, id, before.Status, payload.Status, map[string]any{"via": "PATCH"})

		c.JSON(http.StatusOK, gin.H{"status": payload.Status})
	}
}

// AttackPathsGraph returns nodes/links from existing graph endpoint (wraps GetGraph).
func AttackPathsGraph(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		GetGraph(db)(c)
	}
}
