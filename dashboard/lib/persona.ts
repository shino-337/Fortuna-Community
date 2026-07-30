/**
 * Layer 2 — Persona experience (UX adaptation only).
 * Layer 1 authorization remains in permissions.ts + server JWT.
 */
import type { User } from '../types';
import { can, canAny, P, type PermissionString } from './permissions';
import { normalizeFortunaRoleKey } from './fortunaRoles';

export type PersonaId = 'viewer' | 'operator' | 'admin' | 'user_admin';

export type TableDensity = 'comfortable' | 'compact';

export type GraphSemanticMode = 'blast_radius' | 'exploitability' | 'integrity';

export type WorkflowPhase =
  | 'triage'
  | 'investigation'
  | 'remediation'
  | 'governance'
  | 'executive'
  | 'runtime'
  | 'attack_path';

export interface PersonaProfile {
  id: PersonaId;
  label: string;
  description: string;
  homeRoute: string;
  defaultDashboardMode: 'overview' | 'full';
  tableDensity: TableDensity;
  graphMode: GraphSemanticMode;
  /** Sections emphasized in copy / strip (not separate apps). */
  emphasis: string[];
  showBulkActions: boolean;
  showRemediationControls: boolean;
  showTelemetryMetadata: boolean;
  showExecutiveNarrative: boolean;
}

export type PermUser = Pick<User, 'username' | 'email' | 'role' | 'permissions' | 'scopeJson' | 'operationalScope'> | null | undefined;

/** Derive a user's persona ID based on their Fortuna role and permissions. */
export function derivePersonaId(user: PermUser): PersonaId;
/** @returns The user's persona (viewer/operator/admin/user_admin) derived from role or JWT permissions. */
export function derivePersonaId(user: PermUser): PersonaId {
  const role = normalizeFortunaRoleKey(user?.role);
  if (role === 'user_admin') return 'user_admin';
  if (role === 'admin') return 'admin';
  if (role === 'cluster_admin') return 'operator';
  if (role === 'operator') return 'operator';
  if (role === 'viewer') return 'viewer';

  // Permission-only JWT (custom roles): infer operational posture.
  if (can(user, P.usersRead) && !can(user, P.findingsRead) && !can(user, P.investigationsRead)) {
    return 'user_admin';
  }
  if (
    canAny(user, [P.systemAuditRead, P.observabilityAgentsRead, P.observabilityLogsRead]) &&
    can(user, P.findingsRead)
  ) {
    return 'admin';
  }
  if (
    canAny(user, [
      P.findingsAck,
      P.findingsBulk,
      P.investigationsWrite,
      P.inventoryModify,
      P.inventoryQuarantine,
    ])
  ) {
    return 'operator';
  }
  return 'viewer';
}

const PROFILES: Record<PersonaId, PersonaProfile> = {
  viewer: {
    id: 'viewer',
    label: 'Observer',
    description: 'Exposure narrative and remediation progress — minimal operational noise.',
    homeRoute: '/',
    defaultDashboardMode: 'overview',
    tableDensity: 'comfortable',
    graphMode: 'blast_radius',
    emphasis: ['business_exposure', 'attack_narratives', 'remediation_progress'],
    showBulkActions: false,
    showRemediationControls: false,
    showTelemetryMetadata: false,
    showExecutiveNarrative: true,
  },
  operator: {
    id: 'operator',
    label: 'Responder',
    description: 'Triage velocity, evidence, graph pivoting, and investigation continuity.',
    homeRoute: '/',
    defaultDashboardMode: 'full',
    tableDensity: 'compact',
    graphMode: 'exploitability',
    emphasis: ['triage_queue', 'investigations', 'runtime_confirmed', 'remediation'],
    showBulkActions: true,
    showRemediationControls: true,
    showTelemetryMetadata: false,
    showExecutiveNarrative: false,
  },
  admin: {
    id: 'admin',
    label: 'Administrator',
    description: 'Full platform, security, runtime, inventory, and governance access.',
    homeRoute: '/dashboard',
    defaultDashboardMode: 'full',
    tableDensity: 'compact',
    graphMode: 'integrity',
    emphasis: ['ingestion_health', 'telemetry_gaps', 'rbac', 'investigation_sla'],
    showBulkActions: true,
    showRemediationControls: true,
    showTelemetryMetadata: true,
    showExecutiveNarrative: false,
  },
  user_admin: {
    id: 'user_admin',
    label: 'Access admin',
    description: 'Fortuna accounts and access policies only.',
    homeRoute: '/settings',
    defaultDashboardMode: 'overview',
    tableDensity: 'comfortable',
    graphMode: 'integrity',
    emphasis: ['users', 'rbac', 'sso'],
    showBulkActions: false,
    showRemediationControls: false,
    showTelemetryMetadata: false,
    showExecutiveNarrative: false,
  },
};

/** Get a persona's full configuration by ID. */
export function getPersonaProfile(id: PersonaId): PersonaProfile;
/** @returns The complete persona profile (label, description, UI settings) for the given persona ID. */
export function getPersonaProfile(id: PersonaId): PersonaProfile {
  return PROFILES[id];
}

/** Resolve a user to their persona and profile. */
export function resolvePersona(user: PermUser): { id: PersonaId; profile: PersonaProfile };
/** @returns The resolved persona ID and full configuration for the given user (or viewer defaults if null). */
export function resolvePersona(user: PermUser): { id: PersonaId; profile: PersonaProfile } {
  const id = derivePersonaId(user);
  return { id, profile: getPersonaProfile(id) };
}

/** Get Tailwind class for table chrome based on persona density. */
export function tableDensityClass(density: TableDensity): string {
  return density === 'compact' ? 'text-caption leading-snug' : 'text-body leading-normal';
}

/** Get a human-readable label for a graph semantic mode. */
export function graphModeLabel(mode: GraphSemanticMode): string;
/** @returns A user-friendly label (e.g., "Blast-radius narrative") for the given graph query mode. */
export function graphModeLabel(mode: GraphSemanticMode): string {
  switch (mode) {
    case 'blast_radius':
      return 'Blast-radius narrative';
    case 'exploitability':
      return 'Exploitability & pivot';
    case 'integrity':
      return 'Telemetry integrity';
    default:
      return mode;
  }
}

/** Get a description of what a graph semantic mode emphasizes. */
export function graphModeDescription(mode: GraphSemanticMode): string;
/** @returns A one-sentence description of the focus areas for this graph query mode (empty if unknown). */
export function graphModeDescription(mode: GraphSemanticMode): string {
  switch (mode) {
    case 'blast_radius':
      return 'Emphasizes business impact, crown jewels, and simplified attack flow.';
    case 'exploitability':
      return 'Emphasizes runtime-confirmed edges, privilege escalation, and lateral movement.';
    case 'integrity':
      return 'Emphasizes coverage gaps, inferred vs observed edges, and stale nodes.';
    default:
      return '';
  }
}

/** Derive the current workflow phase from a pathname. */
export function workflowPhaseFromPath(pathname: string): WorkflowPhase;
/** @returns The inferred workflow phase (triage/investigation/remediation/governance/executive/runtime/attack_path) based on URL patterns. Defaults to triage for unknown paths. */
export function workflowPhaseFromPath(pathname: string): WorkflowPhase {
  if (pathname.startsWith('/investigation')) return 'investigation';
  if (pathname.startsWith('/risks')) return 'triage';
  if (pathname.startsWith('/attack-paths')) return 'attack_path';
  if (pathname.startsWith('/network-activity') || pathname.startsWith('/monitoring')) return 'runtime';
  if (pathname.startsWith('/governance') || pathname.startsWith('/settings')) return 'governance';
  if (pathname.startsWith('/reports')) return 'executive';
  if (pathname.includes('remediation')) return 'remediation';
  return 'triage';
}

/** Get a human-readable label for a workflow phase. */
export function workflowPhaseLabel(phase: WorkflowPhase): string;
/** @returns A user-friendly label (e.g., "Triage mode") for the given workflow phase, or the raw phase name if unknown. */
export function workflowPhaseLabel(phase: WorkflowPhase): string {
  const labels: Record<WorkflowPhase, string> = {
    triage: 'Triage mode',
    investigation: 'Investigation mode',
    remediation: 'Remediation mode',
    governance: 'Governance mode',
    executive: 'Executive reporting',
    runtime: 'Runtime incident',
    attack_path: 'Attack path exploration',
  };
  return labels[phase] ?? phase;
}

/** Check if a persona allows an action given the user's permissions and profile settings. */
export function personaAllowsAction(
  profile: PersonaProfile,
  action: 'bulk' | 'remediation' | 'destructive',
  user: PermUser,
  requiredPerm: PermissionString,
): boolean;
/** @returns true only if the user has the required permission AND their persona's UI settings permit this action type (e.g., bulk actions require showBulkActions). */
export function personaAllowsAction(
  profile: PersonaProfile,
  action: 'bulk' | 'remediation' | 'destructive',
  user: PermUser,
  requiredPerm: PermissionString,
): boolean {
  if (!can(user, requiredPerm)) return false;
  if (action === 'bulk' && !profile.showBulkActions) return false;
  if (action === 'remediation' && !profile.showRemediationControls) return false;
  return true;
}
