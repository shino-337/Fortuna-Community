package riskengine

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/fortuna/core/pkg/models"
)

func TestCIS51_4_ExpressionMatchesWhenRulesIsJSONString(t *testing.T) {
	// Build absolute rulesDir based on this file location.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	pkgDir := filepath.Dir(thisFile) // .../core/pkg/riskengine
	repoRoot := filepath.Clean(filepath.Join(pkgDir, "..", "..", "..")) // .../KSAM
	rulesDir := filepath.Join(repoRoot, "core", "rules")

	ye, err := NewYAMLEngine(nil, rulesDir)
	if err != nil {
		t.Fatalf("NewYAMLEngine: %v", err)
	}

	// ClusterRole stores rules in DB as a JSON string. CEL rule cis-5.1.4 expects:
	//   object.rules.exists(r, r.resources.exists(... 'pods') && r.verbs.exists(... 'create'|'*'))
	// so we simulate the same shape here.
	rulesJSONString := `[{"verbs":["create"],"apiGroups":[""],"resources":["pods"]}]`

	resourceData := map[string]interface{}{
		"kind":       "ClusterRole",
		"name":       "test-role",
		"namespace":  "",
		"cluster_id": "cluster-1",
		"uid":        "clusterrole-uid-1",
		"rules":      rulesJSONString,
	}

	insights, err := ye.EvaluateResource(context.Background(), "ClusterRole", resourceData)
	if err != nil {
		t.Fatalf("EvaluateResource: %v", err)
	}

	wantTitle := "Role allows creating pods"
	found := false
	for _, in := range insights {
		if in != nil && in.Title == wantTitle {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected insight title=%q, got %d insights (titles: %v)", wantTitle, len(insights), insightTitlesCIS(insights))
	}
}

// Helpers kept local to avoid pulling in other test dependencies.
func insightTitlesCIS(insights []*models.Insight) []string {
	out := make([]string, 0, len(insights))
	for _, in := range insights {
		if in == nil {
			continue
		}
		out = append(out, in.Title)
	}
	return out
}

