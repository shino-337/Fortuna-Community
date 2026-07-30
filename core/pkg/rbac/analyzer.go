// Package rbac provides shared RBAC analysis logic used by the Pod Capability
// Engine (PCE), the Attack Path Builder, and the Risk Engine.
//
// Previously each subsystem had its own RBAC classification logic:
//   - capability/evaluator.go → hasAPIWriteAccess() / hasWriteVerbs()
//   - graph/relational_path_builder.go → classifyRoleRisk()
//   - riskengine/yaml_engine.go → CEL expression conditions
//
// This package extracts that logic into a single authoritative implementation so
// all three callers produce consistent risk verdicts for the same role data.
package rbac

import (
	"context"
	"encoding/json"
	"strings"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// RiskLevel represents the assessed RBAC risk of a role.
type RiskLevel string

const (
	RiskLevelNone     RiskLevel = "none"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelCritical RiskLevel = "critical"
)

// RBACPath describes a single binding path from a subject to a role.
type RBACPath struct {
	BindingName string `json:"bindingName"`
	BindingKind string `json:"bindingKind"` // "RoleBinding" or "ClusterRoleBinding"
	RoleName    string `json:"roleName"`
	RoleKind    string `json:"roleKind"` // "Role" or "ClusterRole"
	RiskLevel   string `json:"riskLevel"`
}

// RBACAnalysis is the unified result of analysing a pod's RBAC exposure.
type RBACAnalysis struct {
	// HasWrite is true when at least one bound role grants mutating API access.
	HasWrite bool `json:"hasWrite"`
	// RiskLevel is the highest risk level found across all bound roles.
	RiskLevel string `json:"riskLevel"`
	// Paths contains every binding→role path that contributed to the result.
	Paths []RBACPath `json:"paths"`
	// Evidence holds human-readable key/value annotations used for insight
	// generation (e.g. {"role": "edit", "roleKind": "ClusterRole"}).
	Evidence map[string]interface{} `json:"evidence"`
}

// policyRule is the minimal rule shape stored in role.rules / cluster_role.rules JSONB.
type policyRule struct {
	Verbs     []string `json:"verbs"`
	Resources []string `json:"resources"`
	APIGroups []string `json:"apiGroups"`
}

// DerivedCapability maps RBAC verbs+resources to a semantic capability type
// used by the attack path engine for finer-grained objective classification.
type DerivedCapability struct {
	Type  string `json:"type"`
	Scope string `json:"scope"`
}

// DeriveCapabilities extracts semantic capabilities from RBAC policy rules.
// Mapping: secrets get→SECRET_READ, pods create→WORKLOAD_CREATE,
// pods/exec→EXEC_ACCESS, nodes/proxy→NODE_PROXY, csr approve→CERT_ISSUE.
func DeriveCapabilities(rulesJSON string) []DerivedCapability {
	if rulesJSON == "" || rulesJSON == "null" {
		return nil
	}
	var rules []policyRule
	if err := json.Unmarshal([]byte(rulesJSON), &rules); err != nil {
		return nil
	}
	seen := map[string]bool{}
	var caps []DerivedCapability
	add := func(t, s string) {
		key := t + ":" + s
		if seen[key] {
			return
		}
		seen[key] = true
		caps = append(caps, DerivedCapability{Type: t, Scope: s})
	}
	for _, r := range rules {
		hasWild := contains(r.Verbs, "*")
		scope := inferRBACScope(r.APIGroups)
		if (containsAny(r.Verbs, "get", "list", "watch") || hasWild) && containsAny(r.Resources, "secrets", "*") {
			add("SECRET_READ", scope)
		}
		if (contains(r.Verbs, "create") || hasWild) && containsAny(r.Resources, "pods", "deployments", "daemonsets", "jobs", "cronjobs", "statefulsets", "replicasets", "*") {
			add("WORKLOAD_CREATE", scope)
		}
		if containsAny(r.Resources, "pods/exec") && (containsAny(r.Verbs, "create", "get") || hasWild) {
			add("EXEC_ACCESS", scope)
		}
		if containsAny(r.Resources, "nodes/proxy", "nodes") && (containsAny(r.Verbs, "create", "get") || hasWild) {
			add("NODE_PROXY", scope)
		}
		if containsAny(r.Resources, "certificatesigningrequests", "certificatesigningrequests/approval") && (containsAny(r.Verbs, "update", "create", "approve") || hasWild) {
			add("CERT_ISSUE", scope)
		}
	}
	return caps
}

func inferRBACScope(apiGroups []string) string {
	if contains(apiGroups, "*") || contains(apiGroups, "") {
		return "cluster"
	}
	return "namespace"
}

// subject is one entry in the subjects JSONB array of a binding.
type subject struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// roleRef is the shape stored in role_bindings.role_ref / cluster_role_bindings.role_ref.
type roleRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// AnalyzePod performs a full RBAC analysis for the given pod and returns an
// RBACAnalysis that covers both boolean write-access detection (used by PCE)
// and granular risk classification (used by the Attack Path Builder).
//
// A gorm.ErrRecordNotFound from the RBAC tables is treated as "no bindings
// found" so the function is safe to call even when those tables are absent.
func AnalyzePod(ctx context.Context, db *gorm.DB, pod *models.Pod) (RBACAnalysis, error) {
	analysis := RBACAnalysis{
		RiskLevel: string(RiskLevelNone),
		Paths:     []RBACPath{},
		Evidence:  map[string]interface{}{},
	}

	// --- RoleBindings (namespace-scoped) ---
	var roleBindings []models.RoleBinding
	if err := db.WithContext(ctx).
		Where("cluster_id = ? AND namespace = ? AND deleted_at IS NULL", pod.ClusterID, pod.Namespace).
		Find(&roleBindings).Error; err != nil {
		if !isTableMissingError(err) {
			return analysis, err
		}
	}

	for _, rb := range roleBindings {
		if !bindingRefersToSA(rb.Subjects, pod.ServiceAccount, pod.Namespace) {
			continue
		}

		var ref roleRef
		if err := json.Unmarshal([]byte(rb.RoleRef), &ref); err != nil {
			continue
		}

		switch ref.Kind {
		case "Role":
			var role models.Role
			if err := db.WithContext(ctx).
				Where("cluster_id = ? AND name = ? AND namespace = ? AND deleted_at IS NULL",
					pod.ClusterID, ref.Name, pod.Namespace).
				First(&role).Error; err != nil {
				continue
			}
			rl := ClassifyRoleRisk(role.Name, role.Rules)
			path := RBACPath{
				BindingName: rb.Name,
				BindingKind: "RoleBinding",
				RoleName:    role.Name,
				RoleKind:    "Role",
				RiskLevel:   rl,
			}
			analysis.Paths = append(analysis.Paths, path)
			analysis.RiskLevel = string(maxRisk(RiskLevel(analysis.RiskLevel), RiskLevel(rl)))
			if HasWriteVerbs(role.Rules) {
				analysis.HasWrite = true
				analysis.Evidence["role"] = role.Name
				analysis.Evidence["roleKind"] = "Role"
			}
		case "ClusterRole":
			var cr models.ClusterRole
			if err := db.WithContext(ctx).
				Where("cluster_id = ? AND name = ? AND deleted_at IS NULL",
					pod.ClusterID, ref.Name).
				First(&cr).Error; err != nil {
				continue
			}
			rl := ClassifyRoleRisk(cr.Name, cr.Rules)
			path := RBACPath{
				BindingName: rb.Name,
				BindingKind: "RoleBinding",
				RoleName:    cr.Name,
				RoleKind:    "ClusterRole",
				RiskLevel:   rl,
			}
			analysis.Paths = append(analysis.Paths, path)
			analysis.RiskLevel = string(maxRisk(RiskLevel(analysis.RiskLevel), RiskLevel(rl)))
			if HasWriteVerbs(cr.Rules) {
				analysis.HasWrite = true
				analysis.Evidence["role"] = cr.Name
				analysis.Evidence["roleKind"] = "ClusterRole"
			}
		}
	}

	// --- ClusterRoleBindings (cluster-scoped) ---
	var clusterRoleBindings []models.ClusterRoleBinding
	if err := db.WithContext(ctx).
		Where("cluster_id = ? AND deleted_at IS NULL", pod.ClusterID).
		Find(&clusterRoleBindings).Error; err != nil {
		if !isTableMissingError(err) {
			return analysis, err
		}
	}

	for _, crb := range clusterRoleBindings {
		if !bindingRefersToSA(crb.Subjects, pod.ServiceAccount, pod.Namespace) {
			continue
		}

		var ref roleRef
		if err := json.Unmarshal([]byte(crb.RoleRef), &ref); err != nil {
			continue
		}

		if ref.Kind != "ClusterRole" {
			continue
		}

		var cr models.ClusterRole
		if err := db.WithContext(ctx).
			Where("cluster_id = ? AND name = ? AND deleted_at IS NULL",
				pod.ClusterID, ref.Name).
			First(&cr).Error; err != nil {
			continue
		}
		rl := ClassifyRoleRisk(cr.Name, cr.Rules)
		path := RBACPath{
			BindingName: crb.Name,
			BindingKind: "ClusterRoleBinding",
			RoleName:    cr.Name,
			RoleKind:    "ClusterRole",
			RiskLevel:   rl,
		}
		analysis.Paths = append(analysis.Paths, path)
		analysis.RiskLevel = string(maxRisk(RiskLevel(analysis.RiskLevel), RiskLevel(rl)))
		if HasWriteVerbs(cr.Rules) {
			analysis.HasWrite = true
			analysis.Evidence["role"] = cr.Name
			analysis.Evidence["roleKind"] = "ClusterRole"
		}
	}

	return analysis, nil
}

// ClassifyRoleRisk returns the risk level ("critical", "high", "medium", or "none")
// for a role identified by name and its JSON-encoded policy rules.
//
// This is the canonical implementation extracted from graph/relational_path_builder.go.
// Both PCE and the Attack Path Builder call this function to ensure consistent classification.
func ClassifyRoleRisk(roleName, rulesJSON string) string {
	lower := strings.ToLower(roleName)
	if lower == "cluster-admin" || strings.Contains(lower, "cluster-admin") {
		return string(RiskLevelCritical)
	}
	if strings.Contains(lower, "admin") {
		return string(RiskLevelHigh)
	}

	if rulesJSON == "" || rulesJSON == "null" {
		return string(RiskLevelNone)
	}

	var rules []map[string]interface{}
	if err := json.Unmarshal([]byte(rulesJSON), &rules); err != nil {
		return string(RiskLevelNone)
	}

	highest := RiskLevelNone
	for _, rule := range rules {
		verbs := toStringSlice(rule["verbs"])
		resources := toStringSlice(rule["resources"])
		apiGroups := toStringSlice(rule["apiGroups"])

		hasWildcardVerb := contains(verbs, "*")
		hasWildcardResource := contains(resources, "*")
		hasWildcardAPI := contains(apiGroups, "*") || contains(apiGroups, "")

		if hasWildcardVerb && hasWildcardResource {
			return string(RiskLevelCritical)
		}

		// Privilege escalation verbs
		if containsAny(verbs, "escalate", "bind", "impersonate") {
			return string(RiskLevelCritical)
		}

		// Read-only secret theft: get/list/watch on secrets is high risk
		hasReadVerbs := containsAny(verbs, "get", "list", "watch") || hasWildcardVerb
		if hasReadVerbs && containsAny(resources, "secrets", "*") {
			highest = maxRisk(highest, RiskLevelHigh)
		}

		dangerousVerbs := hasDangerousVerbs(verbs, hasWildcardVerb)
		sensitiveResources := HasSensitiveResources(resources)

		// Critical: mutate/wildcard on nodes or CSR/PKI or token-related resources
		if dangerousVerbs && hasCriticalResources(resources) {
			return string(RiskLevelCritical)
		}

		if dangerousVerbs && sensitiveResources && hasWildcardAPI {
			highest = maxRisk(highest, RiskLevelHigh)
		}

		if dangerousVerbs && sensitiveResources {
			highest = maxRisk(highest, RiskLevelMedium)
		}
	}

	return string(highest)
}

// HasWriteVerbs returns true when the JSON-encoded policy rules grant any
// mutating verb (create, update, delete, patch, or wildcard).
//
// This is the canonical implementation extracted from capability/evaluator.go.
func HasWriteVerbs(rulesJSON string) bool {
	var rules []policyRule
	if err := json.Unmarshal([]byte(rulesJSON), &rules); err != nil {
		return false
	}
	for _, r := range rules {
		for _, verb := range r.Verbs {
			v := strings.ToLower(verb)
			if v == "*" || v == "create" || v == "update" || v == "delete" || v == "patch" {
				return true
			}
		}
	}
	return false
}

// HasSensitiveResources returns true if the slice includes sensitive K8s resource types.
func HasSensitiveResources(resources []string) bool {
	return containsAny(resources,
		"secrets",
		"pods", "deployments", "daemonsets",
		"jobs", "cronjobs", "statefulsets", "replicasets",
		"pods/exec", "pods/portforward", "pods/proxy", "pods/attach",
		"clusterroles", "clusterrolebindings",
		"roles", "rolebindings",
		"serviceaccounts",
		"configmaps", "persistentvolumeclaims", "persistentvolumes",
		"endpoints", "services", "ingresses",
	)
}

// --- helpers ---

func hasDangerousVerbs(verbs []string, hasWildcard bool) bool {
	return hasWildcard ||
		contains(verbs, "create") ||
		contains(verbs, "update") ||
		contains(verbs, "patch") ||
		contains(verbs, "delete")
}

func hasCriticalResources(resources []string) bool {
	return containsAny(resources,
		"nodes", "nodes/proxy", "nodes/metrics", "nodes/stats",
		"serviceaccounts/token", "tokenreviews",
		"certificatesigningrequests", "certificatesigningrequests/approval",
	)
}

func maxRisk(a, b RiskLevel) RiskLevel {
	order := map[RiskLevel]int{
		RiskLevelNone:     0,
		RiskLevelMedium:   1,
		RiskLevelHigh:     2,
		RiskLevelCritical: 3,
	}
	if order[b] > order[a] {
		return b
	}
	return a
}

func bindingRefersToSA(subjectsJSON, saName, saNamespace string) bool {
	var subs []subject
	if err := json.Unmarshal([]byte(subjectsJSON), &subs); err != nil {
		return false
	}
	for _, s := range subs {
		if s.Kind == "ServiceAccount" && s.Name == saName && s.Namespace == saNamespace {
			return true
		}
	}
	return false
}

func toStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	switch s := v.(type) {
	case []string:
		return s
	case []interface{}:
		out := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out
	}
	return nil
}

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

func containsAny(slice []string, vals ...string) bool {
	for _, val := range vals {
		if contains(slice, val) {
			return true
		}
	}
	return false
}

func isTableMissingError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "relation") && strings.Contains(msg, "does not exist")
}
