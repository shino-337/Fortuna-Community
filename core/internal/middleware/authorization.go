package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/activitylog"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
)

const (
	// CtxPermissions holds []authorization.Permission for the current request (authoritative).
	CtxPermissions = "permissions"
	// CtxNormalizedRole holds canonical role after legacy mapping (e.g. user → operator).
	CtxNormalizedRole = "normalized_role"
)

// GrantedPermissions returns permissions from Gin context (nil if unset).
func GrantedPermissions(c *gin.Context) []authorization.Permission {
	raw, ok := c.Get(CtxPermissions)
	if !ok || raw == nil {
		return nil
	}
	perms, ok := raw.([]authorization.Permission)
	if !ok {
		return nil
	}
	return perms
}

// RequirePermission enforces a single permission (deny-by-default).
func RequirePermission(db *gorm.DB, need authorization.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		granted := GrantedPermissions(c)
		if !authorization.HasPermission(granted, need) {
			auditAuthzDenied(db, c, string(need), granted)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden", "required_permission": string(need)})
			return
		}
		c.Next()
	}
}

// RequireAnyPermission allows the request if the caller has at least one of the permissions.
func RequireAnyPermission(db *gorm.DB, required ...authorization.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		granted := GrantedPermissions(c)
		if !authorization.HasAnyPermission(granted, required...) {
			names := make([]string, len(required))
			for i, p := range required {
				names[i] = string(p)
			}
			auditAuthzDenied(db, c, "any:"+strings.Join(names, ","), granted)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden", "required_permissions": names})
			return
		}
		c.Next()
	}
}

// RequireAllPermissions requires every listed permission.
func RequireAllPermissions(db *gorm.DB, required ...authorization.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		granted := GrantedPermissions(c)
		if !authorization.HasAllPermissions(granted, required...) {
			names := make([]string, len(required))
			for i, p := range required {
				names[i] = string(p)
			}
			auditAuthzDenied(db, c, "all:"+strings.Join(names, ","), granted)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden", "required_permissions": names})
			return
		}
		c.Next()
	}
}

// RequireScopedPermission enforces an atomic permission then optional cluster scope on route param `param` (RBAC v2).
func RequireScopedPermission(db *gorm.DB, param string, need authorization.Permission) gin.HandlerFunc {
	perm := RequirePermission(db, need)
	scope := RequireClusterScope(db, param)
	return func(c *gin.Context) {
		perm(c)
		if c.IsAborted() {
			return
		}
		scope(c)
	}
}

func auditAuthzDenied(db *gorm.DB, c *gin.Context, required string, granted []authorization.Permission) {
	if db == nil {
		return
	}
	if !db.Migrator().HasTable(&models.AuditLog{}) {
		return
	}
	uid := uint(0)
	uname := ""
	if u, ok := c.Get("user"); ok && u != nil {
		if userObj, ok2 := u.(*models.User); ok2 && userObj != nil {
			uid = userObj.ID
			uname = userObj.Username
		}
	}
	details, _ := json.Marshal(map[string]interface{}{
		"required":    required,
		"granted":     authorization.ToStrings(granted),
		"path":        c.Request.URL.Path,
		"method":      c.Request.Method,
		"request_id":  strings.TrimSpace(c.GetHeader("X-Request-ID")),
		"client_role": c.GetString(CtxNormalizedRole),
	})
	entry := models.AuditLog{
		Action:     "authorization_denied",
		Resource:   "http",
		ResourceID: c.FullPath(),
		Details:    string(details),
		User:       uname,
		IP:         c.ClientIP(),
	}
	if uid > 0 {
		entry.UserID = uid
		_ = db.Create(&entry).Error
	} else {
		_ = db.Omit("UserID").Create(&entry).Error
	}
	activitylog.LogAuthzDenied(db, c, c.GetString(CtxJWTSessionID), required, granted)
}
