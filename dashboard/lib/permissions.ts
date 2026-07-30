/**
 * Client-side permission helpers (UX only). Backend enforces authorization.
 * Strings must match core/pkg/authorization/permissions.go (RBAC v2).
 */
export const P = {
  authSession: 'auth.session',
  authPasswordChange: 'auth.password.change',
  authRegister: 'auth.register',

  findingsRead: 'findings.read',
  findingsAck: 'findings.ack',
  findingsDismiss: 'findings.dismiss',
  findingsResolve: 'findings.resolve',
  findingsReopen: 'findings.reopen',
  findingsBulk: 'findings.bulk',
  findingsDelete: 'findings.delete',
  findingsExceptionCreate: 'findings.exception.create',
  findingsExceptionApprove: 'findings.exception.approve',
  findingsExceptionDelete: 'findings.exception.delete',

  investigationsRead: 'investigations.read',
  investigationsWrite: 'investigations.write',
  investigationsDelete: 'investigations.delete',

  riskEvaluate: 'risk.evaluate',

  policiesRead: 'policies.read',
  policiesDraft: 'policies.draft',
  policiesReview: 'policies.review',
  policiesApprove: 'policies.approve',
  policiesPublish: 'policies.publish',
  policiesDelete: 'policies.delete',

  rulesRead: 'rules.read',
  rulesWrite: 'rules.write',
  rulesDelete: 'rules.delete',
  rulesImport: 'rules.import',
  rulesExport: 'rules.export',

  inventoryRead: 'inventory.read',
  inventoryAnnotate: 'inventory.annotate',
  inventoryModify: 'inventory.modify',
  inventoryQuarantine: 'inventory.quarantine',
  inventoryDelete: 'inventory.delete',
  inventoryBulk: 'inventory.bulk',

  runtimeRead: 'runtime.read',
  runtimeMappingWrite: 'runtime.mapping.write',

  graphReadSummary: 'graph.read.summary',
  graphReadPaths: 'graph.read.paths',
  graphQueryEntity: 'graph.query.entity',
  graphQueryTraversal: 'graph.query.traversal',
  graphQueryAdvanced: 'graph.query.advanced',
  graphExport: 'graph.export',

  /** Governed egress for risk findings export (CSV/PDF). */
  exportFindings: 'export.findings',

  /** JWT-bound server sessions (list/revoke). */
  sessionsRead: 'sessions.read',
  sessionsRevoke: 'sessions.revoke',
  sessionsRevokeAll: 'sessions.revoke_all',

  malwareRead: 'malware.read',
  malwareUpload: 'malware.upload',

  usersRead: 'users.read',
  usersCreate: 'users.create',
  usersUpdate: 'users.update',
  usersDisable: 'users.disable',
  usersDelete: 'users.delete',
  usersPasswordReset: 'users.password.reset',
  usersRoleAssign: 'users.role.assign',

  systemAuditRead: 'system.audit.read',
  observabilityMetricsRead: 'observability.metrics.read',
  observabilityLogsRead: 'observability.logs.read',
  observabilityAgentsRead: 'observability.agents.read',
  observabilityDebugRead: 'observability.debug.read',

  systemDebug: 'system.debug',
  internalCveTrigger: 'internal.cve.trigger',
  clusterCertificatesRotate: 'cluster.certificates.rotate',
} as const;

export type PermissionString = (typeof P)[keyof typeof P];

/** Build a Set of lowercase permission strings from user.permissions (if present). */
export function permissionSet(user: { permissions?: string[] } | null | undefined): Set<string>;
/** @returns A set containing all granted permissions in lowercase for case-insensitive comparison. */
export function permissionSet(user: { permissions?: string[] } | null | undefined): Set<string> {
  const list = user?.permissions ?? [];
  return new Set(list.map((s) => String(s).toLowerCase()));
}

/** Check if a single permission is granted to the user. */
export function can(user: { permissions?: string[] } | null | undefined, need: string): boolean;
/** @returns true if the user has the specified permission (case-insensitive). */
export function can(user: { permissions?: string[] } | null | undefined, need: string): boolean {
  return permissionSet(user).has(need.toLowerCase());
}

/** Check if any of the listed permissions are granted to the user. */
export function canAny(user: { permissions?: string[] } | null | undefined, needs: string[]): boolean;
/** @returns true if at least one permission in the list is granted (case-insensitive). */
export function canAny(user: { permissions?: string[] } | null | undefined, needs: string[]): boolean {
  const s = permissionSet(user);
  return needs.some((n) => s.has(n.toLowerCase()));
}

/** Check if all of the listed permissions are granted to the user. */
export function canAll(user: { permissions?: string[] } | null | undefined, needs: string[]): boolean;
/** @returns true only if every permission in the list is granted (case-insensitive). Useful for compound actions like bulk operations. */
export function canAll(user: { permissions?: string[] } | null | undefined, needs: string[]): boolean {
  const s = permissionSet(user);
  return needs.every((n) => s.has(n.toLowerCase()));
}
