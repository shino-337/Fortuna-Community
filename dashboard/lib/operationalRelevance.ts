/**
 * Layer 5 — Operational relevance: visible ≠ meaningful for this user's workflow.
 */
import type { FeatureId } from './featureRegistry';
import { getFeature } from './featureRegistry';
import type { PersonaId } from './persona';
import type { OwnershipContext } from './ownershipContext';
import type { TelemetryContext } from './telemetryContext';
import { evaluateVisibility, type VisibilityResult } from './visibilityEngine';
import type { PermUser } from './persona';

export interface OperationalRelevanceSignals {
  /** Open findings in current scope (dashboard/risk). */
  hasOpenFindings?: boolean;
  /** Attack paths or chains available in scope. */
  hasAttackPaths?: boolean;
  /** Investigation cases assigned or shared with user. */
  hasAssignedCases?: boolean;
  /** User can run remediation actions in current workflow. */
  hasActiveRemediation?: boolean;
}

export interface OperationalRelevanceInput {
  feature: FeatureId;
  user: PermUser;
  personaId: PersonaId;
  ownership: OwnershipContext;
  telemetry: TelemetryContext;
  signals?: OperationalRelevanceSignals;
}

export interface OperationalRelevanceResult {
  relevant: boolean;
  reason: string;
  visibility: VisibilityResult;
}

export function evaluateOperationalRelevance(input: OperationalRelevanceInput): OperationalRelevanceResult {
  const visibility = evaluateVisibility({
    feature: input.feature,
    user: input.user,
    personaId: input.personaId,
    ownership: input.ownership,
    telemetry: input.telemetry,
  });

  if (!visibility.visible) {
    return { relevant: false, reason: visibility.reason, visibility };
  }

  if (
    visibility.semanticState === 'no_permission' ||
    visibility.semanticState === 'no_scope' ||
    visibility.semanticState === 'no_telemetry'
  ) {
    return { relevant: false, reason: visibility.reason, visibility };
  }

  const def = getFeature(input.feature);
  const { personaId, ownership, telemetry, signals = {} } = input;

  if (personaId === 'viewer') {
    if (def.id === 'monitoring' || def.id === 'governance') {
      return { relevant: false, reason: 'Not part of viewer operational workflow.', visibility };
    }
    if (def.id === 'remediation_controls') {
      return { relevant: false, reason: 'Viewers do not run remediation workflows.', visibility };
    }
  }

  if (personaId === 'operator') {
    if (def.id === 'monitoring' && telemetry.agentsHealthy !== false && telemetry.pipelineDegraded !== true) {
      return {
        relevant: false,
        reason: 'Monitoring widgets surface when telemetry health needs attention.',
        visibility,
      };
    }
    if (def.id === 'investigation' && signals.hasAssignedCases === false) {
      return {
        relevant: false,
        reason: 'No investigation cases are assigned or shared with you.',
        visibility,
      };
    }
    if (def.id === 'attack_paths' && signals.hasAttackPaths === false && signals.hasOpenFindings === false) {
      return {
        relevant: false,
        reason: 'No attack paths or open findings in your scope to investigate.',
        visibility,
      };
    }
    if (def.id === 'remediation_controls' && !signals.hasActiveRemediation && !signals.hasAssignedCases) {
      return {
        relevant: false,
        reason: 'No active remediation or assigned cases in your workflow.',
        visibility,
      };
    }
  }

  if (def.ownershipModel === 'clusterScoped' && !ownership.hasOperationalScope) {
    return {
      relevant: false,
      reason: 'No operational cluster scope assigned.',
      visibility,
    };
  }

  if (def.telemetryRequired && telemetry.agentsHealthy === false) {
    return {
      relevant: false,
      reason: 'Telemetry is required but not healthy for this scope.',
      visibility,
    };
  }

  return {
    relevant: true,
    reason: 'Operationally relevant for your role and scope.',
    visibility,
  };
}

export function isOperationallyRelevant(input: OperationalRelevanceInput): boolean {
  return evaluateOperationalRelevance(input).relevant;
}
