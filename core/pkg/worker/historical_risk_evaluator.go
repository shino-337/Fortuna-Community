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

// HistoricalRiskEvaluator processes historical data from database
type HistoricalRiskEvaluator struct {
	initErr    error
	db         *gorm.DB
	riskEngine *riskengine.Engine
	yamlEngine *riskengine.YAMLEngine
	insightMgr *riskengine.InsightManager
}

func historicalPolicyRules(raw string) ([]interface{}, error) {
	var rules []interface{}
	if err := json.Unmarshal([]byte(raw), &rules); err != nil || rules == nil {
		return nil, fmt.Errorf("invalid policy rules JSON")
	}
	for _, item := range rules {
		rule, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid policy rule object")
		}
		if _, ok := rule["resources"]; !ok {
			// Kubernetes non-resource URL rules legitimately omit resources.
			// The empty list is only a CEL read shape, not stored evidence.
			if _, urlRule := rule["nonResourceURLs"]; !urlRule {
				return nil, fmt.Errorf("policy rule has no resource targets")
			}
			rule["resources"] = []interface{}{}
		}
	}
	return rules, nil
}

func historicalRoleRef(raw string) (map[string]interface{}, error) {
	var ref map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &ref); err != nil || ref == nil {
		return nil, fmt.Errorf("invalid roleRef JSON")
	}
	kind, _ := ref["kind"].(string)
	if strings.TrimSpace(kind) == "" {
		return nil, fmt.Errorf("roleRef kind is required")
	}
	return ref, nil
}

func historicalSubjects(raw string) ([]interface{}, error) {
	if strings.TrimSpace(raw) == "" {
		return []interface{}{}, nil
	}
	var subjects []interface{}
	if err := json.Unmarshal([]byte(raw), &subjects); err != nil || subjects == nil {
		return nil, fmt.Errorf("invalid binding subjects JSON")
	}
	return subjects, nil
}

// NewHistoricalRiskEvaluator creates a new historical risk evaluator
func NewHistoricalRiskEvaluator(db *gorm.DB) *HistoricalRiskEvaluator {
	ye, err := riskengine.NewConfiguredYAMLEngine(db)
	instance := &HistoricalRiskEvaluator{db: db, yamlEngine: ye, initErr: err, insightMgr: riskengine.NewInsightManager(db)}
	if ye != nil {
		instance.riskEngine = ye.Engine
	}
	return instance
}

func (e *HistoricalRiskEvaluator) evaluateResource(ctx context.Context, resourceType string, resourceData map[string]interface{}) ([]*models.Insight, error) {
	if e.yamlEngine != nil {
		return e.yamlEngine.EvaluateResource(ctx, resourceType, resourceData)
	}
	return e.riskEngine.EvaluateResource(ctx, resourceType, resourceData)
}

// EvaluateAllResources evaluates all existing resources in the database
func (e *HistoricalRiskEvaluator) EvaluateAllResources(ctx context.Context) error {
	if e.initErr != nil {
		return fmt.Errorf("risk catalog unavailable: %w", e.initErr)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	log.Printf("[HistoricalRiskEvaluator] Starting evaluation of all historical resources...")
	startTime := time.Now()

	// Track statistics
	stats := struct {
		ServiceAccounts     int
		Roles               int
		ClusterRoles        int
		RoleBindings        int
		ClusterRoleBindings int
		InsightsCreated     int
		Errors              int
	}{}

	// Evaluate ServiceAccounts
	if err := e.evaluateServiceAccounts(ctx, &stats); err != nil {
		log.Printf("[HistoricalRiskEvaluator] Error evaluating ServiceAccounts: %v", err)
		stats.Errors++
	}

	// Evaluate Roles
	if err := e.evaluateRoles(ctx, &stats); err != nil {
		log.Printf("[HistoricalRiskEvaluator] Error evaluating Roles: %v", err)
		stats.Errors++
	}

	// Evaluate ClusterRoles
	if err := e.evaluateClusterRoles(ctx, &stats); err != nil {
		log.Printf("[HistoricalRiskEvaluator] Error evaluating ClusterRoles: %v", err)
		stats.Errors++
	}

	// Evaluate RoleBindings
	if err := e.evaluateRoleBindings(ctx, &stats); err != nil {
		log.Printf("[HistoricalRiskEvaluator] Error evaluating RoleBindings: %v", err)
		stats.Errors++
	}

	// Evaluate ClusterRoleBindings
	if err := e.evaluateClusterRoleBindings(ctx, &stats); err != nil {
		log.Printf("[HistoricalRiskEvaluator] Error evaluating ClusterRoleBindings: %v", err)
		stats.Errors++
	}

	duration := time.Since(startTime)
	log.Printf("[HistoricalRiskEvaluator] Evaluation completed in %v", duration)
	log.Printf("[HistoricalRiskEvaluator] Statistics: SA=%d, Roles=%d, CR=%d, RB=%d, CRB=%d, Insights=%d, Errors=%d",
		stats.ServiceAccounts, stats.Roles, stats.ClusterRoles, stats.RoleBindings, stats.ClusterRoleBindings,
		stats.InsightsCreated, stats.Errors)

	if stats.Errors > 0 {
		return fmt.Errorf("historical evaluation incomplete: %d errors", stats.Errors)
	}

	return nil
}

// evaluateServiceAccounts evaluates all ServiceAccounts
func (e *HistoricalRiskEvaluator) evaluateServiceAccounts(ctx context.Context, stats *struct {
	ServiceAccounts     int
	Roles               int
	ClusterRoles        int
	RoleBindings        int
	ClusterRoleBindings int
	InsightsCreated     int
	Errors              int
}) error {
	var serviceAccounts []models.ServiceAccount
	if err := e.db.WithContext(ctx).Find(&serviceAccounts).Error; err != nil {
		return fmt.Errorf("failed to fetch service accounts: %w", err)
	}

	stats.ServiceAccounts = len(serviceAccounts)

	for _, sa := range serviceAccounts {
		// Convert to normalized format
		normalizedData := map[string]interface{}{
			"kind":       "ServiceAccount",
			"name":       sa.Name,
			"namespace":  sa.Namespace,
			"cluster_id": sa.ClusterID,
			"uid":        sa.UID,
			"labels":     sa.Labels,
			"secrets":    sa.Secrets,
			"linkedPods": sa.LinkedPods,
		}

		insights, err := e.evaluateResource(ctx, "ServiceAccount", normalizedData)
		if err != nil {
			log.Printf("[HistoricalRiskEvaluator] Error evaluating ServiceAccount %s/%s: %v", sa.Namespace, sa.Name, err)
			stats.Errors++
			continue
		}

		for _, insight := range insights {
			if err := e.insightMgr.CreateOrUpdateInsight(insight); err != nil {
				log.Printf("[HistoricalRiskEvaluator] Failed to create insight for SA %s/%s: %v", sa.Namespace, sa.Name, err)
				stats.Errors++
				continue
			}
			stats.InsightsCreated++
		}
	}

	return nil
}

// evaluateRoles evaluates all Roles
func (e *HistoricalRiskEvaluator) evaluateRoles(ctx context.Context, stats *struct {
	ServiceAccounts     int
	Roles               int
	ClusterRoles        int
	RoleBindings        int
	ClusterRoleBindings int
	InsightsCreated     int
	Errors              int
}) error {
	var roles []models.Role
	if err := e.db.WithContext(ctx).Find(&roles).Error; err != nil {
		return fmt.Errorf("failed to fetch roles: %w", err)
	}

	stats.Roles = len(roles)

	for _, role := range roles {
		rules, err := historicalPolicyRules(role.Rules)
		if err != nil {
			log.Printf("[HistoricalRiskEvaluator] Invalid Role %s/%s rules: %v", role.Namespace, role.Name, err)
			stats.Errors++
			continue
		}

		normalizedData := map[string]interface{}{
			"kind":       "Role",
			"name":       role.Name,
			"namespace":  role.Namespace,
			"cluster_id": role.ClusterID,
			"uid":        role.UID,
			"rules":      rules,
		}

		insights, err := e.evaluateResource(ctx, "Role", normalizedData)
		if err != nil {
			log.Printf("[HistoricalRiskEvaluator] Error evaluating Role %s/%s: %v", role.Namespace, role.Name, err)
			stats.Errors++
			continue
		}

		for _, insight := range insights {
			if err := e.insightMgr.CreateOrUpdateInsight(insight); err != nil {
				log.Printf("[HistoricalRiskEvaluator] Failed to create insight for Role %s/%s: %v", role.Namespace, role.Name, err)
				stats.Errors++
				continue
			}
			stats.InsightsCreated++
		}
	}

	return nil
}

// evaluateClusterRoles evaluates all ClusterRoles
func (e *HistoricalRiskEvaluator) evaluateClusterRoles(ctx context.Context, stats *struct {
	ServiceAccounts     int
	Roles               int
	ClusterRoles        int
	RoleBindings        int
	ClusterRoleBindings int
	InsightsCreated     int
	Errors              int
}) error {
	var clusterRoles []models.ClusterRole
	if err := e.db.WithContext(ctx).Find(&clusterRoles).Error; err != nil {
		return fmt.Errorf("failed to fetch cluster roles: %w", err)
	}

	stats.ClusterRoles = len(clusterRoles)

	for _, cr := range clusterRoles {
		rules, err := historicalPolicyRules(cr.Rules)
		if err != nil {
			log.Printf("[HistoricalRiskEvaluator] Invalid ClusterRole %s rules: %v", cr.Name, err)
			stats.Errors++
			continue
		}
		normalizedData := map[string]interface{}{
			"kind":       "ClusterRole",
			"name":       cr.Name,
			"namespace":  "", // ClusterRole has no namespace
			"cluster_id": cr.ClusterID,
			"uid":        cr.UID,
			"rules":      rules,
		}

		insights, err := e.evaluateResource(ctx, "ClusterRole", normalizedData)
		if err != nil {
			log.Printf("[HistoricalRiskEvaluator] Error evaluating ClusterRole %s: %v", cr.Name, err)
			stats.Errors++
			continue
		}

		for _, insight := range insights {
			if err := e.insightMgr.CreateOrUpdateInsight(insight); err != nil {
				log.Printf("[HistoricalRiskEvaluator] Failed to create insight for ClusterRole %s: %v", cr.Name, err)
				stats.Errors++
				continue
			}
			stats.InsightsCreated++
		}
	}

	return nil
}

// evaluateRoleBindings evaluates all RoleBindings
func (e *HistoricalRiskEvaluator) evaluateRoleBindings(ctx context.Context, stats *struct {
	ServiceAccounts     int
	Roles               int
	ClusterRoles        int
	RoleBindings        int
	ClusterRoleBindings int
	InsightsCreated     int
	Errors              int
}) error {
	var roleBindings []models.RoleBinding
	if err := e.db.WithContext(ctx).Find(&roleBindings).Error; err != nil {
		return fmt.Errorf("failed to fetch role bindings: %w", err)
	}

	stats.RoleBindings = len(roleBindings)

	for _, rb := range roleBindings {
		roleRef, err := historicalRoleRef(rb.RoleRef)
		if err != nil {
			log.Printf("[HistoricalRiskEvaluator] Invalid RoleBinding %s/%s roleRef: %v", rb.Namespace, rb.Name, err)
			stats.Errors++
			continue
		}
		subjects, err := historicalSubjects(rb.Subjects)
		if err != nil {
			log.Printf("[HistoricalRiskEvaluator] Invalid RoleBinding %s/%s subjects: %v", rb.Namespace, rb.Name, err)
			stats.Errors++
			continue
		}
		normalizedData := map[string]interface{}{
			"kind":       "RoleBinding",
			"name":       rb.Name,
			"namespace":  rb.Namespace,
			"cluster_id": rb.ClusterID,
			"uid":        rb.UID,
			"roleRef":    roleRef,
			"subjects":   subjects,
		}

		insights, err := e.evaluateResource(ctx, "RoleBinding", normalizedData)
		if err != nil {
			log.Printf("[HistoricalRiskEvaluator] Error evaluating RoleBinding %s/%s: %v", rb.Namespace, rb.Name, err)
			stats.Errors++
			continue
		}

		for _, insight := range insights {
			if err := e.insightMgr.CreateOrUpdateInsight(insight); err != nil {
				log.Printf("[HistoricalRiskEvaluator] Failed to create insight for RoleBinding %s/%s: %v", rb.Namespace, rb.Name, err)
				stats.Errors++
				continue
			}
			stats.InsightsCreated++
		}
	}

	return nil
}

// evaluateClusterRoleBindings evaluates all ClusterRoleBindings
func (e *HistoricalRiskEvaluator) evaluateClusterRoleBindings(ctx context.Context, stats *struct {
	ServiceAccounts     int
	Roles               int
	ClusterRoles        int
	RoleBindings        int
	ClusterRoleBindings int
	InsightsCreated     int
	Errors              int
}) error {
	var clusterRoleBindings []models.ClusterRoleBinding
	if err := e.db.WithContext(ctx).Find(&clusterRoleBindings).Error; err != nil {
		return fmt.Errorf("failed to fetch cluster role bindings: %w", err)
	}

	stats.ClusterRoleBindings = len(clusterRoleBindings)

	for _, crb := range clusterRoleBindings {
		normalizedData := map[string]interface{}{
			"kind":       "ClusterRoleBinding",
			"name":       crb.Name,
			"namespace":  "", // ClusterRoleBinding has no namespace
			"cluster_id": crb.ClusterID,
			"uid":        crb.UID,
			"roleRef":    crb.RoleRef,
			"subjects":   crb.Subjects,
		}

		// Parse roleRef and subjects from JSON strings
		roleRef, err := historicalRoleRef(crb.RoleRef)
		if err != nil {
			log.Printf("[HistoricalRiskEvaluator] Invalid ClusterRoleBinding %s roleRef: %v", crb.Name, err)
			stats.Errors++
			continue
		}
		normalizedData["roleRef"] = roleRef
		subjects, err := historicalSubjects(crb.Subjects)
		if err != nil {
			log.Printf("[HistoricalRiskEvaluator] Invalid ClusterRoleBinding %s subjects: %v", crb.Name, err)
			stats.Errors++
			continue
		}
		normalizedData["subjects"] = subjects

		insights, err := e.evaluateResource(ctx, "ClusterRoleBinding", normalizedData)
		if err != nil {
			log.Printf("[HistoricalRiskEvaluator] Error evaluating ClusterRoleBinding %s: %v", crb.Name, err)
			stats.Errors++
			continue
		}

		for _, insight := range insights {
			if err := e.insightMgr.CreateOrUpdateInsight(insight); err != nil {
				log.Printf("[HistoricalRiskEvaluator] Failed to create insight for ClusterRoleBinding %s: %v", crb.Name, err)
				stats.Errors++
				continue
			}
			stats.InsightsCreated++
		}
	}

	return nil
}
