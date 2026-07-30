import { useMemo } from 'react';
import { useInvestigationStore } from '../store/investigationStore';
import { useInvestigationCases } from './useInvestigationCases';
import { buildIncidentContext } from '../lib/incidentContext';
import {
  incidentDashboardWidgetPriority,
  incidentGraphSemanticMode,
  incidentModeLabel,
  incidentNavCondensed,
  isIncidentActive,
  type IncidentModeContext,
} from '../lib/incidentMode';

/**
 * Hook that determines the current incident mode and related UI state.
 * @param opts.runtimeConfirmed - Whether runtime confirmation is enabled (affects context building)
 */
export function useIncidentMode(opts?: { runtimeConfirmed?: boolean }): {
  context: IncidentModeContext;
  active: boolean;
  label: string;
  navCondensed: boolean;
  graphSemanticMode: ReturnType<typeof incidentGraphSemanticMode>;
  widgetPriority: string[];
} {
  const { cases } = useInvestigationCases();
  const activeCaseId = useInvestigationStore((s) => s.activeCaseId);

  const context = useMemo(
    () => buildIncidentContext(cases, activeCaseId, { runtimeConfirmed: opts?.runtimeConfirmed }),
    [cases, activeCaseId, opts?.runtimeConfirmed],
  );

  return useMemo(
    () => ({
      context,
      active: isIncidentActive(context.phase),
      label: incidentModeLabel(context.phase),
      navCondensed: incidentNavCondensed(context.phase),
      graphSemanticMode: incidentGraphSemanticMode(context.phase),
      widgetPriority: incidentDashboardWidgetPriority(context.phase),
    }),
    [context],
  );
}
