import type { PersonaId } from './persona';
import { can, canAny, P } from './permissions';
import type { PermUser } from './persona';

/** Route → minimum permission(s). Server still enforces on API. */
const ROUTE_PERMISSIONS: { prefix: string; perm?: string; permAny?: string[] }[] = [
  { prefix: '/investigation', perm: P.investigationsRead },
  { prefix: '/risks', perm: P.findingsRead },
  { prefix: '/attack-paths', perm: P.graphReadPaths },
  { prefix: '/network-activity', perm: P.runtimeRead },
  { prefix: '/monitoring', permAny: [P.observabilityMetricsRead, P.observabilityLogsRead, P.observabilityAgentsRead] },
  { prefix: '/governance', perm: P.systemAuditRead },
  { prefix: '/resources', perm: P.inventoryRead },
  { prefix: '/clusters', perm: P.inventoryRead },
  { prefix: '/capabilities', perm: P.inventoryRead },
  { prefix: '/rules', perm: P.policiesRead },
  { prefix: '/reports', perm: P.findingsRead },
  { prefix: '/settings', permAny: [P.usersRead, P.usersUpdate, P.rulesRead, P.policiesRead, P.inventoryRead] },
  { prefix: '/certificates', perm: P.clusterCertificatesRotate },
  { prefix: '/', perm: P.findingsRead },
];

const USER_ADMIN_ALLOWED = ['/settings', '/login'];

const PERSONA_DENIED: Partial<Record<PersonaId, string[]>> = {
  user_admin: [
    '/',
    '/investigation',
    '/risks',
    '/attack-paths',
    '/network-activity',
    '/monitoring',
    '/governance',
    '/resources',
    '/clusters',
    '/capabilities',
    '/rules',
    '/reports',
    '/certificates',
  ],
};

function pathMatches(pathname: string, prefix: string): boolean {
  if (prefix === '/') return pathname === '/' || pathname === '';
  return pathname === prefix || pathname.startsWith(`${prefix}/`);
}

export function routeAllowedForUser(pathname: string, user: PermUser): boolean {
  const path = pathname.split('?')[0] || '/';
  const rule =
    ROUTE_PERMISSIONS.find((r) => pathMatches(path, r.prefix)) ??
    ROUTE_PERMISSIONS[ROUTE_PERMISSIONS.length - 1];
  if (rule.permAny?.length) return canAny(user, rule.permAny);
  if (rule.perm) return can(user, rule.perm);
  return true;
}

export function routeDeniedForPersona(pathname: string, personaId: PersonaId): boolean {
  const path = pathname.split('?')[0] || '/';
  if (personaId === 'user_admin') {
    return !USER_ADMIN_ALLOWED.some((p) => pathMatches(path, p));
  }
  const denied = PERSONA_DENIED[personaId];
  if (!denied) return false;
  return denied.some((p) => pathMatches(path, p));
}

export function resolveRouteAccess(
  pathname: string,
  user: PermUser,
  personaId: PersonaId,
): { allowed: boolean; reason?: 'permission' | 'persona' } {
  if (routeDeniedForPersona(pathname, personaId)) {
    return { allowed: false, reason: 'persona' };
  }
  if (!routeAllowedForUser(pathname, user)) {
    return { allowed: false, reason: 'permission' };
  }
  return { allowed: true };
}
