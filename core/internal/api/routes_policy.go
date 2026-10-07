package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/api/policy"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/securityaudit"
)

// registerPolicyRoutes registers /api/v1/policy/* (rules engine + policy templates/instances).
func registerPolicyRoutes(api *gin.RouterGroup, db *gorm.DB) {
	pol := api.Group("/policy")
	p := func(perm authorization.Permission) gin.HandlerFunc {
		return middleware.RequirePermission(db, perm)
	}

	pol.GET("/rules", p(authorization.PermissionPoliciesRead), GetRules(db))
	pol.GET("/rules/uid/:uid", p(authorization.PermissionPoliciesRead), GetRule(db))
	pol.POST("/rules", p(authorization.PermissionPoliciesDraft), auditOnSuccess(db, securityaudit.ActionPolicyRuleCreate, "policy_rule", ""), CreateRule(db))
	pol.PUT("/rules/uid/:uid", p(authorization.PermissionPoliciesDraft), auditOnSuccess(db, securityaudit.ActionPolicyRuleUpdate, "policy_rule", "uid"), UpdateRule(db))
	pol.DELETE("/rules/uid/:uid", p(authorization.PermissionPoliciesDelete), auditOnSuccess(db, securityaudit.ActionPolicyRuleDelete, "policy_rule", "uid"), DeleteRule(db))
	pol.POST("/rules/uid/:uid/test", p(authorization.PermissionPoliciesRead), TestRule(db))
	pol.POST("/rules/reload", p(authorization.PermissionPoliciesPublish), ReloadRules(db))
	pol.GET("/rules/uid/:uid/metrics", p(authorization.PermissionPoliciesRead), GetRuleMetrics(db))
	pol.GET("/rules/uid/:uid/matches", p(authorization.PermissionPoliciesRead), GetRuleMatches(db))

	policyHandler := policy.NewPolicyHandler(db)
	pol.GET("/templates", p(authorization.PermissionPoliciesRead), policyHandler.ListTemplates)
	pol.GET("/templates/:templateId", p(authorization.PermissionPoliciesRead), policyHandler.GetTemplate)
	pol.POST("/templates", p(authorization.PermissionPoliciesDraft), auditOnSuccess(db, securityaudit.ActionPolicyTemplateCreate, "policy_template", ""), policyHandler.CreateTemplate)
	pol.PUT("/templates/:templateId/:version", p(authorization.PermissionPoliciesDraft), auditOnSuccess(db, securityaudit.ActionPolicyTemplateUpdate, "policy_template", "templateId"), policyHandler.UpdateTemplate)
	pol.DELETE("/templates/:templateId/:version", p(authorization.PermissionPoliciesDelete), auditOnSuccess(db, securityaudit.ActionPolicyTemplateDelete, "policy_template", "templateId"), policyHandler.DeleteTemplate)
	pol.GET("/instances", p(authorization.PermissionPoliciesRead), policyHandler.ListInstances)
	pol.GET("/instances/:instanceName", p(authorization.PermissionPoliciesRead), policyHandler.GetInstance)
	pol.POST("/instances", p(authorization.PermissionPoliciesDraft), auditOnSuccess(db, securityaudit.ActionPolicyInstanceCreate, "policy_instance", ""), policyHandler.CreateInstance)
	pol.PUT("/instances/:instanceName", p(authorization.PermissionPoliciesDraft), auditOnSuccess(db, securityaudit.ActionPolicyInstanceUpdate, "policy_instance", "instanceName"), policyHandler.UpdateInstance)
	pol.DELETE("/instances/:instanceName", p(authorization.PermissionPoliciesDelete), auditOnSuccess(db, securityaudit.ActionPolicyInstanceDelete, "policy_instance", "instanceName"), policyHandler.DeleteInstance)
}
