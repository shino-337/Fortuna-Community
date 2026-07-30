package api

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/securityaudit"
)

// createExceptionRequest is the request body for creating an exception policy.
type createExceptionRequest struct {
	ResourceUID string     `json:"resourceUid"`
	CVEID       string     `json:"cveId"`
	InsightType string     `json:"insightType"`
	Reason      string     `json:"reason"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
}

// CreateException creates a new exception policy (POST /api/v1/risk/exceptions).
func CreateException(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createExceptionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
			return
		}
		// All three fields are required so that the exception matches the isExempted() lookup
		// which queries on (resource_uid, cve_id, insight_type).
		if req.ResourceUID == "" || req.CVEID == "" || req.InsightType == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "resourceUid, cveId, and insightType are all required"})
			return
		}
		if !requireResourceUIDClusterScope(db, c, req.ResourceUID) {
			return
		}

		createdBy := "system"
		if v, ok := c.Get("username"); ok {
			if s, ok := v.(string); ok && s != "" {
				createdBy = s
			}
		}

		policy := &models.ExceptionPolicy{
			ResourceUID: req.ResourceUID,
			CVEID:       req.CVEID,
			InsightType: req.InsightType,
			Reason:      req.Reason,
			ExpiresAt:   req.ExpiresAt,
			CreatedBy:   createdBy,
		}
		if err := db.Create(policy).Error; err != nil {
			log.Printf("[ExceptionHandler] Failed to create exception policy: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create exception policy"})
			return
		}

		log.Printf("[ExceptionHandler] Created exception policy ID=%d resource_uid=%s cve_id=%s type=%s by=%s",
			policy.ID, policy.ResourceUID, policy.CVEID, policy.InsightType, policy.CreatedBy)
		ev := securityaudit.FromRequest(
			c,
			authorization.ToStrings(middleware.GrantedPermissions(c)),
			c.GetString(middleware.CtxJWTSessionID),
			securityaudit.ActionFindingsExceptionCreate,
			"exception_policy",
			strconv.FormatUint(uint64(policy.ID), 10),
			"success",
			"medium",
			"jwt",
			nil,
			map[string]any{"id": policy.ID, "resourceUid": policy.ResourceUID, "cveId": policy.CVEID, "insightType": policy.InsightType},
			nil,
			nil,
		)
		securityaudit.Append(db, &ev)
		c.JSON(http.StatusCreated, policy)
	}
}

// ListExceptions lists exception policies (GET /api/v1/risk/exceptions).
// Optional query param: resource_uid — filter by resource UID.
func ListExceptions(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := db.Model(&models.ExceptionPolicy{})

		if uid := c.Query("resource_uid"); uid != "" {
			query = query.Where("resource_uid = ?", uid)
		}
		if cveID := c.Query("cve_id"); cveID != "" {
			query = query.Where("cve_id = ?", cveID)
		}
		if insightType := c.Query("insight_type"); insightType != "" {
			query = query.Where("insight_type = ?", insightType)
		}
		if clusterIDs, restricted := middleware.ScopedClusterIDs(c); restricted {
			query = query.Where(
				"resource_uid IN (?)",
				db.Model(&models.Pod{}).Select("uid").Where("cluster_id IN ?", clusterIDs),
			)
		}

		var policies []models.ExceptionPolicy
		if err := query.Order("created_at DESC").Find(&policies).Error; err != nil {
			log.Printf("[ExceptionHandler] Failed to list exception policies: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list exception policies"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"exceptions": policies,
			"total":      len(policies),
		})
	}
}

// DeleteException removes an exception policy by ID (DELETE /api/v1/risk/exceptions/:id).
func DeleteException(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var policy models.ExceptionPolicy
		if err := db.First(&policy, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "exception policy not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !requireResourceUIDClusterScope(db, c, policy.ResourceUID) {
			return
		}

		if err := db.Delete(&policy).Error; err != nil {
			log.Printf("[ExceptionHandler] Failed to delete exception policy ID=%s: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete exception policy"})
			return
		}

		log.Printf("[ExceptionHandler] Deleted exception policy ID=%s", id)
		c.JSON(http.StatusOK, gin.H{"message": "exception policy deleted", "id": id})
	}
}
