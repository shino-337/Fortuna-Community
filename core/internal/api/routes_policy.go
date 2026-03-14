package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/api/policy"
)

// registerPolicyRoutes registers /api/v1/policy/* (rules engine + policy templates/instances).
func registerPolicyRoutes(api *gin.RouterGroup, db *gorm.DB) {
	pol := api.Group("/policy")

	// Rules (detection rules engine)
	pol.GET("/rules", GetRules(db))
	pol.GET("/rules/:id", GetRule(db))
	pol.POST("/rules", CreateRule(db))
	pol.PUT("/rules/:id", UpdateRule(db))
	pol.DELETE("/rules/:id", DeleteRule(db))
	pol.POST("/rules/:id/test", TestRule(db))
	pol.POST("/rules/reload", ReloadRules(db))
	pol.GET("/rules/:id/metrics", GetRuleMetrics(db))
	pol.GET("/rules/:id/matches", GetRuleMatches(db))

	// Policy templates & instances
	policyHandler := policy.NewPolicyHandler(db)
	pol.GET("/templates", policyHandler.ListTemplates)
	pol.GET("/templates/:templateId", policyHandler.GetTemplate)
	pol.POST("/templates", policyHandler.CreateTemplate)
	pol.PUT("/templates/:templateId/:version", policyHandler.UpdateTemplate)
	pol.DELETE("/templates/:templateId/:version", policyHandler.DeleteTemplate)
	pol.GET("/instances", policyHandler.ListInstances)
	pol.GET("/instances/:instanceName", policyHandler.GetInstance)
	pol.POST("/instances", policyHandler.CreateInstance)
	pol.PUT("/instances/:instanceName", policyHandler.UpdateInstance)
	pol.DELETE("/instances/:instanceName", policyHandler.DeleteInstance)
}
