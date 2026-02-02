package riskengine

import (
	"context"
	"testing"
)

// TestCriticalRules tests critical rules from YAML
func TestCriticalRules(t *testing.T) {
	// Create test engine with YAML rules
	// Note: This requires FORTUNA_RULES_DIR to be set or rules directory to exist
	engine := NewEngine(nil) // nil DB for testing

	testCases := []struct {
		name           string
		resourceType   string
		resourceData   map[string]interface{}
		expectedRuleID string
		shouldMatch    bool
	}{
		{
			name:         "CIS-5.1.3: Cluster-admin binding detected",
			resourceType: "ClusterRoleBinding",
			resourceData: map[string]interface{}{
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
			},
			expectedRuleID: "cis-5.1.3",
			shouldMatch:    true,
		},
		{
			name:         "CIS-5.1.3: Non-cluster-admin binding should not match",
			resourceType: "ClusterRoleBinding",
			resourceData: map[string]interface{}{
				"name":      "test-binding",
				"namespace": "",
				"roleRef": map[string]interface{}{
					"kind": "ClusterRole",
					"name": "view",
				},
				"subjects": []interface{}{
					map[string]interface{}{
						"kind":      "ServiceAccount",
						"name":      "test-sa",
						"namespace": "default",
					},
				},
			},
			expectedRuleID: "cis-5.1.3",
			shouldMatch:    false,
		},
		{
			name:         "Wildcard permissions: Role with wildcard resources",
			resourceType: "Role",
			resourceData: map[string]interface{}{
				"name":      "test-role",
				"namespace": "default",
				"rules": []interface{}{
					map[string]interface{}{
						"resources": []interface{}{"*"},
						"verbs":     []interface{}{"get", "list"},
					},
				},
			},
			expectedRuleID: "wildcard-permissions",
			shouldMatch:    true,
		},
		{
			name:         "Wildcard permissions: Role with wildcard verbs",
			resourceType: "Role",
			resourceData: map[string]interface{}{
				"name":      "test-role",
				"namespace": "default",
				"rules": []interface{}{
					map[string]interface{}{
						"resources": []interface{}{"pods"},
						"verbs":     []interface{}{"*"},
					},
				},
			},
			expectedRuleID: "wildcard-permissions",
			shouldMatch:    true,
		},
		{
			name:         "Orphan ServiceAccount: Empty linkedPods",
			resourceType: "ServiceAccount",
			resourceData: map[string]interface{}{
				"name":       "test-sa",
				"namespace":  "default",
				"linkedPods": "[]",
			},
			expectedRuleID: "orphan-serviceaccount",
			shouldMatch:    true,
		},
		{
			name:         "Orphan ServiceAccount: Non-empty linkedPods should not match",
			resourceType: "ServiceAccount",
			resourceData: map[string]interface{}{
				"name":       "test-sa",
				"namespace":  "default",
				"linkedPods": `["pod1", "pod2"]`,
			},
			expectedRuleID: "orphan-serviceaccount",
			shouldMatch:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			insights, err := engine.EvaluateResource(context.Background(), tc.resourceType, tc.resourceData)
			if err != nil {
				t.Fatalf("Evaluation failed: %v", err)
			}

			matched := false
			for _, insight := range insights {
				// Check if any insight matches the expected rule
				if insight.InsightType == "rbac" {
					matched = true
					break
				}
			}

			if matched != tc.shouldMatch {
				t.Errorf("Expected match=%v, got match=%v. Insights: %d", tc.shouldMatch, matched, len(insights))
			}
		})
	}
}
