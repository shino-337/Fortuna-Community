package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
)

// GetPodByUIDScoped returns a Pod only after RequirePodUIDClusterScope has
// resolved an unambiguous canonical {cluster_id,pod_uid} identity.
func GetPodByUIDScoped(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := strings.TrimSpace(c.Param("uid"))
		if uid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "uid is required"})
			return
		}
		clusterID, ok := middleware.ResolvedPodClusterID(c)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "resolved pod cluster is required"})
			return
		}

		var pod models.Pod
		if err := db.Preload("Cluster").Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&pod).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var riskCount int64
		if hasTable(db, "insights") {
			if err := db.Model(&models.Insight{}).
				Where("cluster_id = ? AND resource_uid = ? AND deleted_at IS NULL AND (status = 'active' OR status IS NULL)", clusterID, uid).
				Count(&riskCount).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to load pod risk summary"})
				return
			}
		}

		c.JSON(http.StatusOK, newPodDetailResponse(c, db, pod, riskCount))
	}
}
