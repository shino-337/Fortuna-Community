package riskengine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEvaluationReportsRuleFailure(t *testing.T) {
	rule := Rule{ID: "broken", Enabled: true, Category: CategoryRBAC, Conditions: []Condition{{Type: ConditionType("unsupported")}}}
	e := &Engine{rules: []Rule{rule}}
	if _, err := e.EvaluateResource(context.Background(), "Role", map[string]interface{}{}); err == nil {
		t.Fatal("base evaluator hid rule failure")
	}
	ye := &YAMLEngine{Engine: e}
	if _, err := ye.EvaluateResource(context.Background(), "Role", map[string]interface{}{}); err == nil {
		t.Fatal("YAML evaluator hid rule failure")
	}
}
func TestConfiguredCatalogRejectsPartialAndEmptyLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FORTUNA_RULES_DIR", dir)
	if _, err := NewConfiguredYAMLEngine(nil); err == nil {
		t.Fatal("empty catalog accepted")
	}
	valid := []byte("id: valid\nname: Valid\nseverity: high\ncategory: rbac\nbase_score: 5\nenabled: true\nconditions:\n  - type: expression\n    expression: 'false'\n")
	if err := os.WriteFile(filepath.Join(dir, "valid.yaml"), valid, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewConfiguredYAMLEngine(nil); err != nil {
		t.Fatalf("valid catalog: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.yaml"), []byte("id: ["), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewConfiguredYAMLEngine(nil); err == nil {
		t.Fatal("partial catalog accepted")
	}
}
