package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
)

// GetPodCapabilitiesScoped returns capability rows for exactly one canonical
// {cluster_id,pod_uid} identity. It intentionally does not fall back to UID-only
// lookup because duplicate Kubernetes UIDs across clusters are valid inputs.
func GetPodCapabilitiesScoped(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := strings.TrimSpace(c.Param("uid"))
		if podUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podUid is required"})
			return
		}
		clusterID, ok := middleware.ResolvedPodClusterID(c)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "resolved pod cluster is required"})
			return
		}
		if !hasPodCapabilitiesTable(db) {
			c.JSON(http.StatusOK, gin.H{"podUid": podUID, "clusterId": clusterID, "capabilities": []PodCapabilityDTO{}, "total": 0})
			return
		}

		query := db.Where("cluster_id = ? AND pod_uid = ?", clusterID, podUID)
		if class := strings.TrimSpace(c.Query("class")); class != "" {
			query = query.Where("capability_class = ?", class)
		}
		var caps []models.PodCapability
		if err := query.Order("created_at DESC").Find(&caps).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		dtos := mapPodCapabilities(caps)
		c.JSON(http.StatusOK, gin.H{
			"podUid":       podUID,
			"clusterId":    clusterID,
			"capabilities": dtos,
			"total":        len(dtos),
		})
	}
}
