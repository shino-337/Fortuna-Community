import React from 'react';
import { ShieldAlert, Radio, EyeOff, Layers } from 'lucide-react';
import type { GraphTrustPosture } from '../lib/graphTrustSemantics';
import { GraphTrustLegend } from './GraphTrustLegend';

/**
 * Native graph trust completeness strip — complements in-SVG semantics.
 */
export const GraphTrustOverlay: React.FC<{
  posture: GraphTrustPosture;
  showLegend?: boolean;
  className?: string;
}> = ({ posture, showLegend = true, className = '' }) => {
  const showBanner =
    posture.completeness !== 'full' ||
    posture.stale ||
    posture.sampled ||
    posture.policyLimited;

  if (!showBanner && !showLegend) return null;

  return (
    <div className={`space-y-2 ${className}`}>
      {showBanner ? (
        <div
          className="flex items-start gap-2 rounded-lg border border-amber-500/25 bg-amber-500/10 px-3 py-2 text-caption"
          role="note"
        >
          <ShieldAlert className="w-4 h-4 shrink-0 text-amber-300 mt-0.5" aria-hidden />
          <div className="min-w-0 space-y-1">
            <p className="font-medium text-text">{completenessTitle(posture.completeness)}</p>
            <p className="text-muted">{posture.summary}</p>
            <div className="flex flex-wrap gap-2 text-meta text-muted-2">
              {posture.sampled ? (
                <span className="inline-flex items-center gap-1">
                  <Layers className="w-3 h-3" /> Sampled
                </span>
              ) : null}
              {posture.stale ? (
                <span className="inline-flex items-center gap-1">
                  <Radio className="w-3 h-3" /> Stale telemetry
                </span>
              ) : null}
              {posture.policyLimited ? (
                <span className="inline-flex items-center gap-1">
                  <EyeOff className="w-3 h-3" /> Policy-limited
                </span>
              ) : null}
              {posture.hiddenEdgeCount > 0 ? (
                <span>~{posture.hiddenEdgeCount} relationship(s) may be hidden</span>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}
      {showLegend ? <GraphTrustLegend posture={posture} compact /> : null}
    </div>
  );
};

function completenessTitle(c: GraphTrustPosture['completeness']): string {
  switch (c) {
    case 'full':
      return 'Graph integrity: within scope';
    case 'sampled':
      return 'Sampled graph — not complete';
    case 'policy_limited':
      return 'Policy-limited visibility';
    case 'scope_limited':
      return 'Scope-limited topology';
    case 'degraded':
      return 'Telemetry-degraded graph';
    default:
      return 'Graph trust notice';
  }
}
