import React from 'react';
import type { GraphTrustPosture } from '../lib/graphTrustSemantics';
import { graphTrustQuestions } from '../lib/graphTrustSemantics';
import { GRAPH_THEME } from '../lib/graphTheme';

const EDGE_ITEMS: { kind: string; label: string; dash?: string; width: number; opacity: number }[] = [
  { kind: 'observed_runtime', label: 'Runtime observed', width: 2.8, opacity: 0.95 },
  { kind: 'exploit_chain', label: 'Exploit chain', width: 2.6, opacity: 0.9 },
  { kind: 'inferred', label: 'Inferred', width: 1.5, opacity: 0.65, dash: '6 4' },
  { kind: 'sampled', label: 'Sampled / clustered', width: 1.3, opacity: 0.5, dash: '3 5' },
  { kind: 'stale', label: 'Stale telemetry', width: 1.6, opacity: 0.55, dash: '8 4' },
  { kind: 'hidden_by_policy', label: 'Policy-limited', width: 1.1, opacity: 0.35, dash: '2 6' },
];

export const GraphTrustLegend: React.FC<{
  posture: GraphTrustPosture;
  compact?: boolean;
  className?: string;
}> = ({ posture, compact, className = '' }) => {
  const questions = graphTrustQuestions(posture);

  return (
    <div
      className={`rounded-lg border border-border/80 bg-base/95 backdrop-blur px-3 py-2 text-caption ${className}`}
      role="region"
      aria-label="Graph trust legend"
    >
      <p className="text-meta font-semibold uppercase tracking-wide text-muted mb-1.5">Graph trust style</p>
      <div className="flex flex-wrap gap-x-3 gap-y-1 mb-2">
        {EDGE_ITEMS.filter((e) => {
          if (e.kind === 'sampled' && !posture.sampled) return false;
          if (e.kind === 'stale' && !posture.stale) return false;
          if (e.kind === 'hidden_by_policy' && !posture.policyLimited) return false;
          return true;
        }).map((e) => (
          <span key={e.kind} className="inline-flex items-center gap-1 text-muted">
            <svg width={20} height={4} aria-hidden>
              <line
                x1={0}
                y1={2}
                x2={20}
                y2={2}
                stroke={GRAPH_THEME.text}
                strokeWidth={e.width}
                strokeOpacity={e.opacity}
                strokeDasharray={e.dash}
              />
            </svg>
            {e.label}
          </span>
        ))}
      </div>
      <p className="text-meta text-muted-2">Link color identifies relationship type; trust changes dash, opacity, and weight.</p>
      {!compact ? (
        <ul className="space-y-0.5 text-meta text-muted-2 border-t border-border/60 pt-2">
          {questions.slice(0, 4).map((q) => (
            <li key={q.q}>
              <span className="text-muted">{q.q}</span> {q.a}
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
};
