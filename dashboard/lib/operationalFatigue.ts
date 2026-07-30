/**
 * Operational fatigue modeling — overload, queue pressure, burnout heuristics.
 */
import type { IncidentCompetition } from './multiIncidentCognition';
import type { InvestigationCase, InvestigationStatus } from '../store/investigationStore';

export type FatigueLevel = 'low' | 'moderate' | 'high' | 'critical';

export interface OperationalFatigue {
  level: FatigueLevel;
  overloadRisk: number;
  queuePressure: number;
  burnoutHeuristic: string;
  responderFatigue: Record<string, number>;
  recommendations: string[];
}

const ACTIVE: InvestigationStatus[] = ['OPEN', 'TRIAGED', 'ACTIVE', 'CONTAINED', 'REMEDIATING'];

export function analyzeOperationalFatigue(
  multiIncident: IncidentCompetition,
  cases: InvestigationCase[],
  currentUsername?: string,
): OperationalFatigue {
  const active = cases.filter((c) => ACTIVE.includes(c.status));
  const slaBreaches = active.filter((c) => c.slaDueAt && new Date(c.slaDueAt) < new Date()).length;
  const openRemediation = active.reduce(
    (n, c) => n + (c.remediationActions?.filter((a) => a.status === 'pending' || a.status === 'in_progress').length ?? 0),
    0,
  );

  const responderFatigue: Record<string, number> = {};
  for (const [name, load] of Object.entries(multiIncident.responderLoad)) {
    let score = load * 22;
    if (multiIncident.sharedResponders.includes(name)) score += 18;
    const owned = active.filter((c) => c.owner === name).length;
    score += owned * 12;
    responderFatigue[name] = Math.min(100, score);
  }

  const userFatigue = currentUsername ? responderFatigue[currentUsername] ?? 0 : 0;
  const queuePressure = Math.min(
    1,
    active.length * 0.12 + slaBreaches * 0.15 + openRemediation * 0.04 + multiIncident.sharedResponders.length * 0.08,
  );
  const overloadRisk = Math.min(
    1,
    (multiIncident.level === 'critical' ? 0.45 : multiIncident.level === 'high' ? 0.3 : 0.1) +
      queuePressure * 0.55 +
      (userFatigue > 60 ? 0.2 : 0),
  );

  let level: FatigueLevel = 'low';
  if (overloadRisk >= 0.75 || userFatigue >= 80) level = 'critical';
  else if (overloadRisk >= 0.55 || userFatigue >= 60) level = 'high';
  else if (overloadRisk >= 0.35 || userFatigue >= 40) level = 'moderate';

  const recommendations: string[] = [];
  if (level !== 'low') {
    recommendations.push('Delegate containment vs remediation leads to reduce single-responder load.');
  }
  if (multiIncident.escalationOverload) {
    recommendations.push('Escalation overload — defer non-SLA work and assign communications owner.');
  }
  if (userFatigue >= 60 && currentUsername) {
    recommendations.push('Your assignment load is elevated — confirm incident commander for prioritization.');
  }
  if (queuePressure > 0.5) {
    recommendations.push('Queue pressure high — batch triage before deep graph analysis.');
  }

  let burnoutHeuristic = 'Responder load within sustainable bounds for current incident count.';
  if (level === 'critical') {
    burnoutHeuristic = 'Critical fatigue risk — shared responders across multiple active incidents with SLA pressure.';
  } else if (level === 'high') {
    burnoutHeuristic = 'High fatigue risk — consider rotating ownership and pausing non-critical remediation.';
  } else if (level === 'moderate') {
    burnoutHeuristic = 'Moderate fatigue — monitor queue depth and avoid parallel deep investigations without commander.';
  }

  return {
    level,
    overloadRisk,
    queuePressure,
    burnoutHeuristic,
    responderFatigue,
    recommendations,
  };
}
