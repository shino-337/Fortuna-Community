/**
 * Investigation workflow states — adapts UI emphasis per phase.
 */
import type { InvestigationStatus } from '../store/investigationStore';

export type WorkflowState =
  | 'triage'
  | 'validating'
  | 'scoping'
  | 'containing'
  | 'remediating'
  | 'verifying'
  | 'reporting'
  | 'closed';

export interface WorkflowStateProfile {
  state: WorkflowState;
  label: string;
  graphMode: 'exploitability' | 'blast_radius' | 'integrity';
  emphasizeWidgets: string[];
  recommendedActions: string[];
  actionDensity: 'low' | 'medium' | 'high';
}

export function workflowStateFromInvestigation(status: InvestigationStatus | null | undefined): WorkflowState {
  switch (status) {
    case 'OPEN':
    case 'TRIAGED':
      return 'triage';
    case 'ACTIVE':
      return 'scoping';
    case 'CONTAINED':
      return 'containing';
    case 'REMEDIATING':
      return 'remediating';
    case 'RESOLVED':
      return 'verifying';
    case 'ARCHIVED':
      return 'closed';
    default:
      return 'triage';
  }
}

const PROFILES: Record<WorkflowState, Omit<WorkflowStateProfile, 'state'>> = {
  triage: {
    label: 'Triage',
    graphMode: 'exploitability',
    emphasizeWidgets: ['risk_stats', 'entry_points', 'attack_analysis'],
    recommendedActions: ['Pin critical findings', 'Review blast radius', 'Check runtime signals'],
    actionDensity: 'high',
  },
  validating: {
    label: 'Validating',
    graphMode: 'integrity',
    emphasizeWidgets: ['telemetry_health', 'risk_stats'],
    recommendedActions: ['Confirm telemetry freshness', 'Validate finding provenance'],
    actionDensity: 'medium',
  },
  scoping: {
    label: 'Scoping',
    graphMode: 'blast_radius',
    emphasizeWidgets: ['attack_analysis', 'entry_points'],
    recommendedActions: ['Map affected workloads', 'Identify crown jewels'],
    actionDensity: 'high',
  },
  containing: {
    label: 'Containing',
    graphMode: 'blast_radius',
    emphasizeWidgets: ['attack_analysis', 'risk_stats'],
    recommendedActions: ['Isolate entry points', 'Assign remediation owners'],
    actionDensity: 'high',
  },
  remediating: {
    label: 'Remediating',
    graphMode: 'integrity',
    emphasizeWidgets: ['risk_stats', 'activity_feed'],
    recommendedActions: ['Track remediation tasks', 'Update case status'],
    actionDensity: 'medium',
  },
  verifying: {
    label: 'Verifying',
    graphMode: 'integrity',
    emphasizeWidgets: ['telemetry_health', 'risk_stats'],
    recommendedActions: ['Re-run risk evaluation', 'Confirm exploit chain closure'],
    actionDensity: 'medium',
  },
  reporting: {
    label: 'Reporting',
    graphMode: 'integrity',
    emphasizeWidgets: ['activity_feed'],
    recommendedActions: ['Export timeline', 'Document evidence', 'SLA summary'],
    actionDensity: 'low',
  },
  closed: {
    label: 'Closed',
    graphMode: 'integrity',
    emphasizeWidgets: [],
    recommendedActions: ['Archive case', 'Post-incident review'],
    actionDensity: 'low',
  },
};

export function getWorkflowProfile(status: InvestigationStatus | null | undefined): WorkflowStateProfile {
  const state = workflowStateFromInvestigation(status);
  return { state, ...PROFILES[state] };
}
