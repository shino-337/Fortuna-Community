/**
 * Decision consequence modeling — predicted impact of operational actions.
 */
import type { ActionabilityAssessment } from './actionabilityModel';
import type { UncertaintyPropagationResult } from './uncertaintyPropagation';
import type { OperationalContextGraph } from './operationalContextGraph';

export type ConsequenceSeverity = 'low' | 'medium' | 'high' | 'critical';

export type ConsequenceCategory =
  | 'production_disruption'
  | 'crown_jewel_isolation'
  | 'telemetry_blind_spot'
  | 'false_positive_containment'
  | 'ownership_gap'
  | 'escalation_delay'
  | 'shared_responder_conflict';

export interface DecisionConsequence {
  id: string;
  category: ConsequenceCategory;
  severity: ConsequenceSeverity;
  message: string;
  mitigation: string;
}

export interface ConsequenceAssessment {
  consequences: DecisionConsequence[];
  highestSeverity: ConsequenceSeverity;
  summary: string;
}

const SEVERITY_RANK: Record<ConsequenceSeverity, number> = {
  low: 0,
  medium: 1,
  high: 2,
  critical: 3,
};

export function predictDecisionConsequences(input: {
  graph?: Pick<OperationalContextGraph, 'business' | 'multiIncident' | 'uncertainty' | 'actionability' | 'telemetryHealth'>;
  proposedAction?: 'contain' | 'remediate' | 'isolate' | 'escalate';
  crownJewelInScope?: boolean;
  productionWorkload?: boolean;
}): ConsequenceAssessment {
  const consequences: DecisionConsequence[] = [];
  const unc = input.graph?.uncertainty;
  const act = input.graph?.actionability;
  const multi = input.graph?.multiIncident;

  if (input.proposedAction === 'remediate' || input.proposedAction === 'contain') {
    if (input.productionWorkload !== false) {
      consequences.push({
        id: 'prod-disrupt',
        category: 'production_disruption',
        severity: 'high',
        message: 'Remediation or containment may disrupt production traffic on affected workloads.',
        mitigation: 'Validate blast radius and schedule change window with service owner.',
      });
    }
    if (input.crownJewelInScope || input.graph?.business?.crownJewels?.length) {
      consequences.push({
        id: 'cj-isolate',
        category: 'crown_jewel_isolation',
        severity: 'critical',
        message: 'Action may isolate or impair crown-jewel dependencies in operational scope.',
        mitigation: 'Require incident commander sign-off and communications owner notification.',
      });
    }
    if (unc && unc.aggregateConfidence < 0.65) {
      consequences.push({
        id: 'telemetry-blind',
        category: 'telemetry_blind_spot',
        severity: 'high',
        message: 'Telemetry degradation — post-action validation may be incomplete.',
        mitigation: 'Confirm runtime evidence before irreversible containment.',
      });
    }
    if (act?.blockers.includes('runtime_unverified')) {
      consequences.push({
        id: 'fp-contain',
        category: 'false_positive_containment',
        severity: 'medium',
        message: 'Runtime unverified — containment risk includes false-positive service impact.',
        mitigation: 'Correlate with live process/network signals before isolation.',
      });
    }
  }

  if (act?.blockers.includes('owner_unavailable')) {
    consequences.push({
      id: 'owner-gap',
      category: 'ownership_gap',
      severity: 'medium',
      message: 'No remediation owner — actions may not be tracked or reversed properly.',
      mitigation: 'Assign remediation lead in incident command structure.',
    });
  }

  if (multi?.escalationOverload) {
    consequences.push({
      id: 'escalation-delay',
      category: 'escalation_delay',
      severity: 'high',
      message: 'Escalation overload — competing SLA incidents may delay approval for this action.',
      mitigation: 'Commander prioritization required across active cases.',
    });
  }

  if ((multi?.sharedResponders.length ?? 0) > 0) {
    consequences.push({
      id: 'responder-conflict',
      category: 'shared_responder_conflict',
      severity: 'medium',
      message: 'Shared responders across incidents — parallel actions may conflict.',
      mitigation: 'Coordinate via live annotations and delegate non-critical tasks.',
    });
  }

  let highestSeverity: ConsequenceSeverity = 'low';
  for (const c of consequences) {
    if (SEVERITY_RANK[c.severity] > SEVERITY_RANK[highestSeverity]) highestSeverity = c.severity;
  }

  const summary =
    consequences.length === 0
      ? 'No significant predicted consequences for the proposed action profile.'
      : `${consequences.length} predicted consequence(s); highest severity: ${highestSeverity}.`;

  return { consequences, highestSeverity, summary };
}

export function consequencesForActionability(
  actionability: ActionabilityAssessment,
  uncertainty: UncertaintyPropagationResult,
): DecisionConsequence[] {
  const out: DecisionConsequence[] = [];
  if (actionability.blockers.includes('remediation_blocked')) {
    out.push({
      id: 'blocked-remediation',
      category: 'escalation_delay',
      severity: 'medium',
      message: 'Remediation blocked — proceeding without ticket resolution may violate change policy.',
      mitigation: 'Resolve external ticket or escalate to commander.',
    });
  }
  if (uncertainty.aggregateConfidence < 0.5) {
    out.push({
      id: 'low-confidence',
      category: 'telemetry_blind_spot',
      severity: 'high',
      message: 'Aggregate confidence below 50% — decisions may amplify uncertainty.',
      mitigation: 'Restore telemetry pipelines before irreversible actions.',
    });
  }
  return out;
}
