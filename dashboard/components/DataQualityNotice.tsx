import React from 'react';
import clsx from 'clsx';
import { Info } from 'lucide-react';

export type DataQualityLevel = 'exact' | 'sampled' | 'estimated' | 'unavailable';

const LABELS: Record<DataQualityLevel, string> = {
  exact: 'Exact',
  sampled: 'Sampled',
  estimated: 'Estimated',
  unavailable: 'Unavailable',
};

const STYLES: Record<DataQualityLevel, string> = {
  exact: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-200',
  sampled: 'border-amber-500/30 bg-amber-500/10 text-amber-100',
  estimated: 'border-sky-500/30 bg-sky-500/10 text-sky-100',
  unavailable: 'border-border bg-surface/40 text-muted',
};

export const DataQualityNotice: React.FC<{
  level: DataQualityLevel;
  children: React.ReactNode;
  className?: string;
}> = ({ level, children, className }) => (
  <div
    role="status"
    className={clsx(
      'flex items-start gap-2 rounded-lg border px-3 py-2 text-caption leading-relaxed',
      STYLES[level],
      className,
    )}
  >
    <Info className="h-4 w-4 shrink-0 mt-0.5 opacity-80" aria-hidden />
    <div className="min-w-0">
      <span className="font-semibold uppercase tracking-wide text-micro mr-2">{LABELS[level]}</span>
      {children}
    </div>
  </div>
);
