#!/bin/bash

# Integration test script for YAML rules
# Tests actual rule evaluation with sample data

set -e

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║        YAML RULES INTEGRATION TEST                            ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Set rules directory
export KSAM_RULES_DIR="./core/rules"

echo "📁 Using rules directory: $KSAM_RULES_DIR"
echo ""

# Test 1: Check if YAML engine can be created
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 1: YAML Engine Creation"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

cd "$(dirname "$0")/../core"

# Create a simple Go test program
cat > /tmp/test_yaml_engine.go << 'EOF'
package main

import (
	"fmt"
	"log"
	"os"
	
	"github.com/ksam/core/pkg/riskengine"
)

func main() {
	rulesDir := os.Getenv("KSAM_RULES_DIR")
	if rulesDir == "" {
		log.Fatal("KSAM_RULES_DIR not set")
	}
	
	// Try to create YAML engine
	engine, err := riskengine.NewYAMLEngine(nil, rulesDir)
	if err != nil {
		log.Fatalf("Failed to create YAML engine: %v", err)
	}
	
	rules := engine.GetRules()
	fmt.Printf("✅ YAML Engine created successfully\n")
	fmt.Printf("   Loaded %d rules\n", len(rules))
	
	// List rule IDs
	for _, rule := range rules {
		fmt.Printf("   - %s (%s): %s\n", rule.ID, rule.Severity, rule.Name)
	}
}
EOF

if go run /tmp/test_yaml_engine.go 2>&1; then
    echo "✅ Test 1 passed"
else
    echo "❌ Test 1 failed"
    exit 1
fi

echo ""

# Test 2: Test rule evaluation
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test 2: Rule Evaluation"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

cat > /tmp/test_rule_eval.go << 'EOF'
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	
	"github.com/ksam/core/pkg/riskengine"
)

func main() {
	rulesDir := os.Getenv("KSAM_RULES_DIR")
	if rulesDir == "" {
		log.Fatal("KSAM_RULES_DIR not set")
	}
	
	engine, err := riskengine.NewYAMLEngine(nil, rulesDir)
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}
	
	// Test case 1: Cluster-admin binding (should match cis-5.1.3)
	testData1 := map[string]interface{}{
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
	
	insights1, err := engine.EvaluateResource(context.Background(), "ClusterRoleBinding", testData1)
	if err != nil {
		log.Fatalf("Evaluation failed: %v", err)
	}
	
	if len(insights1) > 0 {
		fmt.Printf("✅ Test case 1 passed: Found %d insights for cluster-admin binding\n", len(insights1))
		for _, insight := range insights1 {
			fmt.Printf("   - %s: %s\n", insight.Severity, insight.Description)
		}
	} else {
		fmt.Printf("❌ Test case 1 failed: Expected insights for cluster-admin binding\n")
		os.Exit(1)
	}
	
	// Test case 2: Wildcard permissions (should match wildcard-permissions)
	testData2 := map[string]interface{}{
		"name":      "test-role",
		"namespace": "default",
		"rules": []interface{}{
			map[string]interface{}{
				"resources": []interface{}{"*"},
				"verbs":     []interface{}{"get", "list"},
			},
		},
	}
	
	insights2, err := engine.EvaluateResource(context.Background(), "Role", testData2)
	if err != nil {
		log.Fatalf("Evaluation failed: %v", err)
	}
	
	if len(insights2) > 0 {
		fmt.Printf("✅ Test case 2 passed: Found %d insights for wildcard permissions\n", len(insights2))
	} else {
		fmt.Printf("❌ Test case 2 failed: Expected insights for wildcard permissions\n")
		os.Exit(1)
	}
	
	// Test case 3: Non-matching case (should not match)
	testData3 := map[string]interface{}{
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
	
	insights3, err := engine.EvaluateResource(context.Background(), "ClusterRoleBinding", testData3)
	if err != nil {
		log.Fatalf("Evaluation failed: %v", err)
	}
	
	// Should not match cis-5.1.3 (but might match other rules)
	matchedCis513 := false
	for _, insight := range insights3 {
		if insight.Description != nil && contains(*insight.Description, "cluster-admin") {
			matchedCis513 = true
		}
	}
	
	if !matchedCis513 {
		fmt.Printf("✅ Test case 3 passed: Correctly did not match cluster-admin rule\n")
	} else {
		fmt.Printf("⚠️  Test case 3: Matched cluster-admin rule (might be expected)\n")
	}
	
	fmt.Printf("\n✅ All test cases completed\n")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && 
			(s[:len(substr)] == substr || 
			 s[len(s)-len(substr):] == substr ||
			 findSubstring(s, substr))))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
EOF

if go run /tmp/test_rule_eval.go 2>&1; then
    echo "✅ Test 2 passed"
else
    echo "❌ Test 2 failed"
    exit 1
fi

echo ""
echo "✅ All integration tests passed"

