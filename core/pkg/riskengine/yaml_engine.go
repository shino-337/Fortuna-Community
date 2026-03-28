package riskengine

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fortuna/core/pkg/models"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// YAMLEngine extends Engine with YAML rule loading
type YAMLEngine struct {
	*Engine
	rulesDir    string
	yamlRules   map[string]*Rule
	mu          sync.RWMutex
	celCompiler *CELCompiler
}

// NewYAMLEngine creates a YAML-only engine.
func NewYAMLEngine(db *gorm.DB, rulesDir string) (*YAMLEngine, error) {
	// Create base engine with empty rules; YAML is the only source of truth.
	baseEngine := &Engine{
		db:    db,
		rules: []Rule{},
	}

	// Create CEL compiler
	celCompiler, err := NewCELCompiler()
	if err != nil {
		log.Printf("[YAMLEngine] WARNING: Failed to create CEL compiler: %v. CEL expressions will not work.", err)
		celCompiler = nil
	}

	ye := &YAMLEngine{
		Engine:      baseEngine,
		rulesDir:    rulesDir,
		yamlRules:   make(map[string]*Rule),
		celCompiler: celCompiler,
	}

	// Load YAML rules
	if err := ye.LoadYAMLRules(); err != nil {
		return nil, fmt.Errorf("failed to load YAML rules: %w", err)
	} else {
		ye.mergeRules()
		// Compile CEL expressions in loaded rules
		ye.compileRuleCELs()
	}

	return ye, nil
}

// EvaluateResource evaluates rules using YAML/CEL paths on the YAMLEngine (not only the embedded *Engine).
// Calling EvaluateResource on *Engine after embedding loses method overrides; workers must call this when using YAML rules.
func (ye *YAMLEngine) EvaluateResource(ctx context.Context, resourceType string, resourceData map[string]interface{}) ([]*models.Insight, error) {
	var insights []*models.Insight
	enrichedData := ye.prepareEnrichedResourceData(ctx, resourceType, resourceData)
	ye.mu.RLock()
	applicableRules := ye.getApplicableRules(resourceType)
	ye.mu.RUnlock()
	for _, rule := range applicableRules {
		if !rule.Enabled {
			continue
		}
		matched, score, err := ye.evaluateRule(ctx, rule, enrichedData)
		if err != nil {
			log.Printf("[RiskEngine] Failed to evaluate rule %s: %v", rule.ID, err)
			continue
		}
		if matched {
			insight := ye.createInsight(rule, resourceType, enrichedData, score)
			insights = append(insights, insight)
		}
	}
	return insights, nil
}

// LoadYAMLRules loads all YAML rules from directory
func (ye *YAMLEngine) LoadYAMLRules() error {
	if ye.rulesDir == "" {
		return fmt.Errorf("rules directory not configured")
	}

	// Check if directory exists
	if _, err := os.Stat(ye.rulesDir); os.IsNotExist(err) {
		return fmt.Errorf("rules directory does not exist: %s", ye.rulesDir)
	}

	ye.mu.Lock()
	defer ye.mu.Unlock()

	ye.yamlRules = make(map[string]*Rule)
	var errors []error

	// Walk directory
	err := filepath.WalkDir(ye.rulesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		rule, err := ye.loadRuleFile(path)
		if err != nil {
			errors = append(errors, fmt.Errorf("file %s: %w", path, err))
			return nil
		}

		if err := ye.validateRule(rule); err != nil {
			errors = append(errors, fmt.Errorf("rule %s: %w", rule.ID, err))
			return nil
		}

		ye.yamlRules[rule.ID] = rule
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	if len(errors) > 0 && len(ye.yamlRules) == 0 {
		return fmt.Errorf("failed to load any rules: %v", errors)
	}

	log.Printf("[YAMLEngine] Loaded %d YAML rules", len(ye.yamlRules))
	return nil
}

// loadRuleFile loads a single YAML rule file
func (ye *YAMLEngine) loadRuleFile(filename string) (*Rule, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var rule Rule
	if err := yaml.Unmarshal(data, &rule); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &rule, nil
}

// validateRule performs basic validation
func (ye *YAMLEngine) validateRule(rule *Rule) error {
	if rule.ID == "" {
		return fmt.Errorf("rule ID is required")
	}
	if rule.Name == "" {
		return fmt.Errorf("rule name is required")
	}
	if len(rule.Conditions) == 0 {
		return fmt.Errorf("rule must have at least one condition")
	}

	validSeverities := []string{"critical", "high", "medium", "low", "info"}
	severityValid := false
	for _, s := range validSeverities {
		if string(rule.Severity) == s {
			severityValid = true
			break
		}
	}
	if !severityValid {
		return fmt.Errorf("invalid severity: %s", rule.Severity)
	}

	if rule.BaseScore < 0 || rule.BaseScore > 10 {
		return fmt.Errorf("base_score must be between 0 and 10")
	}

	return nil
}

// mergeRules rebuilds the runtime ruleset from YAML.
func (ye *YAMLEngine) mergeRules() {
	ye.mu.Lock()
	defer ye.mu.Unlock()
	ye.mergeRulesUnlocked()
}

// mergeRulesUnlocked does the merge; caller must hold ye.mu Lock (writes to ye.rules).
func (ye *YAMLEngine) mergeRulesUnlocked() {
	ruleMap := make(map[string]Rule, len(ye.yamlRules))
	for id, rule := range ye.yamlRules {
		ruleMap[id] = *rule
		log.Printf("[YAMLEngine] Loaded YAML rule: %s", id)
	}

	// Convert to slice
	ye.rules = make([]Rule, 0, len(ruleMap))
	for _, rule := range ruleMap {
		ye.rules = append(ye.rules, rule)
	}

	log.Printf("[YAMLEngine] Active YAML rules: %d", len(ye.rules))
}

// ReloadFromDB is retained for interface compatibility only.
// Rules are YAML-only, so this simply reloads YAML.
func (ye *YAMLEngine) ReloadFromDB() error {
	return ye.Reload()
}

// Reload reloads YAML rules and recompiles CEL expressions
func (ye *YAMLEngine) Reload() error {
	// Clear CEL cache before reload
	if ye.celCompiler != nil {
		ye.celCompiler.ClearCache()
	}

	if err := ye.LoadYAMLRules(); err != nil {
		return err
	}
	ye.mergeRules()
	// Recompile CEL expressions
	ye.compileRuleCELs()
	return nil
}

// GetRules returns the current active YAML rules.
func (ye *YAMLEngine) GetRules() []Rule {
	ye.mu.RLock()
	defer ye.mu.RUnlock()
	return ye.rules
}

// GetYAMLRuleIDs returns IDs loaded from YAML files.
func (ye *YAMLEngine) GetYAMLRuleIDs() map[string]struct{} {
	ye.mu.RLock()
	defer ye.mu.RUnlock()
	out := make(map[string]struct{}, len(ye.yamlRules))
	for id := range ye.yamlRules {
		out[id] = struct{}{}
	}
	return out
}

// compileRuleCELs compiles all CEL expressions in loaded rules
func (ye *YAMLEngine) compileRuleCELs() {
	if ye.celCompiler == nil {
		return
	}

	ye.mu.RLock()
	defer ye.mu.RUnlock()

	compiledCount := 0
	for _, rule := range ye.rules {
		for i, condition := range rule.Conditions {
			if condition.Type == CondTypeExpression && condition.Expression != "" {
				// Compile and cache the CEL expression
				if _, err := ye.celCompiler.Compile(condition.Expression); err != nil {
					log.Printf("[YAMLEngine] WARNING: Failed to compile CEL for rule %s, condition %d: %v", rule.ID, i, err)
				} else {
					compiledCount++
				}
			}
		}
	}

	if compiledCount > 0 {
		log.Printf("[YAMLEngine] Compiled %d CEL expressions", compiledCount)
	}
}

// evaluateCELCondition evaluates a CEL expression condition
func (ye *YAMLEngine) evaluateCELCondition(condition Condition, resourceData map[string]interface{}) (bool, error) {
	if ye.celCompiler == nil {
		return false, fmt.Errorf("CEL compiler not available")
	}

	// Normalize RBAC "rules" so CEL sees an actual list.
	// In the DB models Role/ClusterRole store rules as a JSON string; CEL expressions expect:
	//   object.rules.exists(r, ...)
	// If object.rules is still a string, cel-go evaluation will fail (or always false).
	if resourceData != nil {
		if rulesVal, ok := resourceData["rules"]; ok {
			switch v := rulesVal.(type) {
			case string:
				trimmed := strings.TrimSpace(v)
				// Only attempt parsing when it looks like a JSON array/object.
				if trimmed != "" && (strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{")) {
					var parsed []interface{}
					if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
						resourceData["rules"] = parsed
					}
				}
			case []byte:
				// Handle []byte JSON payloads defensively.
				var parsed []interface{}
				if err := json.Unmarshal(v, &parsed); err == nil {
					resourceData["rules"] = parsed
				}
			}
		}
	}

	// Prepare CEL input with object as root
	celInput := map[string]interface{}{
		"object": resourceData,
	}

	// Extract metadata, spec, status if present
	if metadata, ok := resourceData["metadata"].(map[string]interface{}); ok {
		celInput["metadata"] = metadata
	}
	if spec, ok := resourceData["spec"].(map[string]interface{}); ok {
		celInput["spec"] = spec
	}
	if status, ok := resourceData["status"].(map[string]interface{}); ok {
		celInput["status"] = status
	}

	// Evaluate CEL expression
	return ye.celCompiler.Evaluate(condition.Expression, celInput)
}

// evaluateRule overrides Engine's evaluateRule to use CEL for expression conditions
func (ye *YAMLEngine) evaluateRule(ctx context.Context, rule Rule, resourceData map[string]interface{}) (bool, float64, error) {
	// Evaluate all conditions using YAMLEngine's evaluateCondition (which uses CEL)
	conditionResults := make([]bool, 0, len(rule.Conditions))

	for _, condition := range rule.Conditions {
		result, err := ye.evaluateCondition(condition, resourceData)
		if err != nil {
			return false, 0, fmt.Errorf("failed to evaluate condition: %w", err)
		}
		conditionResults = append(conditionResults, result)
	}

	// Aggregate results
	matched := ye.Engine.aggregateResults(conditionResults, rule.Aggregation)
	if !matched {
		return false, 0, nil
	}

	// Return match with base score
	return true, rule.BaseScore, nil
}

// evaluateCondition overrides Engine's evaluateCondition to use CEL for expression conditions
func (ye *YAMLEngine) evaluateCondition(condition Condition, resourceData map[string]interface{}) (bool, error) {
	switch condition.Type {
	case CondTypeResource:
		return ye.Engine.evaluateResourceCondition(condition, resourceData)
	case CondTypeExpression:
		// Use CEL if available, otherwise fall back to simple evaluation
		if ye.celCompiler != nil && condition.Expression != "" {
			return ye.evaluateCELCondition(condition, resourceData)
		}
		return ye.Engine.evaluateExpressionCondition(condition, resourceData)
	default:
		return false, fmt.Errorf("unknown condition type: %s", condition.Type)
	}
}
