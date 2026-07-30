package api

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
)

// LogPlatformSecurityAudit records a high-severity platform audit row (best-effort).
func LogPlatformSecurityAudit(db *gorm.DB, c *gin.Context, action, resource, resourceID string, details map[string]interface{}) {
	if db == nil || !db.Migrator().HasTable(&models.AuditLog{}) {
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
	perms := middleware.GrantedPermissions(c)
	details["actor_permissions"] = authorization.ToStrings(perms)
	details["normalized_role"] = c.GetString(middleware.CtxNormalizedRole)
	b, _ := json.Marshal(details)
	entry := models.AuditLog{
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Details:    string(b),
		User:       uname,
		IP:         c.ClientIP(),
	}
	if uid > 0 {
		entry.UserID = uid
		_ = db.Create(&entry).Error
	} else {
		_ = db.Omit("UserID").Create(&entry).Error
	}
}
