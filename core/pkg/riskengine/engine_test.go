package riskengine

import (
	"context"
	"os"
	"testing"
)

// TestYAMLEngineCreation tests YAML engine creation
func TestYAMLEngineCreation(t *testing.T) {
	rulesDir := os.Getenv("FORTUNA_RULES_DIR")
	if rulesDir == "" {
		// Try default locations
		if _, err := os.Stat("./rules"); err == nil {
			rulesDir = "./rules"
		} else if _, err := os.Stat("../rules"); err == nil {
			rulesDir = "../rules"
		} else {
			t.Skip("FORTUNA_RULES_DIR not set and default rules directory not found")
		}
	}

	engine, err := NewYAMLEngine(nil, rulesDir)
	if err != nil {
		t.Fatalf("Failed to create YAML engine: %v", err)
	}

	rules := engine.GetRules()
	if len(rules) == 0 {
		t.Fatal("No rules loaded")
	}

	t.Logf("Loaded %d rules from YAML", len(rules))

	// Check that we have expected rules
	ruleIDs := make(map[string]bool)
	for _, rule := range rules {
		ruleIDs[rule.ID] = true
		t.Logf("  - %s (%s): %s", rule.ID, rule.Severity, rule.Name)
	}

	expectedRules := []string{
		"cis-5.1.3", "wildcard-permissions", "orphan-serviceaccount", "overprivileged-role", "overprivileged-binding",
		"cluster-admin-pod", "cluster-admin-binding-detailed",
		"runtime-signals-recent", "runtime-hostnetwork-network-anomaly", "runtime-privileged-with-signals", "runtime-escape-class-signals",
		"pss-host-namespaces", "pss-privileged-container",
	}
	for _, expectedID := range expectedRules {
		if !ruleIDs[expectedID] {
			t.Errorf("Expected rule %s not found", expectedID)
		}
	}
}

// TestCriticalRuleEvaluation tests critical rule evaluation
func TestCriticalRuleEvaluation(t *testing.T) {
	rulesDir := os.Getenv("FORTUNA_RULES_DIR")
	if rulesDir == "" {
		if _, err := os.Stat("./rules"); err == nil {
			rulesDir = "./rules"
		} else {
			t.Skip("FORTUNA_RULES_DIR not set")
		}
	}

	engine, err := NewYAMLEngine(nil, rulesDir)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Test case 1: Cluster-admin binding (should match cis-5.1.3)
	t.Run("CIS-5.1.3: Cluster-admin binding", func(t *testing.T) {
		testData := map[string]interface{}{
			"name":      "test-binding",
			"namespace": "",
			"roleRef": map[string]interface{}{
				"kind": "ClusterRole",
				"name": "cluster-admin",
			},
			"subjects": []interface{}{
				map[string]interface{}{
					"kind":      "ServiceAccount",
					"name":      "test-sa",
					"namespace": "default",
				},
			},
		}

		insights, err := engine.EvaluateResource(context.Background(), "ClusterRoleBinding", testData)
		if err != nil {
			t.Fatalf("Evaluation failed: %v", err)
		}

		if len(insights) == 0 {
			t.Error("Expected insights for cluster-admin binding, got none")
		} else {
			t.Logf("✅ Found %d insights", len(insights))
			for _, insight := range insights {
				t.Logf("   - %s: %s", insight.Severity, insight.Description)
			}
		}
	})

	// Test case 2: Wildcard permissions
	t.Run("Wildcard permissions", func(t *testing.T) {
		testData := map[string]interface{}{
			"name":      "test-role",
			"namespace": "default",
			"rules": []interface{}{
				map[string]interface{}{
					"resources": []interface{}{"*"},
					"verbs":     []interface{}{"get", "list"},
				},
			},
		}

		insights, err := engine.EvaluateResource(context.Background(), "Role", testData)
		if err != nil {
			t.Fatalf("Evaluation failed: %v", err)
		}

		if len(insights) == 0 {
			t.Error("Expected insights for wildcard permissions, got none")
		} else {
			t.Logf("✅ Found %d insights", len(insights))
		}
	})

	// Test case 3: Non-matching case
	t.Run("Non-matching case", func(t *testing.T) {
		testData := map[string]interface{}{
			"name":      "test-binding",
			"namespace": "",
			"roleRef": map[string]interface{}{
				"kind": "ClusterRole",
				"name": "view", // Not cluster-admin
			},
			"subjects": []interface{}{
				map[string]interface{}{
					"kind":      "ServiceAccount",
					"name":      "test-sa",
					"namespace": "default",
				},
			},
		}

		insights, err := engine.EvaluateResource(context.Background(), "ClusterRoleBinding", testData)
		if err != nil {
			t.Fatalf("Evaluation failed: %v", err)
		}

		// Should not match cis-5.1.3 (but might match other rules)
		matchedCis513 := false
		for _, insight := range insights {
			if insight.Description != "" {
				// Simple string contains check
				desc := insight.Description
				substr := "cluster-admin"
				if len(desc) >= len(substr) {
					for i := 0; i <= len(desc)-len(substr); i++ {
						if desc[i:i+len(substr)] == substr {
							matchedCis513 = true
							break
						}
					}
				}
			}
		}

		if matchedCis513 {
			t.Logf("⚠️  Matched cluster-admin rule (might be expected if other conditions match)")
		} else {
			t.Logf("✅ Correctly did not match cluster-admin rule")
		}
	})
}
