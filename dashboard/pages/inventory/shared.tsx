import React from 'react';
import { getSeverityBadgeClass } from '../../lib/severity';

export const LEVELS = ['critical', 'high', 'medium', 'low'] as const;
export type Level = (typeof LEVELS)[number];

export const LEVEL_LABEL: Record<Level | 'unscored', string> = {
  critical: 'Critical',
  high: 'High',
  medium: 'Medium',
  low: 'Low',
  unscored: 'No score',
};

/** Risk level badge with the 0-100 score; "No score" when the workload has not been scored. */
export function RiskBadge({ level, score }: { level?: string; score?: number }) {
  if (!level) return <span className="text-caption text-muted">No score</span>;
  return (
    <span className="inline-flex items-center gap-1.5 whitespace-nowrap">
      <span className={`rounded border px-1.5 py-0.5 text-meta font-semibold uppercase ${getSeverityBadgeClass(level)}`}>{level}</span>
      {score != null ? <span className="text-caption tabular-nums text-muted">{Math.round(score)}</span> : null}
    </span>
  );
}

/** A filter chip that shows how many rows it would return. */
export function CountChip({
  label,
  count,
  active,
  onClick,
  tone,
}: {
  label: string;
  count?: number;
  active: boolean;
  onClick: () => void;
  tone?: string;
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={`inline-flex min-h-9 items-center gap-1.5 rounded-lg border px-3 text-caption font-semibold transition-colors ${
        active ? 'border-brand bg-brand/15 text-text' : 'border-border text-muted hover:border-muted hover:text-text'
      }`}
    >
      {tone ? <span className={`h-2 w-2 rounded-full ${tone}`} aria-hidden /> : null}
      {label}
      {count != null ? <span className="tabular-nums text-muted">{count.toLocaleString()}</span> : null}
    </button>
  );
}

export const LEVEL_TONE: Record<Level | 'unscored', string> = {
  critical: 'bg-red-500',
  high: 'bg-orange-500',
  medium: 'bg-yellow-400',
  low: 'bg-slate-400',
  unscored: 'bg-transparent border border-muted',
};

/** A titled block in a side panel with an "Open in …" link on the right. */
export function PanelSection({ title, link, children }: { title: string; link?: React.ReactNode; children: React.ReactNode }) {
  return (
    <section className="flex flex-col gap-2">
      <div className="flex items-baseline justify-between gap-3">
        <h3 className="text-caption font-semibold uppercase tracking-wider text-muted">{title}</h3>
        {link}
      </div>
      {children}
    </section>
  );
}
