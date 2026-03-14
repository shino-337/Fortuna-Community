package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// registerAuditRoutes registers /api/v1/audit/* (logs, reports).
// Legacy paths /audit-logs and /reports are removed; clients use /audit/logs and /audit/reports.
func registerAuditRoutes(api *gin.RouterGroup, db *gorm.DB) {
	audit := api.Group("/audit")
	audit.GET("/logs", GetAuditLogs(db))
	audit.GET("/reports", GetAuditReports(db))
}
