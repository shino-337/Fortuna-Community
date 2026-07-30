import React from 'react';
import { useDecisionPriority } from '../hooks/useDecisionPriority';
import { Button } from './ui/Button';
import { useNavigate } from 'react-router-dom';
import { Ban, HelpCircle } from 'lucide-react';

/** Strip showing top operational priorities based on signals — what matters now. */
export const IncidentPriorityStrip: React.FC<{
  criticalCount: number;
  attackPathCount: number;
  hasOpenFindings: boolean;
  telemetryDegraded?: boolean;
}> = (signals) => {
  const { decisions } = useDecisionPriority(signals);
  const navigate = useNavigate();
  const top = decisions.slice(0, 4);

  if (top.length === 0) return null;

  return (
    <section
      className="rounded-xl border border-border/80 bg-surface/50 p-3 space-y-2"
      aria-label="Operational priorities"
    >
      <h2 className="text-caption font-semibold uppercase tracking-wide text-muted">What matters now</h2>
      <ul className="grid gap-2 sm:grid-cols-2">
        {top.map((d) => (
          <li
            key={d.id}
            className={`rounded-lg border px-3 py-2 ${
              d.priority === 'critical'
                ? 'border-red-500/30 bg-red-500/5'
                : d.priority === 'high'
                  ? 'border-amber-500/25 bg-amber-500/5'
                  : 'border-border bg-base/30'
            }`}
          >
            <p className="text-body font-semibold text-text">{d.title}</p>
            <p className="text-caption text-muted mt-0.5 flex items-start gap-1">
              <HelpCircle className="w-3.5 h-3.5 shrink-0 mt-0.5" aria-hidden />
              <span>{d.whyNow}</span>
            </p>
            {d.factors.length > 0 ? (
              <ul className="mt-1.5 text-meta text-muted-2 space-y-0.5">
                {d.factors.slice(0, 3).map((f) => (
                  <li key={f.code}>
                    {f.label} ({f.direction === 'increases' ? '+' : '−'}
                    {Math.round(f.weight * 100)}%)
                  </li>
                ))}
              </ul>
            ) : null}
            {d.blockers.length > 0 ? (
              <p className="mt-2 text-meta text-amber-300/90 flex items-start gap-1">
                <Ban className="w-3 h-3 shrink-0" />
                {d.blockers[0]}
              </p>
            ) : null}
            {d.consequences && d.consequences.length > 0 ? (
              <p className="mt-1.5 text-meta text-rose-300/90">
                If acted: {d.consequences[0].message}
              </p>
            ) : null}
            <p className="text-meta text-muted-2 mt-1">
              Confidence: {Math.round(d.confidence * 100)}%
              {!d.actionable ? ' · Not actionable now' : ''}
            </p>
            {d.actionable && d.href ? (
              <Button
                type="button"
                size="sm"
                variant="secondary"
                className="mt-2"
                onClick={() => navigate(d.href!)}
              >
                Act
              </Button>
            ) : null}
          </li>
        ))}
      </ul>
    </section>
  );
};
