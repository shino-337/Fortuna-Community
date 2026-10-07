// Package authorization implements Fortuna application RBAC (permission-based).
// RBAC v2: fine-grained permissions (FORTUNA_RBAC_V2_ENHANCEMENT_SPEC). Deny-by-default.
// Distinct from core/pkg/rbac (Kubernetes RBAC analysis).
package authorization

// Permission is an atomic application capability string (stable API contract).
type Permission string

const (
	// Auth & session
	PermissionAuthSession        Permission = "auth.session"
	PermissionAuthPasswordChange Permission = "auth.password.change"
	PermissionAuthRegister       Permission = "auth.register"

	// Sessions (JWT binding + governance)
	PermissionSessionsRead      Permission = "sessions.read"
	PermissionSessionsRevoke    Permission = "sessions.revoke"
	PermissionSessionsRevokeAll Permission = "sessions.revoke_all"

	// Export governance (sensitive data egress)
	PermissionExportFindings  Permission = "export.findings"
	PermissionUsersRead       Permission = "users.read"
	PermissionUsersUpdate     Permission = "users.update"
	PermissionUsersDelete     Permission = "users.delete"
	PermissionUsersRoleAssign Permission = "users.role.assign"

	// Findings (v2 — replaces findings.write / findings.exceptions)
	PermissionFindingsRead            Permission = "findings.read"
	PermissionFindingsAck             Permission = "findings.ack"
	PermissionFindingsDismiss         Permission = "findings.dismiss"
	PermissionFindingsResolve         Permission = "findings.resolve"
	PermissionFindingsReopen          Permission = "findings.reopen"
	PermissionFindingsBulk            Permission = "findings.bulk"
	PermissionFindingsDelete          Permission = "findings.delete"
	PermissionFindingsExceptionCreate Permission = "findings.exception.create"
	PermissionFindingsExceptionDelete Permission = "findings.exception.delete"

	// Investigations (SOC case workspace)
	PermissionInvestigationsRead   Permission = "investigations.read"
	PermissionInvestigationsWrite  Permission = "investigations.write"
	PermissionInvestigationsDelete Permission = "investigations.delete"

	PermissionRiskEvaluate Permission = "risk.evaluate"

	// Policies (v2 — replaces policies.write)
	PermissionPoliciesRead    Permission = "policies.read"
	PermissionPoliciesDraft   Permission = "policies.draft"
	PermissionPoliciesPublish Permission = "policies.publish"
	PermissionPoliciesDelete  Permission = "policies.delete"

	PermissionRulesRead   Permission = "rules.read"
	PermissionRulesWrite  Permission = "rules.write"
	PermissionRulesDelete Permission = "rules.delete"
	PermissionRulesImport Permission = "rules.import"
	PermissionRulesExport Permission = "rules.export"

	// Inventory (v2 — replaces inventory.write)
	PermissionInventoryRead   Permission = "inventory.read"
	PermissionInventoryModify Permission = "inventory.modify"
	PermissionInventoryDelete Permission = "inventory.delete"

	PermissionRuntimeRead         Permission = "runtime.read"
	PermissionRuntimeMappingWrite Permission = "runtime.mapping.write"

	// Graph (v2 — replaces graph.read + graph.query.safe)
	PermissionGraphReadSummary Permission = "graph.read.summary"
	PermissionGraphReadPaths   Permission = "graph.read.paths"

	PermissionMalwareRead Permission = "malware.read"

	PermissionSystemAuditRead Permission = "system.audit.read"

	// Observability (v2 — replaces system.observability.read)
	PermissionObservabilityMetricsRead Permission = "observability.metrics.read"
	PermissionObservabilityLogsRead    Permission = "observability.logs.read"
	PermissionObservabilityAgentsRead  Permission = "observability.agents.read"

	PermissionClusterCertificatesRotate Permission = "cluster.certificates.rotate"
)

// PermissionLevel classifies sensitivity / SoD (spec §9).
type PermissionLevel string

const (
	LevelRead             PermissionLevel = "READ"
	LevelWrite            PermissionLevel = "WRITE"
	LevelDestructive      PermissionLevel = "DESTRUCTIVE"
	LevelPlatform         PermissionLevel = "PLATFORM"
	LevelSecurityCritical PermissionLevel = "SECURITY_CRITICAL"
)

// ClassifyPermission returns coarse classification for governance UI / policy.
func ClassifyPermission(p Permission) PermissionLevel {
	switch p {
	case PermissionUsersDelete, PermissionUsersRoleAssign, PermissionSessionsRevokeAll,
		PermissionFindingsDelete, PermissionFindingsBulk,
		PermissionInvestigationsDelete,
		PermissionInventoryDelete, PermissionRulesDelete, PermissionPoliciesDelete,
		PermissionClusterCertificatesRotate:
		return LevelDestructive
	case PermissionUsersUpdate, PermissionSessionsRevoke,
		PermissionFindingsAck, PermissionFindingsDismiss, PermissionFindingsResolve, PermissionFindingsReopen,
		PermissionFindingsExceptionCreate, PermissionFindingsExceptionDelete,
		PermissionInvestigationsWrite,
		PermissionInventoryModify,
		PermissionRulesWrite, PermissionRulesImport,
		PermissionPoliciesDraft, PermissionPoliciesPublish,
		PermissionRuntimeMappingWrite, PermissionRiskEvaluate,
		PermissionExportFindings:
		return LevelWrite
	case PermissionAuthRegister, PermissionSystemAuditRead:
		return LevelSecurityCritical
	default:
		return LevelRead
	}
}

// AllPermissions returns every known permission (admin grant set).
func AllPermissions() []Permission {
	return []Permission{
		PermissionAuthSession,
		PermissionAuthPasswordChange,
		PermissionAuthRegister,
		PermissionSessionsRead,
		PermissionSessionsRevoke,
		PermissionSessionsRevokeAll,
		PermissionExportFindings,
		PermissionUsersRead,
		PermissionUsersUpdate,
		PermissionUsersDelete,
		PermissionUsersRoleAssign,
		PermissionFindingsRead,
		PermissionFindingsAck,
		PermissionFindingsDismiss,
		PermissionFindingsResolve,
		PermissionFindingsReopen,
		PermissionFindingsBulk,
		PermissionFindingsDelete,
		PermissionFindingsExceptionCreate,
		PermissionFindingsExceptionDelete,
		PermissionInvestigationsRead,
		PermissionInvestigationsWrite,
		PermissionInvestigationsDelete,
		PermissionRiskEvaluate,
		PermissionPoliciesRead,
		PermissionPoliciesDraft,
		PermissionPoliciesPublish,
		PermissionPoliciesDelete,
		PermissionRulesRead,
		PermissionRulesWrite,
		PermissionRulesDelete,
		PermissionRulesImport,
		PermissionRulesExport,
		PermissionInventoryRead,
		PermissionInventoryModify,
		PermissionInventoryDelete,
		PermissionRuntimeRead,
		PermissionRuntimeMappingWrite,
		PermissionGraphReadSummary,
		PermissionGraphReadPaths,
		PermissionMalwareRead,
		PermissionSystemAuditRead,
		PermissionObservabilityMetricsRead,
		PermissionObservabilityLogsRead,
		PermissionObservabilityAgentsRead,
		PermissionClusterCertificatesRotate,
	}
}

var validPermissionSet map[Permission]struct{}

func init() {
	validPermissionSet = make(map[Permission]struct{}, len(AllPermissions()))
	for _, p := range AllPermissions() {
		validPermissionSet[p] = struct{}{}
	}
}

// IsValidPermission is true only for permissions in the v2 taxonomy (unknown strings are invalid).
func IsValidPermission(p Permission) bool {
	_, ok := validPermissionSet[p]
	return ok
}
