package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

type SubjectReport struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type BindingReport struct {
	Kind           string          `json:"kind"`
	Name           string          `json:"name"`
	Namespace      string          `json:"namespace"`
	RoleRefKind    string          `json:"roleRefKind"`
	RoleRefName    string          `json:"roleRefName"`
	Subjects       []SubjectReport `json:"subjects"`
	IsClusterAdmin bool            `json:"isClusterAdmin"`
}

type RoleReport struct {
	Kind                string `json:"kind"`
	Name                string `json:"name"`
	Namespace           string `json:"namespace"`
	HasWildcard         bool   `json:"hasWildcard"`
	HasSensitiveActions bool   `json:"hasSensitiveActions"`
	RulesCount          int    `json:"rulesCount"`
}

type PodRiskReport struct {
	PodUID          string          `json:"podUid"`
	PodName         string          `json:"podName"`
	Namespace       string          `json:"namespace"`
	ClusterID       string          `json:"clusterId"`
	ServiceAccount  string          `json:"serviceAccount"`
	ServiceAccountUID string        `json:"serviceAccountUid"`
	Bindings        []BindingReport `json:"bindings"`
	Roles           []RoleReport    `json:"roles"`
	Insights        []models.Insight `json:"insights"`
	Summary         map[string]interface{} `json:"summary"`
}

// GetPodRiskReport builds a detailed RBAC risk report for a pod.
func GetPodRiskReport(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := c.Param("uid")
		var pod models.Pod
		if err := db.Where("uid = ? AND deleted_at IS NULL", podUID).First(&pod).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "pod not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		report := PodRiskReport{
			PodUID:         pod.UID,
			PodName:        pod.Name,
			Namespace:      pod.Namespace,
			ClusterID:      pod.ClusterID,
			ServiceAccount: pod.ServiceAccount,
			Bindings:       []BindingReport{},
			Roles:          []RoleReport{},
			Insights:       []models.Insight{},
			Summary:        map[string]interface{}{},
		}

		var sa models.ServiceAccount
		if pod.ServiceAccount != "" {
			if err := db.Where("cluster_id = ? AND name = ? AND namespace = ? AND deleted_at IS NULL",
				pod.ClusterID, pod.ServiceAccount, pod.Namespace).First(&sa).Error; err == nil {
				report.ServiceAccountUID = sa.UID
			}
		}

		var roleBindings []models.RoleBinding
		db.Where("cluster_id = ? AND namespace = ? AND deleted_at IS NULL", pod.ClusterID, pod.Namespace).Find(&roleBindings)

		var clusterRoleBindings []models.ClusterRoleBinding
		db.Where("cluster_id = ? AND deleted_at IS NULL", pod.ClusterID).Find(&clusterRoleBindings)

		matchedRoleRefs := make([]map[string]string, 0)
		resourceUIDs := []string{pod.UID}
		if report.ServiceAccountUID != "" {
			resourceUIDs = append(resourceUIDs, report.ServiceAccountUID)
		}

		for _, rb := range roleBindings {
			var subjects []SubjectReport
			if err := json.Unmarshal([]byte(rb.Subjects), &subjects); err != nil {
				continue
			}
			matches := false
			for _, sub := range subjects {
				if sub.Kind == "ServiceAccount" && sub.Name == pod.ServiceAccount && sub.Namespace == pod.Namespace {
					matches = true
					break
				}
			}
			if !matches {
				continue
			}

			var roleRef struct {
				Kind string `json:"kind"`
				Name string `json:"name"`
			}
			if err := json.Unmarshal([]byte(rb.RoleRef), &roleRef); err != nil {
				continue
			}

			report.Bindings = append(report.Bindings, BindingReport{
				Kind:           "RoleBinding",
				Name:           rb.Name,
				Namespace:      rb.Namespace,
				RoleRefKind:    roleRef.Kind,
				RoleRefName:    roleRef.Name,
				Subjects:       subjects,
				IsClusterAdmin: strings.EqualFold(roleRef.Name, "cluster-admin"),
			})
			matchedRoleRefs = append(matchedRoleRefs, map[string]string{"kind": roleRef.Kind, "name": roleRef.Name, "namespace": rb.Namespace})
			resourceUIDs = append(resourceUIDs, rb.UID)
		}

		for _, crb := range clusterRoleBindings {
			var subjects []SubjectReport
			if err := json.Unmarshal([]byte(crb.Subjects), &subjects); err != nil {
				continue
			}
			matches := false
			for _, sub := range subjects {
				if sub.Kind == "ServiceAccount" && sub.Name == pod.ServiceAccount && sub.Namespace == pod.Namespace {
					matches = true
					break
				}
			}
			if !matches {
				continue
			}

			var roleRef struct {
				Kind string `json:"kind"`
				Name string `json:"name"`
			}
			if err := json.Unmarshal([]byte(crb.RoleRef), &roleRef); err != nil {
				continue
			}

			report.Bindings = append(report.Bindings, BindingReport{
				Kind:           "ClusterRoleBinding",
				Name:           crb.Name,
				Namespace:      "",
				RoleRefKind:    roleRef.Kind,
				RoleRefName:    roleRef.Name,
				Subjects:       subjects,
				IsClusterAdmin: strings.EqualFold(roleRef.Name, "cluster-admin"),
			})
			matchedRoleRefs = append(matchedRoleRefs, map[string]string{"kind": roleRef.Kind, "name": roleRef.Name, "namespace": ""})
			resourceUIDs = append(resourceUIDs, crb.UID)
		}

		wildcardRoles := 0
		overprivilegedRoles := 0
		clusterAdminBindings := 0
		for _, b := range report.Bindings {
			if b.IsClusterAdmin {
				clusterAdminBindings++
			}
		}

		for _, ref := range matchedRoleRefs {
			switch ref["kind"] {
			case "Role":
				var role models.Role
				if err := db.Where("cluster_id = ? AND name = ? AND namespace = ? AND deleted_at IS NULL",
					pod.ClusterID, ref["name"], ref["namespace"]).First(&role).Error; err != nil {
					continue
				}
				resourceUIDs = append(resourceUIDs, role.UID)
				roleReport := buildRoleReport("Role", role.Name, role.Namespace, role.Rules)
				if roleReport.HasWildcard {
					wildcardRoles++
				}
				if roleReport.HasSensitiveActions {
					overprivilegedRoles++
				}
				report.Roles = append(report.Roles, roleReport)
			case "ClusterRole":
				var cr models.ClusterRole
				if err := db.Where("cluster_id = ? AND name = ? AND deleted_at IS NULL",
					pod.ClusterID, ref["name"]).First(&cr).Error; err != nil {
					continue
				}
				resourceUIDs = append(resourceUIDs, cr.UID)
				roleReport := buildRoleReport("ClusterRole", cr.Name, "", cr.Rules)
				if roleReport.HasWildcard {
					wildcardRoles++
				}
				if roleReport.HasSensitiveActions {
					overprivilegedRoles++
				}
				report.Roles = append(report.Roles, roleReport)
			}
		}

		if len(resourceUIDs) > 0 {
			if err := db.Where("resource_uid IN ? AND deleted_at IS NULL", resourceUIDs).
				Order("detected_at DESC").Find(&report.Insights).Error; err != nil {
				log.Printf("[GetPodRiskReport] Failed to load insights: %v", err)
			}
		}

		riskLevel := "low"
		if clusterAdminBindings > 0 {
			riskLevel = "critical"
		} else if wildcardRoles > 0 || overprivilegedRoles > 0 {
			riskLevel = "high"
		}
		report.Summary["clusterAdminBindings"] = clusterAdminBindings
		report.Summary["wildcardRoles"] = wildcardRoles
		report.Summary["overprivilegedRoles"] = overprivilegedRoles
		report.Summary["riskLevel"] = riskLevel

		c.JSON(http.StatusOK, report)
	}
}

func buildRoleReport(kind, name, namespace, rulesJSON string) RoleReport {
	report := RoleReport{
		Kind:                kind,
		Name:                name,
		Namespace:           namespace,
		HasWildcard:         false,
		HasSensitiveActions: false,
		RulesCount:          0,
	}

	var rules []map[string]interface{}
	if err := json.Unmarshal([]byte(rulesJSON), &rules); err != nil {
		return report
	}
	report.RulesCount = len(rules)

	for _, rule := range rules {
		if containsWildcard(rule["resources"]) || containsWildcard(rule["verbs"]) {
			report.HasWildcard = true
		}
		if hasSensitiveResources(rule["resources"]) && hasSensitiveVerbs(rule["verbs"]) {
			report.HasSensitiveActions = true
		}
	}
	return report
}

func containsWildcard(val interface{}) bool {
	switch v := val.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && s == "*" {
				return true
			}
		}
	}
	return false
}

func hasSensitiveResources(val interface{}) bool {
	sensitive := map[string]bool{"secrets": true, "configmaps": true, "pods": true, "*": true}
	switch v := val.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && sensitive[strings.ToLower(s)] {
				return true
			}
		}
	}
	return false
}

func hasSensitiveVerbs(val interface{}) bool {
	sensitive := map[string]bool{"*": true, "create": true, "update": true, "delete": true, "patch": true}
	switch v := val.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && sensitive[strings.ToLower(s)] {
				return true
			}
		}
	}
	return false
}
