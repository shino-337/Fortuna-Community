/**
 * Route materialization — only operationally valid routes exist in the route graph.
 */
import { can, canAny, P } from './permissions';
import type { PermUser } from './persona';
import type { PersonaId } from './persona';
import type { OwnershipContext } from './ownershipContext';
import type { TelemetryContext } from './telemetryContext';
import type { OperationalRelevanceSignals } from './operationalRelevance';
import {
  SURFACE_REGISTRY,
  type OperationalSurface,
  type ShellVariant,
  type SurfaceDefinition,
} from './operationalSurfaceRegistry';

export interface MaterializationContext {
  personaId: PersonaId;
  user: PermUser;
  ownership: OwnershipContext;
  telemetry: TelemetryContext;
  signals: OperationalRelevanceSignals;
}

export interface MaterializedOperationalPlane {
  surfaces: OperationalSurface[];
  surfaceDefinitions: SurfaceDefinition[];
  shellVariant: ShellVariant;
  allowedRoutes: string[];
  hiddenRoutes: string[];
  defaultRoute: string;
  dashboardSections: string[];
  graphMode: import('./persona').GraphSemanticMode;
  identityLabel: string;
  identityDescription: string;
}

/** Child/detail routes unlocked when a parent prefix is materialized. */
const ROUTE_EXPANSION: Record<string, string[]> = {
  '/': ['/dashboard'],
  '/dashboard': [],
  '/risks': ['/risks/findings', '/risks/pce', '/risks/evidence', '/risks/:id'],
  '/resources': [
    '/resources/pods/uid/:uid',
    '/resources/pods/:id',
    '/identities/uid/:uid',
    '/identities/:id',
  ],
  '/clusters': ['/clusters/:id', '/clusters/:clusterId/nodes/:nodeName'],
  '/capabilities': ['/capabilities/:id'],
  '/rules': ['/rules/uid/:uid', '/rules/:id'],
  '/attack-paths': [],
  '/investigation': [],
  '/network-activity': [],
  '/monitoring': ['/notifications'],
  '/governance': [],
  '/settings': [],
  '/reports': [],
  '/notifications': [],
};

const ALL_APP_ROUTES = [
  '/',
  '/dashboard',
  '/clusters',
  '/clusters/:id',
  '/clusters/:clusterId/nodes/:nodeName',
  '/resources',
  '/resources/pods/uid/:uid',
  '/resources/pods/:id',
  '/identities/uid/:uid',
  '/identities/:id',
  '/network-activity',
  '/risks',
  '/risks/findings',
  '/risks/pce',
  '/risks/evidence',
  '/risks/:id',
  '/investigation',
  '/capabilities',
  '/capabilities/:id',
  '/rules',
  '/rules/uid/:uid',
  '/rules/:id',
  '/attack-paths',
  '/monitoring',
  '/governance',
  '/settings',
  '/certificates',
  '/reports',
  '/notifications',
];

const IDENTITY: Record<
  ShellVariant,
  { label: string; description: string; defaultRoute: string }
> = {
  viewer: {
    label: 'Scoped Exposure Observer',
    description: 'Observe exposure and assigned investigations within your operational scope.',
    defaultRoute: '/',
  },
  operator: {
    label: 'Active Incident Responder',
    description: 'Triage, investigate, and remediate threats with runtime and attack-path context.',
    defaultRoute: '/',
  },
  admin: {
    label: 'Security Operations Overseer',
    description: 'Govern platform integrity, telemetry reliability, and operational oversight.',
    defaultRoute: '/dashboard',
  },
  user_admin: {
    label: 'Identity & Access Administrator',
    description: 'Manage Fortuna accounts, roles, and access policies.',
    defaultRoute: '/settings',
  },
};

function hasPermissions(user: PermUser, required: string[]): boolean {
  if (required.length === 0) return true;
  return canAny(
    user,
    required as Parameters<typeof canAny>[1],
  );
}

function expandRoutes(prefixes: string[]): Set<string> {
  const out = new Set<string>();
  for (const p of prefixes) {
    out.add(p);
    for (const child of ROUTE_EXPANSION[p] ?? []) {
      out.add(child);
    }
  }
  return out;
}

/**
 * Permission-route baseline.
 *
 * Surface relevance is allowed to shape navigation and dashboard emphasis, but
 * it must not make a valid deep link disappear while clusters, telemetry, or
 * graph signals are still loading. This baseline keeps RBAC-gated routes
 * reachable whenever the user has the underlying read capability.
 */
function entitlementRoutePrefixes(ctx: MaterializationContext): string[] {
  if (ctx.personaId === 'user_admin') {
    return can(ctx.user, P.usersRead) ? ['/settings'] : [];
  }

  const routes = new Set<string>();
  const hasOperationalRead = canAny(ctx.user, [
    P.findingsRead,
    P.inventoryRead,
    P.runtimeRead,
    P.graphReadSummary,
    P.graphReadPaths,
    P.investigationsRead,
    P.observabilityMetricsRead,
  ]);

  if (hasOperationalRead) routes.add('/');
  if (can(ctx.user, P.findingsRead)) routes.add('/risks');
  if (can(ctx.user, P.inventoryRead)) {
    routes.add('/resources');
    routes.add('/clusters');
    routes.add('/capabilities');
  }
  if (can(ctx.user, P.runtimeRead)) routes.add('/network-activity');
  if (can(ctx.user, P.graphReadPaths)) routes.add('/attack-paths');
  if (can(ctx.user, P.investigationsRead)) routes.add('/investigation');
  if (canAny(ctx.user, [P.policiesRead, P.rulesRead])) routes.add('/rules');
  if (canAny(ctx.user, [P.observabilityMetricsRead, P.observabilityLogsRead, P.observabilityAgentsRead])) {
    routes.add('/monitoring');
  }
  if (can(ctx.user, P.systemAuditRead)) routes.add('/governance');
  if (can(ctx.user, P.clusterCertificatesRotate)) routes.add('/certificates');
  if (can(ctx.user, P.usersRead)) routes.add('/settings');
  if (can(ctx.user, P.exportFindings) || can(ctx.user, P.findingsRead)) routes.add('/reports');

  return [...routes];
}

export function materializeOperationalPlane(ctx: MaterializationContext): MaterializedOperationalPlane {
  const activeSurfaces: SurfaceDefinition[] = [];

  for (const def of SURFACE_REGISTRY) {
    if (!def.personas.includes(ctx.personaId)) continue;
    if (!hasPermissions(ctx.user, def.requiredPermissions)) continue;
    if (!def.relevance(ctx)) continue;
    activeSurfaces.push(def);
  }

  activeSurfaces.sort((a, b) => b.priority - a.priority);

  const shellVariant: ShellVariant =
    activeSurfaces[0]?.shellVariant ??
    (ctx.personaId === 'user_admin'
      ? 'user_admin'
      : ctx.personaId === 'admin'
        ? 'admin'
        : ctx.personaId === 'operator'
          ? 'operator'
          : 'viewer');

  const routePrefixes = new Set<string>();
  for (const r of entitlementRoutePrefixes(ctx)) routePrefixes.add(r);
  for (const s of activeSurfaces) {
    for (const r of s.routes) routePrefixes.add(r);
  }

  if (shellVariant === 'user_admin') {
    routePrefixes.clear();
    routePrefixes.add('/settings');
  }

  const allowedSet = expandRoutes([...routePrefixes]);
  const allowedRoutes = ALL_APP_ROUTES.filter((r) => allowedSet.has(r));
  const hiddenRoutes = ALL_APP_ROUTES.filter((r) => !allowedSet.has(r));

  const dashboardSections = [
    ...new Set(activeSurfaces.flatMap((s) => s.dashboardSections)),
  ];

  const graphModes = activeSurfaces.flatMap((s) => s.graphModes);
  const graphMode = graphModes[0] ?? 'blast_radius';

  const identity = IDENTITY[shellVariant];
  let defaultRoute = identity.defaultRoute;
  if (!allowedSet.has(defaultRoute)) {
    defaultRoute = allowedRoutes[0] ?? '/settings';
  }

  return {
    surfaces: activeSurfaces.map((s) => s.id),
    surfaceDefinitions: activeSurfaces,
    shellVariant,
    allowedRoutes,
    hiddenRoutes,
    defaultRoute,
    dashboardSections,
    graphMode,
    identityLabel: identity.label,
    identityDescription: identity.description,
  };
}

export function isRouteMaterialized(
  pathname: string,
  allowedRoutes: string[],
): boolean {
  const path = pathname.split('?')[0].replace(/\/$/, '') || '/';
  if (allowedRoutes.includes(path)) return true;
  for (const pattern of allowedRoutes) {
    if (pattern.includes(':')) {
      const re = new RegExp(
        `^${pattern.replace(/:[^/]+/g, '[^/]+')}$`,
      );
      if (re.test(path)) return true;
    }
  }
  return false;
}

export function materializeRoutes(ctx: MaterializationContext): {
  allowedRoutes: string[];
  hiddenRoutes: string[];
  defaultRoute: string;
} {
  const plane = materializeOperationalPlane(ctx);
  return {
    allowedRoutes: plane.allowedRoutes,
    hiddenRoutes: plane.hiddenRoutes,
    defaultRoute: plane.defaultRoute,
  };
}
