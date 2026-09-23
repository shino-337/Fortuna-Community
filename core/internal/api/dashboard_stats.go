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
		db := db.WithContext(c.Request.Context())
		filter, ok := aggregateScope(db, c)
		if !ok {
			return
		}
		since, _ := strconv.Atoi(c.DefaultQuery("sinceMinutes", "0"))
		result := DashboardStatsDTO{DataStatus: "available"}
		fail := func(err error, code string) bool {
			if err != nil {
				respondDataUnavailable(c, code, "Dashboard statistics could not be loaded")
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
			if fail(err, "dashboard_stats_clusters_unavailable") {
				return
			}
			result.ClusterName = cluster.Name
			result.TotalClusters = 1
		} else {
			if fail(clusters().Where("id IN (?)", pods().Select("cluster_id")).Count(&result.TotalClusters).Error, "dashboard_stats_clusters_unavailable") {
				return
			}
		}
		activePods := func() *gorm.DB {
			return pods().Where("cluster_id IN (?)", clusters().Select("id"))
		}
		podIdentities := activePods().
			Select("cluster_id, uid").
			Group("cluster_id, uid")
		if fail(db.Table("(?) AS scoped_pods", podIdentities).Count(&result.RunningPods).Error,
			"dashboard_stats_pods_unavailable") {
			return
		}
		if db.Migrator().HasTable(&models.Agent{}) {
			if fail(db.Model(&models.Agent{}).Where("(status = ? OR status IS NULL) AND cluster_id IN (?)", "ready", clusters().Select("id")).Count(&result.ActiveAgents).Error, "dashboard_stats_agents_unavailable") {
				return
			}
		}
		unresolved := func() *gorm.DB {
			q := db.Model(&models.Insight{}).
				Where("resource_type = ?", "Pod").
				Where("status IN ? OR status IS NULL", []string{"active", "acknowledged"}).
				Where(`EXISTS (
					SELECT 1 FROM pods p
					WHERE p.cluster_id = insights.cluster_id
					  AND p.uid = insights.resource_uid
					  AND p.deleted_at IS NULL
					  AND p.cluster_id IN (?)
				)`, clusters().Select("id"))
			q = scopedAggregateQuery(db, q, filter, "resource_uid")
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
		if fail(riskCounts().Count(&result.TotalRisks).Error, "dashboard_stats_risks_unavailable") {
			return
		}
		if fail(riskCounts().Where("LOWER(severity) = ?", "critical").Count(&result.CriticalRisks).Error, "dashboard_stats_critical_risks_unavailable") {
			return
		}
		affected := unresolved().
			Select("cluster_id, resource_uid").
			Group("cluster_id, resource_uid")
		if fail(db.Table("(?) AS affected_pods", affected).Count(&result.AffectedPodCount).Error,
			"dashboard_stats_affected_pods_unavailable") {
			return
		}
		resolved := db.Model(&models.Insight{}).Where("status = ? AND updated_at > ?", "resolved", time.Now().Add(-24*time.Hour))
		resolved = scopedAggregateQuery(db, resolved, filter, "resource_uid")
		if fail(resolved.Count(&result.Resolved24h).Error, "dashboard_stats_resolved_unavailable") {
			return
		}
		c.JSON(http.StatusOK, result)
	}
}
