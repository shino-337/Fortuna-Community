/**
 * Operational Surface Registry — surfaces materialize BEFORE render (no render-then-hide).
 */
import type { PersonaId, GraphSemanticMode } from './persona';
import type { PermissionString } from './permissions';
import { P } from './permissions';
import type { MaterializationContext } from './routeMaterialization';

export type OperationalSurface =
  | 'viewer_overview'
  | 'viewer_exposure'
  | 'viewer_attack_analysis'
  | 'viewer_investigations'
  | 'viewer_runtime'
  | 'viewer_capabilities'
  | 'viewer_monitoring'
  | 'operator_triage'
  | 'operator_runtime'
  | 'operator_investigation'
  | 'operator_attack_analysis'
  | 'operator_remediation'
  | 'operator_rules'
  | 'operator_capabilities'
  | 'admin_governance'
  | 'admin_monitoring'
  | 'admin_telemetry'
  | 'admin_platform'
  | 'admin_runtime'
  | 'admin_investigation'
  | 'admin_attack_analysis'
  | 'admin_rules'
  | 'admin_capabilities'
  | 'admin_certificates'
  | 'admin_settings'
  | 'user_admin_settings';

export type ShellVariant = 'viewer' | 'operator' | 'admin' | 'user_admin';

export interface SurfaceDefinition {
  id: OperationalSurface;
  personas: PersonaId[];
  requiredPermissions: PermissionString[];
  relevance: (ctx: MaterializationContext) => boolean;
  routes: string[];
  dashboardSections: string[];
  graphModes: GraphSemanticMode[];
  shellVariant: ShellVariant;
  priority: number;
}

export const SURFACE_REGISTRY: SurfaceDefinition[] = [
  {
    id: 'viewer_overview',
    personas: ['viewer'],
    requiredPermissions: [P.findingsRead],
    relevance: (ctx) => ctx.ownership.hasOperationalScope,
    routes: ['/', '/reports'],
    dashboardSections: ['exposure_summary', 'threat_narrative', 'scoped_trends'],
    graphModes: ['blast_radius'],
    shellVariant: 'viewer',
    priority: 100,
  },
  {
    id: 'viewer_exposure',
    personas: ['viewer'],
    requiredPermissions: [P.findingsRead],
    relevance: () => true,
    routes: ['/risks', '/risks/findings', '/resources', '/clusters'],
    dashboardSections: ['exposure_summary'],
    graphModes: ['blast_radius'],
    shellVariant: 'viewer',
    priority: 90,
  },
  {
    id: 'viewer_attack_analysis',
    personas: ['viewer'],
    requiredPermissions: [P.graphReadPaths],
    relevance: (ctx) =>
      ctx.signals.hasAttackPaths === true || ctx.signals.hasOpenFindings === true,
    routes: ['/attack-paths'],
    dashboardSections: ['attack_chains'],
    graphModes: ['blast_radius'],
    shellVariant: 'viewer',
    priority: 86,
  },
  {
    id: 'viewer_investigations',
    personas: ['viewer'],
    requiredPermissions: [P.investigationsRead],
    relevance: (ctx) => ctx.signals.hasAssignedCases !== false,
    routes: ['/investigation'],
    dashboardSections: ['assigned_investigations'],
    graphModes: ['blast_radius'],
    shellVariant: 'viewer',
    priority: 85,
  },
  {
    id: 'viewer_runtime',
    personas: ['viewer'],
    requiredPermissions: [P.runtimeRead],
    relevance: (ctx) =>
      ctx.telemetry.agentsHealthy !== false && ctx.telemetry.ingestionStale !== true,
    routes: ['/network-activity'],
    dashboardSections: ['runtime_alerts'],
    graphModes: ['blast_radius'],
    shellVariant: 'viewer',
    priority: 80,
  },
  {
    id: 'viewer_capabilities',
    personas: ['viewer'],
    requiredPermissions: [P.inventoryRead],
    relevance: (ctx) => ctx.ownership.hasOperationalScope,
    routes: ['/capabilities'],
    dashboardSections: ['capability_context'],
    graphModes: ['blast_radius'],
    shellVariant: 'viewer',
    priority: 78,
  },
  {
    id: 'viewer_monitoring',
    personas: ['viewer'],
    requiredPermissions: [P.observabilityMetricsRead, P.observabilityAgentsRead],
    relevance: () => true,
    routes: ['/monitoring'],
    dashboardSections: ['telemetry_reliability'],
    graphModes: ['blast_radius'],
    shellVariant: 'viewer',
    priority: 76,
  },
  {
    id: 'operator_triage',
    personas: ['operator'],
    requiredPermissions: [P.findingsRead],
    relevance: (ctx) => ctx.ownership.hasOperationalScope,
    routes: ['/', '/risks', '/risks/findings', '/risks/pce', '/risks/evidence'],
    dashboardSections: ['triage_queue', 'sla_pressure'],
    graphModes: ['exploitability'],
    shellVariant: 'operator',
    priority: 100,
  },
  {
    id: 'operator_runtime',
    personas: ['operator'],
    requiredPermissions: [P.runtimeRead],
    relevance: (ctx) =>
      ctx.telemetry.agentsHealthy !== false && ctx.telemetry.ingestionStale !== true,
    routes: ['/network-activity'],
    dashboardSections: ['runtime_alerts'],
    graphModes: ['exploitability'],
    shellVariant: 'operator',
    priority: 88,
  },
  {
    id: 'operator_investigation',
    personas: ['operator'],
    requiredPermissions: [P.investigationsRead],
    relevance: () => true,
    routes: ['/investigation'],
    dashboardSections: ['active_investigations', 'incident_context'],
    graphModes: ['exploitability'],
    shellVariant: 'operator',
    priority: 95,
  },
  {
    id: 'operator_attack_analysis',
    personas: ['operator'],
    requiredPermissions: [P.graphReadPaths],
    relevance: (ctx) =>
      ctx.signals.hasAttackPaths === true || ctx.signals.hasOpenFindings === true,
    routes: ['/attack-paths'],
    dashboardSections: ['attack_chains'],
    graphModes: ['exploitability'],
    shellVariant: 'operator',
    priority: 82,
  },
  {
    id: 'operator_remediation',
    personas: ['operator'],
    requiredPermissions: [P.findingsAck],
    relevance: (ctx) =>
      ctx.signals.hasOpenFindings === true || ctx.signals.hasActiveRemediation === true,
    routes: [],
    dashboardSections: ['remediation_blockers'],
    graphModes: ['exploitability'],
    shellVariant: 'operator',
    priority: 80,
  },
  {
    id: 'operator_rules',
    personas: ['operator'],
    requiredPermissions: [P.policiesRead, P.rulesRead],
    relevance: () => true,
    routes: ['/rules'],
    dashboardSections: ['policy_rules'],
    graphModes: ['exploitability'],
    shellVariant: 'operator',
    priority: 78,
  },
  {
    id: 'operator_capabilities',
    personas: ['operator'],
    requiredPermissions: [P.inventoryRead],
    relevance: (ctx) => ctx.ownership.hasOperationalScope,
    routes: ['/resources', '/capabilities'],
    dashboardSections: ['capability_context'],
    graphModes: ['exploitability'],
    shellVariant: 'operator',
    priority: 76,
  },
  {
    id: 'admin_platform',
    personas: ['admin'],
    requiredPermissions: [P.findingsRead],
    relevance: (ctx) => ctx.ownership.hasOperationalScope,
    routes: ['/', '/clusters', '/resources', '/reports'],
    dashboardSections: ['platform_integrity', 'cluster_posture'],
    graphModes: ['integrity'],
    shellVariant: 'admin',
    priority: 100,
  },
  {
    id: 'admin_governance',
    personas: ['admin'],
    requiredPermissions: [P.systemAuditRead],
    relevance: () => true,
    routes: ['/governance'],
    dashboardSections: ['governance_exposure'],
    graphModes: ['integrity'],
    shellVariant: 'admin',
    priority: 95,
  },
  {
    id: 'admin_monitoring',
    personas: ['admin'],
    requiredPermissions: [P.observabilityMetricsRead],
    relevance: () => true,
    routes: ['/monitoring'],
    dashboardSections: ['operational_oversight'],
    graphModes: ['integrity'],
    shellVariant: 'admin',
    priority: 92,
  },
  {
    id: 'admin_telemetry',
    personas: ['admin'],
    requiredPermissions: [P.observabilityAgentsRead],
    relevance: (ctx) =>
      ctx.telemetry.pipelineDegraded === true ||
      ctx.telemetry.agentsHealthy === false ||
      ctx.telemetry.ingestionStale === true,
    routes: [],
    dashboardSections: ['telemetry_degradation', 'telemetry_reliability'],
    graphModes: ['integrity'],
    shellVariant: 'admin',
    priority: 88,
  },
  {
    id: 'admin_runtime',
    personas: ['admin'],
    requiredPermissions: [P.runtimeRead],
    relevance: (ctx) =>
      ctx.telemetry.agentsHealthy !== false && ctx.telemetry.ingestionStale !== true,
    routes: ['/network-activity'],
    dashboardSections: ['runtime_alerts'],
    graphModes: ['integrity'],
    shellVariant: 'admin',
    priority: 87,
  },
  {
    id: 'admin_investigation',
    personas: ['admin'],
    requiredPermissions: [P.investigationsRead],
    relevance: () => true,
    routes: ['/investigation'],
    dashboardSections: ['investigation_sla'],
    graphModes: ['integrity'],
    shellVariant: 'admin',
    priority: 86,
  },
  {
    id: 'admin_attack_analysis',
    personas: ['admin'],
    requiredPermissions: [P.graphReadPaths],
    relevance: (ctx) =>
      ctx.signals.hasAttackPaths === true || ctx.signals.hasOpenFindings === true,
    routes: ['/attack-paths'],
    dashboardSections: ['attack_chains'],
    graphModes: ['integrity'],
    shellVariant: 'admin',
    priority: 85,
  },
  {
    id: 'admin_rules',
    personas: ['admin'],
    requiredPermissions: [P.policiesRead, P.rulesRead],
    relevance: () => true,
    routes: ['/rules'],
    dashboardSections: ['policy_rules'],
    graphModes: ['integrity'],
    shellVariant: 'admin',
    priority: 84,
  },
  {
    id: 'admin_capabilities',
    personas: ['admin'],
    requiredPermissions: [P.inventoryRead],
    relevance: (ctx) => ctx.ownership.hasOperationalScope,
    routes: ['/capabilities'],
    dashboardSections: ['capability_context'],
    graphModes: ['integrity'],
    shellVariant: 'admin',
    priority: 83,
  },
  {
    id: 'admin_certificates',
    personas: ['admin'],
    requiredPermissions: [P.clusterCertificatesRotate],
    relevance: () => true,
    routes: ['/certificates'],
    dashboardSections: ['certificate_posture'],
    graphModes: ['integrity'],
    shellVariant: 'admin',
    priority: 82,
  },
  {
    id: 'admin_settings',
    personas: ['admin'],
    requiredPermissions: [P.usersRead],
    relevance: () => true,
    routes: ['/settings'],
    dashboardSections: [],
    graphModes: ['integrity'],
    shellVariant: 'admin',
    priority: 81,
  },
  {
    id: 'user_admin_settings',
    personas: ['user_admin'],
    requiredPermissions: [P.usersRead],
    relevance: () => true,
    routes: ['/settings'],
    dashboardSections: [],
    graphModes: ['integrity'],
    shellVariant: 'user_admin',
    priority: 100,
  },
];

export function getSurfaceDefinition(id: OperationalSurface): SurfaceDefinition | undefined {
  return SURFACE_REGISTRY.find((s) => s.id === id);
}
