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
	case models.RoleOperator, models.RoleUser:
		return models.RoleOperator
	case models.RoleRiskEvaluator:
		return models.RoleRiskEvaluator
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
	case models.RoleOperator:
		return operatorPermissions()
	case models.RoleViewer:
		return viewerPermissions()
	case models.RoleRiskEvaluator:
		return []Permission{PermissionAuthSession, PermissionRiskEvaluate}
	default:
		return nil
	}
}

// IsServiceRole reports whether role belongs to a non-human account that
// deployment configuration owns. Such roles are never assigned through the API.
func IsServiceRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case models.RoleRiskEvaluator, models.RoleSystem:
		return true
	}
	return false
}

// PermissionsForUser resolves permissions from stored user.Role (legacy "user" included).
func PermissionsForUser(role string) []Permission {
	return PermissionsForRole(NormalizeRole(role))
}

// viewerPermissions: read-only security data inside the account's cluster scope.
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
		PermissionMalwareRead,
		PermissionInvestigationsRead,
		PermissionObservabilityMetricsRead,
		PermissionObservabilityAgentsRead,
	}
}

// operatorPermissions: day-to-day triage and cases inside the account's
// cluster scope. Suppressing or deleting findings, changing Kubernetes and
// changing configuration that applies to every cluster are left to roles
// above it.
func operatorPermissions() []Permission {
	return append(viewerPermissions(),
		PermissionExportFindings,
		PermissionFindingsAck,
		PermissionFindingsDismiss,
		PermissionFindingsResolve,
		PermissionFindingsReopen,
		PermissionFindingsBulk,
		PermissionRiskEvaluate,
		PermissionInventoryModify,
		PermissionPoliciesRead,
		PermissionRulesRead,
		PermissionRulesExport,
		PermissionInvestigationsWrite,
	)
}

// clusterAdminPermissions: runs security for the clusters in its scope (which
// must name at least one cluster). Everything an operator does, plus accepting
// risk (exceptions), deleting findings, archiving cases and revoking or
// deleting ServiceAccounts in those clusters. It does not manage users, read
// the platform audit or change rules and policies, which apply to every cluster.
func clusterAdminPermissions() []Permission {
	return append(operatorPermissions(),
		PermissionFindingsDelete,
		PermissionFindingsExceptionCreate,
		PermissionFindingsExceptionDelete,
		PermissionInventoryDelete,
		PermissionInvestigationsDelete,
	)
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

// HasAnyPermission returns true if at least one required permission is granted.
func HasAnyPermission(granted []Permission, required ...Permission) bool {
	for _, r := range required {
		if HasPermission(granted, r) {
			return true
		}
	}
	return false
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

// RolesGrantingPermission lists canonical Fortuna roles whose default grant includes permission p.
func RolesGrantingPermission(p Permission) []string {
	if !IsValidPermission(p) {
		return nil
	}
	var out []string
	for _, role := range []string{models.RoleAdmin, models.RoleClusterAdmin, models.RoleOperator, models.RoleViewer, models.RoleRiskEvaluator} {
		for _, g := range PermissionsForRole(role) {
			if g == p {
				out = append(out, role)
				break
			}
		}
	}
	return out
}
