package api

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
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

// resolveExceptionCluster binds an exception to one authoritative cluster. A
// globally unrestricted caller must provide clusterId when a Pod UID is present
// in more than one cluster; ambiguous ownership is never guessed.
func resolveExceptionCluster(db *gorm.DB, scope riskGovernanceScope, uid string) (string, error) {
	q := db.Unscoped().Model(&models.Pod{}).
		Distinct("cluster_id").
		Where("uid = ? AND cluster_id <> ''", uid)
	if scope.clusterID != "" {
		q = q.Where("cluster_id = ?", scope.clusterID)
	}
	if scope.restricted {
		q = q.Where("cluster_id IN ?", scope.clusterIDs)
	}
	var clusters []string
	if err := q.Pluck("cluster_id", &clusters).Error; err != nil {
		return "", err
	}
	if len(clusters) == 0 {
		return "", gorm.ErrRecordNotFound
	}
	if len(clusters) != 1 {
		return "", errAmbiguousExceptionResource
	}
	return clusters[0], nil
}

var errAmbiguousExceptionResource = &exceptionScopeError{"resource UID exists in multiple clusters; specify clusterId"}

type exceptionScopeError struct{ message string }
func (e *exceptionScopeError) Error() string { return e.message }

// CreateException creates a new exception policy (POST /api/v1/risk/exceptions).
func CreateException(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := resolveRiskGovernanceScope(db, c)
		if !ok { return }
		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		defer cancel()
		db := db.WithContext(ctx)
		var req createExceptionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
			return
		}
		req.ResourceUID = strings.TrimSpace(req.ResourceUID)
		req.CVEID = strings.TrimSpace(req.CVEID)
		req.InsightType = strings.TrimSpace(req.InsightType)
		if req.ResourceUID == "" || req.CVEID == "" || req.InsightType == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "resourceUid, cveId, and insightType are all required"})
			return
		}

		clusterID, err := resolveExceptionCluster(db, scope, req.ResourceUID)
		if err != nil {
			switch err {
			case gorm.ErrRecordNotFound:
				c.JSON(http.StatusNotFound, gin.H{"error": "resource not found in allowed cluster scope"})
			case errAmbiguousExceptionResource:
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve resource ownership"})
			}
			return
		}
		if !middleware.ClusterAllowed(c, clusterID) {
			middleware.AbortClusterScopeDenied(db, c, clusterID)
			return
		}

		createdBy := "system"
		if v, ok := c.Get("username"); ok {
			if s, ok := v.(string); ok && s != "" { createdBy = s }
		}

		policy := &models.ExceptionPolicy{
			ClusterID: clusterID, ResourceUID: req.ResourceUID, CVEID: req.CVEID,
			InsightType: req.InsightType, Reason: req.Reason, ExpiresAt: req.ExpiresAt, CreatedBy: createdBy,
		}
		if err := db.Create(policy).Error; err != nil {
			log.Printf("[ExceptionHandler] Failed to create exception policy: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create exception policy"})
			return
		}

		log.Printf("[ExceptionHandler] Created exception policy ID=%d cluster_id=%s resource_uid=%s cve_id=%s type=%s by=%s",
			policy.ID, policy.ClusterID, policy.ResourceUID, policy.CVEID, policy.InsightType, policy.CreatedBy)
		ev := securityaudit.FromRequest(c, authorization.ToStrings(middleware.GrantedPermissions(c)),
			c.GetString(middleware.CtxJWTSessionID), securityaudit.ActionFindingsExceptionCreate,
			"exception_policy", strconv.FormatUint(uint64(policy.ID), 10), "success", "medium", "jwt", nil,
			map[string]any{"id": policy.ID, "clusterId": policy.ClusterID, "resourceUid": policy.ResourceUID, "cveId": policy.CVEID, "insightType": policy.InsightType}, nil, nil)
		securityaudit.Append(db, &ev)
		c.JSON(http.StatusCreated, policy)
	}
}

// ListExceptions lists exception policies (GET /api/v1/risk/exceptions).
func ListExceptions(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := resolveRiskGovernanceScope(db, c)
		if !ok { return }
		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		defer cancel()
		db := db.WithContext(ctx)
		query := db.Model(&models.ExceptionPolicy{})

		if uid := strings.TrimSpace(c.Query("resource_uid")); uid != "" { query = query.Where("resource_uid = ?", uid) }
		if cveID := strings.TrimSpace(c.Query("cve_id")); cveID != "" { query = query.Where("cve_id = ?", cveID) }
		if insightType := strings.TrimSpace(c.Query("insight_type")); insightType != "" { query = query.Where("insight_type = ?", insightType) }
		if scope.clusterID != "" { query = query.Where("cluster_id = ?", scope.clusterID) }
		if scope.restricted { query = query.Where("cluster_id IN ?", scope.clusterIDs) }

		var policies []models.ExceptionPolicy
		if err := query.Order("created_at DESC").Find(&policies).Error; err != nil {
			log.Printf("[ExceptionHandler] Failed to list exception policies: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list exception policies"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"exceptions": policies, "total": len(policies)})
	}
}

// DeleteException removes an exception policy by ID (DELETE /api/v1/risk/exceptions/:id).
func DeleteException(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := resolveRiskGovernanceScope(db, c)
		if !ok { return }
		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		defer cancel()
		db := db.WithContext(ctx)
		id := c.Param("id")
		if parsed, err := strconv.ParseUint(id, 10, 64); err != nil || parsed == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid exception ID"})
			return
		}

		var policy models.ExceptionPolicy
		if err := db.First(&policy, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound { c.JSON(http.StatusNotFound, gin.H{"error": "exception policy not found"}); return }
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch exception policy"})
			return
		}
		if policy.ClusterID == "" || !middleware.ClusterAllowed(c, policy.ClusterID) || (scope.clusterID != "" && scope.clusterID != policy.ClusterID) {
			middleware.AbortClusterScopeDenied(db, c, policy.ClusterID)
			return
		}
		if scope.restricted {
			allowed := false
			for _, clusterID := range scope.clusterIDs { if clusterID == policy.ClusterID { allowed = true; break } }
			if !allowed { middleware.AbortClusterScopeDenied(db, c, policy.ClusterID); return }
		}

		if err := db.Delete(&policy).Error; err != nil {
			log.Printf("[ExceptionHandler] Failed to delete exception policy ID=%s: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete exception policy"})
			return
		}
		log.Printf("[ExceptionHandler] Deleted exception policy ID=%s cluster_id=%s", id, policy.ClusterID)
		c.JSON(http.StatusOK, gin.H{"message": "exception policy deleted", "id": id})
	}
}
