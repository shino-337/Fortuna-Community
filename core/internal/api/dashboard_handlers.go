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
	TotalClusters  int64 `json:"totalClusters"`
	ActiveAgents   int64 `json:"activeAgents"`
	RunningPods    int64 `json:"runningPods"`
	TotalRisks     int64 `json:"totalRisks"`
	CriticalRisks  int64 `json:"criticalRisks"`
	Resolved24h   int64 `json:"resolved24h"` // Insights resolved in last 24h
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
func GetDashboardStats(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var clusters int64
		if db.Migrator().HasTable("clusters") {
			// Only count clusters that have synced recently (same cutoff as GetClusters / GetClustersStats)
			cutoff := time.Now().Add(-ActiveClusterCutoff)
			db.Table("clusters").Where("last_sync >= ?", cutoff).Count(&clusters)
		} else {
			db.Table("insights").Distinct("resource_namespace").Count(&clusters)
		}

		var pods int64
		db.Table("pods").Where("deleted_at IS NULL").Count(&pods)

		var agents int64
		if db.Migrator().HasTable("agents") {
			// Only count agents that have been seen recently (within last 10 minutes)
			// This ensures we only count active agents, not stale entries
			tenMinutesAgo := time.Now().Add(-10 * time.Minute)
			db.Table("agents").
				Where("deleted_at IS NULL AND status = ? AND (last_seen_at > ? OR last_seen_at IS NULL)", "ready", tenMinutesAgo).
				Count(&agents)
		} else {
			agents = 0
		}

		var critical int64
		db.Table("insights").
			Where("insight_type = ? AND LOWER(severity) = ? AND deleted_at IS NULL", "vulnerability", "critical").
			Count(&critical)

		var totalRisks int64
		db.Table("insights").
			Where("insight_type = ? AND deleted_at IS NULL", "vulnerability").
			Count(&totalRisks)

		var resolved24h int64
		if db.Migrator().HasTable("insights") {
			twentyFourHoursAgo := time.Now().Add(-24 * time.Hour)
			db.Table("insights").
				Where("deleted_at IS NULL AND status = ? AND updated_at > ?", "resolved", twentyFourHoursAgo).
				Count(&resolved24h)
		}

		c.JSON(http.StatusOK, DashboardStatsDTO{
			TotalClusters:  clusters,
			ActiveAgents:   agents,
			RunningPods:    pods,
			TotalRisks:     totalRisks,
			CriticalRisks:  critical,
			Resolved24h:    resolved24h,
		})
	}
}

// GetThreatVelocity returns daily counts of insights grouped by severity.
func GetThreatVelocity(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		days := 7
		if d := c.Query("days"); d != "" {
			if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 30 {
				days = parsed
			}
		}

		start := time.Now().AddDate(0, 0, -days+1).Truncate(24 * time.Hour)

		var rows []struct {
			Date     time.Time
			Severity string
			Count    int64
		}

		db.Model(&models.Insight{}).
			Select("date_trunc('day', detected_at) as date, LOWER(severity) as severity, COUNT(*) as count").
			Where("insight_type = ? AND detected_at >= ?", "vulnerability", start).
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
	Severity string `form:"severity"`
	Status   string `form:"status"`
	Search   string `form:"search"`
	Type     string `form:"type"`
}

// GetInsightsList is reused for /risks (with filters).
// Default to vulnerability insights so counts match dashboard/stats (totalRisks).
func GetInsightsList(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var filter RiskFilter
		if err := c.ShouldBindQuery(&filter); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if filter.Type == "" {
			filter.Type = "vulnerability"
		}

		query := db.Model(&models.Insight{})

		if filter.Severity != "" {
			query = query.Where("LOWER(severity) = ?", filter.Severity)
		}
		if filter.Status != "" {
			query = query.Where("status = ?", filter.Status)
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
