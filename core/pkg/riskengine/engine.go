package riskengine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ksam/core/pkg/models"
	"gorm.io/gorm"
)

// Engine is the risk evaluation engine
type Engine struct {
	db    *gorm.DB
	rules []Rule
}

// NewEngine creates a new risk engine
// It now prioritizes YAML rules over hardcoded rules
func NewEngine(db *gorm.DB) *Engine {
	engine := &Engine{
		db: db,
	}

	// Try to load YAML rules first (primary source)
	rulesDir := getRulesDirectory()
	if rulesDir != "" {
		if yamlEngine, err := NewYAMLEngine(nil, rulesDir); err == nil {
			// Use YAML engine (which includes hardcoded as fallback)
			// Note: Pass nil for db to avoid circular dependency in NewYAMLEngine
			// The YAMLEngine will create its own base engine
			engine.rules = yamlEngine.GetRules()
			engine.db = db // Set db after getting rules
			log.Printf("[RiskEngine] ✅ Using YAML engine with %d rules (YAML + hardcoded fallback)", len(engine.rules))
			return engine
		} else {
			log.Printf("[RiskEngine] ⚠️  Failed to load YAML rules: %v, using hardcoded rules only", err)
		}
	} else {
		log.Printf("[RiskEngine] ℹ️  No rules directory configured (KSAM_RULES_DIR), using hardcoded rules")
	}

	// Fallback to hardcoded rules only
	engine.rules = GetBuiltInRules()
	log.Printf("[RiskEngine] Using %d hardcoded rules", len(engine.rules))
	return engine
}

// getRulesDirectory returns the rules directory path
func getRulesDirectory() string {
	// Try environment variable first
	if dir := os.Getenv("KSAM_RULES_DIR"); dir != "" {
		return dir
	}

	// Default to ./rules relative to current directory (for development)
	// In production, this should be set via environment variable
	if _, err := os.Stat("./rules"); err == nil {
		return "./rules"
	}

	// Try relative path from core directory
	if _, err := os.Stat("core/rules"); err == nil {
		return "core/rules"
	}

	return ""
}

// EvaluateResource evaluates a resource against all rules
func (e *Engine) EvaluateResource(ctx context.Context, resourceType string, resourceData map[string]interface{}) ([]*models.Insight, error) {
	var insights []*models.Insight

	// Parse raw_json and merge with resourceData
	enrichedData := e.enrichResourceData(resourceData)

	// Get applicable rules for this resource type
	applicableRules := e.getApplicableRules(resourceType)

	for _, rule := range applicableRules {
		if !rule.Enabled {
			continue
		}

		// Evaluate rule
		matched, score, err := e.evaluateRule(ctx, rule, enrichedData)
		if err != nil {
			log.Printf("[RiskEngine] Failed to evaluate rule %s: %v", rule.ID, err)
			continue
		}

		if matched {
			// Create insight
			insight := e.createInsight(rule, resourceType, enrichedData, score)
			insights = append(insights, insight)
		}
	}

	return insights, nil
}

// enrichResourceData parses raw_json and merges it with resourceData
func (e *Engine) enrichResourceData(resourceData map[string]interface{}) map[string]interface{} {
	// Start with a copy of resourceData
	enriched := make(map[string]interface{})
	for k, v := range resourceData {
		enriched[k] = v
	}

	// Parse raw_json if it exists
	if rawJSON, ok := resourceData["raw_json"].(string); ok && rawJSON != "" {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(rawJSON), &parsed); err == nil {
			// Merge parsed data into enriched (parsed data takes precedence for overlapping fields)
			for k, v := range parsed {
				// Only merge if not already in enriched (to preserve normalized fields)
				if _, exists := enriched[k]; !exists {
					enriched[k] = v
				}
			}
			// Also add specific fields that Risk Engine needs
			if spec, ok := parsed["spec"].(map[string]interface{}); ok {
				enriched["spec"] = spec
			}
			if roleRef, ok := parsed["roleRef"].(map[string]interface{}); ok {
				enriched["roleRef"] = roleRef
			}
			if rules, ok := parsed["rules"].([]interface{}); ok {
				enriched["rules"] = rules
			}
			if subjects, ok := parsed["subjects"].([]interface{}); ok {
				enriched["subjects"] = subjects
			}
		}
	}

	return enriched
}

// evaluateRule evaluates a single rule against resource data
func (e *Engine) evaluateRule(ctx context.Context, rule Rule, resourceData map[string]interface{}) (bool, float64, error) {
	// Evaluate all conditions
	conditionResults := make([]bool, 0, len(rule.Conditions))

	for _, condition := range rule.Conditions {
		result, err := e.evaluateCondition(condition, resourceData)
		if err != nil {
			return false, 0, fmt.Errorf("failed to evaluate condition: %w", err)
		}
		conditionResults = append(conditionResults, result)
	}

	// Aggregate results
	matched := e.aggregateResults(conditionResults, rule.Aggregation)
	if !matched {
		return false, 0, nil
	}

	// Return match with base score
	return true, rule.BaseScore, nil
}

// evaluateCondition evaluates a single condition
func (e *Engine) evaluateCondition(condition Condition, resourceData map[string]interface{}) (bool, error) {
	switch condition.Type {
	case CondTypeResource:
		return e.evaluateResourceCondition(condition, resourceData)
	case CondTypeExpression:
		return e.evaluateExpressionCondition(condition, resourceData)
	default:
		return false, fmt.Errorf("unknown condition type: %s", condition.Type)
	}
}

// evaluateResourceCondition evaluates a resource field condition
func (e *Engine) evaluateResourceCondition(condition Condition, resourceData map[string]interface{}) (bool, error) {
	// Use enhanced field parser
	fieldValue := e.getFieldValue(resourceData, condition.Field)

	// Handle array field access (e.g., "subjects[].kind")
	// The parser now handles this, but we need to check if result is array
	if arr, ok := fieldValue.([]interface{}); ok {
		// Array result - check if any element matches
		for _, item := range arr {
			if item == condition.Value {
				return true, nil
			}
		}
		return false, nil
	}

	switch condition.Operator {
	case OpEquals:
		return fieldValue == condition.Value, nil
	case OpNotEquals:
		return fieldValue != condition.Value, nil
	case OpContains:
		if str, ok := fieldValue.(string); ok {
			if valStr, ok := condition.Value.(string); ok {
				return contains(str, valStr), nil
			}
		}
		return false, nil
	case OpIn:
		if arr, ok := condition.Value.([]interface{}); ok {
			for _, v := range arr {
				if fieldValue == v {
					return true, nil
				}
			}
		}
		return false, nil
	case OpExists:
		return fieldValue != nil, nil
	default:
		return false, fmt.Errorf("unknown operator: %s", condition.Operator)
	}
}

// evaluateExpressionCondition evaluates a simple expression condition
// If YAMLEngine is available and has CEL compiler, use it; otherwise fall back to simple evaluation
func (e *Engine) evaluateExpressionCondition(condition Condition, resourceData map[string]interface{}) (bool, error) {
	// Check if we're using YAMLEngine with CEL support
	// This is a bit of a hack - we check if the engine has a method to evaluate CEL
	// In practice, YAMLEngine will override this method
	
	// For now, try simple expression evaluation as fallback
	expr := condition.Expression

	// Check for cluster-admin pattern
	if contains(expr, "cluster-admin") {
		roleRef, _ := resourceData["roleRef"].(map[string]interface{})
		if roleRef != nil {
			roleName, _ := roleRef["name"].(string)
			if roleName == "cluster-admin" {
				return true, nil
			}
		}
	}

	// Check for wildcard pattern
	if contains(expr, "wildcard") || contains(expr, "'*'") {
		rules, _ := resourceData["rules"].([]interface{})
		if rules != nil {
			for _, rule := range rules {
				if ruleMap, ok := rule.(map[string]interface{}); ok {
					resources, _ := ruleMap["resources"].([]interface{})
					verbs, _ := ruleMap["verbs"].([]interface{})

					// Check for wildcard in resources or verbs
					if hasWildcard(resources) || hasWildcard(verbs) {
						return true, nil
					}
				}
			}
		}
	}

	// Check for empty linked pods (orphan SA)
	if contains(expr, "linkedPods") {
		linkedPods, _ := resourceData["linkedPods"].(string)
		if linkedPods == "[]" || linkedPods == "" {
			return true, nil
		}
	}

	return false, nil
}

// aggregateResults aggregates condition results based on aggregation type
func (e *Engine) aggregateResults(results []bool, aggregation AggregationType) bool {
	if len(results) == 0 {
		return false
	}

	switch aggregation {
	case AggregationAND:
		for _, result := range results {
			if !result {
				return false
			}
		}
		return true
	case AggregationOR:
		for _, result := range results {
			if result {
				return true
			}
		}
		return false
	case AggregationTHRESHOLD:
		// For threshold, at least 50% must match
		matched := 0
		for _, result := range results {
			if result {
				matched++
			}
		}
		return matched >= len(results)/2
	default:
		return false
	}
}

// createInsight creates an insight from a matched rule
func (e *Engine) createInsight(rule Rule, resourceType string, resourceData map[string]interface{}, score float64) *models.Insight {
	// Extract resource identifiers
	clusterID, _ := resourceData["cluster_id"].(string)
	if clusterID == "" {
		clusterID = "default"
	}

	name, _ := resourceData["name"].(string)
	namespace, _ := resourceData["namespace"].(string)

	// Build affected resources
	affectedResources := []map[string]interface{}{
		{
			"type":      resourceType,
			"name":      name,
			"namespace": namespace,
		},
	}
	affectedResourcesJSON, _ := json.Marshal(affectedResources)

	// Build description
	description := fmt.Sprintf("%s: %s", rule.Name, rule.Description)
	if namespace != "" {
		description = fmt.Sprintf("%s in namespace %s", description, namespace)
	}

	// Build recommended action
	recommendedAction := e.getRecommendedAction(rule, resourceType)

	return &models.Insight{
		Type:              string(rule.Category),
		Description:       description,
		AffectedResources: string(affectedResourcesJSON),
		Severity:          string(rule.Severity),
		RecommendedAction: recommendedAction,
		Status:            "active", // Explicitly set status to 'active'
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
}

// getRecommendedAction returns recommended action for a rule
func (e *Engine) getRecommendedAction(rule Rule, resourceType string) string {
	switch rule.ID {
	case "cis-5.1.3":
		return "Remove cluster-admin binding from ServiceAccount. Use least-privilege roles instead."
	case "wildcard-permissions":
		return "Replace wildcard permissions with specific resource and verb lists."
	case "orphan-serviceaccount":
		return "Review if ServiceAccount is needed. Remove if unused."
	case "overprivileged-role":
		return "Review role permissions and apply principle of least privilege."
	case "overprivileged-binding":
		return "Review role binding and ensure ServiceAccount only has necessary permissions."
	default:
		return "Review and apply security best practices."
	}
}

// getFieldValue gets a field value from resource data (supports simple dot notation and array indexing)
// Uses enhanced field parser for better support
func (e *Engine) getFieldValue(data map[string]interface{}, field string) interface{} {
	// Use enhanced field parser (inline to avoid import cycle)
	return e.simpleFieldAccess(data, field)
}

// simpleFieldAccess is a lightweight field accessor (moved here to avoid import cycle)
func (e *Engine) simpleFieldAccess(data map[string]interface{}, field string) interface{} {
	if field == "" {
		return nil
	}

	// Try simple field access first (fast path)
	if val, ok := data[field]; ok {
		return val
	}

	// Try simple dot notation (no arrays)
	if !strings.Contains(field, "[") {
		parts := strings.Split(field, ".")
		current := interface{}(data)
		for _, part := range parts {
			if m, ok := current.(map[string]interface{}); ok {
				val, exists := m[part]
				if !exists {
					return nil
				}
				current = val
			} else {
				return nil
			}
		}
		return current
	}

	// Handle array notation (e.g., "subjects[].kind")
	// Simple implementation without full parser
	if idx := strings.Index(field, "[]"); idx > 0 {
		arrayField := field[:idx]
		childField := field[idx+3:] // Skip "[]."

		// Get the array
		arr, ok := data[arrayField].([]interface{})
		if !ok {
			// Try parsing from JSON string
			if arrJSON, ok := data[arrayField].(string); ok {
				if err := json.Unmarshal([]byte(arrJSON), &arr); err != nil {
					return nil
				}
			} else {
				return nil
			}
		}

		// Extract field from each item
		var results []interface{}
		for _, item := range arr {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if val, exists := itemMap[childField]; exists {
					results = append(results, val)
				}
			}
		}
		return results
	}

	return nil
}

// indexOf finds the index of a character in a string
func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// getApplicableRules returns rules applicable to a resource type
func (e *Engine) getApplicableRules(resourceType string) []Rule {
	var applicable []Rule

	for _, rule := range e.rules {
		// Simple matching based on resource type and rule category
		switch resourceType {
		case "ServiceAccount":
			if rule.Category == CategoryRBAC {
				applicable = append(applicable, rule)
			}
		case "Role", "ClusterRole":
			if rule.Category == CategoryRBAC {
				applicable = append(applicable, rule)
			}
		case "RoleBinding", "ClusterRoleBinding":
			if rule.Category == CategoryRBAC {
				applicable = append(applicable, rule)
			}
		}
	}

	return applicable
}

// Helper functions
func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	if s == substr {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func hasWildcard(arr []interface{}) bool {
	for _, v := range arr {
		if str, ok := v.(string); ok && str == "*" {
			return true
		}
	}
	return false
}
