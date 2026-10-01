package worker

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/fortuna/core/pkg/riskengine"
)

func TestHistoricalRBACNormalizationHandlesNonResourceRulesAndRoleRefs(t *testing.T) {
	rules, err := historicalPolicyRules(`[{"verbs":["get"],"nonResourceURLs":["/api"]}]`)
	if err != nil {
		t.Fatal(err)
	}
	rule := rules[0].(map[string]interface{})
	if resources, ok := rule["resources"].([]interface{}); !ok || len(resources) != 0 {
		t.Fatalf("non-resource rule needs an empty CEL resource-list shape: %#v", rule)
	}
	roleRef, err := historicalRoleRef(`{"kind":"Role","name":"reader"}`)
	if err != nil {
		t.Fatal(err)
	}
	subjects, err := historicalSubjects(`[]`)
	if err != nil {
		t.Fatal(err)
	}
	ye, err := riskengine.NewYAMLEngine(nil, filepath.Join("..", "..", "rules"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ye.EvaluateResource(context.Background(), "ClusterRole", map[string]interface{}{
		"cluster_id": "cluster-a", "uid": "role-uid", "kind": "ClusterRole", "name": "discovery", "namespace": "", "rules": rules,
	}); err != nil {
		t.Fatalf("valid non-resource policy rule failed evaluation: %v", err)
	}
	if _, err := ye.EvaluateResource(context.Background(), "RoleBinding", map[string]interface{}{
		"cluster_id": "cluster-a", "uid": "binding-uid", "kind": "RoleBinding", "name": "binding", "namespace": "ns", "roleRef": roleRef, "subjects": subjects,
	}); err != nil {
		t.Fatalf("valid RoleBinding roleRef failed evaluation: %v", err)
	}
}

func TestHistoricalRBACNormalizationRejectsMalformedEvidence(t *testing.T) {
	for _, raw := range []string{"", "not-json", `[{"verbs":["get"]}]`} {
		if _, err := historicalPolicyRules(raw); err == nil {
			t.Fatalf("accepted incomplete policy rules: %q", raw)
		}
	}
	for _, raw := range []string{"", "not-json", `{"name":"reader"}`} {
		if _, err := historicalRoleRef(raw); err == nil {
			t.Fatalf("accepted incomplete roleRef: %q", raw)
		}
	}
	if _, err := historicalSubjects("not-json"); err == nil {
		t.Fatal("accepted malformed subjects")
	}
}
