import React, { useMemo } from 'react';
import clsx from 'clsx';
import { AlertTriangle, EyeOff } from 'lucide-react';
import {
  resolveMetricSemantics,
  type MetricSemanticsInput,
  type MetricSemanticState,
} from '../../lib/metricSemantics';

const STATE_TONE: Partial<Record<MetricSemanticState, string>> = {
  hidden: 'text-muted-2',
  unknown: 'text-muted',
  stale: 'text-amber-300/90',
  sampled: 'text-sky-300/90',
  partial: 'text-amber-200/90',
  inferred: 'text-muted',
  exact_zero: 'text-muted',
  scoped_exact: 'text-text',
};

export const ScopedMetric: React.FC<
  MetricSemanticsInput & {
    className?: string;
    valueClassName?: string;
    compact?: boolean;
    onClick?: () => void;
    icon?: React.ReactNode;
  }
> = ({ className = '', valueClassName = '', compact, onClick, icon, ...input }) => {
  const sem = useMemo(() => resolveMetricSemantics(input), [input]);

  const inner = (
    <>
      <div className="flex items-start justify-between gap-2 min-w-0">
        <div className="min-w-0 flex-1">
          <p className={clsx('font-medium text-muted', compact ? 'text-meta' : 'text-caption')}>{input.label}</p>
          <p
            className={clsx(
              'break-words font-bold tabular-nums',
              compact ? 'text-metric' : 'text-metric-lg',
              STATE_TONE[sem.semanticState] ?? 'text-text',
              valueClassName,
            )}
          >
            {sem.displayValue}
          </p>
        </div>
        {icon ? <div className="shrink-0">{icon}</div> : null}
      </div>
      <p className={clsx('mt-1 leading-snug text-muted-2', compact ? 'text-meta' : 'text-caption')}>{sem.subtitle}</p>
      {sem.showCaveat ? (
        <p className="mt-1 flex items-center gap-1 text-meta text-amber-300/80">
          {sem.semanticState === 'hidden' ? (
            <EyeOff className="w-3 h-3 shrink-0" aria-hidden />
          ) : (
            <AlertTriangle className="w-3 h-3 shrink-0" aria-hidden />
          )}
          <span>Metric may not reflect full platform state.</span>
        </p>
      ) : null}
    </>
  );

  const shell = clsx(
    'h-full min-w-0 rounded-xl border border-border/80 bg-surface/80 p-3 text-left w-full transition-colors sm:p-4',
    onClick && 'hover:border-brand/40 hover:bg-surface-2/50 cursor-pointer focus-visible:ring-2 focus-visible:ring-brand/40',
    className,
  );

  if (onClick) {
    return (
      <button type="button" className={shell} onClick={onClick} aria-label={sem.ariaLabel}>
        {inner}
      </button>
    );
  }
  return <div className={shell}>{inner}</div>;
};
