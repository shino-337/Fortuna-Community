package api

import (
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// GetDashboardStats applies the same authorized inventory scope to all counts.
func GetDashboardStats(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		filter, ok := aggregateScope(db, c)
		if !ok {
			return
		}
		since, _ := strconv.Atoi(c.DefaultQuery("sinceMinutes", "0"))
		result := DashboardStatsDTO{}
		fail := func(err error) bool {
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load dashboard statistics"})
				return true
			}
			return false
		}
		clusters := func() *gorm.DB {
			q := db.Model(&models.Cluster{})
			if filter.ClusterID != "" {
				return q.Where("id = ?", filter.ClusterID)
			}
			if len(filter.ScopedClusterIDs) > 0 {
				q = q.Where("id IN ?", filter.ScopedClusterIDs)
			}
			return q.Where("source IN ? AND last_sync >= ?", []string{"auto", "env"}, time.Now().Add(-ActiveClusterCutoff))
		}
		pods := func() *gorm.DB {
			q := db.Model(&models.Pod{})
			if filter.ClusterID != "" {
				return q.Where("cluster_id = ?", filter.ClusterID)
			}
			if len(filter.ScopedClusterIDs) > 0 {
				q = q.Where("cluster_id IN ?", filter.ScopedClusterIDs)
			}
			return q
		}
		if filter.ClusterID != "" {
			var cluster models.Cluster
			err := clusters().First(&cluster).Error
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusOK, result)
				return
			}
			if fail(err) {
				return
			}
			result.ClusterName = cluster.Name
			result.TotalClusters = 1
		} else {
			if fail(clusters().Where("id IN (?)", pods().Select("cluster_id")).Count(&result.TotalClusters).Error) {
				return
			}
		}
		if fail(pods().Distinct("uid").Count(&result.RunningPods).Error) {
			return
		}
		activePods := func() *gorm.DB { return pods().Where("cluster_id IN (?)", clusters().Select("id")) }
		if db.Migrator().HasTable(&models.Agent{}) {
			if fail(db.Model(&models.Agent{}).Where("(status = ? OR status IS NULL) AND cluster_id IN (?)", "ready", clusters().Select("id")).Count(&result.ActiveAgents).Error) {
				return
			}
		}
		unresolved := func() *gorm.DB {
			q := db.Model(&models.Insight{}).Where("resource_uid IN (?)", activePods().Select("uid")).Where("status IN ? OR status IS NULL", []string{"active", "acknowledged"})
			if since > 0 {
				q = q.Where("detected_at >= ?", time.Now().Add(-time.Duration(since)*time.Minute))
			}
			return q
		}
		riskCounts := func() *gorm.DB {
			q := unresolved()
			if strings.ToLower(strings.TrimSpace(c.DefaultQuery("byType", "vulnerability"))) != "all" {
				q = q.Where("insight_type IN ?", []string{"vulnerability", "supply_chain_malware"})
			}
			return q
		}
		if fail(riskCounts().Count(&result.TotalRisks).Error) {
			return
		}
		if fail(riskCounts().Where("LOWER(severity) = ?", "critical").Count(&result.CriticalRisks).Error) {
			return
		}
		if fail(unresolved().Where("resource_type = ?", "Pod").Distinct("resource_uid").Count(&result.AffectedPodCount).Error) {
			return
		}
		resolved := db.Model(&models.Insight{}).Where("status = ? AND updated_at > ?", "resolved", time.Now().Add(-24*time.Hour))
		resolved = scopedAggregateQuery(db, resolved, filter, "resource_uid")
		if fail(resolved.Count(&result.Resolved24h).Error) {
			return
		}
		c.JSON(http.StatusOK, result)
	}
}
