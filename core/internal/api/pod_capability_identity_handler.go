package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/api/listlimit"
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
		if !requirePodCapabilitiesTable(c, db.WithContext(c.Request.Context())) {
			return
		}

		query := db.Where("cluster_id = ? AND pod_uid = ?", clusterID, podUID)
		if class := strings.TrimSpace(c.Query("class")); class != "" {
			query = query.Where("capability_class = ?", class)
		}
		limit := listlimit.Parse(c, podCapabilitiesDefault, podCapabilitiesMax)
		var caps []models.PodCapability
		if err := query.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&caps).Error; err != nil {
			respondDataUnavailable(c, "capability_inventory_query_failed", "Capability inventory could not be loaded")
			return
		}
		caps, truncated := listlimit.Trim(caps, limit)
		dtos := mapPodCapabilities(caps)
		c.JSON(http.StatusOK, gin.H{
			"podUid":       podUID,
			"clusterId":    clusterID,
			"capabilities": dtos,
			"total":        len(dtos),
			"truncated":    truncated,
		})
	}
}
