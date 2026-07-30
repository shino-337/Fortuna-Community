package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
)

// registerAuditRoutes registers /api/v1/audit/* (logs, reports).
func registerAuditRoutes(api *gin.RouterGroup, db *gorm.DB) {
	audit := api.Group("/audit")
	p := func(perm authorization.Permission) gin.HandlerFunc {
		return middleware.RequirePermission(db, perm)
	}
	audit.GET("/logs", p(authorization.PermissionSystemAuditRead), GetAuditLogs(db))
	audit.GET("/reports", p(authorization.PermissionSystemAuditRead), GetAuditReports(db))
}
