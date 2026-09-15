package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/riskengine"
	"gorm.io/gorm"
)

// InsightStatusUpdater updates insight status based on current risk evaluation
// This is called after historical risk evaluation to auto-resolve insights
// when risks no longer exist
type InsightStatusUpdater struct {
	initErr    error
	db         *gorm.DB
	riskEngine *riskengine.Engine
	yamlEngine *riskengine.YAMLEngine
	insightMgr *riskengine.InsightManager
}

// NewInsightStatusUpdater creates a new insight status updater.
// When FORTUNA_RULES_DIR is set, uses YAMLEngine.EvaluateResource (same as Risk worker) so Pod/runtime CEL rules re-evaluate correctly.
func NewInsightStatusUpdater(db *gorm.DB) *InsightStatusUpdater {
	ye, err := riskengine.NewConfiguredYAMLEngine(db)
	instance := &InsightStatusUpdater{db: db, yamlEngine: ye, initErr: err, insightMgr: riskengine.NewInsightManager(db)}
	if ye != nil {
		instance.riskEngine = ye.Engine
	}
	return instance
}

func (u *InsightStatusUpdater) evaluateResource(ctx context.Context, resourceType string, resourceData map[string]interface{}) ([]*models.Insight, error) {
	if u.yamlEngine != nil {
		return u.yamlEngine.EvaluateResource(ctx, resourceType, resourceData)
	}
	return u.riskEngine.EvaluateResource(ctx, resourceType, resourceData)
}

// UpdateStatusForResolvedRisks checks all active insights and auto-resolves
// those where the risk no longer exists
func (u *InsightStatusUpdater) UpdateStatusForResolvedRisks(ctx context.Context) error {
	if u.initErr != nil {
		return fmt.Errorf("risk catalog unavailable: %w", u.initErr)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	log.Printf("[InsightStatusUpdater] Starting status update for resolved risks...")
	startTime := time.Now()

	// Get all active insights
	var activeInsights []models.Insight
	if err := u.db.WithContext(ctx).Where("status IN ? OR status IS NULL", []string{"active", "acknowledged"}).Find(&activeInsights).Error; err != nil {
		return err
	}

	log.Printf("[InsightStatusUpdater] Found %d active insights to check", len(activeInsights))

	resolvedCount := 0
	errorCount := 0
	for _, insight := range activeInsights {
		// Use new Insight schema with direct resource fields (no JSONB parsing needed)
		resourceType := insight.ResourceType
		resourceName := insight.ResourceName
		resourceNamespace := insight.ResourceNamespace

		// Check if resource still exists and has the risk
		stillHasRisk, err := u.checkIfRiskStillExists(ctx, resourceType, resourceName, resourceNamespace, &insight)
		if err != nil {
			log.Printf("[InsightStatusUpdater] Error checking risk for insight %d: %v", insight.ID, err)
			errorCount++
			continue
		}

		if !stillHasRisk {
			// Risk no longer exists - auto-resolve
			updates := map[string]interface{}{
				"status":      "resolved",
				"updated_at":  time.Now(),
				"resolved_at": time.Now(),
			}
			changed := false
			err := u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				result := tx.Model(&models.Insight{}).Where("id = ?", insight.ID).
					Where("status IN ? OR status IS NULL", []string{"active", "acknowledged"}).Updates(updates)
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected == 0 {
					return nil
				}
				details, _ := json.Marshal(map[string]any{"reason": "risk no longer matched", "resource_uid": insight.ResourceUID, "previous_status": insight.Status})
				audit := models.AuditLog{Action: "auto_resolve", Resource: "insight", ResourceID: fmt.Sprint(insight.ID), User: "system:risk-reconciliation", Details: string(details)}
				if err := tx.Omit("UserID").Create(&audit).Error; err != nil {
					return err
				}
				changed = true
				return nil
			})
			if err != nil {
				errorCount++
				log.Printf("[InsightStatusUpdater] Failed to persist resolution and audit: %v", err)
				continue
			}
			if !changed {
				continue
			}

			resolvedCount++
			log.Printf("[InsightStatusUpdater] Auto-resolved insight ID=%d (risk no longer exists for %s/%s/%s)",
				insight.ID, resourceType, resourceNamespace, resourceName)
		}
	}

	duration := time.Since(startTime)
	log.Printf("[InsightStatusUpdater] Status update completed in %v: %d insights auto-resolved", duration, resolvedCount)
	if errorCount > 0 {
		return fmt.Errorf("status reconciliation incomplete: %d errors", errorCount)
	}
	return nil
}

// checkIfRiskStillExists checks if the risk described by the insight still exists
// by re-evaluating the resource
func (u *InsightStatusUpdater) checkIfRiskStillExists(ctx context.Context, resourceType, resourceName, resourceNamespace string, insight *models.Insight) (bool, error) {
	// SBOM/CVE pipeline findings are not reproduced by Pod YAML/CEL EvaluateResource.
	// Without this guard, the updater concludes "no matching insight" and incorrectly auto-resolves them.
	switch strings.TrimSpace(insight.InsightType) {
	case "supply_chain_malware", "vulnerability":
		return true, nil
	}

	if strings.TrimSpace(insight.ResourceUID) == "" {
		return true, fmt.Errorf("cannot reconcile insight %d without resource UID", insight.ID)
	}

	var resourceData map[string]interface{}

	switch resourceType {
	case "ServiceAccount":
		var sa models.ServiceAccount
		if err := u.db.WithContext(ctx).Where("uid = ? AND deleted_at IS NULL", insight.ResourceUID).First(&sa).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// Resource doesn't exist - risk is resolved
				return false, nil
			}
			return false, err
		}
		// Build resource data for evaluation
		resourceData = map[string]interface{}{
			"name":       sa.Name,
			"namespace":  sa.Namespace,
			"uid":        sa.UID,
			"cluster_id": sa.ClusterID,
		}
	case "Role":
		var role models.Role
		// Reconcile the exact observed resource. A same-name replacement is a different identity.
		if err := u.db.WithContext(ctx).Where("uid = ? AND deleted_at IS NULL", insight.ResourceUID).
			Order("updated_at DESC").First(&role).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				log.Printf("[InsightStatusUpdater] Role %s/%s not found in database - risk resolved", resourceNamespace, resourceName)
				return false, nil
			}
			return false, err
		}
		log.Printf("[InsightStatusUpdater] Loaded role %s/%s (UID: %s, updated_at: %v) for re-evaluation",
			resourceNamespace, resourceName, role.UID, role.UpdatedAt)
		// Parse rules JSON
		var rules interface{}
		if role.Rules != "" {
			json.Unmarshal([]byte(role.Rules), &rules)
		} else {
			rules = []interface{}{}
		}
		resourceData = map[string]interface{}{
			"name":       role.Name,
			"namespace":  role.Namespace,
			"uid":        role.UID,
			"cluster_id": role.ClusterID,
			"rules":      rules,
		}
	case "ClusterRole":
		var cr models.ClusterRole
		if err := u.db.WithContext(ctx).Where("uid = ? AND deleted_at IS NULL", insight.ResourceUID).First(&cr).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return false, nil
			}
			return false, err
		}
		// Parse rules JSON
		var rules interface{}
		json.Unmarshal([]byte(cr.Rules), &rules)
		resourceData = map[string]interface{}{
			"name":       cr.Name,
			"uid":        cr.UID,
			"cluster_id": cr.ClusterID,
			"rules":      rules,
		}
	case "RoleBinding":
		var rb models.RoleBinding
		if err := u.db.WithContext(ctx).Where("uid = ? AND deleted_at IS NULL", insight.ResourceUID).First(&rb).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return false, nil
			}
			return true, err
		}
		var roleRef interface{}
		_ = json.Unmarshal([]byte(rb.RoleRef), &roleRef)
		var subjects interface{}
		_ = json.Unmarshal([]byte(rb.Subjects), &subjects)
		resourceData = map[string]interface{}{
			"kind":       "RoleBinding",
			"name":       rb.Name,
			"namespace":  rb.Namespace,
			"uid":        rb.UID,
			"cluster_id": rb.ClusterID,
			"roleRef":    roleRef,
			"subjects":   subjects,
		}
	case "ClusterRoleBinding":
		var crb models.ClusterRoleBinding
		if err := u.db.WithContext(ctx).Where("uid = ? AND deleted_at IS NULL", insight.ResourceUID).First(&crb).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return false, nil
			}
			return true, err
		}
		var roleRef interface{}
		_ = json.Unmarshal([]byte(crb.RoleRef), &roleRef)
		var subjects interface{}
		_ = json.Unmarshal([]byte(crb.Subjects), &subjects)
		resourceData = map[string]interface{}{
			"kind":       "ClusterRoleBinding",
			"name":       crb.Name,
			"namespace":  "",
			"uid":        crb.UID,
			"cluster_id": crb.ClusterID,
			"roleRef":    roleRef,
			"subjects":   subjects,
		}
	case "Pod":
		var pod models.Pod
		if err := u.db.WithContext(ctx).Where("uid = ? AND deleted_at IS NULL", insight.ResourceUID).First(&pod).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				log.Printf("[InsightStatusUpdater] Pod with uid=%s not found in DB - risk resolved (orphan insight)", insight.ResourceUID)
				return false, nil
			}
			return true, err
		}
		var containers interface{}
		if pod.Containers != "" {
			if err := json.Unmarshal([]byte(pod.Containers), &containers); err != nil {
				return true, fmt.Errorf("invalid synchronized pod containers: %w", err)
			}
		} else {
			containers = []interface{}{}
		}
		k8sPod := map[string]interface{}{
			"kind":       "Pod",
			"apiVersion": "v1",
			"metadata": map[string]interface{}{
				"name": pod.Name, "namespace": pod.Namespace, "uid": pod.UID,
			},
			"spec": map[string]interface{}{
				"serviceAccountName": pod.ServiceAccount,
				"hostNetwork":        pod.HostNetwork,
				"hostPID":            pod.HostPID,
				"hostIPC":            pod.HostIPC,
				"containers":         containers,
			},
		}
		rawBytes, err := json.Marshal(k8sPod)
		if err != nil {
			return true, err
		}
		resourceData = map[string]interface{}{
			"kind":       "Pod",
			"uid":        pod.UID,
			"name":       pod.Name,
			"namespace":  pod.Namespace,
			"cluster_id": pod.ClusterID,
			"raw_json":   string(rawBytes),
		}
	default:
		// Unknown resource type - assume risk still exists
		return true, nil
	}

	if resourceData == nil {
		return true, nil
	}

	// Re-evaluate the resource
	if u.yamlEngine == nil {
		return true, fmt.Errorf("YAML evaluator unavailable")
	}
	if !u.yamlEngine.CanReevaluateFinding(resourceType, insight.CVEID, insight.Title, insight.InsightType) {
		return true, fmt.Errorf("original finding detector is missing, disabled, or no longer applies")
	}

	insights, err := u.evaluateResource(ctx, resourceType, resourceData)
	if err != nil {
		log.Printf("[InsightStatusUpdater] Error re-evaluating resource %s/%s/%s: %v", resourceType, resourceNamespace, resourceName, err)
		return true, err // On error, assume risk still exists (safer)
	}

	// Check if any of the new insights match this insight's description or type
	// Use flexible matching: exact description match OR same type + severity for wildcard insights
	insightType := insight.InsightType
	insightSeverity := insight.Severity
	insightDescLower := strings.ToLower(insight.Description)

	for _, newInsight := range insights {
		// Exact description match - risk still exists
		if newInsight.Description == insight.Description {
			log.Printf("[InsightStatusUpdater] Risk still exists: exact description match for insight %d", insight.ID)
			return true, nil
		}
		// Stable match for YAML-driven insights (Pod runtime, PSS, cluster-admin, etc.)
		if strings.TrimSpace(insight.Title) != "" &&
			newInsight.Title == insight.Title &&
			newInsight.InsightType == insight.InsightType {
			log.Printf("[InsightStatusUpdater] Risk still exists: title+insightType match for insight %d", insight.ID)
			return true, nil
		}

		// For wildcard/overprivileged insights, check if same type and severity
		// This handles cases where description might vary slightly but risk is the same
		// Also check if both descriptions mention the same resource name/namespace
		newInsightDescLower := strings.ToLower(newInsight.Description)
		if (insightType == "rbac" && newInsight.InsightType == "rbac") &&
			(insightSeverity == newInsight.Severity) &&
			(contains(insightDescLower, "wildcard") || contains(insightDescLower, "overprivileged")) &&
			(contains(newInsightDescLower, "wildcard") || contains(newInsightDescLower, "overprivileged")) {
			// Additional check: if insight mentions specific resource name, ensure new insight mentions it too
			// Extract resource name from description if present
			resourceNameInDesc := extractResourceNameFromDescription(insight.Description)
			if resourceNameInDesc != "" {
				if contains(newInsight.Description, resourceNameInDesc) {
					log.Printf("[InsightStatusUpdater] Risk still exists: similar wildcard/overprivileged insight found for same resource (insight %d)", insight.ID)
					return true, nil
				}
			} else {
				// No specific resource name - match by type and severity
				log.Printf("[InsightStatusUpdater] Risk still exists: similar wildcard/overprivileged insight found (insight %d)", insight.ID)
				return true, nil
			}
		}
	}

	// No matching insight found - risk is resolved
	log.Printf("[InsightStatusUpdater] Risk resolved: no matching insights found for insight %d (was: %s)", insight.ID, insight.Description)
	return false, nil
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// extractResourceNameFromDescription extracts resource name from insight description
// Format: "Role with wildcard permissions: Role or ClusterRole contains wildcard (*) permissions in namespace default"
// Or: "Role with wildcard permissions: Role or ClusterRole contains wildcard (*) permissions"
func extractResourceNameFromDescription(description string) string {
	// Try to extract from "in namespace X" pattern
	parts := strings.Split(description, "in namespace")
	if len(parts) > 1 {
		// Could extract namespace, but for now just return empty
		// Resource name is usually in affectedResources, not description
	}
	return ""
}

// countRules counts the number of rules in the rules interface
func countRules(rules interface{}) int {
	if rules == nil {
		return 0
	}
	if rulesArr, ok := rules.([]interface{}); ok {
		return len(rulesArr)
	}
	return 0
}
