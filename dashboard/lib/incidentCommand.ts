/**
 * Formal incident command structure — ICS-style roles (client hints until server authority).
 */
import type { InvestigationCase } from '../store/investigationStore';

export type CommandRole =
  | 'commander'
  | 'containment_lead'
  | 'remediation_lead'
  | 'communications_owner'
  | 'delegate';

export interface IncidentCommandAssignment {
  caseId: string;
  roles: Partial<Record<CommandRole, string>>;
  delegates: string[];
  updatedAt: string;
}

export interface IncidentCommandStructure {
  assignment: IncidentCommandAssignment;
  gaps: CommandRole[];
  coveragePercent: number;
  summary: string;
}

const REQUIRED_ROLES: CommandRole[] = [
  'commander',
  'containment_lead',
  'remediation_lead',
  'communications_owner',
];

const ROLE_LABELS: Record<CommandRole, string> = {
  commander: 'Incident commander',
  containment_lead: 'Containment lead',
  remediation_lead: 'Remediation lead',
  communications_owner: 'Communications owner',
  delegate: 'Delegate',
};

export function roleLabel(role: CommandRole): string {
  return ROLE_LABELS[role];
}

/** Derive default command from case owner + collaboration when no explicit assignment. */
export function deriveCommandAssignment(caseRow: InvestigationCase | null): IncidentCommandAssignment {
  if (!caseRow) {
    return { caseId: '', roles: {}, delegates: [], updatedAt: new Date().toISOString() };
  }
  const owner = caseRow.owner?.trim() || '';
  const assignees = caseRow.collaboration?.assignees ?? [];
  const roles: Partial<Record<CommandRole, string>> = {};
  if (owner) {
    roles.commander = owner;
    roles.containment_lead = owner;
  }
  if (assignees[0]) roles.remediation_lead = assignees[0];
  if (assignees[1]) roles.communications_owner = assignees[1];
  else if (assignees[0] && assignees[0] !== owner) roles.communications_owner = assignees[0];
  return {
    caseId: caseRow.id,
    roles,
    delegates: assignees.filter((a) => a !== owner).slice(2),
    updatedAt: caseRow.updatedAt,
  };
}

export function buildIncidentCommandStructure(
  assignment: IncidentCommandAssignment,
): IncidentCommandStructure {
  const gaps = REQUIRED_ROLES.filter((r) => !assignment.roles[r]?.trim());
  const filled = REQUIRED_ROLES.length - gaps.length;
  const coveragePercent = Math.round((filled / REQUIRED_ROLES.length) * 100);
  let summary = 'Command structure complete — roles assigned.';
  if (gaps.length > 0) {
    summary = `Missing: ${gaps.map((g) => ROLE_LABELS[g]).join(', ')}. Assign before destructive actions.`;
  }
  return { assignment, gaps, coveragePercent, summary };
}

export function mergeCommandAssignment(
  base: IncidentCommandAssignment,
  patch: Partial<Record<CommandRole, string>> & { delegates?: string[] },
): IncidentCommandAssignment {
  return {
    ...base,
    roles: { ...base.roles, ...patch },
    delegates: patch.delegates ?? base.delegates,
    updatedAt: new Date().toISOString(),
  };
}
