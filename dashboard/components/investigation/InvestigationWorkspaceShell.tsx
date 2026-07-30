import React, { useEffect } from 'react';
import clsx from 'clsx';
import type { InvestigationCase } from '../../store/investigationStore';
import {
  EMPTY_WORKSPACE,
  useInvestigationWorkspaceStore,
  type WorkspacePanel,
} from '../../store/investigationWorkspaceStore';
import { DecisionLogPanel } from './DecisionLogPanel';
import { WorkspaceGraphPanel } from './WorkspaceGraphPanel';
import { InvestigationTimeline } from '../InvestigationTimeline';
import { Card } from '../../design-system/components/Card';

const PANELS: { id: WorkspacePanel; label: string }[] = [
  { id: 'overview', label: 'Overview' },
  { id: 'evidence', label: 'Evidence' },
  { id: 'graph', label: 'Graph' },
  { id: 'decisions', label: 'Decision log' },
  { id: 'remediation', label: 'Remediation' },
  { id: 'timeline', label: 'Timeline' },
];

export const InvestigationWorkspaceShell: React.FC<{
  activeCase: InvestigationCase;
  author: string;
  canWrite: boolean;
  overview: React.ReactNode;
  evidence: React.ReactNode;
  remediation: React.ReactNode;
}> = ({ activeCase, author, canWrite, overview, evidence, remediation }) => {
  const touchCase = useInvestigationWorkspaceStore((s) => s.touchCase);
  const setActivePanel = useInvestigationWorkspaceStore((s) => s.setActivePanel);
  const activePanel = useInvestigationWorkspaceStore((s) => (s.byCaseId[activeCase.id] ?? EMPTY_WORKSPACE).activePanel);

  useEffect(() => {
    touchCase(activeCase.id);
  }, [activeCase.id, touchCase]);

  return (
    <div className="space-y-4 min-w-0">
      <nav
        className="flex gap-1 overflow-x-auto rounded-lg border border-border/80 bg-surface/40 p-1"
        aria-label="Investigations workspace"
      >
        {PANELS.map((p) => (
          <button
            key={p.id}
            type="button"
            onClick={() => setActivePanel(activeCase.id, p.id)}
            className={clsx(
              'shrink-0 rounded-md px-3 py-1.5 text-caption font-semibold transition-colors',
              activePanel === p.id
                ? 'bg-brand/15 text-brand border border-brand/30'
                : 'text-muted hover:text-text hover:bg-surface-2/60',
            )}
          >
            {p.label}
          </button>
        ))}
      </nav>

      {activePanel === 'overview' ? overview : null}
      {activePanel === 'evidence' ? evidence : null}
      {activePanel === 'graph' ? (
        <Card variant="secondary" className="p-4">
          <WorkspaceGraphPanel activeCase={activeCase} />
        </Card>
      ) : null}
      {activePanel === 'decisions' ? (
        <Card variant="secondary" className="p-4">
          <h3 className="text-caption font-semibold text-muted uppercase tracking-wider mb-3">Decision log</h3>
          <DecisionLogPanel caseId={activeCase.id} author={author} canWrite={canWrite} />
        </Card>
      ) : null}
      {activePanel === 'remediation' ? remediation : null}
      {activePanel === 'timeline' ? (
        <Card variant="secondary" className="p-4">
          <InvestigationTimeline caseId={activeCase.id} />
        </Card>
      ) : null}
    </div>
  );
};
