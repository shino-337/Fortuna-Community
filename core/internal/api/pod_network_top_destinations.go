package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/networkbucket"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// podNetworkTopDestRow is one aggregated row: remote endpoint seen from this pod (dest tuple), ordered by observation count.
type podNetworkTopDestRow struct {
	DestIP              string    `json:"destIp" gorm:"column:dest_ip"`
	DestPort            int       `json:"destPort" gorm:"column:dest_port"`
	Protocol            string    `json:"protocol" gorm:"column:protocol"`
	ObservationCount    int64     `json:"observationCount" gorm:"column:observation_count"`
	LastObservedAt      time.Time `json:"lastObservedAt" gorm:"column:last_observed_at"`
	DistinctBucketCount int64     `json:"distinctBucketCount" gorm:"column:distinct_bucket_count"`
}

// GetPodNetworkTopDestinationsByUID returns aggregated destinations (dest_ip, dest_port, protocol) for a pod
// over the same time window as GET .../network (bucket_5m >= floor5m(now - sinceMinutes)).
// Each stored row is already one observation per signature per 5m bucket; COUNT(*) is "observation rows" for that triple, not unique sockets.
func GetPodNetworkTopDestinationsByUID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := c.Param("uid")
		if podUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podUid is required"})
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

		sinceWall := time.Now().UTC().Add(-time.Duration(sinceMinutes) * time.Minute)
		sinceBucket := networkbucket.FloorBucket5MUTC(sinceWall)

		var rows []podNetworkTopDestRow
		err := db.Model(&models.PodNetworkConnection{}).
			Select(`dest_ip, dest_port, protocol,
				COUNT(*) AS observation_count,
				MAX(observed_at) AS last_observed_at,
				COUNT(DISTINCT bucket_5m) AS distinct_bucket_count`).
			Where("pod_uid = ? AND bucket_5m >= ?", podUID, sinceBucket).
			Group("dest_ip, dest_port, protocol").
			Order("observation_count DESC").
			Limit(limit).
			Scan(&rows).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"podUid":       podUID,
			"sinceMinutes": sinceMinutes,
			"items":        rows,
		})
	}
}
