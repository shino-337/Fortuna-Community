import React from 'react';
import clsx from 'clsx';

export type ProvenanceKind = 'observed' | 'inferred' | 'theoretical' | 'sampled' | 'stale' | 'partial' | 'estimated';

const STYLES: Record<ProvenanceKind, string> = {
  observed: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-300',
  inferred: 'border-amber-500/40 bg-amber-500/10 text-amber-200',
  theoretical: 'border-violet-500/40 bg-violet-500/10 text-violet-200',
  sampled: 'border-sky-500/40 bg-sky-500/10 text-sky-200',
  stale: 'border-rose-500/40 bg-rose-500/10 text-rose-200',
  partial: 'border-orange-500/40 bg-orange-500/10 text-orange-200',
  estimated: 'border-border bg-surface/60 text-muted',
};

const LABELS: Record<ProvenanceKind, string> = {
  observed: 'Observed',
  inferred: 'Inferred',
  theoretical: 'Theoretical',
  sampled: 'Sampled',
  stale: 'Stale',
  partial: 'Partial',
  estimated: 'Estimated',
};

export const ProvenanceBadge: React.FC<{
  kind: ProvenanceKind;
  title?: string;
  className?: string;
}> = ({ kind, title, className }) => (
  <span
    className={clsx(
      'inline-flex items-center rounded px-1.5 py-0.5 text-micro font-semibold uppercase tracking-wide border',
      STYLES[kind],
      className,
    )}
    title={title ?? `Evidence provenance: ${LABELS[kind]}`}
  >
    {LABELS[kind]}
  </span>
);
