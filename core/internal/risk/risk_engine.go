package risk

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/ksam/core/pkg/models"
)

// RiskEngine generates risk insights
type RiskEngine struct {
	db *gorm.DB
}

// NewRiskEngine creates a new risk engine
func NewRiskEngine(db *gorm.DB) *RiskEngine {
	return &RiskEngine{db: db}
}

// EvaluateRBACRisks evaluates RBAC-related risks
func (r *RiskEngine) EvaluateRBACRisks() error {
	log.Println("[RiskEngine] Evaluating RBAC risks...")
	
	// Clear existing insights for fresh evaluation
	// In production, you might want to keep historical insights
	// r.db.Where("type LIKE ?", "RBAC_%").Delete(&models.Insight{})
	
	// Evaluate all risk types
	if err := r.detectClusterAdminBindings(); err != nil {
		log.Printf("[RiskEngine] Error detecting cluster-admin bindings: %v", err)
	}
	
	if err := r.detectWildcardPermissions(); err != nil {
		log.Printf("[RiskEngine] Error detecting wildcard permissions: %v", err)
	}
	
	if err := r.detectOrphanServiceAccounts(); err != nil {
		log.Printf("[RiskEngine] Error detecting orphan service accounts: %v", err)
	}
	
	if err := r.detectOverprivilegedRoles(); err != nil {
		log.Printf("[RiskEngine] Error detecting overprivileged roles: %v", err)
	}
	
	if err := r.detectOverprivilegedBindings(); err != nil {
		log.Printf("[RiskEngine] Error detecting overprivileged bindings: %v", err)
	}
	
	log.Printf("[RiskEngine] RBAC risk evaluation completed")
	return nil
}

// detectClusterAdminBindings detects ServiceAccounts bound to cluster-admin role
func (r *RiskEngine) detectClusterAdminBindings() error {
	var clusterRoleBindings []models.ClusterRoleBinding
	
	// Find all ClusterRoleBindings
	if err := r.db.Find(&clusterRoleBindings).Error; err != nil {
		return err
	}
	
	for _, crb := range clusterRoleBindings {
		// Parse roleRef JSON
		var roleRef map[string]interface{}
		if crb.RoleRef == "" {
			continue
		}
		if err := json.Unmarshal([]byte(crb.RoleRef), &roleRef); err != nil {
			log.Printf("[RiskEngine] Failed to parse roleRef for CRB %s: %v", crb.Name, err)
			continue
		}
		
		roleName, ok := roleRef["name"].(string)
		if !ok || roleName != "cluster-admin" {
			continue
		}
		
		// Parse subjects
		if crb.Subjects == "" {
			continue
		}
		var subjects []map[string]interface{}
		if err := json.Unmarshal([]byte(crb.Subjects), &subjects); err != nil {
			log.Printf("[RiskEngine] Failed to parse subjects for CRB %s: %v", crb.Name, err)
			continue
		}
		
		for _, subject := range subjects {
			kind, _ := subject["kind"].(string)
			name, _ := subject["name"].(string)
			
			if kind == "ServiceAccount" {
				affectedResources := []string{
					fmt.Sprintf("ServiceAccount:%s/%s", crb.ClusterID, name),
					fmt.Sprintf("ClusterRoleBinding:%s/%s", crb.ClusterID, crb.Name),
				}
				
				description := fmt.Sprintf(
					"ServiceAccount '%s' is bound to cluster-admin ClusterRole via ClusterRoleBinding '%s'. This grants full cluster access.",
					name, crb.Name,
				)
				
				if err := r.CreateInsight(
					"RBAC_CLUSTER_ADMIN",
					description,
					"Critical",
					affectedResources,
				); err != nil {
					log.Printf("[RiskEngine] Failed to create insight: %v", err)
				}
			}
		}
	}
	
	return nil
}

// detectWildcardPermissions detects roles with wildcard permissions (*)
func (r *RiskEngine) detectWildcardPermissions() error {
	// Check ClusterRoles
	var clusterRoles []models.ClusterRole
	if err := r.db.Find(&clusterRoles).Error; err != nil {
		return err
	}
	
	for _, cr := range clusterRoles {
		var rules []map[string]interface{}
		if err := json.Unmarshal([]byte(cr.Rules), &rules); err != nil {
			continue
		}
		
		hasWildcard := false
		for _, rule := range rules {
			// Check for wildcard in resources
			if resources, ok := rule["resources"].([]interface{}); ok {
				for _, res := range resources {
					if resStr, ok := res.(string); ok && resStr == "*" {
						hasWildcard = true
						break
					}
				}
			}
			
			// Check for wildcard in verbs
			if verbs, ok := rule["verbs"].([]interface{}); ok {
				for _, verb := range verbs {
					if verbStr, ok := verb.(string); ok && verbStr == "*" {
						hasWildcard = true
						break
					}
				}
			}
			
			// Check for wildcard in apiGroups
			if apiGroups, ok := rule["apiGroups"].([]interface{}); ok {
				for _, ag := range apiGroups {
					if agStr, ok := ag.(string); ok && agStr == "*" {
						hasWildcard = true
						break
					}
				}
			}
		}
		
		if hasWildcard {
			affectedResources := []string{
				fmt.Sprintf("ClusterRole:%s/%s", cr.ClusterID, cr.Name),
			}
			
			description := fmt.Sprintf(
				"ClusterRole '%s' contains wildcard permissions (*) which grants overly broad access.",
				cr.Name,
			)
			
			severity := "High"
			if cr.Name == "cluster-admin" || strings.Contains(cr.Name, "admin") {
				severity = "Critical"
			}
			
			if err := r.CreateInsight(
				"RBAC_WILDCARD_PERMISSIONS",
				description,
				severity,
				affectedResources,
			); err != nil {
				log.Printf("[RiskEngine] Failed to create insight: %v", err)
			}
		}
	}
	
	// Check Roles
	var roles []models.Role
	if err := r.db.Find(&roles).Error; err != nil {
		return err
	}
	
	for _, role := range roles {
		if role.Rules == "" {
			continue
		}
		var rules []map[string]interface{}
		if err := json.Unmarshal([]byte(role.Rules), &rules); err != nil {
			log.Printf("[RiskEngine] Failed to parse rules for Role %s/%s: %v", role.Namespace, role.Name, err)
			continue
		}
		
		hasWildcard := false
		for _, rule := range rules {
			if resources, ok := rule["resources"].([]interface{}); ok {
				for _, res := range resources {
					if resStr, ok := res.(string); ok && resStr == "*" {
						hasWildcard = true
						break
					}
				}
			}
			if verbs, ok := rule["verbs"].([]interface{}); ok {
				for _, verb := range verbs {
					if verbStr, ok := verb.(string); ok && verbStr == "*" {
						hasWildcard = true
						break
					}
				}
			}
		}
		
		if hasWildcard {
			affectedResources := []string{
				fmt.Sprintf("Role:%s/%s/%s", role.ClusterID, role.Namespace, role.Name),
			}
			
			description := fmt.Sprintf(
				"Role '%s/%s' contains wildcard permissions (*) which grants overly broad access.",
				role.Namespace, role.Name,
			)
			
			if err := r.CreateInsight(
				"RBAC_WILDCARD_PERMISSIONS",
				description,
				"High",
				affectedResources,
			); err != nil {
				log.Printf("[RiskEngine] Failed to create insight: %v", err)
			}
		}
	}
	
	return nil
}

// detectOrphanServiceAccounts detects ServiceAccounts not used by any pods
func (r *RiskEngine) detectOrphanServiceAccounts() error {
	var serviceAccounts []models.ServiceAccount
	if err := r.db.Find(&serviceAccounts).Error; err != nil {
		return err
	}
	
	for _, sa := range serviceAccounts {
		// Check if ServiceAccount has linked pods
		var linkedPods []string
		if sa.LinkedPods != "" {
			if err := json.Unmarshal([]byte(sa.LinkedPods), &linkedPods); err != nil {
				// If parsing fails, check pods table directly
			}
		}
		
		// Also check pods table directly
		var podCount int64
		r.db.Model(&models.Pod{}).Where(
			"cluster_id = ? AND namespace = ? AND service_account = ?",
			sa.ClusterID, sa.Namespace, sa.Name,
		).Count(&podCount)
		
		// Check last_used timestamp
		isOrphan := false
		if (len(linkedPods) == 0 && podCount == 0) || (sa.LastUsed == nil) {
			// ServiceAccount has never been used
			isOrphan = true
		} else if sa.LastUsed != nil {
			// ServiceAccount hasn't been used in 90 days
			daysSinceLastUse := time.Since(*sa.LastUsed).Hours() / 24
			if daysSinceLastUse > 90 {
				isOrphan = true
			}
		}
		
		// Skip default service accounts
		if sa.Name == "default" {
			continue
		}
		
		if isOrphan {
			affectedResources := []string{
				fmt.Sprintf("ServiceAccount:%s/%s/%s", sa.ClusterID, sa.Namespace, sa.Name),
			}
			
			description := fmt.Sprintf(
				"ServiceAccount '%s/%s' appears to be unused (no pods linked, or not used in 90+ days). Consider removing if no longer needed.",
				sa.Namespace, sa.Name,
			)
			
			if err := r.CreateInsight(
				"RBAC_ORPHAN_SERVICE_ACCOUNT",
				description,
				"Low",
				affectedResources,
			); err != nil {
				log.Printf("[RiskEngine] Failed to create insight: %v", err)
			}
		}
	}
	
	return nil
}

// detectOverprivilegedRoles detects roles with excessive permissions
func (r *RiskEngine) detectOverprivilegedRoles() error {
	// Check ClusterRoles for excessive permissions
	var clusterRoles []models.ClusterRole
	if err := r.db.Find(&clusterRoles).Error; err != nil {
		return err
	}
	
	for _, cr := range clusterRoles {
		var rules []map[string]interface{}
		if err := json.Unmarshal([]byte(cr.Rules), &rules); err != nil {
			continue
		}
		
		// Count permissions
		totalResources := 0
		hasWritePermissions := false
		hasSecretsAccess := false
		
		for _, rule := range rules {
			if resources, ok := rule["resources"].([]interface{}); ok {
				totalResources += len(resources)
				for _, res := range resources {
					if resStr, ok := res.(string); ok && resStr == "secrets" {
						hasSecretsAccess = true
					}
				}
			}
			
			if verbs, ok := rule["verbs"].([]interface{}); ok {
				for _, verb := range verbs {
					if verbStr, ok := verb.(string); ok {
						if verbStr == "create" || verbStr == "update" || verbStr == "patch" || verbStr == "delete" {
							hasWritePermissions = true
						}
					}
				}
			}
		}
		
		// Flag if role has too many resources or dangerous combinations
		if totalResources > 20 || (hasWritePermissions && hasSecretsAccess && totalResources > 10) {
			affectedResources := []string{
				fmt.Sprintf("ClusterRole:%s/%s", cr.ClusterID, cr.Name),
			}
			
			description := fmt.Sprintf(
				"ClusterRole '%s' has excessive permissions (%d resources). Consider applying least-privilege principle.",
				cr.Name, totalResources,
			)
			
			severity := "Medium"
			if hasWritePermissions && hasSecretsAccess {
				severity = "High"
			}
			
			if err := r.CreateInsight(
				"RBAC_OVERPRIVILEGED_ROLE",
				description,
				severity,
				affectedResources,
			); err != nil {
				log.Printf("[RiskEngine] Failed to create insight: %v", err)
			}
		}
	}
	
	return nil
}

// detectOverprivilegedBindings detects bindings that grant excessive permissions
func (r *RiskEngine) detectOverprivilegedBindings() error {
	// Check ClusterRoleBindings
	var clusterRoleBindings []models.ClusterRoleBinding
	if err := r.db.Find(&clusterRoleBindings).Error; err != nil {
		return err
	}
	
	for _, crb := range clusterRoleBindings {
		if crb.RoleRef == "" {
			continue
		}
		var roleRef map[string]interface{}
		if err := json.Unmarshal([]byte(crb.RoleRef), &roleRef); err != nil {
			log.Printf("[RiskEngine] Failed to parse roleRef for CRB %s: %v", crb.Name, err)
			continue
		}
		
		roleName, ok := roleRef["name"].(string)
		if !ok {
			continue
		}
		
		// Check if role has excessive permissions
		var clusterRole models.ClusterRole
		if err := r.db.Where("cluster_id = ? AND name = ?", crb.ClusterID, roleName).First(&clusterRole).Error; err != nil {
			continue
		}
		
		if clusterRole.Rules == "" {
			continue
		}
		var rules []map[string]interface{}
		if err := json.Unmarshal([]byte(clusterRole.Rules), &rules); err != nil {
			log.Printf("[RiskEngine] Failed to parse rules for ClusterRole %s: %v", roleName, err)
			continue
		}
		
		// Count permissions
		totalResources := 0
		for _, rule := range rules {
			if resources, ok := rule["resources"].([]interface{}); ok {
				totalResources += len(resources)
			}
		}
		
		// Flag if binding grants excessive permissions
		if totalResources > 15 {
			if crb.Subjects == "" {
				continue
			}
			var subjects []map[string]interface{}
			if err := json.Unmarshal([]byte(crb.Subjects), &subjects); err != nil {
				log.Printf("[RiskEngine] Failed to parse subjects for CRB %s: %v", crb.Name, err)
				continue
			}
			
			for _, subject := range subjects {
				kind, _ := subject["kind"].(string)
				name, _ := subject["name"].(string)
				
				if kind == "ServiceAccount" {
					affectedResources := []string{
						fmt.Sprintf("ServiceAccount:%s/%s", crb.ClusterID, name),
						fmt.Sprintf("ClusterRoleBinding:%s/%s", crb.ClusterID, crb.Name),
						fmt.Sprintf("ClusterRole:%s/%s", crb.ClusterID, roleName),
					}
					
					description := fmt.Sprintf(
						"ServiceAccount '%s' is bound to ClusterRole '%s' via ClusterRoleBinding '%s' which grants access to %d+ resources. Consider using more restrictive roles.",
						name, roleName, crb.Name, totalResources,
					)
					
					if err := r.CreateInsight(
						"RBAC_OVERPRIVILEGED_BINDING",
						description,
						"Medium",
						affectedResources,
					); err != nil {
						log.Printf("[RiskEngine] Failed to create insight: %v", err)
					}
				}
			}
		}
	}
	
	return nil
}

// CreateInsight creates a new insight (prevents duplicates)
func (r *RiskEngine) CreateInsight(insightType, description, severity string, affectedResources []string) error {
	// Check if insight already exists
	var existing models.Insight
	resourcesJSON, _ := json.Marshal(affectedResources)
	
	result := r.db.Where("type = ? AND description = ?", insightType, description).First(&existing)
	if result.Error == nil {
		// Insight already exists, update it
		existing.Severity = severity
		existing.AffectedResources = string(resourcesJSON)
		existing.UpdatedAt = time.Now()
		return r.db.Save(&existing).Error
	}
	
	// Create new insight
	insight := models.Insight{
		Type:              insightType,
		Description:       description,
		AffectedResources: string(resourcesJSON),
		Severity:          severity,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	
	return r.db.Create(&insight).Error
}

// EvaluateRBACRisksForServiceAccount evaluates risks for a specific ServiceAccount
func (r *RiskEngine) EvaluateRBACRisksForServiceAccount(clusterID, namespace, saName string) error {
	// This can be called after correlating a specific ServiceAccount
	// For now, we'll just trigger full evaluation
	// In the future, we can optimize to only evaluate for this SA
	return r.EvaluateRBACRisks()
}

