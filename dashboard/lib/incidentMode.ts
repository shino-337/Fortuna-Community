/**
 * Incident-mode cognition — platform behavior during active response.
 */
import type { InvestigationStatus } from '../store/investigationStore';

export type IncidentModePhase =
  | 'normal'
  | 'elevated'
  | 'active_incident'
  | 'containment'
  | 'remediation'
  | 'recovery'
  | 'postmortem';

export interface IncidentModeContext {
  phase: IncidentModePhase;
  activeCaseId: string | null;
  activeCaseTitle: string | null;
  investigationStatus: InvestigationStatus | null;
  runtimeConfirmed: boolean;
  escalated: boolean;
  responderCount: number;
}

/** Map investigation status to the corresponding incident mode phase. */
export function incidentPhaseFromInvestigationStatus(status: InvestigationStatus | null | undefined): IncidentModePhase {
  switch (status) {
    case 'OPEN':
    case 'TRIAGED':
      return 'elevated';
    case 'ACTIVE':
      return 'active_incident';
    case 'CONTAINED':
      return 'containment';
    case 'REMEDIATING':
      return 'remediation';
    case 'RESOLVED':
      return 'recovery';
    case 'ARCHIVED':
      return 'postmortem';
    default:
      return 'normal';
  }
}

/** Check if the incident mode represents an active response state. */
export function isIncidentActive(phase: IncidentModePhase): boolean {
  return phase !== 'normal' && phase !== 'postmortem';
}

/** Get a human-readable label for an incident mode phase. */
export function incidentModeLabel(phase: IncidentModePhase): string {
  const labels: Record<IncidentModePhase, string> = {
    normal: 'Normal operations',
    elevated: 'Elevated awareness',
    active_incident: 'Active incident',
    containment: 'Containment',
    remediation: 'Remediation',
    recovery: 'Recovery',
    postmortem: 'Post-incident review',
  };
  return labels[phase];
}

/** Check if navigation should be condensed for the given phase. */
export function incidentNavCondensed(phase: IncidentModePhase): boolean {
  return isIncidentActive(phase);
}

/** Get the graph semantic mode appropriate for an incident phase. */
export function incidentGraphSemanticMode(phase: IncidentModePhase): 'exploitability' | 'blast_radius' | 'integrity' {
  if (phase === 'containment' || phase === 'active_incident') return 'blast_radius';
  if (phase === 'remediation' || phase === 'recovery') return 'integrity';
  return 'exploitability';
}

/** Get dashboard widget priority order for an incident phase. */
export function incidentDashboardWidgetPriority(phase: IncidentModePhase): string[] {
  if (!isIncidentActive(phase)) {
    return ['hero_metrics', 'risk_stats', 'entry_points', 'attack_analysis', 'activity_feed'];
  }
  return ['persona_strip', 'hero_metrics', 'attack_analysis', 'entry_points', 'risk_stats', 'telemetry_health', 'activity_feed'];
}
