/**
 * Unified operational context graph — single source for incident, workflow, persona, visibility, uncertainty, actionability.
 */
import type { PersonaId, PermUser, PersonaProfile } from './persona';
import type { OwnershipContext } from './ownershipContext';
import type { TelemetryContext } from './telemetryContext';
import type { IncidentModeContext } from './incidentMode';
import type { WorkflowStateProfile } from './workflowStateMachine';
import type { VisibilityResult } from './visibilityEngine';
import type { FeatureId } from './featureRegistry';
import type { GraphTrustPosture } from './graphTrustSemantics';
import { buildIncidentContext } from './incidentContext';
import { getWorkflowProfile } from './workflowStateMachine';
import { evaluateVisibility } from './visibilityEngine';
import { propagateUncertainty, type UncertaintyPropagationResult } from './uncertaintyPropagation';
import { assessTelemetryHealth, type TelemetryHealthCognition } from './telemetryHealthCognition';
import { assessActionability, type ActionabilityAssessment } from './actionabilityModel';
import { analyzeMultiIncident, type IncidentCompetition } from './multiIncidentCognition';
import { analyzeOperationalFatigue, type OperationalFatigue } from './operationalFatigue';
import {
  buildIncidentCommandStructure,
  deriveCommandAssignment,
  type IncidentCommandStructure,
} from './incidentCommand';
import { predictDecisionConsequences, type ConsequenceAssessment } from './decisionConsequences';
import { loadOperationalMemory, type OperationalMemorySnapshot } from './operationalMemory';
import { getOperationalEventVersion } from './operationalEvents';
import { businessContextFromScope } from './businessContext';
import type { InvestigationCase } from '../store/investigationStore';
import { can, P } from './permissions';

export interface OperationalContextGraph {
  user: PermUser;
  personaId: PersonaId;
  profile: PersonaProfile;
  ownership: OwnershipContext;
  telemetry: TelemetryContext;
  incident: IncidentModeContext;
  workflow: WorkflowStateProfile;
  business: ReturnType<typeof businessContextFromScope>;
  telemetryHealth: TelemetryHealthCognition;
  uncertainty: UncertaintyPropagationResult;
  multiIncident: IncidentCompetition;
  fatigue: OperationalFatigue;
  incidentCommand: IncidentCommandStructure;
  consequences: ConsequenceAssessment;
  eventVersion: number;
  memory: OperationalMemorySnapshot;
  visibility: Partial<Record<FeatureId, VisibilityResult>>;
  graphPosture?: GraphTrustPosture;
  actionability: ActionabilityAssessment;
  /** Derived UX mode for shell adaptation. */
  shellMode: 'normal' | 'incident' | 'multi_incident' | 'degraded_telemetry';
}

export interface BuildOperationalContextGraphInput {
  user: PermUser;
  personaId: PersonaId;
  profile: PersonaProfile;
  ownership: OwnershipContext;
  telemetry: TelemetryContext;
  cases: InvestigationCase[];
  activeCaseId: string | null;
  features?: FeatureId[];
  graphPosture?: GraphTrustPosture;
  runtimeVerified?: boolean;
  inferredEdgeRatio?: number;
  identityLagMinutes?: number;
  remediationBlocked?: boolean;
}

export function buildOperationalContextGraph(input: BuildOperationalContextGraphInput): OperationalContextGraph {
  const incident = buildIncidentContext(input.cases, input.activeCaseId, {
    runtimeConfirmed: input.runtimeVerified,
  });
  const activeCase =
    input.cases.find((c) => c.id === input.activeCaseId) ??
    input.cases.find((c) => c.id === incident.activeCaseId) ??
    null;
  const workflow = getWorkflowProfile(activeCase?.status);
  const business = businessContextFromScope(input.ownership.operationalScope);
  const telemetryHealth = assessTelemetryHealth(input.telemetry, {
    identityLagMinutes: input.identityLagMinutes,
  });
  const uncertainty = propagateUncertainty({
    telemetry: input.telemetry,
    graphPosture: input.graphPosture,
    inferredEdgeRatio: input.inferredEdgeRatio,
    runtimeVerified: input.runtimeVerified,
    identityLagMinutes: input.identityLagMinutes,
  });
  const multiIncident = analyzeMultiIncident(input.cases, input.activeCaseId, input.user?.username);
  const fatigue = analyzeOperationalFatigue(multiIncident, input.cases, input.user?.username);
  const memory = loadOperationalMemory();
  const commandAssignment = deriveCommandAssignment(activeCase);
  const incidentCommand = buildIncidentCommandStructure(commandAssignment);

  const visibility: Partial<Record<FeatureId, VisibilityResult>> = {};
  const features = input.features ?? ['dashboard', 'investigation', 'risk_operations', 'attack_paths'];
  for (const f of features) {
    visibility[f] = evaluateVisibility({
      feature: f,
      user: input.user,
      personaId: input.personaId,
      ownership: input.ownership,
      telemetry: input.telemetry,
      ownershipEmpty: f === 'investigation' && input.cases.length === 0,
    });
  }

  const actionability = assessActionability({
    user: input.user,
    personaId: input.personaId,
    telemetry: input.telemetry,
    uncertainty,
    hasWriteOnCase: can(input.user, P.investigationsWrite),
    caseOwner: activeCase?.owner,
    remediationBlocked:
      input.remediationBlocked ??
      Boolean(activeCase?.remediationActions?.some((a) => a.status === 'blocked')),
    runtimeVerified: input.runtimeVerified,
    hasOperationalScope: input.ownership.hasOperationalScope,
    requiresEscalation: multiIncident.escalationOverload,
  });

  let shellMode: OperationalContextGraph['shellMode'] = 'normal';
  if (multiIncident.level === 'high' || multiIncident.level === 'critical') shellMode = 'multi_incident';
  else if (fatigue.level === 'critical' || fatigue.level === 'high') shellMode = 'multi_incident';
  else if (incident.phase !== 'normal' && incident.phase !== 'postmortem') shellMode = 'incident';
  else if (telemetryHealth.overallDegraded) shellMode = 'degraded_telemetry';

  const consequences = predictDecisionConsequences({
    graph: {
      business,
      multiIncident,
      uncertainty,
      actionability,
      telemetryHealth,
    },
    crownJewelInScope: business.crownJewels.length > 0,
    productionWorkload: true,
    proposedAction: incident.phase === 'active_incident' ? 'contain' : undefined,
  });

  return {
    user: input.user,
    personaId: input.personaId,
    profile: input.profile,
    ownership: input.ownership,
    telemetry: input.telemetry,
    incident,
    workflow,
    business,
    telemetryHealth,
    uncertainty,
    multiIncident,
    fatigue,
    incidentCommand,
    consequences,
    eventVersion: getOperationalEventVersion(),
    memory,
    visibility,
    graphPosture: input.graphPosture,
    actionability,
    shellMode,
  };
}
