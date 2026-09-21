package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
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

		clusterID, ok := middleware.ResolvedPodClusterID(c)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "resolved pod cluster is required"})
			return
		}
		var state models.AssetSecurityState
		if err := db.Where("cluster_id = ? AND pod_uid = ?", clusterID, podUID).First(&state).Error; err != nil {
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
// GetPodRuntimeBehaviorFacts is retained only for source compatibility.
// Production routes must use GetPodRuntimeBehaviorFactsScoped and CI rejects legacy registration.
func GetPodRuntimeBehaviorFacts(db *gorm.DB) gin.HandlerFunc { return GetPodRuntimeBehaviorFactsScoped(db) }


// GetPodRuntimeIncidents returns Layer-3 incidents for a pod.
// GetPodRuntimeIncidents is retained only for source compatibility.
// Production routes must use GetPodRuntimeIncidentsScoped and CI rejects legacy registration.
func GetPodRuntimeIncidents(db *gorm.DB) gin.HandlerFunc { return GetPodRuntimeIncidentsScoped(db) }

