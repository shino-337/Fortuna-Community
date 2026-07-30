package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/securityaudit"
)

func auditUserID(c *gin.Context) uint {
	if v, ok := c.Get("userID"); ok {
		if id, ok := v.(uint); ok && id > 0 {
			return id
		}
	}
	return 0
}

func scopedServiceAccountsByIDs(db *gorm.DB, c *gin.Context, ids []uint, includeDeleted bool) ([]models.ServiceAccount, bool, error) {
	allowedClusters, restricted := middleware.ScopedClusterIDs(c)
	if restricted && len(allowedClusters) == 0 {
		return nil, false, nil
	}
	q := db.Model(&models.ServiceAccount{}).Where("id IN ?", ids)
	if includeDeleted {
		q = q.Unscoped()
	}
	if restricted {
		q = q.Where("cluster_id IN ?", allowedClusters)
	}
	var sas []models.ServiceAccount
	if err := q.Find(&sas).Error; err != nil {
		return nil, true, err
	}
	return sas, true, nil
}

func serviceAccountIDs(sas []models.ServiceAccount) []uint {
	ids := make([]uint, 0, len(sas))
	for _, sa := range sas {
		ids = append(ids, sa.ID)
	}
	return ids
}

// BulkDisableServiceAccounts disables multiple service accounts
func BulkDisableServiceAccounts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		type BulkRequest struct {
			IDs []uint `json:"ids" binding:"required,min=1,max=100"`
		}

		var req BulkRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Get user info for audit
		uid := auditUserID(c)
		username, _ := c.Get("username")

		sas, allowed, err := scopedServiceAccountsByIDs(db, c, req.IDs, false)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": "no clusters in scope"})
			return
		}
		scopedIDs := serviceAccountIDs(sas)

		// Update service accounts (soft delete)
		result := db.Where("id IN ?", scopedIDs).Delete(&models.ServiceAccount{})
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
			return
		}

		// Log audit for each disabled SA
		for _, sa := range sas {
			auditLog := models.AuditLog{
				ClusterID:  sa.ClusterID,
				Action:     "disable",
				Resource:   "serviceaccount",
				ResourceID: strconv.Itoa(int(sa.ID)),
				User:       username.(string),
				IP:         c.ClientIP(),
			}
			if uid > 0 {
				auditLog.UserID = uid
				db.Create(&auditLog)
			} else {
				db.Omit("UserID").Create(&auditLog)
			}
		}

		ev := securityaudit.FromRequest(
			c,
			authorization.ToStrings(middleware.GrantedPermissions(c)),
			c.GetString(middleware.CtxJWTSessionID),
			securityaudit.ActionInventoryServiceAccountBulk+"_disable",
			"serviceaccount",
			"bulk",
			"success",
			"high",
			"jwt",
			nil,
			map[string]any{"ids": req.IDs, "rowsAffected": result.RowsAffected},
			nil,
			nil,
		)
		securityaudit.Append(db, &ev)

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
			IDs []uint `json:"ids" binding:"required,min=1,max=100"`
		}

		var req BulkRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Get user info for audit
		uid := auditUserID(c)
		username, _ := c.Get("username")

		// Get service accounts before deletion for audit and cluster-scope filtering.
		sas, allowed, err := scopedServiceAccountsByIDs(db, c, req.IDs, false)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": "no clusters in scope"})
			return
		}
		scopedIDs := serviceAccountIDs(sas)

		// Hard delete
		result := db.Unscoped().Where("id IN ?", scopedIDs).Delete(&models.ServiceAccount{})
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
			return
		}

		// Log audit
		for _, sa := range sas {
			auditLog := models.AuditLog{
				ClusterID:  sa.ClusterID,
				Action:     "delete",
				Resource:   "serviceaccount",
				ResourceID: strconv.Itoa(int(sa.ID)),
				User:       username.(string),
				IP:         c.ClientIP(),
			}
			if uid > 0 {
				auditLog.UserID = uid
				db.Create(&auditLog)
			} else {
				db.Omit("UserID").Create(&auditLog)
			}
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
			if !middleware.ClusterAllowed(c, clusterID) {
				middleware.AbortClusterScopeDenied(db, c, clusterID)
				return
			}
			query = query.Where("cluster_id = ?", clusterID)
		} else if allowedClusters, restricted := middleware.ScopedClusterIDs(c); restricted {
			if len(allowedClusters) == 0 {
				c.JSON(http.StatusForbidden, gin.H{"error": "no clusters in scope"})
				return
			}
			query = query.Where("cluster_id IN ?", allowedClusters)
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
		uid := auditUserID(c)
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
				Action:     "disable",
				Resource:   "serviceaccount",
				ResourceID: strconv.Itoa(int(sa.ID)),
				User:       username.(string),
				IP:         c.ClientIP(),
				Details:    `{"reason":"inactive","days":` + strconv.Itoa(days) + `}`,
			}
			if uid > 0 {
				auditLog.UserID = uid
				db.Create(&auditLog)
			} else {
				db.Omit("UserID").Create(&auditLog)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Inactive ServiceAccounts disabled successfully",
			"count":   result.RowsAffected,
		})
	}
}
