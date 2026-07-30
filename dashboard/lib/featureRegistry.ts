/**
 * Layer 2 — Feature registry: declarative contracts for routes, widgets, and workflows.
 * Authority remains on the server; this governs UX existence only.
 */
import { P, type PermissionString } from './permissions';
import type { PersonaId } from './persona';
import { PAGE_TITLES } from './pageTitles';

export type FeatureId =
  | 'dashboard'
  | 'clusters'
  | 'resources'
  | 'network_activity'
  | 'risk_operations'
  | 'investigation'
  | 'capabilities'
  | 'rules_catalog'
  | 'attack_paths'
  | 'monitoring'
  | 'governance'
  | 'reports'
  | 'settings'
  | 'certificates'
  | 'findings_export'
  | 'findings_bulk'
  | 'remediation_controls';

export type OwnershipModel =
  | 'none'
  | 'clusterScoped'
  | 'caseScoped'
  | 'platform';

export interface FeatureDefinition {
  id: FeatureId;
  label: string;
  /** Any of these permissions grants feature access (Layer 1). */
  permissionsAny: PermissionString[];
  personas: PersonaId[];
  ownershipModel: OwnershipModel;
  /** Feature should not render without agent/ingestion signal. */
  telemetryRequired: boolean;
  /** Shown on main nav when visible. */
  navEligible: boolean;
  graphRelevant: boolean;
}

export const FEATURE_REGISTRY: Record<FeatureId, FeatureDefinition> = {
  dashboard: {
    id: 'dashboard',
    label: PAGE_TITLES.dashboard,
    permissionsAny: [P.findingsRead],
    personas: ['viewer', 'operator', 'admin'],
    ownershipModel: 'clusterScoped',
    telemetryRequired: false,
    navEligible: true,
    graphRelevant: true,
  },
  clusters: {
    id: 'clusters',
    label: 'Clusters',
    permissionsAny: [P.inventoryRead],
    personas: ['viewer', 'operator', 'admin'],
    ownershipModel: 'clusterScoped',
    telemetryRequired: false,
    navEligible: true,
    graphRelevant: false,
  },
  resources: {
    id: 'resources',
    label: PAGE_TITLES.resources,
    permissionsAny: [P.inventoryRead],
    personas: ['viewer', 'operator', 'admin'],
    ownershipModel: 'clusterScoped',
    telemetryRequired: false,
    navEligible: true,
    graphRelevant: false,
  },
  network_activity: {
    id: 'network_activity',
    label: PAGE_TITLES.networkActivity,
    permissionsAny: [P.runtimeRead],
    personas: ['viewer', 'operator', 'admin'],
    ownershipModel: 'clusterScoped',
    telemetryRequired: true,
    navEligible: true,
    graphRelevant: false,
  },
  risk_operations: {
    id: 'risk_operations',
    label: PAGE_TITLES.riskOperations,
    permissionsAny: [P.findingsRead],
    personas: ['viewer', 'operator', 'admin'],
    ownershipModel: 'clusterScoped',
    telemetryRequired: false,
    navEligible: true,
    graphRelevant: false,
  },
  investigation: {
    id: 'investigation',
    label: PAGE_TITLES.investigations,
    permissionsAny: [P.investigationsRead],
    personas: ['viewer', 'operator', 'admin'],
    ownershipModel: 'caseScoped',
    telemetryRequired: false,
    navEligible: true,
    graphRelevant: false,
  },
  capabilities: {
    id: 'capabilities',
    label: PAGE_TITLES.capabilities,
    permissionsAny: [P.inventoryRead],
    personas: ['viewer', 'operator', 'admin'],
    ownershipModel: 'clusterScoped',
    telemetryRequired: false,
    navEligible: true,
    graphRelevant: false,
  },
  rules_catalog: {
    id: 'rules_catalog',
    label: PAGE_TITLES.policyRules,
    permissionsAny: [P.policiesRead, P.rulesRead],
    personas: ['operator', 'admin'],
    ownershipModel: 'platform',
    telemetryRequired: false,
    navEligible: true,
    graphRelevant: false,
  },
  attack_paths: {
    id: 'attack_paths',
    label: PAGE_TITLES.attackAnalysis,
    permissionsAny: [P.graphReadSummary, P.graphReadPaths],
    personas: ['viewer', 'operator', 'admin'],
    ownershipModel: 'clusterScoped',
    telemetryRequired: false,
    navEligible: true,
    graphRelevant: true,
  },
  monitoring: {
    id: 'monitoring',
    label: PAGE_TITLES.monitoring,
    permissionsAny: [P.observabilityMetricsRead, P.observabilityLogsRead, P.observabilityAgentsRead],
    personas: ['viewer', 'admin', 'operator'],
    ownershipModel: 'platform',
    telemetryRequired: false,
    navEligible: true,
    graphRelevant: false,
  },
  governance: {
    id: 'governance',
    label: PAGE_TITLES.governance,
    permissionsAny: [P.systemAuditRead],
    personas: ['admin'],
    ownershipModel: 'platform',
    telemetryRequired: false,
    navEligible: true,
    graphRelevant: false,
  },
  reports: {
    id: 'reports',
    label: PAGE_TITLES.reports,
    permissionsAny: [P.findingsRead],
    personas: ['viewer', 'operator', 'admin'],
    ownershipModel: 'clusterScoped',
    telemetryRequired: false,
    navEligible: true,
    graphRelevant: false,
  },
  settings: {
    id: 'settings',
    label: PAGE_TITLES.settings,
    permissionsAny: [P.usersRead, P.usersUpdate, P.usersRoleAssign, P.usersCreate, P.rulesRead, P.policiesRead],
    personas: ['admin', 'user_admin', 'operator'],
    ownershipModel: 'none',
    telemetryRequired: false,
    navEligible: true,
    graphRelevant: false,
  },
  certificates: {
    id: 'certificates',
    label: PAGE_TITLES.certificates,
    permissionsAny: [P.clusterCertificatesRotate],
    personas: ['admin'],
    ownershipModel: 'platform',
    telemetryRequired: false,
    navEligible: false,
    graphRelevant: false,
  },
  findings_export: {
    id: 'findings_export',
    label: 'Export findings',
    permissionsAny: [P.exportFindings],
    personas: ['operator', 'admin'],
    ownershipModel: 'clusterScoped',
    telemetryRequired: false,
    navEligible: false,
    graphRelevant: false,
  },
  findings_bulk: {
    id: 'findings_bulk',
    label: 'Bulk findings actions',
    permissionsAny: [P.findingsBulk],
    personas: ['operator', 'admin'],
    ownershipModel: 'clusterScoped',
    telemetryRequired: false,
    navEligible: false,
    graphRelevant: false,
  },
  remediation_controls: {
    id: 'remediation_controls',
    label: 'Remediation controls',
    permissionsAny: [P.findingsAck, P.findingsResolve, P.investigationsWrite],
    personas: ['operator', 'admin'],
    ownershipModel: 'caseScoped',
    telemetryRequired: false,
    navEligible: false,
    graphRelevant: false,
  },
};

/** Get a feature definition by ID. */
export function getFeature(id: FeatureId): FeatureDefinition {
  return FEATURE_REGISTRY[id];
}
