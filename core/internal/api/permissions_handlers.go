package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// ServiceAccountPermissions represents permissions for a ServiceAccount
type ServiceAccountPermissions struct {
	ServiceAccountID uint                        `json:"serviceAccountId"`
	RoleBindings     []RoleBindingPermission     `json:"roleBindings"`
	ClusterRoleBindings []ClusterRoleBindingPermission `json:"clusterRoleBindings"`
	EffectiveRules   []Rule                      `json:"effectiveRules"`
}

// RoleBindingPermission represents a RoleBinding with its Role rules
type RoleBindingPermission struct {
	RoleBinding models.RoleBinding `json:"roleBinding"`
	Role        models.Role        `json:"role"`
}

// ClusterRoleBindingPermission represents a ClusterRoleBinding with its ClusterRole rules
type ClusterRoleBindingPermission struct {
	ClusterRoleBinding models.ClusterRoleBinding `json:"clusterRoleBinding"`
	ClusterRole        models.ClusterRole         `json:"clusterRole"`
}

// Rule represents a policy rule
type Rule struct {
	Verbs           []string `json:"verbs"`
	APIGroups       []string `json:"apiGroups"`
	Resources       []string `json:"resources"`
	ResourceNames   []string `json:"resourceNames"`
	NonResourceURLs []string `json:"nonResourceURLs"`
}

// GetServiceAccountPermissions returns permissions for a specific ServiceAccount
func GetServiceAccountPermissions(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var sa models.ServiceAccount
		if err := db.First(&sa, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "ServiceAccount not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		permissions := ServiceAccountPermissions{
			ServiceAccountID: sa.ID,
			RoleBindings:     []RoleBindingPermission{},
			ClusterRoleBindings: []ClusterRoleBindingPermission{},
			EffectiveRules:   []Rule{},
		}

		// Get RoleBindings that reference this ServiceAccount
		var rbs []models.RoleBinding
		db.Where("cluster_id = ?", sa.ClusterID).Find(&rbs)

		for _, rb := range rbs {
			var subjects []map[string]interface{}
			if err := json.Unmarshal([]byte(rb.Subjects), &subjects); err == nil {
				for _, subject := range subjects {
					if kind, ok := subject["kind"].(string); ok && kind == "ServiceAccount" {
						if name, ok := subject["name"].(string); ok && name == sa.Name {
							if ns, ok := subject["namespace"].(string); ok && ns == sa.Namespace {
								// This RoleBinding references our ServiceAccount
								var roleRef map[string]interface{}
								if err := json.Unmarshal([]byte(rb.RoleRef), &roleRef); err == nil {
									if roleName, ok := roleRef["name"].(string); ok {
										var role models.Role
										if err := db.Where("cluster_id = ? AND namespace = ? AND name = ?",
											sa.ClusterID, rb.Namespace, roleName).First(&role).Error; err == nil {
											permissions.RoleBindings = append(permissions.RoleBindings, RoleBindingPermission{
												RoleBinding: rb,
												Role:        role,
											})

											// Extract rules from Role
											var rules []map[string]interface{}
											if err := json.Unmarshal([]byte(role.Rules), &rules); err == nil {
												for _, ruleMap := range rules {
													rule := Rule{
														Verbs:           []string{},
														APIGroups:       []string{},
														Resources:       []string{},
														ResourceNames:   []string{},
														NonResourceURLs: []string{},
													}
													if verbs, ok := ruleMap["verbs"].([]interface{}); ok {
														for _, v := range verbs {
															if verb, ok := v.(string); ok {
																rule.Verbs = append(rule.Verbs, verb)
															}
														}
													}
													if apiGroups, ok := ruleMap["apiGroups"].([]interface{}); ok {
														for _, ag := range apiGroups {
															if apiGroup, ok := ag.(string); ok {
																rule.APIGroups = append(rule.APIGroups, apiGroup)
															}
														}
													}
													if resources, ok := ruleMap["resources"].([]interface{}); ok {
														for _, r := range resources {
															if resource, ok := r.(string); ok {
																rule.Resources = append(rule.Resources, resource)
															}
														}
													}
													if resourceNames, ok := ruleMap["resourceNames"].([]interface{}); ok {
														for _, rn := range resourceNames {
															if resourceName, ok := rn.(string); ok {
																rule.ResourceNames = append(rule.ResourceNames, resourceName)
															}
														}
													}
													if nonResourceURLs, ok := ruleMap["nonResourceURLs"].([]interface{}); ok {
														for _, nru := range nonResourceURLs {
															if nonResourceURL, ok := nru.(string); ok {
																rule.NonResourceURLs = append(rule.NonResourceURLs, nonResourceURL)
															}
														}
													}
													permissions.EffectiveRules = append(permissions.EffectiveRules, rule)
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}

		// Get ClusterRoleBindings that reference this ServiceAccount
		var crbs []models.ClusterRoleBinding
		db.Where("cluster_id = ?", sa.ClusterID).Find(&crbs)

		for _, crb := range crbs {
			var subjects []map[string]interface{}
			if err := json.Unmarshal([]byte(crb.Subjects), &subjects); err == nil {
				for _, subject := range subjects {
					if kind, ok := subject["kind"].(string); ok && kind == "ServiceAccount" {
						if name, ok := subject["name"].(string); ok && name == sa.Name {
							// Check namespace if present in subject, otherwise match any namespace
							subjectNS, hasNS := subject["namespace"].(string)
							if !hasNS || subjectNS == sa.Namespace {
								// This ClusterRoleBinding references our ServiceAccount
								var roleRef map[string]interface{}
								if err := json.Unmarshal([]byte(crb.RoleRef), &roleRef); err == nil {
									if roleName, ok := roleRef["name"].(string); ok {
										var clusterRole models.ClusterRole
										if err := db.Where("cluster_id = ? AND name = ?",
											sa.ClusterID, roleName).First(&clusterRole).Error; err == nil {
											permissions.ClusterRoleBindings = append(permissions.ClusterRoleBindings, ClusterRoleBindingPermission{
												ClusterRoleBinding: crb,
												ClusterRole:        clusterRole,
											})

											// Extract rules from ClusterRole
											var rules []map[string]interface{}
											if err := json.Unmarshal([]byte(clusterRole.Rules), &rules); err == nil {
												for _, ruleMap := range rules {
													rule := Rule{
														Verbs:           []string{},
														APIGroups:       []string{},
														Resources:       []string{},
														ResourceNames:   []string{},
														NonResourceURLs: []string{},
													}
													if verbs, ok := ruleMap["verbs"].([]interface{}); ok {
														for _, v := range verbs {
															if verb, ok := v.(string); ok {
																rule.Verbs = append(rule.Verbs, verb)
															}
														}
													}
													if apiGroups, ok := ruleMap["apiGroups"].([]interface{}); ok {
														for _, ag := range apiGroups {
															if apiGroup, ok := ag.(string); ok {
																rule.APIGroups = append(rule.APIGroups, apiGroup)
															}
														}
													}
													if resources, ok := ruleMap["resources"].([]interface{}); ok {
														for _, r := range resources {
															if resource, ok := r.(string); ok {
																rule.Resources = append(rule.Resources, resource)
															}
														}
													}
													if resourceNames, ok := ruleMap["resourceNames"].([]interface{}); ok {
														for _, rn := range resourceNames {
															if resourceName, ok := rn.(string); ok {
																rule.ResourceNames = append(rule.ResourceNames, resourceName)
															}
														}
													}
													if nonResourceURLs, ok := ruleMap["nonResourceURLs"].([]interface{}); ok {
														for _, nru := range nonResourceURLs {
															if nonResourceURL, ok := nru.(string); ok {
																rule.NonResourceURLs = append(rule.NonResourceURLs, nonResourceURL)
															}
														}
													}
													permissions.EffectiveRules = append(permissions.EffectiveRules, rule)
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}

		c.JSON(http.StatusOK, permissions)
	}
}

