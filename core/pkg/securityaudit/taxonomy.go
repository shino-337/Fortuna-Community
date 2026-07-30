// Package securityaudit provides append-only governance audit writes.
package securityaudit

// Normalized action names for security_activity_logs (prefix: subsystem.entity.verb).
const (
	ActionFindingsAcknowledge       = "findings.status.acknowledge"
	ActionFindingsResolve           = "findings.status.resolve"
	ActionFindingsDismiss           = "findings.status.dismiss"
	ActionFindingsPatch             = "findings.status.patch"
	ActionFindingsBulk              = "findings.bulk"
	ActionFindingsExceptionCreate   = "findings.exception.create"
	ActionFindingsExceptionDelete   = "findings.exception.delete"
	ActionInventoryServiceAccountBulk = "inventory.serviceaccount.bulk"
	ActionMalwareDBUpload           = "malware.db.upload"
	ActionAuthzDeny                 = "authz.deny"
	ActionSessionRevoke             = "session.revoke"
	ActionSessionsRevokeAll         = "sessions.revoke_all"
	ActionUserRBACChange            = "user.rbac.change"
	ActionAuditTamperDenied         = "audit.store.mutation_denied"
	ActionInvestigationsCaseCreate  = "investigations.case.create"
	ActionInvestigationsCaseUpdate  = "investigations.case.update"
	ActionInvestigationsCaseDelete  = "investigations.case.delete"
)
