package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/rbacinventory"
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
	PodUID            string                 `json:"podUid"`
	PodName           string                 `json:"podName"`
	Namespace         string                 `json:"namespace"`
	ClusterID         string                 `json:"clusterId"`
	ServiceAccount    string                 `json:"serviceAccount"`
	ServiceAccountUID string                 `json:"serviceAccountUid"`
	Bindings          []BindingReport        `json:"bindings"`
	Roles             []RoleReport           `json:"roles"`
	Insights          []models.Insight       `json:"insights"`
	Summary           map[string]interface{} `json:"summary"`
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

		grants, err := rbacinventory.Resolve(db.WithContext(c.Request.Context()), &models.ServiceAccount{ClusterID: pod.ClusterID, Namespace: pod.Namespace, Name: pod.ServiceAccount})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to resolve synchronized RBAC inventory"})
			return
		}
		resolvedRoles := make([]RoleReport, 0)
		resourceUIDs := []string{pod.UID}
		if report.ServiceAccountUID != "" {
			resourceUIDs = append(resourceUIDs, report.ServiceAccountUID)
		}
		for _, grant := range grants.RoleBindings {
			rb := grant.RoleBinding
			kind, name := "Role", ""
			if grant.Role != nil {
				name = grant.Role.Name
			} else {
				kind, name = "ClusterRole", grant.ClusterRole.Name
			}
			var subjects []SubjectReport
			if err := json.Unmarshal([]byte(rb.Subjects), &subjects); err != nil {
				c.JSON(500, gin.H{"error": "Invalid binding subjects"})
				return
			}
			report.Bindings = append(report.Bindings, BindingReport{Kind: "RoleBinding", Name: rb.Name, Namespace: rb.Namespace, RoleRefKind: kind, RoleRefName: name, Subjects: subjects, IsClusterAdmin: false})
			if grant.Role != nil {
				resourceUIDs = append(resourceUIDs, grant.Role.UID)
				resolvedRoles = append(resolvedRoles, buildRoleReport("Role", grant.Role.Name, rb.Namespace, grant.Role.Rules))
			} else {
				resourceUIDs = append(resourceUIDs, grant.ClusterRole.UID)
				resolvedRoles = append(resolvedRoles, buildRoleReport("ClusterRole", grant.ClusterRole.Name, "", grant.ClusterRole.Rules))
			}
			resourceUIDs = append(resourceUIDs, rb.UID)
		}
		for _, grant := range grants.ClusterRoleBindings {
			crb := grant.ClusterRoleBinding
			var subjects []SubjectReport
			if err := json.Unmarshal([]byte(crb.Subjects), &subjects); err != nil {
				c.JSON(500, gin.H{"error": "Invalid binding subjects"})
				return
			}
			report.Bindings = append(report.Bindings, BindingReport{Kind: "ClusterRoleBinding", Name: crb.Name, RoleRefKind: "ClusterRole", RoleRefName: grant.ClusterRole.Name, Subjects: subjects, IsClusterAdmin: grant.ClusterRole.Name == "cluster-admin"})
			resourceUIDs = append(resourceUIDs, grant.ClusterRole.UID)
			resolvedRoles = append(resolvedRoles, buildRoleReport("ClusterRole", grant.ClusterRole.Name, "", grant.ClusterRole.Rules))
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

		for _, role := range resolvedRoles {
			if role.HasWildcard {
				wildcardRoles++
			}
			if role.HasSensitiveActions {
				overprivilegedRoles++
			}
			report.Roles = append(report.Roles, role)
		}

		if len(resourceUIDs) > 0 {
			if err := db.Where("resource_uid IN ? AND deleted_at IS NULL", resourceUIDs).
				Order("detected_at DESC").Find(&report.Insights).Error; err != nil {
				log.Printf("[GetPodRiskReport] Failed to load insights: %v", err)
			}
		}

		// Align with risk engine / UI: 24h runtime signal window (same default as FORTUNA_RUNTIME_RISK_LOOKBACK_HOURS).
		signalsSince := time.Now().Add(-24 * time.Hour)
		var runtimeSignals24h int64
		_ = db.Model(&models.RuntimeSignal{}).
			Where("pod_uid = ? AND created_at >= ?", pod.UID, signalsSince).
			Count(&runtimeSignals24h).Error

		podDirectInsightCount := 0
		runtimePolicyInsightCount := 0
		for _, ins := range report.Insights {
			if ins.ResourceUID == pod.UID {
				podDirectInsightCount++
				switch ins.InsightType {
				case "runtime-behavior", "pod-security":
					runtimePolicyInsightCount++
				}
			}
		}
		report.Summary["runtimeSignals24h"] = runtimeSignals24h
		report.Summary["podDirectInsightCount"] = podDirectInsightCount
		report.Summary["runtimePolicyInsightCount"] = runtimePolicyInsightCount
		report.Summary["insightsInReport"] = len(report.Insights)

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
