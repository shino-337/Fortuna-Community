package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// GetRuntimeSignalsList returns runtime signals with optional filters (active pods only when no podUid filter).
func GetRuntimeSignalsList(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := db.Model(&models.RuntimeSignal{})

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

		// Filter by last N minutes (takes precedence over date range for recent data)
		if sinceStr := c.Query("sinceMinutes"); sinceStr != "" {
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

		// Fetch signals
		if err := query.Order("created_at DESC").
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

// GetRuntimeSignalsByPod returns runtime signals for a specific pod
func GetRuntimeSignalsByPod(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := c.Param("uid")
		if podUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "pod_uid is required"})
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
