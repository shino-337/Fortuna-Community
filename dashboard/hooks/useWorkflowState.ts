import { useMemo } from 'react';
import { useInvestigationStore } from '../store/investigationStore';
import { useInvestigationCases } from './useInvestigationCases';
import { getWorkflowProfile, type WorkflowStateProfile } from '../lib/workflowStateMachine';

export function useWorkflowState(): WorkflowStateProfile & { activeCaseId: string | null } {
  const { cases } = useInvestigationCases();
  const activeCaseId = useInvestigationStore((s) => s.activeCaseId);
  const activeCase = useMemo(
    () => cases.find((c) => c.id === activeCaseId) ?? cases[0] ?? null,
    [cases, activeCaseId],
  );

  return useMemo(
    () => ({
      ...getWorkflowProfile(activeCase?.status),
      activeCaseId: activeCase?.id ?? null,
    }),
    [activeCase?.status, activeCase?.id],
  );
}
