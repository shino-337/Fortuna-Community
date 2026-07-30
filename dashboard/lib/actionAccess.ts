import type { PermUser } from './persona';
import { can, canAny, P, type PermissionString } from './permissions';

export const ACTION_IDS = {
  findingAcknowledge: 'finding.acknowledge',
  findingResolve: 'finding.resolve',
  findingDismiss: 'finding.dismiss',
  findingBulk: 'finding.bulk',
  findingExport: 'finding.export',
  riskEvaluate: 'risk.evaluate',
  policyDraft: 'policy.draft',
  policyPublish: 'policy.publish',
  policyDelete: 'policy.delete',
  userCreate: 'user.create',
  userUpdate: 'user.update',
  userDelete: 'user.delete',
  userRoleAssign: 'user.role.assign',
  sessionRevoke: 'session.revoke',
  sessionRevokeAll: 'session.revoke_all',
  certificateRotate: 'certificate.rotate',
} as const;

export type ActionId = (typeof ACTION_IDS)[keyof typeof ACTION_IDS];

const ACTION_PERMISSIONS: Record<ActionId, { all?: PermissionString[]; any?: PermissionString[] }> = {
  [ACTION_IDS.findingAcknowledge]: { any: [P.findingsAck, P.findingsBulk] },
  [ACTION_IDS.findingResolve]: { any: [P.findingsResolve, P.findingsBulk] },
  [ACTION_IDS.findingDismiss]: { any: [P.findingsDismiss, P.findingsBulk] },
  [ACTION_IDS.findingBulk]: { all: [P.findingsBulk] },
  [ACTION_IDS.findingExport]: { all: [P.exportFindings] },
  [ACTION_IDS.riskEvaluate]: { all: [P.riskEvaluate] },
  [ACTION_IDS.policyDraft]: { all: [P.policiesDraft] },
  [ACTION_IDS.policyPublish]: { all: [P.policiesPublish] },
  [ACTION_IDS.policyDelete]: { all: [P.policiesDelete] },
  [ACTION_IDS.userCreate]: { all: [P.usersCreate] },
  [ACTION_IDS.userUpdate]: { all: [P.usersUpdate] },
  [ACTION_IDS.userDelete]: { all: [P.usersDelete] },
  [ACTION_IDS.userRoleAssign]: { all: [P.usersRoleAssign] },
  [ACTION_IDS.sessionRevoke]: { all: [P.sessionsRevoke] },
  [ACTION_IDS.sessionRevokeAll]: { all: [P.sessionsRevokeAll] },
  [ACTION_IDS.certificateRotate]: { all: [P.clusterCertificatesRotate] },
};

/** Check if a user can run an action based on their permissions. */
export function canRunAction(user: PermUser, actionId: ActionId): boolean;
/** @returns true if the user has any permission that grants access to this action. */
export function canRunAction(user: PermUser, actionId: ActionId): boolean {
  const rule = ACTION_PERMISSIONS[actionId];
  if (rule.any?.length) return canAny(user, rule.any);
  if (rule.all?.length) return rule.all.every((permission) => can(user, permission));
  return false;
}

/** Get all permissions required to run an action. */
export function actionRequiredPermissions(actionId: ActionId): PermissionString[];
/** @returns A flat array of permission strings (both "all" and "any") needed for this action, suitable for displaying in UI or validating against user grants. */
export function actionRequiredPermissions(actionId: ActionId): PermissionString[] {
  const rule = ACTION_PERMISSIONS[actionId];
  return [...(rule.all ?? []), ...(rule.any ?? [])];
}
