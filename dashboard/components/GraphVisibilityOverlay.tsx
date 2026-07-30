import React from 'react';
import { AlertTriangle, EyeOff, Layers } from 'lucide-react';
import type { SemanticVisibilityState } from '../lib/visibilityEngine';
import { ProvenanceBadge } from '../design-system/components/ProvenanceBadge';

/**
 * Graph integrity banner — preserves semantic truth when edges/nodes are hidden.
 */
export const GraphVisibilityOverlay: React.FC<{
  semanticState?: SemanticVisibilityState;
  reason?: string;
  clustered?: boolean;
  className?: string;
}> = ({ semanticState, reason, clustered, className = '' }) => {
  if (semanticState === 'visible' && !clustered) return null;

  const showPolicy = semanticState === 'policy_hidden' || semanticState === 'sampled';
  const showStale = semanticState === 'stale';
  const showCluster = clustered;

  if (!showPolicy && !showStale && !showCluster) return null;

  return (
    <div
      className={`mb-2 flex flex-wrap items-start gap-2 rounded-lg border border-amber-500/25 bg-amber-500/10 px-3 py-2 text-caption text-muted ${className}`}
      role="note"
    >
      <AlertTriangle className="w-4 h-4 shrink-0 text-amber-300 mt-0.5" aria-hidden />
      <div className="min-w-0 flex-1 space-y-1">
        {showCluster ? (
          <p className="text-text font-medium flex items-center gap-1.5">
            <Layers className="w-3.5 h-3.5" />
            Graph simplified for scale — individual nodes are clustered.
          </p>
        ) : null}
        {showPolicy ? (
          <p className="text-text font-medium flex items-center gap-1.5">
            <EyeOff className="w-3.5 h-3.5" />
            {reason ?? 'Some relationships may be hidden by policy, sampling, or permissions.'}
          </p>
        ) : null}
        {showStale ? (
          <p className="text-meta">Telemetry freshness is degraded; exploitability paths may be incomplete.</p>
        ) : null}
        <div className="flex flex-wrap gap-1.5 pt-0.5">
          <ProvenanceBadge kind="observed" />
          <ProvenanceBadge kind="inferred" />
          <span className="text-muted-2">Dashed edges = non-observed</span>
        </div>
      </div>
    </div>
  );
};
