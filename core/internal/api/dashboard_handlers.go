package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

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
// Clusters not synced in 7 days are excluded so dashboard reflects current environment.
// Query param clusterId: when set, all counts are scoped to that cluster (sync with global cluster selector).
// Query param sinceMinutes: when > 0, insight counts (totalRisks, criticalRisks, affectedPodCount) are limited to detected_at >= now - sinceMinutes.
func GetDashboardStats(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID := strings.TrimSpace(c.Query("clusterId"))
		if clusterID != "" {
			clusterID = NormalizeClusterID(db, clusterID)
		}
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

			// Risks: insights for Pods in this cluster (join on resource_uid = pods.uid); optional time window
			criticalArgs := []interface{}{clusterID, "vulnerability", "critical"}
			if sinceMinutes > 0 {
				criticalArgs = append(criticalArgs, since)
			}
			db.Raw(`
				SELECT COUNT(*) FROM insights i
				INNER JOIN pods p ON p.uid = i.resource_uid AND p.cluster_id = ? AND p.deleted_at IS NULL
				WHERE i.insight_type = ? AND LOWER(i.severity) = ? AND i.deleted_at IS NULL`+detectedSinceClause,
				criticalArgs...).Scan(&critical)
			totalArgs := []interface{}{clusterID, "vulnerability"}
			if sinceMinutes > 0 {
				totalArgs = append(totalArgs, since)
			}
			db.Raw(`
				SELECT COUNT(*) FROM insights i
				INNER JOIN pods p ON p.uid = i.resource_uid AND p.cluster_id = ? AND p.deleted_at IS NULL
				WHERE i.insight_type = ? AND i.deleted_at IS NULL`+detectedSinceClause,
				totalArgs...).Scan(&totalRisks)

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
			if sinceMinutes > 0 {
				db.Table("insights").
					Where("insight_type = ? AND LOWER(severity) = ? AND deleted_at IS NULL AND detected_at >= ?", "vulnerability", "critical", since).
					Count(&critical)
				db.Table("insights").
					Where("insight_type = ? AND deleted_at IS NULL AND detected_at >= ?", "vulnerability", since).
					Count(&totalRisks)
			} else {
				db.Table("insights").
					Where("insight_type = ? AND LOWER(severity) = ? AND deleted_at IS NULL", "vulnerability", "critical").
					Count(&critical)
				db.Table("insights").
					Where("insight_type = ? AND deleted_at IS NULL", "vulnerability").
					Count(&totalRisks)
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
// Query param days: 1–30 (default 7). Query param clusterId: optional; when set, only insights for pods in that cluster (same scope as insights/summary).
// Pod filter: only count Pod insights when pod exists (deleted_at IS NULL) so trend matches Total findings.
func GetThreatVelocity(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		days := 7
		if d := c.Query("days"); d != "" {
			if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 30 {
				days = parsed
			}
		}
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
		baseQuery := db.Model(&models.Insight{}).
			Where("insight_type = ? AND detected_at >= ? AND deleted_at IS NULL", "vulnerability", start).
			Where("(resource_type != 'Pod' OR resource_uid IN (SELECT uid FROM pods WHERE deleted_at IS NULL))")
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
	Severity     string `form:"severity"`
	Status       string `form:"status"`
	Search       string `form:"search"`
	Type         string `form:"type"`
	ClusterID    string `form:"clusterId"`
	SinceMinutes int    `form:"sinceMinutes"` // when > 0: only insights with detected_at >= now - sinceMinutes
}

// GetInsightsList is reused for /risks (with filters).
// Default to all insight types and active status so capability/runtime risks are visible by default.
// Query param clusterId: when set, only insights for Pods in that cluster are returned (sync with global cluster selector).
func GetInsightsList(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var filter RiskFilter
		if err := c.ShouldBindQuery(&filter); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// Default to active so list total matches dashboard Security Risks and insights/summary
		statusFilter := strings.TrimSpace(filter.Status)
		if statusFilter == "" {
			statusFilter = "active" // default so list matches dashboard/summary
		}

		query := db.Model(&models.Insight{})

		if strings.TrimSpace(filter.ClusterID) != "" {
			clusterID := NormalizeClusterID(db, strings.TrimSpace(filter.ClusterID))
			query = query.Where(
				"resource_type = ? AND resource_uid IN (SELECT uid FROM pods WHERE cluster_id = ? AND deleted_at IS NULL)",
				"Pod", clusterID,
			)
		} else {
			// No cluster: only show Pod insights whose pod still exists (so Total findings matches list)
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
				"LOWER(title) LIKE ? OR LOWER(description) LIKE ? OR LOWER(resource_name) LIKE ?",
				search, search, search,
			)
		}
		if filter.SinceMinutes > 0 {
			since := time.Now().Add(-time.Duration(filter.SinceMinutes) * time.Minute)
			query = query.Where("detected_at >= ?", since)
		}

		var total int64
		query.Count(&total)

		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
		if page < 1 {
			page = 1
		}
		if pageSize < 1 || pageSize > 100 {
			pageSize = 20
		}
		offset := (page - 1) * pageSize

		var insights []models.Insight
		query.Order("detected_at DESC").Offset(offset).Limit(pageSize).Find(&insights)

		c.JSON(http.StatusOK, gin.H{
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
			"insights": insights,
		})
	}
}

// UpdateInsightStatus updates status (acknowledged/resolved).
func UpdateInsightStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("riskId")
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
