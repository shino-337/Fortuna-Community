package riskengine

import (
	"github.com/fortuna/core/pkg/models"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// CanResolveFromRoleSnapshot accepts only detectors whose inputs are entirely
// contained in a Role/ClusterRole snapshot. Unknown dependencies fail closed.
// Runtime and cross-resource absence require collector coverage (work package D2).
func (ye *YAMLEngine) CanResolveFromRoleSnapshot(f *models.Insight) bool {
	if f.ResourceType != "Role" && f.ResourceType != "ClusterRole" {
		return false
	}
	ye.mu.RLock()
	defer ye.mu.RUnlock()
	matches := 0
	for _, r := range ye.rules {
		same := r.ID == f.CVEID
		if f.CVEID == "" {
			same = r.Name == f.Title
		}
		if same && r.Enabled && string(r.Category) == f.InsightType && ruleMatchesResourceType(f.ResourceType, r) {
			matches++
		}
	}
	if matches != 1 {
		return false
	}
	for _, r := range ye.rules {
		same := r.ID == f.CVEID
		if f.CVEID == "" {
			same = r.Name == f.Title
		}
		if !same || !r.Enabled || string(r.Category) != f.InsightType || !ruleMatchesResourceType(f.ResourceType, r) {
			continue
		}
		if r.Category != CategoryRBAC || len(r.Conditions) == 0 || ye.celCompiler == nil {
			return false
		}
		for _, c := range r.Conditions {
			// Expressions are checked structurally; bracket access/aliases cannot bypass
			// the allowlist by hiding a runtime or cross-resource dependency in a string.
			if c.Type != CondTypeExpression {
				return false
			}
			ast, issues := ye.celCompiler.env.Compile(c.Expression)
			if issues != nil && issues.Err() != nil {
				return false
			}
			if !roleSnapshotExpression(ast.Expr()) {
				return false
			}
		}
		return true
	}
	return false
}

func roleSnapshotExpression(e *exprpb.Expr) bool {
	if e == nil {
		return true
	}
	switch v := e.ExprKind.(type) {
	case *exprpb.Expr_ConstExpr:
		return true
	case *exprpb.Expr_IdentExpr:
		switch v.IdentExpr.Name {
		case "object", "metadata", "spec", "status":
			return false
		}
		return true
	case *exprpb.Expr_SelectExpr:
		if id := v.SelectExpr.Operand.GetIdentExpr(); id != nil && id.Name == "object" {
			switch v.SelectExpr.Field {
			case "rules", "name", "namespace", "uid", "cluster_id", "kind":
				return true
			}
			return false
		}
		return roleSnapshotExpression(v.SelectExpr.Operand)
	case *exprpb.Expr_CallExpr:
		if !roleSnapshotExpression(v.CallExpr.Target) {
			return false
		}
		for _, a := range v.CallExpr.Args {
			if !roleSnapshotExpression(a) {
				return false
			}
		}
		return true
	case *exprpb.Expr_ListExpr:
		for _, a := range v.ListExpr.Elements {
			if !roleSnapshotExpression(a) {
				return false
			}
		}
		return true
	case *exprpb.Expr_StructExpr:
		for _, a := range v.StructExpr.Entries {
			if !roleSnapshotExpression(a.GetMapKey()) || !roleSnapshotExpression(a.Value) {
				return false
			}
		}
		return true
	case *exprpb.Expr_ComprehensionExpr:
		c := v.ComprehensionExpr
		for _, n := range []string{c.IterVar, c.AccuVar} {
			switch n {
			case "object", "metadata", "spec", "status":
				return false
			}
		}
		for _, a := range []*exprpb.Expr{c.IterRange, c.AccuInit, c.LoopCondition, c.LoopStep, c.Result} {
			if !roleSnapshotExpression(a) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
