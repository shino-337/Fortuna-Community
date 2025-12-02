# KSAM Risk Engine - Analysis & Modernization Plan

**Date**: 2025-11-29  
**Current Version**: Hardcoded Rules  
**Target Version**: YAML-Based Dynamic Rule System

---

## Table of Contents

- [Current State Analysis](#current-state-analysis)
- [Critical Issues](#critical-issues)
- [Proposed Architecture](#proposed-architecture)
- [YAML Rule System](#yaml-rule-system)
- [Implementation Plan](#implementation-plan)
- [Migration Strategy](#migration-strategy)
- [Testing Strategy](#testing-strategy)
- [Code Examples](#code-examples)

---

## Current State Analysis

### ✅ What's Good

1. **Clean Separation of Concerns**:
   - `engine.go`: Core evaluation logic
   - `rule.go`: Rule definitions
   - `insight_manager.go`: Insight persistence

2. **Type-Safe Rule Structure**:
   - Proper Go structs with typed fields
   - Enum-based constants for categories/severities

3. **Flexible Condition System**:
   - Supports multiple condition types
   - Aggregation options (AND/OR/THRESHOLD)
   - Field-based and expression-based evaluation

4. **Basic Data Enrichment**:
   - Parses `raw_json` field
   - Merges with normalized data

### ❌ Critical Issues

#### Issue #1: Hardcoded Rules
```go
// Current: Rules are hardcoded in code
func GetBuiltInRules() []Rule {
    return []Rule{
        {
            ID: "cis-5.1.3",
            Name: "ServiceAccount granted cluster-admin",
            // ... hardcoded configuration
        },
    }
}
```

**Problems**:
- ❌ Need code rebuild to add/modify rules
- ❌ No dynamic rule updates
- ❌ Hard to test rules in isolation
- ❌ No version control for individual rules
- ❌ Can't disable rules without code change

#### Issue #2: Limited Expression Evaluation
```go
// Current: Basic string matching only
func (e *Engine) evaluateExpressionCondition(...) {
    if contains(expr, "cluster-admin") {
        // Basic pattern matching
    }
}
```

**Problems**:
- ❌ No CEL (Common Expression Language) support
- ❌ Limited to simple string contains
- ❌ Can't express complex conditions
- ❌ Hard to extend

#### Issue #3: Incomplete Field Access
```go
// Current: Manual dot notation parsing
func (e *Engine) getFieldValue(data map[string]interface{}, field string) interface{} {
    // Manual parsing of "field.subfield"
    if idx := indexOf(field, '.'); idx > 0 {
        // ... manual implementation
    }
}
```

**Problems**:
- ❌ Doesn't handle deep nesting (a.b.c.d)
- ❌ No array filtering (items[?(@.status=='Ready')])
- ❌ No JSONPath support
- ❌ Error-prone manual parsing

#### Issue #4: Weak Duplicate Detection
```go
// Current: Uses string matching on JSON
query = query.Where("affected_resources::text LIKE ?", 
    fmt.Sprintf("%% %s %%", resourceIdentifier))
```

**Problems**:
- ❌ Brittle JSON string matching
- ❌ False positives possible
- ❌ Performance issues with LIKE on JSON text
- ❌ No proper unique constraint

#### Issue #5: No Rule Validation
```go
// Current: No validation before use
rules: GetBuiltInRules()
```

**Problems**:
- ❌ Invalid rules crash at runtime
- ❌ No schema validation
- ❌ Typos in field names not caught
- ❌ No testing framework

#### Issue #6: Missing Features
- ❌ No rule versioning
- ❌ No rule dependencies
- ❌ No rule priority/ordering
- ❌ No rule metadata (author, last_updated)
- ❌ No rule testing/dry-run mode
- ❌ No rule impact analysis

---

## Proposed Architecture

### High-Level Design

```
┌─────────────────────────────────────────────────────────────────┐
│                     YAML Rule Files                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │ cis-5.1.3.   │  │ cis-5.2.1.   │  │ custom-rule. │         │
│  │ yaml         │  │ yaml         │  │ yaml         │  ...    │
│  └──────────────┘  └──────────────┘  └──────────────┘         │
└────────────┬────────────────────────────────────────────────────┘
             │
             │ Load & Validate
             ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Rule Loader & Validator                        │
│  • Schema validation (JSON Schema)                              │
│  • Field validation (check if fields exist)                     │
│  • Dependency resolution                                         │
│  • Hot reload support                                            │
└────────────┬────────────────────────────────────────────────────┘
             │
             │ Validated Rules
             ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Rule Engine Core                            │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  CEL Engine (google/cel-go)                             │   │
│  │  • Complex expression evaluation                         │   │
│  │  • Type-safe operations                                  │   │
│  │  • Standard functions + custom extensions               │   │
│  └─────────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  JSONPath Engine (oliveagle/jsonpath)                   │   │
│  │  • Deep nested access: $.spec.containers[0].image       │   │
│  │  • Array filtering: $.items[?(@.status=='Running')]     │   │
│  │  • Wildcard support: $..*                                │   │
│  └─────────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  Condition Evaluator                                     │   │
│  │  • Resource conditions                                   │   │
│  │  • CEL expressions                                       │   │
│  │  • JSONPath queries                                      │   │
│  └─────────────────────────────────────────────────────────┘   │
└────────────┬────────────────────────────────────────────────────┘
             │
             │ Matched Rules → Insights
             ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Insight Manager                               │
│  • Deduplication (proper hash-based)                            │
│  • Aggregation across resources                                 │
│  • Insight lifecycle management                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Component Breakdown

#### 1. Rule Storage: YAML Files
```yaml
# rules/cis-5.1.3.yaml
---
metadata:
  id: cis-5.1.3
  name: "ServiceAccount with cluster-admin binding"
  version: "1.2.0"
  author: "security-team@company.com"
  created: "2024-01-15"
  updated: "2025-11-29"
  tags:
    - cis
    - cluster-admin
    - critical

classification:
  category: rbac
  severity: critical
  cis_section: "5.1.3"
  cis_level: 1
  cvss_score: 10.0

target:
  resource_types:
    - ClusterRoleBinding
  namespaces:
    - "*"
  clusters:
    - "*"

detection:
  aggregation: AND
  conditions:
    - type: cel
      expression: |
        object.roleRef.name == "cluster-admin"
    
    - type: cel
      expression: |
        object.subjects.exists(s, s.kind == "ServiceAccount")

risk_scoring:
  base_score: 9.0
  multipliers:
    - condition: 'object.subjects.exists(s, s.name == "default")'
      factor: 1.2
      reason: "Default ServiceAccount with cluster-admin"
    
    - condition: 'object.subjects.size() > 5'
      factor: 1.1
      reason: "Multiple subjects with cluster-admin"

insight:
  title: "ServiceAccount {{ .subjects[0].name }} has cluster-admin access"
  description: |
    The ServiceAccount {{ .subjects[0].name }} in namespace {{ .subjects[0].namespace }} 
    is bound to cluster-admin role, granting unrestricted cluster access.
    
    **Risk**: Any pod using this ServiceAccount can perform any action in the cluster.
    
  evidence:
    - "Binding: {{ .metadata.name }}"
    - "Role: {{ .roleRef.name }}"
    - "Subjects: {{ .subjects | toJson }}"
  
  remediation:
    - "Review if cluster-admin access is truly required"
    - "Create a custom role with minimum required permissions"
    - "kubectl create role limited-role --verb=get,list --resource=pods"
    - "kubectl create rolebinding limited-binding --role=limited-role --serviceaccount={{ .subjects[0].namespace }}:{{ .subjects[0].name }}"
    - "kubectl delete clusterrolebinding {{ .metadata.name }}"
  
  references:
    - "https://kubernetes.io/docs/reference/access-authn-authz/rbac/"
    - "https://www.cisecurity.org/benchmark/kubernetes"

testing:
  enabled: true
  test_cases:
    - name: "Detects cluster-admin binding"
      resource:
        apiVersion: rbac.authorization.k8s.io/v1
        kind: ClusterRoleBinding
        metadata:
          name: test-binding
        roleRef:
          kind: ClusterRole
          name: cluster-admin
        subjects:
          - kind: ServiceAccount
            name: test-sa
            namespace: default
      expect_match: true
      expected_score: 9.0
    
    - name: "Ignores non-cluster-admin binding"
      resource:
        apiVersion: rbac.authorization.k8s.io/v1
        kind: ClusterRoleBinding
        metadata:
          name: test-binding
        roleRef:
          kind: ClusterRole
          name: view
        subjects:
          - kind: ServiceAccount
            name: test-sa
            namespace: default
      expect_match: false
```

#### 2. Rule Loader with Validation

```go
// File: pkg/riskengine/loader/rule_loader.go

package loader

import (
    "fmt"
    "io/ioutil"
    "path/filepath"
    
    "gopkg.in/yaml.v3"
    "github.com/xeipuuv/gojsonschema"
)

type RuleLoader struct {
    rulesDir      string
    schemaPath    string
    validatedRules map[string]*Rule
}

func NewRuleLoader(rulesDir, schemaPath string) *RuleLoader {
    return &RuleLoader{
        rulesDir:      rulesDir,
        schemaPath:    schemaPath,
        validatedRules: make(map[string]*Rule),
    }
}

// LoadAllRules loads and validates all YAML rules from directory
func (l *RuleLoader) LoadAllRules() ([]*Rule, error) {
    files, err := filepath.Glob(filepath.Join(l.rulesDir, "*.yaml"))
    if err != nil {
        return nil, fmt.Errorf("failed to list rule files: %w", err)
    }
    
    var rules []*Rule
    errors := []error{}
    
    for _, file := range files {
        rule, err := l.loadAndValidateRule(file)
        if err != nil {
            errors = append(errors, fmt.Errorf("file %s: %w", file, err))
            continue
        }
        
        rules = append(rules, rule)
        l.validatedRules[rule.Metadata.ID] = rule
    }
    
    if len(errors) > 0 {
        // Log errors but continue with valid rules
        for _, err := range errors {
            log.Errorf("Rule validation error: %v", err)
        }
    }
    
    log.Infof("Loaded %d valid rules from %d files", len(rules), len(files))
    return rules, nil
}

// loadAndValidateRule loads a single rule file and validates it
func (l *RuleLoader) loadAndValidateRule(filename string) (*Rule, error) {
    // Read file
    data, err := ioutil.ReadFile(filename)
    if err != nil {
        return nil, fmt.Errorf("failed to read file: %w", err)
    }
    
    // Parse YAML
    var rule Rule
    if err := yaml.Unmarshal(data, &rule); err != nil {
        return nil, fmt.Errorf("failed to parse YAML: %w", err)
    }
    
    // Validate against JSON schema
    if err := l.validateAgainstSchema(&rule); err != nil {
        return nil, fmt.Errorf("schema validation failed: %w", err)
    }
    
    // Validate business logic
    if err := l.validateBusinessLogic(&rule); err != nil {
        return nil, fmt.Errorf("business validation failed: %w", err)
    }
    
    // Compile CEL expressions
    if err := l.compileCELExpressions(&rule); err != nil {
        return nil, fmt.Errorf("CEL compilation failed: %w", err)
    }
    
    return &rule, nil
}

// validateAgainstSchema validates rule against JSON schema
func (l *RuleLoader) validateAgainstSchema(rule *Rule) error {
    schemaLoader := gojsonschema.NewReferenceLoader("file://" + l.schemaPath)
    documentLoader := gojsonschema.NewGoLoader(rule)
    
    result, err := gojsonschema.Validate(schemaLoader, documentLoader)
    if err != nil {
        return fmt.Errorf("schema validation error: %w", err)
    }
    
    if !result.Valid() {
        errors := result.Errors()
        return fmt.Errorf("schema validation failed with %d errors: %v", 
            len(errors), errors[0].String())
    }
    
    return nil
}

// validateBusinessLogic validates business rules
func (l *RuleLoader) validateBusinessLogic(rule *Rule) error {
    // Check required fields
    if rule.Metadata.ID == "" {
        return fmt.Errorf("rule ID is required")
    }
    
    if rule.Metadata.Name == "" {
        return fmt.Errorf("rule name is required")
    }
    
    if len(rule.Detection.Conditions) == 0 {
        return fmt.Errorf("at least one condition is required")
    }
    
    // Validate severity
    validSeverities := []string{"critical", "high", "medium", "low", "info"}
    if !contains(validSeverities, string(rule.Classification.Severity)) {
        return fmt.Errorf("invalid severity: %s", rule.Classification.Severity)
    }
    
    // Validate aggregation
    validAggregations := []string{"AND", "OR", "THRESHOLD"}
    if !contains(validAggregations, string(rule.Detection.Aggregation)) {
        return fmt.Errorf("invalid aggregation: %s", rule.Detection.Aggregation)
    }
    
    // Validate risk score
    if rule.RiskScoring.BaseScore < 0 || rule.RiskScoring.BaseScore > 10 {
        return fmt.Errorf("base_score must be between 0 and 10")
    }
    
    return nil
}

// compileCELExpressions pre-compiles CEL expressions for performance
func (l *RuleLoader) compileCELExpressions(rule *Rule) error {
    env, err := cel.NewEnv(
        cel.Types(&ClusterRoleBinding{}, &Pod{}, &ServiceAccount{}),
    )
    if err != nil {
        return fmt.Errorf("failed to create CEL environment: %w", err)
    }
    
    for i, condition := range rule.Detection.Conditions {
        if condition.Type == "cel" {
            ast, issues := env.Compile(condition.Expression)
            if issues != nil && issues.Err() != nil {
                return fmt.Errorf("condition %d: CEL compilation failed: %w", 
                    i, issues.Err())
            }
            
            prg, err := env.Program(ast)
            if err != nil {
                return fmt.Errorf("condition %d: CEL program creation failed: %w", 
                    i, err)
            }
            
            // Store compiled program
            condition.CompiledProgram = prg
        }
    }
    
    // Compile multiplier conditions
    for i, multiplier := range rule.RiskScoring.Multipliers {
        if multiplier.Condition != "" {
            ast, issues := env.Compile(multiplier.Condition)
            if issues != nil && issues.Err() != nil {
                return fmt.Errorf("multiplier %d: CEL compilation failed: %w", 
                    i, issues.Err())
            }
            
            prg, err := env.Program(ast)
            if err != nil {
                return fmt.Errorf("multiplier %d: CEL program creation failed: %w", 
                    i, err)
            }
            
            multiplier.CompiledProgram = prg
        }
    }
    
    return nil
}

// WatchAndReload watches rule directory for changes and hot-reloads
func (l *RuleLoader) WatchAndReload(ctx context.Context, callback func([]*Rule)) error {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return fmt.Errorf("failed to create watcher: %w", err)
    }
    defer watcher.Close()
    
    if err := watcher.Add(l.rulesDir); err != nil {
        return fmt.Errorf("failed to watch directory: %w", err)
    }
    
    log.Infof("Watching rules directory: %s", l.rulesDir)
    
    for {
        select {
        case <-ctx.Done():
            return nil
            
        case event := <-watcher.Events:
            if event.Op&fsnotify.Write == fsnotify.Write || 
               event.Op&fsnotify.Create == fsnotify.Create {
                log.Infof("Rule file modified: %s", event.Name)
                
                // Reload all rules
                rules, err := l.LoadAllRules()
                if err != nil {
                    log.Errorf("Failed to reload rules: %v", err)
                    continue
                }
                
                // Trigger callback with new rules
                callback(rules)
            }
            
        case err := <-watcher.Errors:
            log.Errorf("Watcher error: %v", err)
        }
    }
}
```

#### 3. Enhanced Engine with CEL

```go
// File: pkg/riskengine/engine_v2.go

package riskengine

import (
    "context"
    "fmt"
    
    "github.com/google/cel-go/cel"
    "github.com/oliveagle/jsonpath"
)

type EngineV2 struct {
    db          *gorm.DB
    rules       []*Rule
    ruleLoader  *loader.RuleLoader
    celEnv      *cel.Env
}

func NewEngineV2(db *gorm.DB, rulesDir string) (*EngineV2, error) {
    // Create CEL environment
    celEnv, err := cel.NewEnv(
        cel.Types(
            &models.ClusterRoleBinding{},
            &models.Pod{},
            &models.ServiceAccount{},
        ),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create CEL environment: %w", err)
    }
    
    // Create rule loader
    ruleLoader := loader.NewRuleLoader(rulesDir, "config/rule-schema.json")
    
    // Load initial rules
    rules, err := ruleLoader.LoadAllRules()
    if err != nil {
        return nil, fmt.Errorf("failed to load rules: %w", err)
    }
    
    engine := &EngineV2{
        db:         db,
        rules:      rules,
        ruleLoader: ruleLoader,
        celEnv:     celEnv,
    }
    
    // Start watching for rule changes
    go engine.watchRuleChanges(context.Background())
    
    return engine, nil
}

// EvaluateResource evaluates a resource against all applicable rules
func (e *EngineV2) EvaluateResource(resource interface{}) ([]*models.Insight, error) {
    log.Debugf("[EngineV2] Evaluating resource: %T", resource)
    
    resourceType := getResourceType(resource)
    applicableRules := e.getApplicableRules(resourceType)
    
    log.Debugf("[EngineV2] Found %d applicable rules for %s", 
        len(applicableRules), resourceType)
    
    var insights []*models.Insight
    
    for _, rule := range applicableRules {
        matched, score, err := e.evaluateRule(rule, resource)
        if err != nil {
            log.Errorf("[EngineV2] Failed to evaluate rule %s: %v", rule.Metadata.ID, err)
            continue
        }
        
        if matched {
            log.Infof("[EngineV2] Rule %s MATCHED with score %.2f", 
                rule.Metadata.ID, score)
            
            insight := e.createInsightFromRule(rule, resource, score)
            insights = append(insights, insight)
        }
    }
    
    log.Infof("[EngineV2] Generated %d insights", len(insights))
    return insights, nil
}

// evaluateRule evaluates a single rule against a resource
func (e *EngineV2) evaluateRule(rule *Rule, resource interface{}) (bool, float64, error) {
    // Convert resource to map for evaluation
    resourceMap, err := toMap(resource)
    if err != nil {
        return false, 0, fmt.Errorf("failed to convert resource to map: %w", err)
    }
    
    // Evaluate all conditions
    conditionResults := make([]bool, 0, len(rule.Detection.Conditions))
    
    for i, condition := range rule.Detection.Conditions {
        result, err := e.evaluateCondition(condition, resourceMap, resource)
        if err != nil {
            log.Errorf("[EngineV2] Condition %d failed: %v", i, err)
            return false, 0, err
        }
        
        log.Debugf("[EngineV2] Condition %d: %v", i, result)
        conditionResults = append(conditionResults, result)
    }
    
    // Aggregate results
    matched := aggregateResults(conditionResults, rule.Detection.Aggregation)
    if !matched {
        return false, 0, nil
    }
    
    // Calculate risk score with multipliers
    finalScore := e.calculateRiskScore(rule, resourceMap, resource)
    
    return true, finalScore, nil
}

// evaluateCondition evaluates a single condition
func (e *EngineV2) evaluateCondition(
    condition Condition, 
    resourceMap map[string]interface{}, 
    resource interface{},
) (bool, error) {
    switch condition.Type {
    case "cel":
        return e.evaluateCELCondition(condition, resource)
    
    case "jsonpath":
        return e.evaluateJSONPathCondition(condition, resourceMap)
    
    case "field":
        return e.evaluateFieldCondition(condition, resourceMap)
    
    default:
        return false, fmt.Errorf("unknown condition type: %s", condition.Type)
    }
}

// evaluateCELCondition evaluates a CEL expression
func (e *EngineV2) evaluateCELCondition(condition Condition, resource interface{}) (bool, error) {
    // Use pre-compiled program if available
    if condition.CompiledProgram != nil {
        out, _, err := condition.CompiledProgram.Eval(map[string]interface{}{
            "object": resource,
        })
        if err != nil {
            return false, fmt.Errorf("CEL evaluation failed: %w", err)
        }
        
        result, ok := out.Value().(bool)
        if !ok {
            return false, fmt.Errorf("CEL expression did not return boolean")
        }
        
        return result, nil
    }
    
    // Fallback to runtime compilation (slower)
    ast, issues := e.celEnv.Compile(condition.Expression)
    if issues != nil && issues.Err() != nil {
        return false, issues.Err()
    }
    
    prg, err := e.celEnv.Program(ast)
    if err != nil {
        return false, err
    }
    
    out, _, err := prg.Eval(map[string]interface{}{
        "object": resource,
    })
    if err != nil {
        return false, err
    }
    
    result, ok := out.Value().(bool)
    if !ok {
        return false, fmt.Errorf("CEL expression did not return boolean")
    }
    
    return result, nil
}

// evaluateJSONPathCondition evaluates a JSONPath expression
func (e *EngineV2) evaluateJSONPathCondition(
    condition Condition, 
    resourceMap map[string]interface{},
) (bool, error) {
    // Execute JSONPath query
    result, err := jsonpath.JsonPathLookup(resourceMap, condition.JSONPath)
    if err != nil {
        // Path not found is not an error, just returns false
        return false, nil
    }
    
    // Compare result with expected value using operator
    return compareValues(result, condition.Operator, condition.Value)
}

// calculateRiskScore calculates final risk score with multipliers
func (e *EngineV2) calculateRiskScore(
    rule *Rule, 
    resourceMap map[string]interface{},
    resource interface{},
) float64 {
    score := rule.RiskScoring.BaseScore
    
    // Apply multipliers
    for _, multiplier := range rule.RiskScoring.Multipliers {
        if multiplier.CompiledProgram != nil {
            out, _, err := multiplier.CompiledProgram.Eval(map[string]interface{}{
                "object": resource,
            })
            if err != nil {
                log.Warnf("Failed to evaluate multiplier: %v", err)
                continue
            }
            
            if matched, ok := out.Value().(bool); ok && matched {
                score *= multiplier.Factor
                log.Debugf("Applied multiplier (%.2fx): %s", 
                    multiplier.Factor, multiplier.Reason)
            }
        }
    }
    
    // Cap at 10.0
    if score > 10.0 {
        score = 10.0
    }
    
    return score
}

// watchRuleChanges watches for rule file changes and hot-reloads
func (e *EngineV2) watchRuleChanges(ctx context.Context) {
    err := e.ruleLoader.WatchAndReload(ctx, func(newRules []*Rule) {
        log.Info("[EngineV2] Reloading rules...")
        e.rules = newRules
        log.Infof("[EngineV2] Rules reloaded: %d active rules", len(newRules))
    })
    
    if err != nil {
        log.Errorf("[EngineV2] Rule watcher failed: %v", err)
    }
}
```

---

## YAML Rule System

### Rule Schema (JSON Schema)

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "KSAM Risk Rule",
  "type": "object",
  "required": ["metadata", "classification", "target", "detection", "insight"],
  "properties": {
    "metadata": {
      "type": "object",
      "required": ["id", "name", "version"],
      "properties": {
        "id": {
          "type": "string",
          "pattern": "^[a-z0-9-]+$",
          "description": "Unique rule identifier"
        },
        "name": {
          "type": "string",
          "minLength": 1,
          "description": "Human-readable rule name"
        },
        "version": {
          "type": "string",
          "pattern": "^\\d+\\.\\d+\\.\\d+$",
          "description": "Semantic version"
        },
        "author": {
          "type": "string",
          "format": "email"
        },
        "created": {
          "type": "string",
          "format": "date"
        },
        "updated": {
          "type": "string",
          "format": "date"
        },
        "tags": {
          "type": "array",
          "items": {
            "type": "string"
          }
        }
      }
    },
    "classification": {
      "type": "object",
      "required": ["category", "severity"],
      "properties": {
        "category": {
          "type": "string",
          "enum": ["rbac", "pod-security", "network", "secrets", "compliance"]
        },
        "severity": {
          "type": "string",
          "enum": ["critical", "high", "medium", "low", "info"]
        },
        "cis_section": {
          "type": "string"
        },
        "cis_level": {
          "type": "integer",
          "minimum": 1,
          "maximum": 2
        },
        "cvss_score": {
          "type": "number",
          "minimum": 0,
          "maximum": 10
        }
      }
    },
    "target": {
      "type": "object",
      "required": ["resource_types"],
      "properties": {
        "resource_types": {
          "type": "array",
          "items": {
            "type": "string",
            "enum": ["Pod", "ServiceAccount", "Role", "ClusterRole", "RoleBinding", "ClusterRoleBinding", "Namespace", "NetworkPolicy"]
          },
          "minItems": 1
        },
        "namespaces": {
          "type": "array",
          "items": {
            "type": "string"
          }
        },
        "clusters": {
          "type": "array",
          "items": {
            "type": "string"
          }
        }
      }
    },
    "detection": {
      "type": "object",
      "required": ["aggregation", "conditions"],
      "properties": {
        "aggregation": {
          "type": "string",
          "enum": ["AND", "OR", "THRESHOLD"]
        },
        "conditions": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["type"],
            "properties": {
              "type": {
                "type": "string",
                "enum": ["cel", "jsonpath", "field"]
              },
              "expression": {
                "type": "string"
              },
              "jsonpath": {
                "type": "string"
              },
              "field": {
                "type": "string"
              },
              "operator": {
                "type": "string",
                "enum": ["eq", "ne", "contains", "matches", "in", "exists"]
              },
              "value": {}
            }
          },
          "minItems": 1
        }
      }
    },
    "risk_scoring": {
      "type": "object",
      "required": ["base_score"],
      "properties": {
        "base_score": {
          "type": "number",
          "minimum": 0,
          "maximum": 10
        },
        "multipliers": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["condition", "factor"],
            "properties": {
              "condition": {
                "type": "string"
              },
              "factor": {
                "type": "number",
                "minimum": 0
              },
              "reason": {
                "type": "string"
              }
            }
          }
        }
      }
    },
    "insight": {
      "type": "object",
      "required": ["title", "description", "remediation"],
      "properties": {
        "title": {
          "type": "string"
        },
        "description": {
          "type": "string"
        },
        "evidence": {
          "type": "array",
          "items": {
            "type": "string"
          }
        },
        "remediation": {
          "type": "array",
          "items": {
            "type": "string"
          },
          "minItems": 1
        },
        "references": {
          "type": "array",
          "items": {
            "type": "string",
            "format": "uri"
          }
        }
      }
    },
    "testing": {
      "type": "object",
      "properties": {
        "enabled": {
          "type": "boolean"
        },
        "test_cases": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["name", "resource", "expect_match"],
            "properties": {
              "name": {
                "type": "string"
              },
              "resource": {
                "type": "object"
              },
              "expect_match": {
                "type": "boolean"
              },
              "expected_score": {
                "type": "number"
              }
            }
          }
        }
      }
    }
  }
}
```

### Example Rules

#### CIS 5.2.1: Privileged Container

```yaml
# rules/cis-5.2.1.yaml
---
metadata:
  id: cis-5.2.1
  name: "Privileged container detected"
  version: "1.0.0"
  author: "security@ksam.io"
  created: "2025-01-01"
  updated: "2025-11-29"
  tags:
    - cis
    - pod-security
    - privileged

classification:
  category: pod-security
  severity: critical
  cis_section: "5.2.1"
  cis_level: 1
  cvss_score: 9.0

target:
  resource_types:
    - Pod
  namespaces:
    - "*"
  clusters:
    - "*"

detection:
  aggregation: OR
  conditions:
    # Check containers array
    - type: cel
      expression: |
        object.spec.containers.exists(c, 
          has(c.securityContext) && 
          has(c.securityContext.privileged) && 
          c.securityContext.privileged == true
        )
    
    # Check initContainers array
    - type: cel
      expression: |
        has(object.spec.initContainers) &&
        object.spec.initContainers.exists(c, 
          has(c.securityContext) && 
          has(c.securityContext.privileged) && 
          c.securityContext.privileged == true
        )

risk_scoring:
  base_score: 9.0
  multipliers:
    - condition: 'object.spec.hostNetwork == true'
      factor: 1.1
      reason: "Privileged + hostNetwork"
    
    - condition: 'object.spec.hostPID == true'
      factor: 1.05
      reason: "Privileged + hostPID"

insight:
  title: "Pod {{ .metadata.name }} contains privileged container"
  description: |
    The pod {{ .metadata.name }} in namespace {{ .metadata.namespace }} 
    has one or more containers running in privileged mode.
    
    **Risk**: Privileged containers have access to all Linux kernel capabilities 
    and devices, effectively gaining root access to the host system.
    
  evidence:
    - "Pod: {{ .metadata.namespace }}/{{ .metadata.name }}"
    - "Privileged Containers: {{ .spec.containers | selectattr('securityContext.privileged', 'equalto', true) | map(attribute='name') | join(', ') }}"
  
  remediation:
    - "Remove privileged: true from container security context"
    - "Identify required capabilities and add them specifically:"
    - "  securityContext:"
    - "    capabilities:"
    - "      add: [NET_ADMIN]  # Only add needed capabilities"
    - "kubectl patch pod {{ .metadata.name }} -n {{ .metadata.namespace }} --type='json' -p='[{\"op\": \"remove\", \"path\": \"/spec/containers/0/securityContext/privileged\"}]'"
  
  references:
    - "https://kubernetes.io/docs/concepts/security/pod-security-standards/"
    - "https://www.cisecurity.org/benchmark/kubernetes"

testing:
  enabled: true
  test_cases:
    - name: "Detects privileged container"
      resource:
        apiVersion: v1
        kind: Pod
        metadata:
          name: test-pod
          namespace: default
        spec:
          containers:
            - name: nginx
              image: nginx
              securityContext:
                privileged: true
      expect_match: true
      expected_score: 9.0
    
    - name: "Ignores non-privileged pod"
      resource:
        apiVersion: v1
        kind: Pod
        metadata:
          name: test-pod
          namespace: default
        spec:
          containers:
            - name: nginx
              image: nginx
              securityContext:
                privileged: false
      expect_match: false
```

#### CIS 5.3.1: Missing NetworkPolicy

```yaml
# rules/cis-5.3.1.yaml
---
metadata:
  id: cis-5.3.1
  name: "Namespace without NetworkPolicy"
  version: "1.0.0"
  author: "security@ksam.io"
  created: "2025-01-01"
  updated: "2025-11-29"
  tags:
    - cis
    - network
    - network-policy

classification:
  category: network
  severity: high
  cis_section: "5.3.1"
  cis_level: 2
  cvss_score: 7.5

target:
  resource_types:
    - Namespace
  namespaces:
    - "*"
  clusters:
    - "*"

detection:
  aggregation: AND
  conditions:
    # Namespace has pods
    - type: jsonpath
      jsonpath: "$.metadata.annotations['ksam.io/pod-count']"
      operator: exists
    
    # Namespace has no network policies
    - type: jsonpath
      jsonpath: "$.metadata.annotations['ksam.io/network-policy-count']"
      operator: eq
      value: "0"
    
    # Not a system namespace
    - type: cel
      expression: |
        !(object.metadata.name in ["kube-system", "kube-public", "kube-node-lease", "default"])

risk_scoring:
  base_score: 7.5
  multipliers:
    - condition: 'int(object.metadata.annotations["ksam.io/pod-count"]) > 10'
      factor: 1.2
      reason: "Many pods without network isolation"
    
    - condition: 'has(object.metadata.annotations["ksam.io/has-external-service"]) && object.metadata.annotations["ksam.io/has-external-service"] == "true"'
      factor: 1.3
      reason: "External services without network policies"

insight:
  title: "Namespace {{ .metadata.name }} has no NetworkPolicy"
  description: |
    The namespace {{ .metadata.name }} contains {{ .metadata.annotations["ksam.io/pod-count"] }} 
    pods but has no NetworkPolicy defined.
    
    **Risk**: Without NetworkPolicies, all pod-to-pod traffic is allowed by default, 
    enabling lateral movement in case of compromise.
    
  evidence:
    - "Namespace: {{ .metadata.name }}"
    - "Pod Count: {{ .metadata.annotations['ksam.io/pod-count'] }}"
    - "Network Policies: 0"
  
  remediation:
    - "Create a default deny-all NetworkPolicy:"
    - "kubectl apply -f - <<EOF"
    - "apiVersion: networking.k8s.io/v1"
    - "kind: NetworkPolicy"
    - "metadata:"
    - "  name: default-deny-all"
    - "  namespace: {{ .metadata.name }}"
    - "spec:"
    - "  podSelector: {}"
    - "  policyTypes:"
    - "  - Ingress"
    - "  - Egress"
    - "EOF"
    - "Then create specific allow policies as needed"
  
  references:
    - "https://kubernetes.io/docs/concepts/services-networking/network-policies/"
    - "https://www.cisecurity.org/benchmark/kubernetes"

testing:
  enabled: true
  test_cases:
    - name: "Detects namespace without NetworkPolicy"
      resource:
        apiVersion: v1
        kind: Namespace
        metadata:
          name: test-namespace
          annotations:
            ksam.io/pod-count: "5"
            ksam.io/network-policy-count: "0"
      expect_match: true
      expected_score: 7.5
    
    - name: "Ignores namespace with NetworkPolicy"
      resource:
        apiVersion: v1
        kind: Namespace
        metadata:
          name: test-namespace
          annotations:
            ksam.io/pod-count: "5"
            ksam.io/network-policy-count: "2"
      expect_match: false
```

---

## Implementation Plan

### Phase 1: Core Infrastructure (Week 1)

**Tasks**:
1. ✅ Create rule YAML schema
2. ✅ Implement RuleLoader with validation
3. ✅ Add CEL engine integration
4. ✅ Add JSONPath support
5. ✅ Create EngineV2 with new architecture

**Deliverables**:
- `pkg/riskengine/loader/` package
- `pkg/riskengine/engine_v2.go`
- `config/rule-schema.json`
- Unit tests for loader

### Phase 2: Rule Migration (Week 2)

**Tasks**:
1. ✅ Convert 5 existing rules to YAML
2. ✅ Create all 26 CIS rules in YAML
3. ✅ Test each rule individually
4. ✅ Create rule testing framework

**Deliverables**:
- `rules/` directory with 31+ YAML files
- `pkg/riskengine/testing/` test framework
- Rule validation tests

### Phase 3: Integration (Week 3)

**Tasks**:
1. ✅ Replace old engine with EngineV2
2. ✅ Update Risk Worker to use new engine
3. ✅ Add hot-reload support
4. ✅ Migration script for existing insights

**Deliverables**:
- Updated Risk Worker
- Hot-reload functionality
- Migration documentation

### Phase 4: Advanced Features (Week 4)

**Tasks**:
1. ✅ Rule versioning system
2. ✅ Rule dependencies
3. ✅ Rule impact analysis
4. ✅ Web UI for rule management

**Deliverables**:
- Rule version control
- Dependency resolver
- Impact analyzer
- Rule management UI

---

## Migration Strategy

### Step 1: Parallel Running (Safety Net)

Run both old and new engines in parallel:

```go
// In Risk Worker
func (w *RiskWorker) processMessage(msg *nats.Msg) error {
    // ... parse resource ...
    
    // OLD ENGINE (existing)
    insightsOld, err := w.oldEngine.EvaluateResource(ctx, resourceType, resourceData)
    if err != nil {
        log.Errorf("Old engine failed: %v", err)
    }
    
    // NEW ENGINE (v2)
    insightsNew, err := w.engineV2.EvaluateResource(resource)
    if err != nil {
        log.Errorf("New engine failed: %v", err)
    }
    
    // COMPARISON (for validation)
    compareInsights(insightsOld, insightsNew)
    
    // USE NEW ENGINE RESULTS
    insights = insightsNew
    
    // ... create insights ...
}
```

### Step 2: Gradual Rule Migration

Migrate rules one category at a time:

**Week 1**: RBAC rules (cis-5.1.x)  
**Week 2**: Pod Security rules (cis-5.2.x)  
**Week 3**: Network rules (cis-5.3.x)  
**Week 4**: Secrets rules (cis-5.4.x)  

### Step 3: Validation & Testing

For each migrated rule:

```bash
# Run rule test
go test -v ./pkg/riskengine/testing -run TestRule_CIS_5_1_3

# Compare outputs
./scripts/compare-engine-outputs.sh cis-5.1.3

# Check metrics
curl http://localhost:9090/metrics | grep rule_evaluation
```

### Step 4: Switch Over

After validation:

```go
// Remove old engine
// Use only engineV2
insights, err := w.engineV2.EvaluateResource(resource)
```

---

## Testing Strategy

### 1. Rule Unit Tests

```go
// File: pkg/riskengine/testing/rule_test.go

func TestRule_CIS_5_1_3(t *testing.T) {
    // Load rule
    loader := loader.NewRuleLoader("../../rules", "../../config/rule-schema.json")
    rule, err := loader.LoadRule("rules/cis-5.1.3.yaml")
    require.NoError(t, err)
    
    // Test cases from YAML
    for _, tc := range rule.Testing.TestCases {
        t.Run(tc.Name, func(t *testing.T) {
            engine := NewEngineV2(nil, "../../rules")
            
            matched, score, err := engine.evaluateRule(rule, tc.Resource)
            require.NoError(t, err)
            
            assert.Equal(t, tc.ExpectMatch, matched, "Match result mismatch")
            
            if tc.ExpectedScore != nil {
                assert.InDelta(t, *tc.ExpectedScore, score, 0.1, "Score mismatch")
            }
        })
    }
}
```

### 2. Integration Tests

```go
func TestEngineV2_ClusterRoleBinding_EndToEnd(t *testing.T) {
    // Setup
    db := setupTestDB(t)
    engine := NewEngineV2(db, "testdata/rules")
    
    // Create test ClusterRoleBinding
    crb := &models.ClusterRoleBinding{
        Name: "test-admin-binding",
        RoleRef: models.RoleRef{
            Kind: "ClusterRole",
            Name: "cluster-admin",
        },
        Subjects: []models.Subject{
            {
                Kind:      "ServiceAccount",
                Name:      "test-sa",
                Namespace: "default",
            },
        },
    }
    
    // Evaluate
    insights, err := engine.EvaluateResource(crb)
    require.NoError(t, err)
    
    // Assert
    assert.Len(t, insights, 1, "Should generate 1 insight")
    assert.Equal(t, "cis-5.1.3", insights[0].RuleID)
    assert.Equal(t, "critical", insights[0].Severity)
    assert.GreaterOrEqual(t, insights[0].RiskScore, 9.0)
}
```

### 3. Performance Tests

```go
func BenchmarkEngineV2_Evaluation(b *testing.B) {
    engine := NewEngineV2(nil, "rules")
    crb := createTestClusterRoleBinding()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := engine.EvaluateResource(crb)
        if err != nil {
            b.Fatal(err)
        }
    }
}

// Target: <10ms per evaluation
```

---

## Benefits Summary

### 🎯 Flexibility
- ✅ Add rules without code rebuild
- ✅ Hot-reload rules in production
- ✅ Version control individual rules
- ✅ A/B test rule changes

### 🔒 Safety
- ✅ Schema validation prevents errors
- ✅ Pre-compilation catches CEL errors
- ✅ Test cases in YAML ensure correctness
- ✅ Gradual migration reduces risk

### 🚀 Performance
- ✅ Pre-compiled CEL (5-10x faster)
- ✅ Efficient JSONPath queries
- ✅ Cached rule evaluation
- ✅ Parallel rule execution

### 📊 Observability
- ✅ Per-rule metrics
- ✅ Rule evaluation timing
- ✅ Match rate tracking
- ✅ False positive detection

### 👥 Collaboration
- ✅ Security team can add rules
- ✅ Git-based rule management
- ✅ Pull request workflow
- ✅ Rule documentation in YAML

---

## Next Steps

### Immediate (Do This Week)
1. ✅ Review this proposal with team
2. ✅ Create proof-of-concept with 1-2 rules
3. ✅ Benchmark CEL vs current approach
4. ✅ Decide on migration timeline

### Short-term (Next 2 Weeks)
5. ✅ Implement Phase 1 (Core Infrastructure)
6. ✅ Convert 5 existing rules to YAML
7. ✅ Test in development environment
8. ✅ Create rule authoring guide

### Long-term (Next Month)
9. ✅ Complete Phase 2-4
10. ✅ Migrate all rules to YAML
11. ✅ Deploy to production
12. ✅ Build rule management UI

---

**Status**: Ready for Review  
**Estimated Effort**: 4 weeks (1 developer)  
**Risk Level**: Medium (gradual migration mitigates risk)

