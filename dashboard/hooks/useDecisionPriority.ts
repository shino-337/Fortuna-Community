import { useMemo } from 'react';
import { usePersona } from './usePersona';
import { useOperationalContextGraph } from './useOperationalContextGraph';
import {
  prioritizeOperationalDecisions,
  orderDashboardWidgets,
  type OperationalDecision,
} from '../lib/decisionPriority';
import type { DashboardWidgetId } from '../lib/dashboardComposition';

/**
 * Hook that computes persona-specific decision priorities and widget ordering.
 * @param signals.criticalCount - Number of critical alerts in current context
 * @param signals.attackPathCount - Number of attack path nodes detected
 * @param signals.hasOpenFindings - Whether any investigation findings remain open
 * @param signals.telemetryDegraded - Optional flag if telemetry is degraded (defaults to graph state)
 */
export function useDecisionPriority(signals: {
  criticalCount: number;
  attackPathCount: number;
  hasOpenFindings: boolean;
  telemetryDegraded?: boolean;
}): {
  decisions: OperationalDecision[];
  orderWidgets: (w: DashboardWidgetId[]) => DashboardWidgetId[];
} {
  const { id: personaId } = usePersona();
  const ctx = useOperationalContextGraph({
    runtimeVerified: signals.criticalCount > 0,
  });

  const decisions = useMemo(
    () =>
      prioritizeOperationalDecisions({
        personaId,
        incidentPhase: ctx.incident.phase,
        criticalCount: signals.criticalCount,
        attackPathCount: signals.attackPathCount,
        hasOpenFindings: signals.hasOpenFindings,
        hasAssignedCases: ctx.multiIncident.activeIncidents > 0,
        telemetryDegraded: signals.telemetryDegraded ?? ctx.telemetryHealth.overallDegraded,
        crownJewelExposed: ctx.business.crownJewels.length > 0 && signals.hasOpenFindings,
        slaBreaches: ctx.multiIncident.competingCases.filter((c) => c.priorityScore >= 50).length,
        uncertainty: ctx.uncertainty,
        actionability: ctx.actionability,
      }),
    [ctx, personaId, signals],
  );

  const orderWidgets = useMemo(
    () => (widgets: DashboardWidgetId[]) => orderDashboardWidgets(widgets, ctx.incident.phase, personaId),
    [ctx.incident.phase, personaId],
  );

  return { decisions, orderWidgets };
}
