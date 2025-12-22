package riskengine

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

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

// NewYAMLEngine creates a new engine with YAML rule support
// Note: This creates a base engine with hardcoded rules first, then loads YAML rules
// It does NOT call NewEngine to avoid circular dependency
func NewYAMLEngine(db *gorm.DB, rulesDir string) (*YAMLEngine, error) {
	// Create base engine with hardcoded rules first (to avoid circular dependency)
	baseEngine := &Engine{
		db:    db,
		rules: GetBuiltInRules(), // Start with hardcoded rules
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
		log.Printf("[YAMLEngine] Failed to load YAML rules: %v, using hardcoded only", err)
	} else {
		// Merge YAML rules with hardcoded
		ye.mergeRules()
		// Compile CEL expressions in loaded rules
		ye.compileRuleCELs()
	}

	return ye, nil
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

// mergeRules merges YAML rules with hardcoded rules (YAML takes precedence)
func (ye *YAMLEngine) mergeRules() {
	ye.mu.RLock()
	defer ye.mu.RUnlock()

	// Start with hardcoded rules as fallback
	ruleMap := make(map[string]Rule)
	for _, rule := range ye.rules {
		ruleMap[rule.ID] = rule
	}

	// Override with YAML rules (YAML takes precedence)
	for id, rule := range ye.yamlRules {
		ruleMap[id] = *rule
		log.Printf("[YAMLEngine] Loaded YAML rule: %s (replaces hardcoded if exists)", id)
	}

	// Convert to slice
	ye.rules = make([]Rule, 0, len(ruleMap))
	for _, rule := range ruleMap {
		if rule.Enabled {
			ye.rules = append(ye.rules, rule)
		}
	}

	log.Printf("[YAMLEngine] Merged rules: %d total (%d YAML + %d hardcoded fallback)",
		len(ye.rules), len(ye.yamlRules), len(ye.rules)-len(ye.yamlRules))
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

// GetRules returns the current rules (merged YAML + hardcoded)
func (ye *YAMLEngine) GetRules() []Rule {
	ye.mu.RLock()
	defer ye.mu.RUnlock()
	return ye.rules
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
