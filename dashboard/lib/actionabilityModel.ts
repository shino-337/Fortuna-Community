/**
 * Actionability modeling — what can ACTUALLY be done now vs what merely matters.
 */
import type { PersonaId } from './persona';
import type { PermUser } from './persona';
import { can, canAny, P } from './permissions';
import type { TelemetryContext } from './telemetryContext';
import type { UncertaintyPropagationResult } from './uncertaintyPropagation';

export type ActionBlocker =
  | 'no_permission'
  | 'owner_unavailable'
  | 'telemetry_degraded'
  | 'runtime_unverified'
  | 'remediation_blocked'
  | 'case_read_only'
  | 'no_operational_scope'
  | 'escalation_required';

export interface ActionabilityAssessment {
  canAct: boolean;
  blockers: ActionBlocker[];
  blockerMessages: string[];
  recommendedAction: string | null;
  urgencyWithoutAction: 'low' | 'medium' | 'high' | 'critical';
}

export interface ActionabilityInput {
  user: PermUser;
  personaId: PersonaId;
  telemetry: TelemetryContext;
  uncertainty: UncertaintyPropagationResult;
  hasWriteOnCase: boolean;
  caseOwner?: string | null;
  remediationBlocked?: boolean;
  runtimeVerified?: boolean;
  hasOperationalScope: boolean;
  requiresEscalation?: boolean;
}

const BLOCKER_COPY: Record<ActionBlocker, string> = {
  no_permission: 'Your role cannot perform this action (missing permission).',
  owner_unavailable: 'No remediation owner is assigned — actions may not be tracked.',
  telemetry_degraded: 'Telemetry is degraded — validate findings before containment actions.',
  runtime_unverified: 'Runtime evidence is not confirmed — exploitability actions are high-risk.',
  remediation_blocked: 'A remediation step is blocked pending external approval or ticket.',
  case_read_only: 'Investigation case is read-only for your role.',
  no_operational_scope: 'Selected scope is outside your assigned clusters or namespaces.',
  escalation_required: 'Escalation is required before destructive remediation.',
};

/** Assess what actions a user can take given their permissions and context. */
export function assessActionability(input: ActionabilityInput): ActionabilityAssessment {
  const blockers: ActionBlocker[] = [];
  const username = input.user?.username ?? '';

  if (!input.hasOperationalScope) blockers.push('no_operational_scope');
  if (!input.hasWriteOnCase) blockers.push('case_read_only');
  if (!can(input.user, P.investigationsWrite) && !canAny(input.user, [P.findingsAck, P.findingsResolve])) {
    blockers.push('no_permission');
  }
  if (input.uncertainty.aggregateConfidence < 0.65) blockers.push('telemetry_degraded');
  if (input.runtimeVerified === false) blockers.push('runtime_unverified');
  if (input.remediationBlocked) blockers.push('remediation_blocked');
  if (!input.caseOwner?.trim() && input.personaId === 'operator') blockers.push('owner_unavailable');
  if (input.requiresEscalation && input.personaId !== 'admin') blockers.push('escalation_required');

  const canAct = blockers.length === 0 || (blockers.length === 1 && blockers[0] === 'owner_unavailable');

  let recommendedAction: string | null = null;
  if (canAct && can(input.user, P.investigationsWrite)) {
    recommendedAction = 'Update case status, assign owner, and add remediation steps.';
  } else if (canAny(input.user, [P.findingsAck, P.findingsBulk])) {
    recommendedAction = 'Acknowledge or triage findings in Risk Operations.';
  } else if (can(input.user, P.findingsRead)) {
    recommendedAction = 'Review evidence and escalate to an operator or admin.';
  }

  const urgencyWithoutAction =
    input.uncertainty.aggregateConfidence < 0.5
      ? 'critical'
      : blockers.includes('runtime_unverified')
        ? 'high'
        : 'medium';

  return {
    canAct,
    blockers,
    blockerMessages: blockers.map((b) => BLOCKER_COPY[b]),
    recommendedAction,
    urgencyWithoutAction,
  };
}
