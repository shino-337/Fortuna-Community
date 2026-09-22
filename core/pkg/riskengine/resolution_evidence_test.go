package riskengine

import (
	"github.com/fortuna/core/pkg/models"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestResolutionDetectorDependencies(t *testing.T) {
	compiler, err := NewCELCompiler()
	require.NoError(t, err)
	for _, tc := range []struct {
		name, expression string
		eligible         bool
	}{
		{"role-fields", "object.rules.exists(r, '*' in r.verbs)", true},
		{"constant", "false", true},
		{"runtime", "object.fortuna.signal_total_24h > 0", false},
		{"bracket-runtime", "object['fortuna']['signal_total_24h'] > 0", false},
		{"bare-object", "size(object) > 0", false},
		{"cross-resource", "object.bindings.size() > 0", false},
		{"alias", "[object].exists(x, x.fortuna.active)", false},
		{"metadata", "metadata.name == 'x'", false},
		{"shadow", "object.rules.exists(object, object.verbs.size() > 0)", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ye := &YAMLEngine{Engine: &Engine{rules: []Rule{{ID: "test", Name: "Test", Enabled: true, Category: CategoryRBAC, Conditions: []Condition{{Type: CondTypeExpression, Expression: tc.expression}}}}}, celCompiler: compiler}
			f := &models.Insight{ResourceType: "Role", CVEID: "test", InsightType: "rbac"}
			require.Equal(t, tc.eligible, ye.CanResolveFromRoleSnapshot(f))
			ye.rules = append(ye.rules, ye.rules[0])
			require.False(t, ye.CanResolveFromRoleSnapshot(f), "ambiguous detector")
			ye.rules = ye.rules[:1]
			f.ResourceType = "Pod"
			require.False(t, ye.CanResolveFromRoleSnapshot(f))
			f.ResourceType = "Role"
			ye.rules[0].Enabled = false
			require.False(t, ye.CanResolveFromRoleSnapshot(f))
		})
	}
}
