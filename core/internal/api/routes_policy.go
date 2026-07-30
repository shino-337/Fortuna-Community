package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/api/policy"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
)

// registerPolicyRoutes registers /api/v1/policy/* (rules engine + policy templates/instances).
func registerPolicyRoutes(api *gin.RouterGroup, db *gorm.DB) {
	pol := api.Group("/policy")
	p := func(perm authorization.Permission) gin.HandlerFunc {
		return middleware.RequirePermission(db, perm)
	}

	pol.GET("/rules", p(authorization.PermissionPoliciesRead), GetRules(db))
	pol.GET("/rules/uid/:uid", p(authorization.PermissionPoliciesRead), GetRule(db))
	pol.GET("/rules/:id", p(authorization.PermissionPoliciesRead), GetRule(db))
	pol.POST("/rules", p(authorization.PermissionPoliciesDraft), CreateRule(db))
	pol.PUT("/rules/uid/:uid", p(authorization.PermissionPoliciesDraft), UpdateRule(db))
	pol.PUT("/rules/:id", p(authorization.PermissionPoliciesDraft), UpdateRule(db))
	pol.DELETE("/rules/uid/:uid", p(authorization.PermissionPoliciesDelete), DeleteRule(db))
	pol.DELETE("/rules/:id", p(authorization.PermissionPoliciesDelete), DeleteRule(db))
	pol.POST("/rules/uid/:uid/test", p(authorization.PermissionPoliciesRead), TestRule(db))
	pol.POST("/rules/:id/test", p(authorization.PermissionPoliciesRead), TestRule(db))
	pol.POST("/rules/reload", p(authorization.PermissionPoliciesPublish), ReloadRules(db))
	pol.GET("/rules/uid/:uid/metrics", p(authorization.PermissionPoliciesRead), GetRuleMetrics(db))
	pol.GET("/rules/:id/metrics", p(authorization.PermissionPoliciesRead), GetRuleMetrics(db))
	pol.GET("/rules/uid/:uid/matches", p(authorization.PermissionPoliciesRead), GetRuleMatches(db))
	pol.GET("/rules/:id/matches", p(authorization.PermissionPoliciesRead), GetRuleMatches(db))

	policyHandler := policy.NewPolicyHandler(db)
	pol.GET("/templates", p(authorization.PermissionPoliciesRead), policyHandler.ListTemplates)
	pol.GET("/templates/:templateId", p(authorization.PermissionPoliciesRead), policyHandler.GetTemplate)
	pol.POST("/templates", p(authorization.PermissionPoliciesDraft), policyHandler.CreateTemplate)
	pol.PUT("/templates/:templateId/:version", p(authorization.PermissionPoliciesDraft), policyHandler.UpdateTemplate)
	pol.DELETE("/templates/:templateId/:version", p(authorization.PermissionPoliciesDelete), policyHandler.DeleteTemplate)
	pol.GET("/instances", p(authorization.PermissionPoliciesRead), policyHandler.ListInstances)
	pol.GET("/instances/:instanceName", p(authorization.PermissionPoliciesRead), policyHandler.GetInstance)
	pol.POST("/instances", p(authorization.PermissionPoliciesDraft), policyHandler.CreateInstance)
	pol.PUT("/instances/:instanceName", p(authorization.PermissionPoliciesDraft), policyHandler.UpdateInstance)
	pol.DELETE("/instances/:instanceName", p(authorization.PermissionPoliciesDelete), policyHandler.DeleteInstance)
}
