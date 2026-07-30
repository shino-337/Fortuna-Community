package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/activitylog"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
)

// RequireClusterScope enforces optional per-user cluster scope from users.scope_json (RBAC v2).
// Admins are unrestricted. Empty/missing scope = unrestricted. When cluster allow-list is non-empty,
// requests with :id route param must match one of the listed IDs (string match).
func RequireClusterScope(db *gorm.DB, param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		enforceClusterScope(c, db, strings.TrimSpace(c.Param(param)))
	}
}

// RequireClusterQueryScope enforces cluster scope from a query parameter.
func RequireClusterQueryScope(db *gorm.DB, queryKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		enforceClusterScope(c, db, strings.TrimSpace(c.Query(queryKey)))
	}
}

// RequirePodUIDClusterScope resolves a Kubernetes pod UID route parameter to its
// cluster ID, then applies the same per-user cluster allow-list as cluster routes.
func RequirePodUIDClusterScope(db *gorm.DB, param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.Next()
			return
		}
		podUID := strings.TrimSpace(c.Param(param))
		if podUID == "" {
			c.Next()
			return
		}
		var clusterID string
		err := db.Model(&models.Pod{}).
			Select("cluster_id").
			Where("uid = ?", podUID).
			Limit(1).
			Scan(&clusterID).Error
		if err != nil || strings.TrimSpace(clusterID) == "" {
			// Unknown pod UIDs should be handled by the endpoint itself as 404/empty.
			c.Next()
			return
		}
		enforceClusterScope(c, db, strings.TrimSpace(clusterID))
	}
}

func enforceClusterScope(c *gin.Context, db *gorm.DB, clusterKey string) {
	if db == nil {
		c.Next()
		return
	}
	if clusterKey == "" {
		c.Next()
		return
	}
	if ClusterAllowed(c, clusterKey) {
		c.Next()
		return
	}
	AbortClusterScopeDenied(db, c, clusterKey)
}

// ClusterAllowed reports whether the current request user may access clusterKey.
// Empty keys, admins, and unrestricted scope documents are allowed. Missing or malformed
// user context fails closed because scoped routes must run behind authentication.
func ClusterAllowed(c *gin.Context, clusterKey string) bool {
	clusterKey = strings.TrimSpace(clusterKey)
	if clusterKey == "" {
		return true
	}
	raw, ok := c.Get("user")
	if !ok || raw == nil {
		return false
	}
	u, ok := raw.(*models.User)
	if !ok || u == nil {
		return false
	}
	if strings.EqualFold(authorization.NormalizeRole(u.Role), models.RoleAdmin) {
		return true
	}
	doc := authorization.ParseScopeDocument(u.ScopeJSON)
	if !doc.RestrictsClusters() {
		return true
	}
	return doc.ClusterAllowed(clusterKey)
}

// AbortClusterScopeDenied emits the standard cluster-scope authorization failure.
func AbortClusterScopeDenied(db *gorm.DB, c *gin.Context, clusterKey string) {
	clusterKey = strings.TrimSpace(clusterKey)
	granted := GrantedPermissions(c)
	auditAuthzDenied(db, c, "scope:cluster:"+clusterKey, granted)
	activitylog.LogClusterScopeDenied(db, c, c.GetString(CtxJWTSessionID), clusterKey, granted)
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"error":               "forbidden",
		"reason":              "cluster_scope",
		"required_cluster_id": clusterKey,
	})
}

// ScopedClusterIDs returns the non-admin user's explicit cluster allow-list.
// The boolean is false when no cluster restriction applies.
func ScopedClusterIDs(c *gin.Context) ([]string, bool) {
	raw, ok := c.Get("user")
	if !ok || raw == nil {
		return nil, false
	}
	u, ok := raw.(*models.User)
	if !ok || u == nil {
		return nil, false
	}
	if strings.EqualFold(authorization.NormalizeRole(u.Role), models.RoleAdmin) {
		return nil, false
	}
	doc := authorization.ParseScopeDocument(u.ScopeJSON)
	if !doc.RestrictsClusters() {
		return nil, false
	}
	return doc.ClusterIDs(), true
}
