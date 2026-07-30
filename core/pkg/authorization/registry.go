package authorization

import (
	"strings"

	"github.com/fortuna/core/pkg/models"
)

// NormalizeRole maps legacy DB roles to canonical roles used for permission lookup.
// Spec: existing "user" → operator (migration compatibility).
func NormalizeRole(role string) string {
	r := strings.ToLower(strings.TrimSpace(role))
	switch r {
	case "":
		return ""
	case models.RoleViewer:
		return models.RoleViewer
	case models.RoleAdmin:
		return models.RoleAdmin
	case models.RoleClusterAdmin:
		return models.RoleClusterAdmin
	case models.RoleUserAdmin:
		return models.RoleUserAdmin
	case models.RoleOperator, models.RoleUser:
		return models.RoleOperator
	default:
		return r
	}
}

// PermissionsForRole returns the permission set for a canonical role string.
// Deny-by-default: unknown roles get no permissions.
func PermissionsForRole(canonicalRole string) []Permission {
	switch strings.ToLower(strings.TrimSpace(canonicalRole)) {
	case models.RoleAdmin:
		return AllPermissions()
	case models.RoleClusterAdmin:
		return clusterAdminPermissions()
	case models.RoleUserAdmin:
		return userAdminPermissions()
	case models.RoleOperator:
		return operatorPermissions()
	case models.RoleViewer:
		return viewerPermissions()
	default:
		return nil
	}
}

// PermissionsForUser resolves permissions from stored user.Role (legacy "user" included).
func PermissionsForUser(role string) []Permission {
	return PermissionsForRole(NormalizeRole(role))
}

// userAdminPermissions: Fortuna account administration only (no security data / cluster scope by default).
func userAdminPermissions() []Permission {
	return []Permission{
		PermissionAuthSession,
		PermissionAuthPasswordChange,
		PermissionAuthRegister,
		PermissionSessionsRead,
		PermissionSessionsRevoke,
		PermissionUsersRead,
		PermissionUsersCreate,
		PermissionUsersUpdate,
		PermissionUsersDisable,
		PermissionUsersDelete,
		PermissionUsersPasswordReset,
		PermissionUsersRoleAssign,
	}
}

// clusterAdminPermissions: cluster-scoped security administration. User scope
// must still be enforced by API handlers for resource-ID writes.
func clusterAdminPermissions() []Permission {
	return []Permission{
		PermissionAuthSession,
		PermissionAuthPasswordChange,
		PermissionExportFindings,
		PermissionSessionsRead,
		PermissionSessionsRevoke,
		PermissionFindingsRead,
		PermissionFindingsAck,
		PermissionFindingsDismiss,
		PermissionFindingsResolve,
		PermissionFindingsReopen,
		PermissionFindingsBulk,
		PermissionRiskEvaluate,
		PermissionInventoryRead,
		PermissionInventoryAnnotate,
		PermissionInventoryModify,
		PermissionInventoryQuarantine,
		PermissionInventoryBulk,
		PermissionRuntimeRead,
		PermissionGraphReadSummary,
		PermissionGraphReadPaths,
		PermissionGraphQueryEntity,
		PermissionGraphQueryTraversal,
		PermissionPoliciesRead,
		PermissionRulesRead,
		PermissionRulesExport,
		PermissionMalwareRead,
		PermissionInvestigationsRead,
		PermissionInvestigationsWrite,
		PermissionObservabilityMetricsRead,
		PermissionObservabilityAgentsRead,
	}
}

func viewerPermissions() []Permission {
	return []Permission{
		PermissionAuthSession,
		PermissionAuthPasswordChange,
		PermissionSessionsRead,
		PermissionSessionsRevoke,
		PermissionFindingsRead,
		PermissionInventoryRead,
		PermissionRuntimeRead,
		PermissionGraphReadSummary,
		PermissionGraphReadPaths,
		PermissionGraphQueryEntity,
		PermissionMalwareRead,
		PermissionInvestigationsRead,
		PermissionObservabilityMetricsRead,
		PermissionObservabilityAgentsRead,
	}
}

func operatorPermissions() []Permission {
	return []Permission{
		PermissionAuthSession,
		PermissionAuthPasswordChange,
		PermissionExportFindings,
		PermissionSessionsRead,
		PermissionSessionsRevoke,
		PermissionFindingsRead,
		PermissionFindingsAck,
		PermissionFindingsDismiss,
		PermissionFindingsResolve,
		PermissionFindingsReopen,
		PermissionFindingsBulk,
		PermissionFindingsDelete,
		PermissionFindingsExceptionCreate,
		PermissionFindingsExceptionApprove,
		PermissionFindingsExceptionDelete,
		PermissionRiskEvaluate,
		PermissionInventoryRead,
		PermissionInventoryAnnotate,
		PermissionInventoryModify,
		PermissionInventoryDelete,
		PermissionInventoryBulk,
		PermissionRuntimeRead,
		PermissionRuntimeMappingWrite,
		PermissionGraphReadSummary,
		PermissionGraphReadPaths,
		PermissionGraphQueryEntity,
		PermissionGraphQueryTraversal,
		PermissionPoliciesRead,
		PermissionRulesRead,
		PermissionRulesWrite,
		PermissionRulesDelete,
		PermissionRulesImport,
		PermissionRulesExport,
		PermissionMalwareRead,
		PermissionInvestigationsRead,
		PermissionInvestigationsWrite,
		PermissionInvestigationsDelete,
		PermissionObservabilityMetricsRead,
		PermissionObservabilityLogsRead,
		PermissionObservabilityAgentsRead,
	}
}

// HasPermission reports whether need is satisfied by exactly one entry in granted (deny-by-default).
func HasPermission(granted []Permission, need Permission) bool {
	ns := string(need)
	for _, g := range granted {
		if string(g) == ns {
			return true
		}
	}
	return false
}

// HasPermissionString matches a raw permission string (e.g. from JWT cache) against need.
func HasPermissionString(granted []string, need Permission) bool {
	ns := string(need)
	for _, g := range granted {
		if strings.EqualFold(strings.TrimSpace(g), ns) {
			return true
		}
	}
	return false
}

// HasAnyPermission returns true if at least one required permission is granted.
func HasAnyPermission(granted []Permission, required ...Permission) bool {
	for _, r := range required {
		if HasPermission(granted, r) {
			return true
		}
	}
	return false
}

// HasAllPermissions returns true if every required permission is granted.
func HasAllPermissions(granted []Permission, required ...Permission) bool {
	if len(required) == 0 {
		return true
	}
	for _, r := range required {
		if !HasPermission(granted, r) {
			return false
		}
	}
	return true
}

// ToStrings converts a permission slice to JWT/API string form.
func ToStrings(perms []Permission) []string {
	out := make([]string, len(perms))
	for i, p := range perms {
		out[i] = string(p)
	}
	return out
}

// FromStrings parses permission strings into known Permission values only (unknown dropped — deny-by-default at grant time).
func FromStrings(in []string) []Permission {
	var out []Permission
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		p := Permission(s)
		if IsValidPermission(p) {
			out = append(out, p)
		}
	}
	return out
}

// IsGraphAdvanced reports whether the grant set allows advanced (less restricted) graph query execution.
func IsGraphAdvanced(granted []Permission) bool {
	return HasPermission(granted, PermissionGraphQueryAdvanced)
}

// MaxGraphTraversalDepth returns the effective max depth for graph traversal endpoints from query params.
func MaxGraphTraversalDepth(granted []Permission, requested int) int {
	if HasPermission(granted, PermissionGraphQueryAdvanced) {
		if requested < 1 {
			return 5
		}
		if requested > 10 {
			return 10
		}
		return requested
	}
	if HasPermission(granted, PermissionGraphQueryTraversal) {
		if requested < 1 {
			return 3
		}
		if requested > 5 {
			return 5
		}
		return requested
	}
	// Summary / entity / paths only: tight cap (graph DOS mitigation)
	if requested < 1 {
		return 2
	}
	if requested > 3 {
		return 3
	}
	return requested
}

// RolesGrantingPermission lists canonical Fortuna roles whose default grant includes permission p.
func RolesGrantingPermission(p Permission) []string {
	if !IsValidPermission(p) {
		return nil
	}
	var out []string
	for _, role := range []string{models.RoleAdmin, models.RoleClusterAdmin, models.RoleUserAdmin, models.RoleOperator, models.RoleViewer} {
		for _, g := range PermissionsForRole(role) {
			if g == p {
				out = append(out, role)
				break
			}
		}
	}
	return out
}
