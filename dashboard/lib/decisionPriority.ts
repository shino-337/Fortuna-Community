/**
 * Decision-centric operational prioritization with explainable WHY chains.
 */
import type { PersonaId } from './persona';
import type { IncidentModePhase } from './incidentMode';
import { isIncidentActive } from './incidentMode';
import type { DashboardWidgetId } from './dashboardComposition';
import type { ActionabilityAssessment } from './actionabilityModel';
import type { UncertaintyPropagationResult } from './uncertaintyPropagation';
import type { DecisionConsequence } from './decisionConsequences';
import { consequencesForActionability } from './decisionConsequences';

export type DecisionPriority = 'critical' | 'high' | 'medium' | 'low';

export interface ExplainableFactor {
  code: string;
  label: string;
  weight: number;
  direction: 'increases' | 'decreases';
}

export interface OperationalDecision {
  id: string;
  title: string;
  priority: DecisionPriority;
  category:
    | 'runtime_exploit'
    | 'blast_radius'
    | 'crown_jewel'
    | 'remediation'
    | 'telemetry'
    | 'incident'
    | 'sla'
    | 'privilege_chain';
  actionable: boolean;
  trustworthy: boolean;
  href?: string;
  reason: string;
  /** Structured explainability — WHY this matters now. */
  whyNow: string;
  factors: ExplainableFactor[];
  blockers: string[];
  confidence: number;
  /** Predicted impact if action proceeds despite blockers. */
  consequences?: DecisionConsequence[];
}

export interface DecisionPriorityInput {
  personaId: PersonaId;
  incidentPhase: IncidentModePhase;
  criticalCount: number;
  attackPathCount: number;
  hasOpenFindings: boolean;
  hasAssignedCases: boolean;
  telemetryDegraded: boolean;
  crownJewelExposed?: boolean;
  slaBreaches?: number;
  uncertainty?: UncertaintyPropagationResult;
  actionability?: ActionabilityAssessment;
}

function withExplainability(
  base: Omit<OperationalDecision, 'whyNow' | 'factors' | 'blockers' | 'confidence' | 'consequences'>,
  whyNow: string,
  factors: ExplainableFactor[],
  blockers: string[] = [],
  confidence = 1,
  consequences?: DecisionConsequence[],
): OperationalDecision {
  return { ...base, whyNow, factors, blockers, confidence, consequences };
}

export function prioritizeOperationalDecisions(input: DecisionPriorityInput): OperationalDecision[] {
  const decisions: OperationalDecision[] = [];
  const incident = isIncidentActive(input.incidentPhase);
  const conf = input.uncertainty?.aggregateConfidence ?? 1;
  const blockers = input.actionability?.blockerMessages ?? [];
  const globalConsequences =
    input.uncertainty && input.actionability
      ? consequencesForActionability(input.actionability, input.uncertainty)
      : [];

  if (incident && input.hasAssignedCases) {
    decisions.push(
      withExplainability(
        {
          id: 'active-incident',
          title: 'Continue active investigation',
          priority: 'critical',
          category: 'incident',
          actionable: input.actionability?.canAct ?? true,
          trustworthy: true,
          href: '/investigation',
          reason: 'An investigation case is in an active response phase.',
        },
        'Incident workflow is in an active phase — continuity prevents evidence loss and ownership drift.',
        [
          { code: 'incident_active', label: 'Active incident phase', weight: 0.9, direction: 'increases' },
          { code: 'workflow_continuity', label: 'Pinned evidence at risk if context switches', weight: 0.7, direction: 'increases' },
        ],
        blockers.filter((b) => b.includes('read-only')),
        conf,
        globalConsequences,
      ),
    );
  }

  if (input.criticalCount > 0) {
    decisions.push(
      withExplainability(
        {
          id: 'critical-findings',
          title: `${input.criticalCount} critical finding(s) in scope`,
          priority: incident ? 'critical' : 'high',
          category: 'runtime_exploit',
          actionable: (input.actionability?.canAct ?? true) && !input.telemetryDegraded,
          trustworthy: !input.telemetryDegraded && conf > 0.65,
          href: '/risks/findings?finalLevel=critical',
          reason: input.telemetryDegraded
            ? 'Counts may be incomplete due to telemetry degradation.'
            : 'Runtime-confirmed or high-confidence critical risks need triage.',
        },
        input.telemetryDegraded
          ? 'Critical count is elevated but telemetry degradation means zero may still hide active exploitation.'
          : 'Critical findings in assigned scope represent highest exploitability and business impact.',
        [
          { code: 'critical_count', label: `${input.criticalCount} critical in scope`, weight: 0.85, direction: 'increases' },
          { code: 'persona', label: `Persona: ${input.personaId}`, weight: 0.3, direction: 'increases' },
        ],
        input.telemetryDegraded ? blockers : [],
        input.telemetryDegraded ? conf * 0.6 : conf,
      ),
    );
  }

  if (input.attackPathCount > 0) {
    decisions.push(
      withExplainability(
        {
          id: 'attack-paths',
          title: 'Review exploit paths',
          priority: incident ? 'critical' : 'high',
          category: 'blast_radius',
          actionable: true,
          trustworthy: conf > 0.55,
          href: '/attack-paths',
          reason: 'Attack path graph shows plausible privilege or lateral movement chains.',
        },
        'Graph paths connect entry points to sensitive privileges — containment decisions depend on path order.',
        [
          { code: 'paths', label: `${input.attackPathCount} path(s) in scope`, weight: 0.8, direction: 'increases' },
          { code: 'blast_radius', label: 'Blast-radius semantics', weight: 0.75, direction: 'increases' },
        ],
        [],
        conf,
      ),
    );
  }

  if (input.crownJewelExposed) {
    decisions.push(
      withExplainability(
        {
          id: 'crown-jewel',
          title: 'Crown-jewel exposure detected',
          priority: 'critical',
          category: 'crown_jewel',
          actionable: input.actionability?.canAct ?? true,
          trustworthy: true,
          href: '/risks',
          reason: 'Business-critical assets appear in the current blast radius.',
        },
        'Regulated or crown-jewel services in scope raise impact above generic workload risk.',
        [{ code: 'crown_jewel', label: 'Crown-jewel in blast radius', weight: 0.95, direction: 'increases' }],
        [],
        conf,
      ),
    );
  }

  if (input.telemetryDegraded) {
    decisions.push(
      withExplainability(
        {
          id: 'telemetry',
          title: 'Telemetry integrity degraded',
          priority: input.personaId === 'admin' ? 'critical' : 'high',
          category: 'telemetry',
          actionable: input.personaId === 'admin',
          trustworthy: false,
          href: '/monitoring',
          reason: input.uncertainty?.summary ?? 'Ingestion or pipeline health may hide active threats.',
        },
        'Decision quality is capped until telemetry dependencies recover — graph and metrics may lie by omission.',
        [
          { code: 'telemetry', label: 'Pipeline / ingestion degraded', weight: 0.9, direction: 'increases' },
          { code: 'confidence', label: `Confidence ×${conf.toFixed(2)}`, weight: 0.85, direction: 'decreases' },
        ],
        blockers,
        conf,
      ),
    );
  }

  if ((input.slaBreaches ?? 0) > 0) {
    decisions.push(
      withExplainability(
        {
          id: 'sla',
          title: `${input.slaBreaches} SLA breach(es)`,
          priority: 'high',
          category: 'sla',
          actionable: true,
          trustworthy: true,
          href: '/investigation',
          reason: 'Investigation or remediation SLA targets are overdue.',
        },
        'SLA breach escalates operational and compliance risk independent of technical severity.',
        [{ code: 'sla', label: 'Overdue SLA', weight: 0.88, direction: 'increases' }],
        [],
        1,
      ),
    );
  }

  if (!input.hasOpenFindings && !incident && input.personaId === 'viewer') {
    decisions.push(
      withExplainability(
        {
          id: 'narrative',
          title: 'No active threats in assigned scope',
          priority: 'low',
          category: 'remediation',
          actionable: false,
          trustworthy: true,
          reason: 'Legitimate quiet state within your visibility scope.',
        },
        'Absence of findings in scope is a trustworthy quiet state, not a visibility error.',
        [{ code: 'empty_scope', label: 'Zero findings in scope', weight: 0.5, direction: 'decreases' }],
        [],
        1,
      ),
    );
  }

  const rank: Record<DecisionPriority, number> = { critical: 0, high: 1, medium: 2, low: 3 };
  return decisions.sort((a, b) => {
    const pr = rank[a.priority] - rank[b.priority];
    if (pr !== 0) return pr;
    return b.confidence - a.confidence;
  });
}

export function orderDashboardWidgets(
  widgets: DashboardWidgetId[],
  incidentPhase: IncidentModePhase,
  personaId: PersonaId,
): DashboardWidgetId[] {
  const priority = isIncidentActive(incidentPhase)
    ? ['persona_strip', 'hero_metrics', 'attack_analysis', 'entry_points', 'risk_stats', 'telemetry_health', 'exposure_trend', 'cluster_health', 'activity_feed']
    : personaId === 'admin'
      ? ['telemetry_health', 'hero_metrics', 'cluster_health', 'risk_stats', 'exposure_trend', 'activity_feed', 'entry_points', 'attack_analysis']
      : ['hero_metrics', 'risk_stats', 'entry_points', 'attack_analysis', 'exposure_trend', 'activity_feed', 'cluster_health'];

  return [...widgets].sort((a, b) => {
    const ia = priority.indexOf(a);
    const ib = priority.indexOf(b);
    return (ia === -1 ? 99 : ia) - (ib === -1 ? 99 : ib);
  });
}
