package activitylog

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/securityaudit"
)

// LogAuthzDenied records an authorization denial in security_activity_logs (best-effort).
func LogAuthzDenied(db *gorm.DB, c *gin.Context, sessionID, required string, granted []authorization.Permission) {
	if db == nil || !db.Migrator().HasTable(&models.SecurityActivityLog{}) {
		return
	}
	details, _ := json.Marshal(map[string]interface{}{
		"required": required,
		"path":     c.Request.URL.Path,
		"method":   c.Request.Method,
	})
	ev := securityaudit.FromRequest(
		c,
		authorization.ToStrings(granted),
		sessionID,
		"authorization_denied",
		"http",
		c.FullPath(),
		"deny",
		"high",
		"jwt",
		nil,
		nil,
		json.RawMessage(details),
		nil,
	)
	securityaudit.Append(db, &ev)
}

// LogClusterScopeDenied records a cluster scope boundary violation (RBAC v2).
func LogClusterScopeDenied(db *gorm.DB, c *gin.Context, sessionID, clusterID string, granted []authorization.Permission) {
	if db == nil || !db.Migrator().HasTable(&models.SecurityActivityLog{}) {
		return
	}
	details, _ := json.Marshal(map[string]interface{}{
		"cluster_id": clusterID,
		"path":       c.Request.URL.Path,
		"method":     c.Request.Method,
	})
	ev := securityaudit.FromRequest(
		c,
		authorization.ToStrings(granted),
		sessionID,
		"cluster_scope_denied",
		"cluster",
		clusterID,
		"deny",
		"high",
		"jwt",
		nil,
		nil,
		json.RawMessage(details),
		nil,
	)
	securityaudit.Append(db, &ev)
}
