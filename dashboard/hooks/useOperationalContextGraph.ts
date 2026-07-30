import { useMemo } from 'react';
import { useOperationalEvents } from './useOperationalEvents';
import { usePermUser } from './usePermUser';
import { usePersona } from './usePersona';
import { useOperationalContext } from './useOperationalContext';
import { useInvestigationCases } from './useInvestigationCases';
import { useInvestigationStore } from '../store/investigationStore';
import {
  buildOperationalContextGraph,
  type OperationalContextGraph,
} from '../lib/operationalContextGraph';
import type { GraphTrustPosture } from '../lib/graphTrustSemantics';
import { prioritizeOperationalDecisions } from '../lib/decisionPriority';

export function useOperationalContextGraph(opts?: {
  graphPosture?: GraphTrustPosture;
  runtimeVerified?: boolean;
  inferredEdgeRatio?: number;
  identityLagMinutes?: number;
}): OperationalContextGraph & {
  decisions: ReturnType<typeof prioritizeOperationalDecisions>;
} {
  const user = usePermUser();
  const { id: personaId, profile } = usePersona();
  const { ownership, telemetry } = useOperationalContext();
  const { cases } = useInvestigationCases();
  const activeCaseId = useInvestigationStore((s) => s.activeCaseId);
  const { version: eventVersion } = useOperationalEvents();

  const graph = useMemo(
    () =>
      buildOperationalContextGraph({
        user,
        personaId,
        profile,
        ownership,
        telemetry,
        cases,
        activeCaseId,
        graphPosture: opts?.graphPosture,
        runtimeVerified: opts?.runtimeVerified,
        inferredEdgeRatio: opts?.inferredEdgeRatio,
        identityLagMinutes: opts?.identityLagMinutes,
      }),
    [
      user,
      personaId,
      profile,
      ownership,
      telemetry,
      cases,
      activeCaseId,
      opts?.graphPosture,
      opts?.runtimeVerified,
      opts?.inferredEdgeRatio,
      opts?.identityLagMinutes,
      eventVersion,
    ],
  );

  const decisions = useMemo(
    () =>
      prioritizeOperationalDecisions({
        personaId,
        incidentPhase: graph.incident.phase,
        criticalCount: 0,
        attackPathCount: 0,
        hasOpenFindings: false,
        hasAssignedCases: cases.length > 0,
        telemetryDegraded: graph.telemetryHealth.overallDegraded,
        uncertainty: graph.uncertainty,
        actionability: graph.actionability,
        slaBreaches: cases.filter((c) => c.slaDueAt && new Date(c.slaDueAt) < new Date()).length,
      }),
    [graph, personaId, cases],
  );

  return { ...graph, decisions };
}
