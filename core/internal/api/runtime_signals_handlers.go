package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
)

// scopeRuntimeQuery restricts both rows and counts to authorized pod identities.
// Unscoped pod lookup preserves ownership for retained evidence of deleted pods.
func scopeRuntimeQuery(db *gorm.DB, c *gin.Context, query *gorm.DB) (*gorm.DB, bool) {
	clusterID := strings.TrimSpace(c.Query("clusterId"))
	if clusterID != "" && !middleware.ClusterAllowed(c, clusterID) {
		middleware.AbortClusterScopeDenied(db, c, clusterID)
		return query, false
	}
	if uid := strings.TrimSpace(c.Query("podUid")); uid != "" {
		if !requireResourceUIDClusterScope(db, c, uid) {
			return query, false
		}
	}
	pods := db.Unscoped().Model(&models.Pod{}).Select("uid")
	restricted := false
	if ids, scoped := middleware.ScopedClusterIDs(c); scoped {
		pods = pods.Where("cluster_id IN ?", ids)
		restricted = true
	}
	if clusterID != "" {
		pods = pods.Where("cluster_id = ?", clusterID)
		restricted = true
	}
	if restricted {
		query = query.Where("pod_uid IN (?)", pods)
	}
	return query, true
}

// GetRuntimeSignalsList returns runtime signals with optional filters (active pods only when no podUid filter).
func GetRuntimeSignalsList(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		query, allowed := scopeRuntimeQuery(db, c, db.Model(&models.RuntimeSignal{}))
		if !allowed {
			return
		}

		// Filter by pod_uid
		if podUID := c.Query("podUid"); podUID != "" {
			query = query.Where("pod_uid = ?", podUID)
		} else {
			// Only show signals for pods that still exist (not soft-deleted)
			query = query.Where("pod_uid IN (SELECT uid FROM pods WHERE deleted_at IS NULL)")
		}

		// Filter by signal_type
		if signalType := c.Query("signalType"); signalType != "" {
			query = query.Where("signal_type = ?", signalType)
		}

		// Filter by category
		if category := c.Query("category"); category != "" {
			query = query.Where("category = ?", category)
		}

		if search := strings.TrimSpace(c.Query("search")); search != "" {
			pattern := "%" + strings.ToLower(search) + "%"
			query = query.Where("LOWER(signal_type) LIKE ? OR LOWER(category) LIKE ? OR LOWER(pod_uid) LIKE ?", pattern, pattern, pattern)
		}

		// Explicit dates take precedence over the relative time window.
		if sinceStr := c.Query("sinceMinutes"); sinceStr != "" && c.Query("startDate") == "" && c.Query("endDate") == "" {
			if sinceMin, err := strconv.Atoi(sinceStr); err == nil && sinceMin > 0 && sinceMin <= 43200 {
				since := time.Now().Add(-time.Duration(sinceMin) * time.Minute)
				query = query.Where("created_at >= ?", since)
			}
		} else {
			// Filter by date range (when sinceMinutes not set)
			if startDate := c.Query("startDate"); startDate != "" {
				if t, err := time.Parse("2006-01-02", startDate); err == nil {
					query = query.Where("created_at >= ?", t)
				}
			}
			if endDate := c.Query("endDate"); endDate != "" {
				if t, err := time.Parse("2006-01-02", endDate); err == nil {
					t = t.Add(24 * time.Hour)
					query = query.Where("created_at < ?", t)
				}
			}
		}

		// Pagination
		limit := 100 // default
		if limitStr := c.Query("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
				limit = l
			}
		}
		offset := 0
		if offsetStr := c.Query("offset"); offsetStr != "" {
			if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
				offset = o
			}
		}

		var signals []models.RuntimeSignal
		var total int64

		// Count total
		if err := query.Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		orders := map[string]string{"newest": "created_at DESC", "oldest": "created_at ASC", "confidence_desc": "confidence DESC", "confidence_asc": "confidence ASC", "signal_asc": "signal_type ASC"}
		order := orders[c.DefaultQuery("sort", "newest")]
		if order == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sort"})
			return
		}
		// Apply ordering before pagination, with a stable tie-breaker.
		if err := query.Order(order).Order("id ASC").
			Limit(limit).
			Offset(offset).
			Find(&signals).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"signals": signals,
			"count":   len(signals),
			"total":   total,
			"limit":   limit,
			"offset":  offset,
		})
	}
}

// GetRuntimeSignalSuppressionStats returns quick observability stats for NETWORK_TXRX_QUEUE_SPIKE events.
// Note: suppressed events are not persisted; this endpoint reports emitted events and key distribution.
func GetRuntimeSignalSuppressionStats(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sinceMin := 60
		if s := c.Query("sinceMinutes"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 43200 {
				sinceMin = n
			}
		}
		from := time.Now().Add(-time.Duration(sinceMin) * time.Minute)
		query := db.Model(&models.RuntimeEvent{}).
			Where("capability = ? AND created_at >= ?", "NETWORK_TXRX_QUEUE_SPIKE", from)
		var allowed bool
		query, allowed = scopeRuntimeQuery(db, c, query)
		if !allowed {
			return
		}
		if podUID := c.Query("podUid"); podUID != "" {
			query = query.Where("pod_uid = ?", podUID)
		}
		type row struct {
			PodUID     string
			TargetPath string
		}
		var rows []row
		if err := query.Select("pod_uid, target_path").Order("created_at DESC").Limit(2000).Find(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		perKey := map[string]int{}
		maxRatio := 0.0
		for i := range rows {
			key := parseTargetToken(rows[i].TargetPath, "key")
			if key == "" {
				key = "unknown"
			}
			perKey[key]++
			r := parseTargetTokenFloat(rows[i].TargetPath, "ratio")
			if r > maxRatio {
				maxRatio = r
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"sinceMinutes":  sinceMin,
			"emittedEvents": len(rows),
			"uniqueKeys":    len(perKey),
			"maxRatio":      maxRatio,
			"perKey":        perKey,
		})
	}
}

func parseTargetToken(target, key string) string {
	if target == "" || key == "" {
		return ""
	}
	prefix := key + "="
	for _, t := range strings.Fields(target) {
		if strings.HasPrefix(t, prefix) {
			return strings.TrimPrefix(t, prefix)
		}
	}
	return ""
}

func parseTargetTokenFloat(target, key string) float64 {
	v := parseTargetToken(target, key)
	if v == "" {
		return 0
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	return n
}

// GetRuntimeSignalsByPod returns runtime signals for a specific pod
func GetRuntimeSignalsByPod(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := c.Param("uid")
		if podUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "pod_uid is required"})
			return
		}

		if !requireResourceUIDClusterScope(db, c, podUID) {
			return
		}

		// Optional filters
		query := db.Where("pod_uid = ?", podUID)

		if sinceStr := c.Query("sinceMinutes"); sinceStr != "" {
			if sinceMin, err := strconv.Atoi(sinceStr); err == nil && sinceMin > 0 && sinceMin <= 43200 {
				since := time.Now().Add(-time.Duration(sinceMin) * time.Minute)
				query = query.Where("created_at >= ?", since)
			}
		}
		if signalType := c.Query("signalType"); signalType != "" {
			query = query.Where("signal_type = ?", signalType)
		}
		if category := c.Query("category"); category != "" {
			query = query.Where("category = ?", category)
		}

		// Limit
		limit := 100
		if limitStr := c.Query("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
				limit = l
			}
		}

		var signals []models.RuntimeSignal
		if err := query.Order("created_at DESC").
			Limit(limit).
			Find(&signals).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"podUid":  podUID,
			"signals": signals,
			"count":   len(signals),
		})
	}
}
