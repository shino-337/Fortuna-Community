import React from 'react';
import { Users } from 'lucide-react';
import { useInvestigationCases } from '../hooks/useInvestigationCases';
import { useInvestigationStore } from '../store/investigationStore';

/**
 * Displays the active responders (owner and assignees) for the currently open investigation case.
 */
export const ActiveRespondersPanel: React.FC<{ className?: string; /** Optional CSS class applied to the panel container */ }> = ({ className = '' }) => {
  const { cases } = useInvestigationCases();
  const activeCaseId = useInvestigationStore((s) => s.activeCaseId);
  const active = cases.find((c) => c.id === activeCaseId) ?? cases[0];

  if (!active) return null;

  const assignees = active.collaboration?.assignees ?? [];
  const watchers = active.collaboration?.watchers ?? [];
  const responders = [...new Set([active.owner, ...assignees].filter(Boolean))] as string[];

  return (
    <div className={`rounded-lg border border-border/80 bg-surface/40 px-3 py-2 ${className}`}>
      <h3 className="text-caption font-semibold text-muted uppercase tracking-wide flex items-center gap-1.5 mb-2">
        <Users className="w-4 h-4" aria-hidden />
        Active responders
      </h3>
      {responders.length === 0 ? (
        <p className="text-caption text-muted">No owner assigned — claim this case in Investigation.</p>
      ) : (
        <ul className="flex flex-wrap gap-1.5">
          {responders.map((r) => (
            <li
              key={r}
              className="rounded-full border border-brand/30 bg-brand/10 px-2 py-0.5 text-meta font-medium text-brand"
            >
              {r}
            </li>
          ))}
        </ul>
      )}
      {watchers.length > 0 ? (
        <p className="text-meta text-muted-2 mt-2">Watching: {watchers.join(', ')}</p>
      ) : null}
    </div>
  );
};
