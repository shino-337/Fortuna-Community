import React from 'react';
import { UserCheck, ArrowRightLeft } from 'lucide-react';
import { useWorkflowState } from '../hooks/useWorkflowState';
import { ActiveRespondersPanel } from './ActiveRespondersPanel';

/** Strip showing the current investigation workflow and recommended actions. */
export const InvestigationPresenceStrip: React.FC = () => {
  const workflow = useWorkflowState();

  return (
    <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,14rem)] mb-4">
      <ActiveRespondersPanel />
      <div className="rounded-lg border border-border/80 bg-surface/40 px-3 py-2">
        <p className="text-caption font-semibold text-muted uppercase tracking-wide flex items-center gap-1.5">
          <UserCheck className="w-4 h-4" aria-hidden />
          Workflow: {workflow.label}
        </p>
        <ul className="mt-2 space-y-1 text-caption text-muted">
          {workflow.recommendedActions.slice(0, 3).map((a) => (
            <li key={a} className="flex items-start gap-1.5">
              <ArrowRightLeft className="w-3 h-3 shrink-0 mt-0.5 text-brand" aria-hidden />
              {a}
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
};
