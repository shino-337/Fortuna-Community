package api

import (
	"encoding/csv"
	"encoding/json"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/metrics"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DashboardStatsDTO struct {
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
}

// GetDashboardStats returns totals for active clusters, pods, agents, critical risks.
// Query param clusterId: when set, all counts are scoped to that cluster.
// Query param sinceMinutes: when > 0, insight counts limited to detected_at >= now - sinceMinutes.
// Query param byType: "vulnerability" (default) = CVE + supply_chain_malware; "all" = all insight types for totalRisks and criticalRisks.
func GetDashboardStats(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID := strings.TrimSpace(c.Query("clusterId"))
		if clusterID != "" {
			clusterID = NormalizeClusterID(db, clusterID)
		}
		byType := strings.ToLower(strings.TrimSpace(c.DefaultQuery("byType", "vulnerability")))
		sinceMinutes, _ := strconv.Atoi(c.DefaultQuery("sinceMinutes", "0"))
		var since time.Time
		if sinceMinutes > 0 {
			since = time.Now().Add(-time.Duration(sinceMinutes) * time.Minute)
		}
		detectedSinceClause := ""
		if sinceMinutes > 0 {
			detectedSinceClause = " AND i.detected_at >= ?"
		}

		var clusters int64
		var pods int64
		var agents int64
		var critical int64
		var totalRisks int64
		var resolved24h int64
		var affectedPodCount int64

		var clusterName string
		if clusterID != "" {
			// Verify cluster exists and load display name (from K8s via agent sync) for dashboard labels
			var cluster models.Cluster
			if err := db.First(&cluster, "id = ?", clusterID).Error; err != nil || cluster.ID == "" {
				c.JSON(http.StatusOK, DashboardStatsDTO{
					TotalClusters:    0,
					ActiveAgents:     0,
					RunningPods:      0,
					TotalRisks:       0,
					CriticalRisks:    0,
					Resolved24h:      0,
					AffectedPodCount: 0,
				})
				return
			}
			clusters = 1
			clusterName = cluster.Name

			// Same as GetClustersStats: count distinct pod UIDs so dashboard and cluster cards match
			db.Raw("SELECT COUNT(DISTINCT uid) FROM pods WHERE cluster_id = ? AND deleted_at IS NULL", clusterID).Scan(&pods)

			if db.Migrator().HasTable("agents") {
				db.Raw(`
					SELECT COUNT(*) FROM agents a
					WHERE a.deleted_at IS NULL AND (a.status = ? OR a.status IS NULL)
					AND a.node_name IN (
						SELECT DISTINCT node_name FROM pods WHERE cluster_id = ? AND deleted_at IS NULL AND node_name IS NOT NULL AND node_name != ''
					)
				`, "ready", clusterID).Scan(&agents)
			}

			// Risks: insights for Pods in this cluster; optional time window; byType=all counts all insight types
			if byType == "all" {
				criticalArgs := []interface{}{clusterID, "critical"}
				if sinceMinutes > 0 {
					criticalArgs = append(criticalArgs, since)
				}
				db.Raw(`
					SELECT COUNT(*) FROM insights i
					INNER JOIN pods p ON p.uid = i.resource_uid AND p.cluster_id = ? AND p.deleted_at IS NULL
					WHERE LOWER(i.severity) = ? AND i.deleted_at IS NULL`+detectedSinceClause,
					criticalArgs...).Scan(&critical)
				totalArgs := []interface{}{clusterID}
				if sinceMinutes > 0 {
					totalArgs = append(totalArgs, since)
				}
				db.Raw(`
					SELECT COUNT(*) FROM insights i
					INNER JOIN pods p ON p.uid = i.resource_uid AND p.cluster_id = ? AND p.deleted_at IS NULL
					WHERE i.deleted_at IS NULL`+detectedSinceClause,
					totalArgs...).Scan(&totalRisks)
			} else {
				criticalArgs := []interface{}{clusterID, "vulnerability", "supply_chain_malware", "critical"}
				if sinceMinutes > 0 {
					criticalArgs = append(criticalArgs, since)
				}
				db.Raw(`
					SELECT COUNT(*) FROM insights i
					INNER JOIN pods p ON p.uid = i.resource_uid AND p.cluster_id = ? AND p.deleted_at IS NULL
					WHERE i.insight_type IN (?, ?) AND LOWER(i.severity) = ? AND i.deleted_at IS NULL`+detectedSinceClause,
					criticalArgs...).Scan(&critical)
				totalArgs := []interface{}{clusterID, "vulnerability", "supply_chain_malware"}
				if sinceMinutes > 0 {
					totalArgs = append(totalArgs, since)
				}
				db.Raw(`
					SELECT COUNT(*) FROM insights i
					INNER JOIN pods p ON p.uid = i.resource_uid AND p.cluster_id = ? AND p.deleted_at IS NULL
					WHERE i.insight_type IN (?, ?) AND i.deleted_at IS NULL`+detectedSinceClause,
					totalArgs...).Scan(&totalRisks)
			}

			twentyFourHoursAgo := time.Now().Add(-24 * time.Hour)
			db.Raw(`
				SELECT COUNT(*) FROM insights i
				INNER JOIN pods p ON p.uid = i.resource_uid AND p.cluster_id = ? AND p.deleted_at IS NULL
				WHERE i.deleted_at IS NULL AND i.status = ? AND i.updated_at > ?
			`, clusterID, "resolved", twentyFourHoursAgo).Scan(&resolved24h)

			affectedArgs := []interface{}{clusterID}
			if sinceMinutes > 0 {
				affectedArgs = append(affectedArgs, since)
			}
			db.Raw(`
				SELECT COUNT(DISTINCT i.resource_uid) FROM insights i
				INNER JOIN pods p ON p.uid = i.resource_uid AND p.cluster_id = ? AND p.deleted_at IS NULL
				WHERE i.deleted_at IS NULL AND (i.status = 'active' OR i.status IS NULL) AND i.resource_type = 'Pod'`+detectedSinceClause,
				affectedArgs...).Scan(&affectedPodCount)
		} else {
			cutoff := time.Now().Add(-ActiveClusterCutoff)
			if db.Migrator().HasTable("clusters") {
				db.Raw(`
					SELECT COUNT(*) FROM clusters c
					WHERE c.source IN (?, ?) AND c.last_sync >= ?
					AND EXISTS (
						SELECT 1 FROM pods p
						WHERE p.cluster_id = c.id AND p.deleted_at IS NULL
					)
				`, "auto", "env", cutoff).Scan(&clusters)
			} else {
				db.Table("insights").Distinct("resource_namespace").Count(&clusters)
			}
			// Global pod count: all pods in DB (same scope as GET /pods) so Dashboard and Resources show the same total and match cluster reality.
			db.Raw("SELECT COUNT(DISTINCT uid) FROM pods WHERE deleted_at IS NULL").Scan(&pods)
			if db.Migrator().HasTable("agents") {
				// Only agents whose node is in a pod of an active cluster (same definition as GetClustersStats)
				db.Raw(`
					SELECT COUNT(*) FROM agents a
					WHERE a.deleted_at IS NULL AND (a.status = ? OR a.status IS NULL)
					AND a.node_name IN (
						SELECT DISTINCT p.node_name FROM pods p
						INNER JOIN clusters c ON c.id = p.cluster_id AND c.source IN (?, ?) AND c.last_sync >= ?
						WHERE p.deleted_at IS NULL AND p.node_name IS NOT NULL AND p.node_name != ''
					)
				`, "ready", "auto", "env", cutoff).Scan(&agents)
			}
			if byType == "all" {
				if sinceMinutes > 0 {
					db.Table("insights").Where("LOWER(severity) = ? AND deleted_at IS NULL AND detected_at >= ?", "critical", since).Count(&critical)
					db.Table("insights").Where("deleted_at IS NULL AND detected_at >= ?", since).Count(&totalRisks)
				} else {
					db.Table("insights").Where("LOWER(severity) = ? AND deleted_at IS NULL", "critical").Count(&critical)
					db.Table("insights").Where("deleted_at IS NULL").Count(&totalRisks)
				}
			} else {
				supplyTypes := []string{"vulnerability", "supply_chain_malware"}
				if sinceMinutes > 0 {
					db.Table("insights").
						Where("insight_type IN ? AND LOWER(severity) = ? AND deleted_at IS NULL AND detected_at >= ?", supplyTypes, "critical", since).
						Count(&critical)
					db.Table("insights").
						Where("insight_type IN ? AND deleted_at IS NULL AND detected_at >= ?", supplyTypes, since).
						Count(&totalRisks)
				} else {
					db.Table("insights").
						Where("insight_type IN ? AND LOWER(severity) = ? AND deleted_at IS NULL", supplyTypes, "critical").
						Count(&critical)
					db.Table("insights").
						Where("insight_type IN ? AND deleted_at IS NULL", supplyTypes).
						Count(&totalRisks)
				}
			}
			if db.Migrator().HasTable("insights") {
				twentyFourHoursAgo := time.Now().Add(-24 * time.Hour)
				db.Table("insights").
					Where("deleted_at IS NULL AND status = ? AND updated_at > ?", "resolved", twentyFourHoursAgo).
					Count(&resolved24h)
			}
			if db.Migrator().HasTable("insights") {
				if sinceMinutes > 0 {
					db.Raw(`
						SELECT COUNT(DISTINCT resource_uid) FROM insights
						WHERE deleted_at IS NULL AND (status = 'active' OR status IS NULL)
						AND resource_type = 'Pod' AND detected_at >= ?
					`, since).Scan(&affectedPodCount)
				} else {
					db.Raw(`
						SELECT COUNT(DISTINCT resource_uid) FROM insights
						WHERE deleted_at IS NULL AND (status = 'active' OR status IS NULL)
						AND resource_type = 'Pod'
					`).Scan(&affectedPodCount)
				}
			}
		}

		c.JSON(http.StatusOK, DashboardStatsDTO{
			TotalClusters:    clusters,
			ActiveAgents:     agents,
			RunningPods:      pods,
			TotalRisks:       totalRisks,
			CriticalRisks:    critical,
			Resolved24h:      resolved24h,
			AffectedPodCount: affectedPodCount,
			ClusterName:      clusterName,
		})
	}
}

// GetThreatVelocity returns daily counts of insights grouped by severity.
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

		start := time.Now().AddDate(0, 0, -days+1).Truncate(24 * time.Hour)

		var rows []struct {
			Date     time.Time
			Severity string
			Count    int64
		}

		// Pod filter: same as insights/summary – only count Pod insights when pod still exists
		var baseQuery *gorm.DB
		if byType == "all" {
			baseQuery = db.Model(&models.Insight{}).
				Where("detected_at >= ? AND deleted_at IS NULL", start).
				Where("(resource_type != 'Pod' OR resource_uid IN (SELECT uid FROM pods WHERE deleted_at IS NULL))")
		} else {
			// Default "vulnerability" mode: CVE findings plus supply-chain malware (same operational slice as Risk Findings / SBOM threats).
			baseQuery = db.Model(&models.Insight{}).
				Where("insight_type IN ? AND detected_at >= ? AND deleted_at IS NULL", []string{"vulnerability", "supply_chain_malware"}, start).
				Where("(resource_type != 'Pod' OR resource_uid IN (SELECT uid FROM pods WHERE deleted_at IS NULL))")
		}
		if clusterID != "" {
			baseQuery = baseQuery.Where("resource_uid IN (SELECT uid FROM pods WHERE cluster_id = ? AND deleted_at IS NULL)", clusterID)
		}
		baseQuery.
			Select("date_trunc('day', detected_at) as date, LOWER(severity) as severity, COUNT(*) as count").
			Group("date_trunc('day', detected_at), LOWER(severity)").
			Order("date_trunc('day', detected_at)").
			Scan(&rows)

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
			switch row.Severity {
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
	Severity          string `form:"severity"`
	Status            string `form:"status"`
	Search            string `form:"search"`
	Type              string `form:"type"`
	ClusterID         string `form:"clusterId"`
	ResourceNamespace string `form:"resourceNamespace"` // namespace filter (Phase 1)
	SinceMinutes      int    `form:"sinceMinutes"`      // when > 0: only insights with detected_at >= now - sinceMinutes
	WithScores        int    `form:"withScores"`       // when != 0: include totalScore, priorityLevel from risk_scores (Phase 3.1)
	PriorityLevel     string `form:"priorityLevel"`      // when set (e.g. P0, P1): only insights whose resource has this risk_scores.priority_level
	ScoreBin          int    `form:"scoreBin"`          // when 0,10,...,90: filter to findings whose resource score is in [scoreBin, scoreBin+10) (histogram click)
}

// InsightWithScore extends Insight with optional risk score fields (from risk_scores join).
type InsightWithScore struct {
	models.Insight
	TotalScore          *float64 `json:"totalScore,omitempty"`
	PriorityLevel       string   `json:"priorityLevel,omitempty"`
	ExploitabilityScore *float64 `json:"exploitabilityScore,omitempty"`
	BusinessImpactScore *float64 `json:"businessImpactScore,omitempty"`
	TimeDecay           *float64 `json:"timeDecay,omitempty"`
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
	statusFilter := strings.TrimSpace(filter.Status)
	if statusFilter == "" {
		statusFilter = "active"
	}
	query := db.Model(&models.Insight{})
	if strings.TrimSpace(filter.ClusterID) != "" {
		clusterID := NormalizeClusterID(db, strings.TrimSpace(filter.ClusterID))
		query = query.Where(
			"resource_type = ? AND resource_uid IN (SELECT uid FROM pods WHERE cluster_id = ? AND deleted_at IS NULL)",
			"Pod", clusterID,
		)
	} else {
		query = query.Where("(resource_type != 'Pod' OR resource_uid IN (SELECT uid FROM pods WHERE deleted_at IS NULL))")
	}
	if filter.Severity != "" {
		query = query.Where("LOWER(severity) = ?", filter.Severity)
	}
	if statusFilter != "" && statusFilter != "all" {
		query = query.Where("status = ?", statusFilter)
	}
	if filter.Type != "" {
		query = query.Where("insight_type = ?", filter.Type)
	}
	if strings.TrimSpace(filter.ResourceNamespace) != "" {
		query = query.Where("resource_namespace = ?", strings.TrimSpace(filter.ResourceNamespace))
	}
	if filter.Search != "" {
		search := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where(
			"LOWER(title) LIKE ? OR LOWER(description) LIKE ? OR LOWER(resource_name) LIKE ? OR LOWER(cve_id) LIKE ? OR LOWER(affected_component) LIKE ?",
			search, search, search, search, search,
		)
	}
	if filter.SinceMinutes > 0 {
		since := time.Now().Add(-time.Duration(filter.SinceMinutes) * time.Minute)
		query = query.Where("detected_at >= ?", since)
	}
	if strings.TrimSpace(filter.PriorityLevel) != "" {
		query = query.Where("resource_uid IN (SELECT resource_uid FROM risk_scores WHERE priority_level = ? AND deleted_at IS NULL)", strings.TrimSpace(filter.PriorityLevel))
	}
	var total int64
	query.Count(&total)
	offset := (page - 1) * pageSize
	var insights []models.Insight
	query.Order("detected_at DESC").Offset(offset).Limit(pageSize).Find(&insights)

	if filter.WithScores == 0 || len(insights) == 0 {
		listLen = len(insights)
		return gin.H{
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
		scoreByUID[scores[i].ResourceUID] = &scores[i]
	}
	withScores := make([]InsightWithScore, len(insights))
	for i := range insights {
		withScores[i] = InsightWithScore{Insight: insights[i]}
		if s := scoreByUID[insights[i].ResourceUID]; s != nil {
			withScores[i].TotalScore = &s.TotalScore
			withScores[i].PriorityLevel = s.PriorityLevel
			withScores[i].ExploitabilityScore = &s.ExploitabilityScore
			withScores[i].BusinessImpactScore = &s.BusinessImpactScore
			withScores[i].TimeDecay = &s.TimeDecay
		}
	}
	listLen = len(insights)
	return gin.H{
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
		_ = c.ShouldBindQuery(&filter)
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
		}
		key := BuildRisksListCacheKey(clusterID, statusFilter, filter.Severity, filter.Search, strings.TrimSpace(filter.PriorityLevel), strings.TrimSpace(filter.ResourceNamespace), strings.TrimSpace(filter.Type), filter.SinceMinutes, page, pageSize, filter.WithScores, filter.ScoreBin)
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
		sinceMinutes, _ := strconv.Atoi(c.DefaultQuery("sinceMinutes", "30"))
		if sinceMinutes <= 0 {
			sinceMinutes = 30
		}
		key := BuildRiskHistogramCacheKey(clusterID, sinceMinutes)
		if defaultRisksCache != nil {
			if b, ok := defaultRisksCache.Get(key); ok {
				c.Data(http.StatusOK, "application/json", b)
				return
			}
		}

		// Base filter: insights joined to risk_scores, active only, optional cluster + since
		joinWhere := "i.deleted_at IS NULL AND (i.status = 'active' OR i.status IS NULL) AND rs.deleted_at IS NULL"
		args := []interface{}{}
		if clusterID != "" {
			joinWhere += " AND i.resource_type = 'Pod' AND i.resource_uid IN (SELECT uid FROM pods WHERE cluster_id = ? AND deleted_at IS NULL)"
			args = append(args, clusterID)
		} else {
			joinWhere += " AND (i.resource_type != 'Pod' OR i.resource_uid IN (SELECT uid FROM pods WHERE deleted_at IS NULL))"
		}
		if sinceMinutes > 0 {
			joinWhere += " AND i.detected_at >= ?"
			args = append(args, time.Now().Add(-time.Duration(sinceMinutes)*time.Minute))
		}

		// Bins: FLOOR(rs.total_score/10)*10, count and severity breakdown
		binQuery := `SELECT (FLOOR(rs.total_score / 10) * 10)::int AS bin,
  COUNT(*)::int AS count,
  COALESCE(SUM(CASE WHEN LOWER(i.severity) = 'critical' THEN 1 ELSE 0 END), 0)::int AS critical_count,
  COALESCE(SUM(CASE WHEN LOWER(i.severity) = 'high' THEN 1 ELSE 0 END), 0)::int AS high_count,
  COALESCE(SUM(CASE WHEN LOWER(i.severity) = 'medium' THEN 1 ELSE 0 END), 0)::int AS medium_count,
  COALESCE(SUM(CASE WHEN LOWER(i.severity) = 'low' THEN 1 ELSE 0 END), 0)::int AS low_count
FROM insights i
INNER JOIN risk_scores rs ON rs.resource_uid = i.resource_uid
WHERE ` + joinWhere + `
GROUP BY FLOOR(rs.total_score / 10) * 10
ORDER BY bin`
		var bins []riskHistogramBin
		if err := db.Raw(binQuery, args...).Scan(&bins).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Totals and average score, P0 count
		totQuery := `SELECT COUNT(*)::int AS total_findings,
  COALESCE(AVG(rs.total_score), 0)::float AS average_score,
  COALESCE(SUM(CASE WHEN rs.priority_level = 'P0' THEN 1 ELSE 0 END), 0)::int AS p0_count
FROM insights i
INNER JOIN risk_scores rs ON rs.resource_uid = i.resource_uid
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
				"bin":                b,
				"count":              row.Count,
				"percent":            roundPercent(percent),
				"severityBreakdown":  gin.H{"critical": row.CriticalCount, "high": row.HighCount, "medium": row.MediumCount, "low": row.LowCount},
				"critical_count":     row.CriticalCount,
				"high_count":         row.HighCount,
				"medium_count":       row.MediumCount,
				"low_count":          row.LowCount,
			})
		}

		resp := gin.H{
			"bins":           outBins,
			"totalFindings":  totalFindings,
			"averageScore":   roundPercent(averageScore),
			"p0Count":        p0Count,
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

// writeRisksExportHTML writes print-optimized HTML table for "Print to PDF" (Phase 3.3).
func writeRisksExportHTML(w http.ResponseWriter, insights []models.Insight) {
	w.Write([]byte(`<!DOCTYPE html><html><head><meta charset="utf-8"/><title>Risks Export</title>`))
	w.Write([]byte(`<style>body{font-family:sans-serif;margin:1rem;} table{border-collapse:collapse;width:100%;} th,td{border:1px solid #333;padding:6px;text-align:left;} th{background:#444;color:#fff;} @media print{body{margin:0;}}</style></head><body>`))
	w.Write([]byte(`<h1>Risks Export</h1><p>Generated at ` + time.Now().Format(time.RFC3339) + ` — ` + strconv.Itoa(len(insights)) + ` findings. Use browser Print → Save as PDF.</p><table><thead><tr>`))
	headers := []string{"ID", "Title", "Severity", "Status", "Type", "Resource", "Namespace", "Finding ref", "Detected At"}
	for _, h := range headers {
		w.Write([]byte("<th>" + html.EscapeString(h) + "</th>"))
	}
	w.Write([]byte("</tr></thead><tbody>"))
	for _, i := range insights {
		w.Write([]byte("<tr><td>" + strconv.FormatUint(uint64(i.ID), 10) + "</td><td>" + html.EscapeString(i.Title) + "</td><td>" + html.EscapeString(i.Severity) + "</td><td>" + html.EscapeString(i.Status) + "</td><td>" + html.EscapeString(i.InsightType) + "</td><td>" + html.EscapeString(i.ResourceName) + "</td><td>" + html.EscapeString(i.ResourceNamespace) + "</td><td>" + html.EscapeString(i.CVEID) + "</td><td>" + i.DetectedAt.Format(time.RFC3339) + "</td></tr>"))
	}
	w.Write([]byte("</tbody></table></body></html>"))
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// ExportRisksCSV returns risks (insights) as CSV or PDF (print-optimized HTML) with same filters as GetInsightsList.
// Query params: clusterId, severity, status, search, type, sinceMinutes, format=csv|pdf (default csv).
func ExportRisksCSV(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var filter RiskFilter
		if err := c.ShouldBindQuery(&filter); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		format := strings.ToLower(strings.TrimSpace(c.Query("format")))
		hasScoreBin := strings.TrimSpace(c.Query("scoreBin")) != ""
		statusFilter := strings.TrimSpace(filter.Status)
		if statusFilter == "" {
			statusFilter = "active"
		}

		query := db.Model(&models.Insight{})
		if strings.TrimSpace(filter.ClusterID) != "" {
			clusterID := NormalizeClusterID(db, strings.TrimSpace(filter.ClusterID))
			query = query.Where(
				"resource_type = ? AND resource_uid IN (SELECT uid FROM pods WHERE cluster_id = ? AND deleted_at IS NULL)",
				"Pod", clusterID,
			)
		} else {
			query = query.Where("(resource_type != 'Pod' OR resource_uid IN (SELECT uid FROM pods WHERE deleted_at IS NULL))")
		}
		if filter.Severity != "" {
			query = query.Where("LOWER(severity) = ?", filter.Severity)
		}
		if statusFilter != "" && statusFilter != "all" {
			query = query.Where("status = ?", statusFilter)
		}
		if filter.Type != "" {
			query = query.Where("insight_type = ?", filter.Type)
		}
		if filter.Search != "" {
			search := "%" + strings.ToLower(filter.Search) + "%"
			query = query.Where(
				"LOWER(title) LIKE ? OR LOWER(description) LIKE ? OR LOWER(resource_name) LIKE ? OR LOWER(cve_id) LIKE ? OR LOWER(affected_component) LIKE ?",
				search, search, search, search, search,
			)
		}
	if filter.SinceMinutes > 0 {
		since := time.Now().Add(-time.Duration(filter.SinceMinutes) * time.Minute)
		query = query.Where("detected_at >= ?", since)
	}
	if strings.TrimSpace(filter.ResourceNamespace) != "" {
		query = query.Where("resource_namespace = ?", strings.TrimSpace(filter.ResourceNamespace))
	}
	// Histogram bin filter: score in [scoreBin, scoreBin+10) (e.g. scoreBin=10 → 10–19)
	// Only apply when scoreBin query param is explicitly provided.
	if hasScoreBin && filter.ScoreBin >= 0 && filter.ScoreBin <= 90 && (filter.ScoreBin%10) == 0 {
		query = query.Where("resource_uid IN (SELECT resource_uid FROM risk_scores WHERE total_score >= ? AND total_score < ? AND deleted_at IS NULL)",
			filter.ScoreBin, filter.ScoreBin+10)
	}

		if format == "pdf" {
			// PDF: load up to limit for HTML (single response)
			var insights []models.Insight
			query.Order("detected_at DESC").Limit(maxRisksExportLimit).Find(&insights)
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.Header("Content-Disposition", `attachment; filename="risks-export.html"`)
			writeRisksExportHTML(c.Writer, insights)
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
			for _, i := range chunk {
				_ = csvW.Write([]string{
					strconv.FormatUint(uint64(i.ID), 10),
					i.Title,
					i.Description,
					i.Severity,
					i.Status,
					i.InsightType,
					i.ResourceType,
					i.ResourceName,
					i.ResourceNamespace,
					i.ResourceUID,
					i.CVEID,
					i.DetectedAt.Format(time.RFC3339),
					i.CreatedAt.Format(time.RFC3339),
					i.UpdatedAt.Format(time.RFC3339),
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

		if payload.Status != "acknowledged" && payload.Status != "resolved" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status must be acknowledged or resolved"})
			return
		}

		if err := db.Model(&models.Insight{}).Where("id = ?", id).Update("status", payload.Status).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": payload.Status})
	}
}

// AttackPathsGraph returns nodes/links from existing graph endpoint (wraps GetGraph).
func AttackPathsGraph(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		GetGraph(db)(c)
	}
}
