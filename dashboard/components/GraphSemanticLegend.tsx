import React from 'react';
import { ProvenanceBadge, type ProvenanceKind } from '../design-system/components/ProvenanceBadge';
import type { GraphSemanticMode } from '../lib/persona';
import { GRAPH_EDGE_COLORS } from '../lib/graphTheme';

const LEGEND: Record<GraphSemanticMode, { label: string; kinds: ProvenanceKind[] }> = {
  blast_radius: {
    label: 'Blast-radius edges',
    kinds: ['observed', 'inferred', 'theoretical'],
  },
  exploitability: {
    label: 'Exploitability edges',
    kinds: ['observed', 'inferred', 'theoretical'],
  },
  integrity: {
    label: 'Telemetry integrity',
    kinds: ['observed', 'inferred', 'stale', 'partial'],
  },
};

const EDGE_LEGEND: ReadonlyArray<{ type: string; label: string; dashed?: boolean }> = [
  { type: 'PATH_ENTRY', label: 'Path start' },
  { type: 'SERVICE_ACCOUNT_ACCESS', label: 'SA token' },
  { type: 'RBAC_BINDING', label: 'Binding' },
  { type: 'GRANTS_ROLE', label: 'Grants role' },
  { type: 'NETWORK_REACH', label: 'Network' },
  { type: 'NETWORK_REACH_SOFT', label: 'Soft reach', dashed: true },
  { type: 'CONTAINER_ESCAPE', label: 'Escape' },
  { type: 'HOST_ACCESS', label: 'Host/node' },
  { type: 'LATERAL_MOVE', label: 'Lateral' },
];

export const GraphSemanticLegend: React.FC<{ mode: GraphSemanticMode; className?: string }> = ({
  mode,
  className = '',
}) => {
  const entry = LEGEND[mode];
  return (
    <div
      className={`flex flex-col gap-2 text-meta text-muted ${className}`}
      role="note"
      aria-label="Graph edge legend"
    >
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-semibold text-text">Link type:</span>
        {EDGE_LEGEND.map((edge) => (
          <span key={edge.type} className="inline-flex items-center gap-1.5">
            <span
              className="inline-block w-6 border-t-2"
              style={{
                borderColor: GRAPH_EDGE_COLORS[edge.type],
                borderStyle: edge.dashed ? 'dashed' : 'solid',
              }}
            />
            <span>{edge.label}</span>
          </span>
        ))}
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-semibold text-text">{entry.label}:</span>
        {entry.kinds.map((k) => (
          <ProvenanceBadge key={k} kind={k} />
        ))}
        <span className="text-muted-2">Dash/opacity = lower confidence or non-observed</span>
      </div>
    </div>
  );
};
