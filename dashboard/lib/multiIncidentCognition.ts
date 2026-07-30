/**
 * Multi-incident cognition — competition, shared responders, escalation overload.
 */
import type { InvestigationCase, InvestigationStatus } from '../store/investigationStore';

export type IncidentCompetitionLevel = 'none' | 'low' | 'moderate' | 'high' | 'critical';

export interface IncidentCompetition {
  level: IncidentCompetitionLevel;
  activeIncidents: number;
  competingCases: { id: string; title: string; status: InvestigationStatus; priorityScore: number }[];
  sharedResponders: string[];
  responderLoad: Record<string, number>;
  escalationOverload: boolean;
  resourceContentionMessage: string;
}

const ACTIVE_STATUSES: InvestigationStatus[] = ['OPEN', 'TRIAGED', 'ACTIVE', 'CONTAINED', 'REMEDIATING'];

function priorityScore(c: InvestigationCase): number {
  let s = 0;
  if (c.status === 'ACTIVE') s += 40;
  if (c.status === 'REMEDIATING') s += 35;
  if (c.status === 'CONTAINED') s += 25;
  if (c.status === 'TRIAGED') s += 15;
  if (c.slaDueAt && new Date(c.slaDueAt) < new Date()) s += 50;
  s += Math.min(20, c.entities.length * 2);
  return s;
}

export function analyzeMultiIncident(
  cases: InvestigationCase[],
  activeCaseId: string | null,
  currentUsername?: string,
): IncidentCompetition {
  const active = cases.filter((c) => ACTIVE_STATUSES.includes(c.status));
  const competing = active
    .map((c) => ({
      id: c.id,
      title: c.title,
      status: c.status,
      priorityScore: priorityScore(c),
    }))
    .sort((a, b) => b.priorityScore - a.priorityScore);

  const responderLoad: Record<string, number> = {};
  for (const c of active) {
    const people = [c.owner, ...(c.collaboration?.assignees ?? [])].filter(Boolean) as string[];
    for (const p of people) {
      responderLoad[p] = (responderLoad[p] ?? 0) + 1;
    }
  }

  const sharedResponders = Object.entries(responderLoad)
    .filter(([, n]) => n > 1)
    .map(([name]) => name);

  const escalationOverload = active.filter((c) => c.slaDueAt && new Date(c.slaDueAt) < new Date()).length >= 2;

  let level: IncidentCompetitionLevel = 'none';
  if (active.length >= 4 || escalationOverload) level = 'critical';
  else if (active.length >= 3 || sharedResponders.length >= 2) level = 'high';
  else if (active.length >= 2) level = 'moderate';
  else if (active.length === 1) level = 'low';

  const userLoad = currentUsername ? responderLoad[currentUsername] ?? 0 : 0;
  let resourceContentionMessage = 'No competing active incidents in your workspace.';
  if (level !== 'none' && level !== 'low') {
    resourceContentionMessage = `${active.length} active incident(s); ${sharedResponders.length} responder(s) span multiple cases.`;
    if (userLoad > 1) {
      resourceContentionMessage += ` You are assigned to ${userLoad} active cases — prioritize by SLA and runtime confirmation.`;
    }
    if (activeCaseId && competing[0]?.id !== activeCaseId && competing[0]) {
      resourceContentionMessage += ` Higher-priority case: "${competing[0].title}".`;
    }
  }

  return {
    level,
    activeIncidents: active.length,
    competingCases: competing,
    sharedResponders,
    responderLoad,
    escalationOverload,
    resourceContentionMessage,
  };
}
