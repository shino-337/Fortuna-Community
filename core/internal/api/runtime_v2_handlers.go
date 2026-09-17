package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// GetPodAssetSecurityState returns unified runtime security snapshot (Layer 4 minimal).
func GetPodAssetSecurityState(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := c.Param("uid")
		if podUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podUid is required"})
			return
		}
		if !db.Migrator().HasTable(&models.AssetSecurityState{}) {
			c.JSON(http.StatusNotFound, gin.H{"error": "asset_security_state not available"})
			return
		}

		var state models.AssetSecurityState
		if err := db.Where("pod_uid = ?", podUID).First(&state).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "asset security state not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, state)
	}
}

// GetPodRuntimeBehaviorFacts returns Layer-2 behavior facts for a pod.
func GetPodRuntimeBehaviorFacts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := c.Param("uid")
		if podUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podUid is required"})
			return
		}
		if !db.Migrator().HasTable(&models.RuntimeBehaviorFact{}) {
			c.JSON(http.StatusOK, gin.H{"podUid": podUID, "facts": []models.RuntimeBehaviorFact{}, "total": 0})
			return
		}

		limit := 100
		if s := c.Query("limit"); s != "" {
			if v, err := strconv.Atoi(s); err == nil && v > 0 && v <= 1000 {
				limit = v
			}
		}

		var facts []models.RuntimeBehaviorFact
		if err := db.Where("pod_uid = ?", podUID).
			Order("observed_at DESC").
			Limit(limit).
			Find(&facts).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"podUid": podUID, "facts": facts, "total": len(facts)})
	}
}

// GetPodRuntimeIncidents returns Layer-3 incidents for a pod.
func GetPodRuntimeIncidents(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := c.Param("uid")
		if podUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podUid is required"})
			return
		}
		if !db.Migrator().HasTable(&models.RuntimeIncident{}) {
			c.JSON(http.StatusOK, gin.H{"podUid": podUID, "incidents": []models.RuntimeIncident{}, "total": 0})
			return
		}

		limit := 100
		if s := c.Query("limit"); s != "" {
			if v, err := strconv.Atoi(s); err == nil && v > 0 && v <= 1000 {
				limit = v
			}
		}

		var incidents []models.RuntimeIncident
		if err := db.Where("pod_uid = ?", podUID).
			Order("last_seen_at DESC").
			Limit(limit).
			Find(&incidents).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"podUid": podUID, "incidents": incidents, "total": len(incidents)})
	}
}
