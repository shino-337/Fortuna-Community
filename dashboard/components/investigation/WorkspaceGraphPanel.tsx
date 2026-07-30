import React from 'react';
import { Link } from 'react-router-dom';
import { Route, ExternalLink } from 'lucide-react';
import {
  EMPTY_WORKSPACE,
  useInvestigationWorkspaceStore,
} from '../../store/investigationWorkspaceStore';
import type { InvestigationCase } from '../../store/investigationStore';

export const WorkspaceGraphPanel: React.FC<{ activeCase: InvestigationCase }> = ({ activeCase }) => {
  const graphScope = useInvestigationWorkspaceStore((s) => (s.byCaseId[activeCase.id] ?? EMPTY_WORKSPACE).graphScope);
  const attackPathPins = activeCase.entities.filter((e) => e.type === 'attack_path');
  const podPins = activeCase.entities.filter((e) => e.type === 'pod');

  const graphQs = new URLSearchParams();
  if (activeCase.clusterId) graphQs.set('clusterId', activeCase.clusterId);
  if (graphScope.focusedEntityId) graphQs.set('podUid', graphScope.focusedEntityId);
  const attackPathsHref = `/attack-paths${graphQs.toString() ? `?${graphQs}` : ''}`;

  return (
    <div className="space-y-4">
      <p className="text-caption text-muted">
        Active graph scope follows this case. Open Attack Analysis for full topology with trust semantics.
      </p>
      <div className="rounded-lg border border-border bg-base/30 p-3 text-caption space-y-2">
        <p>
          <span className="text-muted">Cluster:</span>{' '}
          <span className="text-text font-medium">{graphScope.clusterId ?? activeCase.clusterId ?? 'All'}</span>
        </p>
        {graphScope.focusedEntityId ? (
          <p>
            <span className="text-muted">Focus:</span>{' '}
            <span className="font-mono text-text">{graphScope.focusedEntityId}</span>
          </p>
        ) : null}
        <p className="text-meta text-muted-2">
          {attackPathPins.length} attack path pin(s) · {podPins.length} pod pin(s)
        </p>
      </div>
      <Link
        to={attackPathsHref}
        className="inline-flex items-center gap-2 rounded-lg border border-brand/40 bg-brand/10 px-3 py-2 text-caption font-semibold text-brand hover:bg-brand/15"
      >
        <Route className="w-4 h-4" />
        Open scoped attack graph
        <ExternalLink className="w-3.5 h-3.5" />
      </Link>
      {attackPathPins.length > 0 ? (
        <ul className="space-y-1 text-caption">
          {attackPathPins.map((e) => (
            <li key={e.id}>
              <Link to={e.href?.replace(/^#/, '') ?? attackPathsHref} className="text-brand hover:underline">
                {e.label}
              </Link>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
};
