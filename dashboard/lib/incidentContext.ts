/**
 * Builds incident context from investigations + operational signals.
 */
import type { InvestigationCase } from '../store/investigationStore';
import {
  incidentPhaseFromInvestigationStatus,
  type IncidentModeContext,
  type IncidentModePhase,
} from './incidentMode';

export function buildIncidentContext(
  cases: InvestigationCase[],
  activeCaseId: string | null,
  opts?: { runtimeConfirmed?: boolean },
): IncidentModeContext {
  const active =
    cases.find((c) => c.id === activeCaseId) ??
    cases.find((c) => ['ACTIVE', 'CONTAINED', 'REMEDIATING', 'OPEN', 'TRIAGED'].includes(c.status)) ??
    null;

  const phase: IncidentModePhase = active
    ? incidentPhaseFromInvestigationStatus(active.status)
    : cases.some((c) => ['ACTIVE', 'REMEDIATING', 'CONTAINED'].includes(c.status))
      ? 'elevated'
      : 'normal';

  const assignees = active?.collaboration?.assignees ?? [];
  const watchers = active?.collaboration?.watchers ?? [];
  const responderCount = new Set([active?.owner, ...assignees, ...watchers].filter(Boolean)).size;

  return {
    phase,
    activeCaseId: active?.id ?? null,
    activeCaseTitle: active?.title ?? null,
    investigationStatus: active?.status ?? null,
    runtimeConfirmed: Boolean(opts?.runtimeConfirmed),
    escalated: phase === 'active_incident' || phase === 'containment',
    responderCount,
  };
}
