/**
 * Centralized feature visibility evaluation (Layers 2–4).
 */
import { canAny } from './permissions';
import type { FeatureId } from './featureRegistry';
import { getFeature } from './featureRegistry';
import type { PersonaId, PermUser } from './persona';
import type { OwnershipContext } from './ownershipContext';
import { DEFAULT_TELEMETRY_CONTEXT, type TelemetryContext } from './telemetryContext';

export type SemanticVisibilityState =
  | 'visible'
  | 'no_permission'
  | 'no_scope'
  | 'no_telemetry'
  | 'no_data'
  | 'sampled'
  | 'policy_hidden'
  | 'stale';

export interface VisibilityResult {
  visible: boolean;
  semanticState: SemanticVisibilityState;
  reason: string;
  feature: FeatureId;
}

export interface VisibilityInput {
  feature: FeatureId;
  user: PermUser;
  personaId: PersonaId;
  ownership: OwnershipContext;
  telemetry?: TelemetryContext;
  /** Widget-level: data layer confirmed empty in scope. */
  dataEmpty?: boolean;
  /** Widget-level: user lacks ownership of cases/resources but feature is otherwise OK. */
  ownershipEmpty?: boolean;
}

export function evaluateVisibility(input: VisibilityInput): VisibilityResult {
  const def = getFeature(input.feature);
  const { user, personaId, ownership } = input;
  const telemetry = input.telemetry ?? DEFAULT_TELEMETRY_CONTEXT;

  if (!canAny(user, def.permissionsAny)) {
    return {
      visible: false,
      semanticState: 'no_permission',
      reason: `Requires one of: ${def.permissionsAny.join(', ')}`,
      feature: input.feature,
    };
  }

  if (!def.personas.includes(personaId)) {
    return {
      visible: false,
      semanticState: 'policy_hidden',
      reason: `Not available for ${personaId} experience profile`,
      feature: input.feature,
    };
  }

  if (def.ownershipModel === 'clusterScoped') {
    if (!ownership.platformHasClusters) {
      return {
        visible: false,
        semanticState: 'no_scope',
        reason: 'No Kubernetes clusters are registered in this environment yet.',
        feature: input.feature,
      };
    }
    if (!ownership.hasOperationalScope) {
      return {
        visible: false,
        semanticState: 'no_scope',
        reason: ownership.restrictsClusters
          ? 'You are not assigned to any monitored clusters. Ask an administrator to update your scope.'
          : 'No operational scope is available for your account.',
        feature: input.feature,
      };
    }
    if (ownership.selectedClusterOutOfScope) {
      return {
        visible: false,
        semanticState: 'no_scope',
        reason: 'The selected cluster is outside your assigned scope.',
        feature: input.feature,
      };
    }
  }

  if (def.telemetryRequired) {
    if (telemetry.ingestionStale === true) {
      return {
        visible: false,
        semanticState: 'stale',
        reason: 'Telemetry ingestion is stale for this scope; runtime views may be incomplete.',
        feature: input.feature,
      };
    }
    if (telemetry.agentsHealthy === false) {
      return {
        visible: false,
        semanticState: 'no_telemetry',
        reason: 'No healthy agents are reporting for the selected scope.',
        feature: input.feature,
      };
    }
  }

  if (telemetry.graphPolicyLimited && def.graphRelevant) {
    return {
      visible: true,
      semanticState: 'policy_hidden',
      reason: 'Some graph relationships may be hidden by policy or query permissions.',
      feature: input.feature,
    };
  }

  if (telemetry.graphSampled && def.graphRelevant) {
    return {
      visible: true,
      semanticState: 'sampled',
      reason: 'Large graphs are sampled or clustered; not all nodes and edges are shown.',
      feature: input.feature,
    };
  }

  if (telemetry.pipelineDegraded === true && def.telemetryRequired) {
    return {
      visible: true,
      semanticState: 'stale',
      reason: 'Pipeline degradation may affect completeness of displayed telemetry.',
      feature: input.feature,
    };
  }

  if (input.ownershipEmpty) {
    return {
      visible: false,
      semanticState: 'no_scope',
      reason:
        def.ownershipModel === 'caseScoped'
          ? 'No investigation cases are shared with you or assigned to you in this scope.'
          : 'Nothing in this view is assigned to your operational scope.',
      feature: input.feature,
    };
  }

  if (input.dataEmpty) {
    return {
      visible: true,
      semanticState: 'no_data',
      reason: 'No records exist in your current scope and filters — this is a legitimate empty state.',
      feature: input.feature,
    };
  }

  return {
    visible: true,
    semanticState: 'visible',
    reason: 'Feature is available for your role, scope, and telemetry posture.',
    feature: input.feature,
  };
}

/** Nav/route: hide entirely unless visible (not sampled/stale banners on nav). */
export function isNavVisible(input: VisibilityInput): boolean {
  const r = evaluateVisibility(input);
  return r.visible && r.semanticState !== 'policy_hidden';
}
