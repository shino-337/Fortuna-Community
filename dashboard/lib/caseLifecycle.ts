import type { InvestigationStatus } from '../store/investigationStore';

/** The steps a case moves through, in order. Archived is not a step: it removes the case from work. */
export const CASE_STEPS: { status: InvestigationStatus; label: string }[] = [
  { status: 'OPEN', label: 'Open' },
  { status: 'TRIAGED', label: 'Triaged' },
  { status: 'ACTIVE', label: 'Investigating' },
  { status: 'CONTAINED', label: 'Contained' },
  { status: 'REMEDIATING', label: 'Remediating' },
  { status: 'RESOLVED', label: 'Resolved' },
];

export function caseStatusLabel(status: InvestigationStatus): string {
  if (status === 'ARCHIVED') return 'Archived';
  return CASE_STEPS.find((s) => s.status === status)?.label ?? status;
}

// Mirrors core/pkg/investigation/statemachine.go; the server checks every change again.
const TRANSITIONS: Record<InvestigationStatus, InvestigationStatus[]> = {
  OPEN: ['TRIAGED', 'ACTIVE', 'ARCHIVED'],
  TRIAGED: ['ACTIVE', 'CONTAINED', 'ARCHIVED'],
  ACTIVE: ['CONTAINED', 'REMEDIATING', 'TRIAGED', 'ARCHIVED'],
  CONTAINED: ['REMEDIATING', 'RESOLVED', 'ACTIVE', 'ARCHIVED'],
  REMEDIATING: ['RESOLVED', 'CONTAINED', 'ARCHIVED'],
  RESOLVED: ['ARCHIVED', 'ACTIVE'],
  ARCHIVED: [],
};

/** Whether a case may move from one status to another. Admins may make any change. */
export function canMoveCase(from: InvestigationStatus, to: InvestigationStatus, isAdmin: boolean): boolean {
  if (from === to) return false;
  if (isAdmin) return true;
  return TRANSITIONS[from]?.includes(to) ?? false;
}

export function isCaseClosed(status: InvestigationStatus): boolean {
  return status === 'RESOLVED' || status === 'ARCHIVED';
}

/** A linked finding still needs work while it is new or in review. */
export function isFindingOpen(status: string | undefined): boolean {
  const s = String(status ?? '').toLowerCase();
  return s === 'active' || s === 'new' || s === 'acknowledged';
}
