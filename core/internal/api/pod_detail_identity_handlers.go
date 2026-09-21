package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/networkbucket"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func resolvedPodDetailIdentity(c *gin.Context) (string, string, bool) {
	uid := strings.TrimSpace(c.Param("uid"))
	if uid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "podUid is required"})
		return "", "", false
	}
	clusterID, ok := middleware.ResolvedPodClusterID(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "resolved pod cluster is required"})
		return "", "", false
	}
	return clusterID, uid, true
}

func GetPodRuntimeMetricsByUIDScoped(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID, podUID, ok := resolvedPodDetailIdentity(c)
		if !ok {
			return
		}
		var list []models.PodRuntimeMetrics
		if err := db.Where("cluster_id = ? AND pod_uid = ?", clusterID, podUID).
			Order("last_observed_at DESC").Limit(500).Find(&list).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"clusterId": clusterID, "podUid": podUID, "items": list})
	}
}

func GetPodProcessesByUIDScoped(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID, podUID, ok := resolvedPodDetailIdentity(c)
		if !ok {
			return
		}
		var list []models.PodProcess
		if err := db.Where("cluster_id = ? AND pod_uid = ?", clusterID, podUID).
			Order("observed_at DESC").Limit(1000).Find(&list).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		decryptProcessList(list)
		viewerJSON(c, http.StatusOK, gin.H{"clusterId": clusterID, "podUid": podUID, "items": list})
	}
}

func GetPodNetworkConnectionsByUIDScoped(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID, podUID, ok := resolvedPodDetailIdentity(c)
		if !ok {
			return
		}
		sinceMinutes := 1440
		if v := strings.TrimSpace(c.Query("sinceMinutes")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				sinceMinutes = n
			}
		}
		limit := 500
		if v := strings.TrimSpace(c.Query("limit")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
				if limit > 2000 {
					limit = 2000
				}
			}
		}
		sinceBucket := networkbucket.FloorBucket5MUTC(time.Now().UTC().Add(-time.Duration(sinceMinutes) * time.Minute))
		var list []models.PodNetworkConnection
		if err := db.Where("cluster_id = ? AND pod_uid = ? AND bucket_5m >= ?", clusterID, podUID, sinceBucket).
			Order("bucket_5m DESC, observed_at DESC").Limit(limit).Find(&list).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		viewerJSON(c, http.StatusOK, gin.H{"clusterId": clusterID, "podUid": podUID, "items": list})
	}
}

func GetPodEventsByUIDScoped(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID, podUID, ok := resolvedPodDetailIdentity(c)
		if !ok {
			return
		}
		var list []models.K8sEvent
		if err := db.Where("cluster_id = ? AND involved_uid = ?", clusterID, podUID).
			Order("last_timestamp DESC NULLS LAST").Limit(200).Find(&list).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		viewerJSON(c, http.StatusOK, gin.H{"clusterId": clusterID, "podUid": podUID, "items": list})
	}
}

func GetPodNetworkTopDestinationsByUIDScoped(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID, podUID, ok := resolvedPodDetailIdentity(c)
		if !ok {
			return
		}
		sinceMinutes := 1440
		if v := strings.TrimSpace(c.Query("sinceMinutes")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				sinceMinutes = n
			}
		}
		limit := 15
		if v := strings.TrimSpace(c.Query("limit")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
				if limit > 50 {
					limit = 50
				}
			}
		}
		sinceBucket := networkbucket.FloorBucket5MUTC(time.Now().UTC().Add(-time.Duration(sinceMinutes) * time.Minute))
		var rows []podNetworkTopDestRow
		err := db.Model(&models.PodNetworkConnection{}).
			Select(`dest_ip, dest_port, protocol,
				COUNT(*) AS observation_count,
				MAX(observed_at) AS last_observed_at,
				COUNT(DISTINCT bucket_5m) AS distinct_bucket_count`).
			Where("cluster_id = ? AND pod_uid = ? AND bucket_5m >= ?", clusterID, podUID, sinceBucket).
			Group("dest_ip, dest_port, protocol").
			Order("observation_count DESC").Limit(limit).Scan(&rows).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"clusterId": clusterID, "podUid": podUID, "sinceMinutes": sinceMinutes, "items": rows})
	}
}
