package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/riskengine"
	"gorm.io/gorm"
)

// HistoricalRiskEvaluator processes historical data from database
type HistoricalRiskEvaluator struct {
	db         *gorm.DB
	riskEngine *riskengine.Engine
	insightMgr *riskengine.InsightManager
}

// NewHistoricalRiskEvaluator creates a new historical risk evaluator
func NewHistoricalRiskEvaluator(db *gorm.DB) *HistoricalRiskEvaluator {
	// Try YAML engine first, fallback to standard
	rulesDir := os.Getenv("KSAM_RULES_DIR")
	var engine *riskengine.Engine
	
	if rulesDir != "" {
		if yamlEngine, err := riskengine.NewYAMLEngine(db, rulesDir); err == nil {
			engine = yamlEngine.Engine
		} else {
			engine = riskengine.NewEngine(db)
		}
	} else {
		engine = riskengine.NewEngine(db)
	}
	
	return &HistoricalRiskEvaluator{
		db:         db,
		riskEngine: engine,
		insightMgr: riskengine.NewInsightManager(db),
	}
}

// EvaluateAllResources evaluates all existing resources in the database
func (e *HistoricalRiskEvaluator) EvaluateAllResources(ctx context.Context) error {
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
	if err := e.db.Find(&serviceAccounts).Error; err != nil {
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

		insights, err := e.riskEngine.EvaluateResource(ctx, "ServiceAccount", normalizedData)
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
	if err := e.db.Find(&roles).Error; err != nil {
		return fmt.Errorf("failed to fetch roles: %w", err)
	}

	stats.Roles = len(roles)

	for _, role := range roles {
		// Parse rules JSON to ensure proper format
		var rules interface{}
		if role.Rules != "" {
			if err := json.Unmarshal([]byte(role.Rules), &rules); err != nil {
				log.Printf("[HistoricalRiskEvaluator] Failed to parse rules JSON for Role %s/%s: %v", role.Namespace, role.Name, err)
				rules = []interface{}{}
			}
		} else {
			rules = []interface{}{}
		}
		
		normalizedData := map[string]interface{}{
			"kind":       "Role",
			"name":       role.Name,
			"namespace":  role.Namespace,
			"cluster_id": role.ClusterID,
			"uid":        role.UID,
			"rules":      rules,
		}

		insights, err := e.riskEngine.EvaluateResource(ctx, "Role", normalizedData)
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
	if err := e.db.Find(&clusterRoles).Error; err != nil {
		return fmt.Errorf("failed to fetch cluster roles: %w", err)
	}

	stats.ClusterRoles = len(clusterRoles)

	for _, cr := range clusterRoles {
		normalizedData := map[string]interface{}{
			"kind":       "ClusterRole",
			"name":       cr.Name,
			"namespace":  "", // ClusterRole has no namespace
			"cluster_id": cr.ClusterID,
			"uid":        cr.UID,
			"rules":      cr.Rules,
		}

		insights, err := e.riskEngine.EvaluateResource(ctx, "ClusterRole", normalizedData)
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
	if err := e.db.Find(&roleBindings).Error; err != nil {
		return fmt.Errorf("failed to fetch role bindings: %w", err)
	}

	stats.RoleBindings = len(roleBindings)

	for _, rb := range roleBindings {
		normalizedData := map[string]interface{}{
			"kind":       "RoleBinding",
			"name":       rb.Name,
			"namespace":  rb.Namespace,
			"cluster_id": rb.ClusterID,
			"uid":        rb.UID,
			"roleRef":    rb.RoleRef,
			"subjects":   rb.Subjects,
		}

		insights, err := e.riskEngine.EvaluateResource(ctx, "RoleBinding", normalizedData)
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
	if err := e.db.Find(&clusterRoleBindings).Error; err != nil {
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
		if crb.RoleRef != "" {
			var roleRef map[string]interface{}
			if err := json.Unmarshal([]byte(crb.RoleRef), &roleRef); err == nil {
				normalizedData["roleRef"] = roleRef
			}
		}
		if crb.Subjects != "" {
			var subjects []interface{}
			if err := json.Unmarshal([]byte(crb.Subjects), &subjects); err == nil {
				normalizedData["subjects"] = subjects
			}
		}

		insights, err := e.riskEngine.EvaluateResource(ctx, "ClusterRoleBinding", normalizedData)
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

