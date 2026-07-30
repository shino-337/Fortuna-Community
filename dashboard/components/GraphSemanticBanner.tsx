import React from 'react';
import { Info } from 'lucide-react';
import { graphModeDescription, graphModeLabel, type GraphSemanticMode } from '../lib/persona';

/** Banner explaining the current graph semantic mode — observed vs inferred edges. */
export const GraphSemanticBanner: React.FC<{ mode: GraphSemanticMode; className?: string }> = ({
  mode,
  className = '',
}) => (
  <div
    className={`mb-3 flex gap-2 rounded-lg border border-border/80 bg-surface/40 px-3 py-2 text-caption text-muted ${className}`}
    role="note"
  >
    <Info className="w-4 h-4 shrink-0 text-brand mt-0.5" aria-hidden />
    <div>
      <span className="font-semibold text-text">{graphModeLabel(mode)}</span>
      <span className="text-muted-2"> — </span>
      {graphModeDescription(mode)}
      <span className="block text-meta text-muted-2 mt-0.5">
        Distinguish observed vs inferred edges using runtime confirmation and confidence lanes.
      </span>
    </div>
  </div>
);
