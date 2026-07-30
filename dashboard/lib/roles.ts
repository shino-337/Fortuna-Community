import type { PermUser } from './persona';
import { normalizeFortunaRoleKey } from './fortunaRoles';

export const ROLES = {
  admin: 'admin',
  clusterAdmin: 'cluster_admin',
  userAdmin: 'user_admin',
  operator: 'operator',
  viewer: 'viewer',
} as const;

export type CanonicalRole = (typeof ROLES)[keyof typeof ROLES];

/** Map a user's role string to the closest Fortuna canonical role. */
export function canonicalRole(user: PermUser): CanonicalRole | 'unknown' {
  const role = normalizeFortunaRoleKey(user?.role);
  if (role === ROLES.admin) return ROLES.admin;
  if (role === ROLES.clusterAdmin) return ROLES.clusterAdmin;
  if (role === ROLES.userAdmin) return ROLES.userAdmin;
  if (role === ROLES.operator) return ROLES.operator;
  if (role === ROLES.viewer) return ROLES.viewer;
  return 'unknown';
}

/** Check if the user has platform admin role. */
export function isPlatformAdmin(user: PermUser): boolean {
  return canonicalRole(user) === ROLES.admin;
}

/** Check if the user has user admin role. */
export function isUserAdmin(user: PermUser): boolean {
  return canonicalRole(user) === ROLES.userAdmin;
}
