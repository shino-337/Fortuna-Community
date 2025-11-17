package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ksam/core/pkg/models"
)

// BulkDisableServiceAccounts disables multiple service accounts
func BulkDisableServiceAccounts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		type BulkRequest struct {
			IDs []uint `json:"ids" binding:"required"`
		}

		var req BulkRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Get user info for audit
		userID, _ := c.Get("userID")
		username, _ := c.Get("username")

		// Update service accounts (soft delete)
		result := db.Where("id IN ?", req.IDs).Delete(&models.ServiceAccount{})
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
			return
		}

		// Log audit for each disabled SA
		for _, id := range req.IDs {
			var sa models.ServiceAccount
			if err := db.Unscoped().First(&sa, id).Error; err == nil {
				auditLog := models.AuditLog{
					ClusterID:  sa.ClusterID,
					UserID:     userID.(uint),
					Action:     "disable",
					Resource:   "serviceaccount",
					ResourceID: strconv.Itoa(int(sa.ID)),
					User:       username.(string),
					IP:         c.ClientIP(),
				}
				db.Create(&auditLog)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "ServiceAccounts disabled successfully",
			"count":   result.RowsAffected,
		})
	}
}

// BulkDeleteServiceAccounts deletes multiple service accounts
func BulkDeleteServiceAccounts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		type BulkRequest struct {
			IDs []uint `json:"ids" binding:"required"`
		}

		var req BulkRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Get user info for audit
		userID, _ := c.Get("userID")
		username, _ := c.Get("username")

		// Get service accounts before deletion for audit
		var sas []models.ServiceAccount
		db.Where("id IN ?", req.IDs).Find(&sas)

		// Hard delete
		result := db.Unscoped().Where("id IN ?", req.IDs).Delete(&models.ServiceAccount{})
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
			return
		}

		// Log audit
		for _, sa := range sas {
			auditLog := models.AuditLog{
				ClusterID:  sa.ClusterID,
				UserID:     userID.(uint),
				Action:     "delete",
				Resource:   "serviceaccount",
				ResourceID: strconv.Itoa(int(sa.ID)),
				User:       username.(string),
				IP:         c.ClientIP(),
			}
			db.Create(&auditLog)
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "ServiceAccounts deleted successfully",
			"count":   result.RowsAffected,
		})
	}
}

// DisableInactiveServiceAccounts disables service accounts that haven't been used
func DisableInactiveServiceAccounts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		days, _ := strconv.Atoi(c.DefaultQuery("days", "90"))
		if days <= 0 {
			days = 90
		}

		// Find service accounts not used by any pods in the last N days
		// This is a simplified version - in production, you'd check pod usage
		var inactiveSAs []models.ServiceAccount
		query := db.Model(&models.ServiceAccount{}).
			Where("updated_at < NOW() - INTERVAL '? days'", days).
			Where("deleted_at IS NULL")

		// Optionally filter by cluster
		if clusterID := c.Query("cluster"); clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}

		if err := query.Find(&inactiveSAs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if len(inactiveSAs) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"message": "No inactive ServiceAccounts found",
				"count":   0,
			})
			return
		}

		// Get user info for audit
		userID, _ := c.Get("userID")
		username, _ := c.Get("username")

		// Disable inactive SAs
		ids := make([]uint, len(inactiveSAs))
		for i, sa := range inactiveSAs {
			ids[i] = sa.ID
		}

		result := db.Where("id IN ?", ids).Delete(&models.ServiceAccount{})
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
			return
		}

		// Log audit
		for _, sa := range inactiveSAs {
			auditLog := models.AuditLog{
				ClusterID:  sa.ClusterID,
				UserID:     userID.(uint),
				Action:     "disable",
				Resource:   "serviceaccount",
				ResourceID: strconv.Itoa(int(sa.ID)),
				User:       username.(string),
				IP:         c.ClientIP(),
				Details:    `{"reason":"inactive","days":` + strconv.Itoa(days) + `}`,
			}
			db.Create(&auditLog)
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Inactive ServiceAccounts disabled successfully",
			"count":   result.RowsAffected,
		})
	}
}

