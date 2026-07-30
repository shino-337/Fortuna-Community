import React from 'react';
import { ArrowRight, Circle, Diamond, Triangle, CircleDot } from 'lucide-react';
import { Button } from '../../components/ui/Button';

export type DecisionSpineStep = {
  label: string;
  value: string;
  detail?: string;
};

export type DecisionSpineMetric = {
  label: string;
  value: React.ReactNode;
  tone?: 'critical' | 'high' | 'medium' | 'low' | 'neutral';
};

const toneClass: Record<NonNullable<DecisionSpineMetric['tone']>, string> = {
  critical: 'border-red-500/35 bg-red-500/10 text-red-200',
  high: 'border-orange-500/35 bg-orange-500/10 text-orange-200',
  medium: 'border-yellow-400/30 bg-yellow-400/10 text-yellow-100',
  low: 'border-sky-400/30 bg-sky-400/10 text-sky-100',
  neutral: 'border-border bg-base/35 text-text',
};

const severityIcon: Record<NonNullable<DecisionSpineMetric['tone']>, React.ReactNode> = {
  critical: <Triangle className="h-3.5 w-3.5 fill-current" aria-hidden />,
  high: <Diamond className="h-3.5 w-3.5 fill-current" aria-hidden />,
  medium: <CircleDot className="h-3.5 w-3.5" aria-hidden />,
  low: <Circle className="h-3.5 w-3.5" aria-hidden />,
  neutral: <Circle className="h-3.5 w-3.5" aria-hidden />,
};

export const DecisionSpine: React.FC<{
  roleLabel: string;
  title: string;
  situation: string;
  impact: string;
  owner: string;
  actionLabel: string;
  onAction: () => void;
  secondaryAction?: React.ReactNode;
  steps: DecisionSpineStep[];
  metrics?: DecisionSpineMetric[];
}> = ({
  roleLabel,
  title,
  situation,
  impact,
  owner,
  actionLabel,
  onAction,
  secondaryAction,
  steps,
  metrics = [],
}) => (
  <section className="rounded-xl border border-border/80 bg-surface/60 p-4 sm:p-5" aria-label={`${roleLabel} decision lane`}>
    <div className="grid gap-5 xl:grid-cols-[minmax(0,1.35fr)_minmax(20rem,0.65fr)]">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <span className="rounded-md border border-brand/30 bg-brand/10 px-2 py-1 text-caption font-semibold text-brand">
            {roleLabel}
          </span>
          <span className="text-caption text-muted">Decision lane</span>
        </div>
        <h2 className="mt-3 text-section-title text-text">{title}</h2>
        <p className="mt-2 fortuna-prose">{situation}</p>

        <ol className="mt-5 grid gap-2 md:grid-cols-2 xl:grid-cols-5" aria-label="Decision path">
          {steps.map((step, index) => (
            <li key={step.label} className="min-w-0 rounded-lg border border-border/70 bg-base/25 p-3">
              <div className="flex items-center gap-2">
                <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full border border-border bg-surface text-meta font-semibold text-muted">
                  {index + 1}
                </span>
                <p className="min-w-0 truncate text-caption font-semibold text-muted" title={step.label}>
                  {step.label}
                </p>
              </div>
              <p className="mt-2 min-w-0 truncate text-body font-semibold text-text" title={step.value}>
                {step.value}
              </p>
              {step.detail ? <p className="mt-0.5 min-w-0 truncate text-caption font-normal text-muted-2" title={step.detail}>{step.detail}</p> : null}
            </li>
          ))}
        </ol>
      </div>

      <div className="flex min-w-0 flex-col gap-3 rounded-lg border border-border/70 bg-base/25 p-3">
        <div>
          <p className="text-caption font-semibold text-text">Operational impact</p>
          <p className="mt-1 text-caption text-muted">{impact}</p>
        </div>
        <div>
          <p className="text-caption font-semibold text-text">Owner</p>
          <p className="mt-1 text-caption text-muted">{owner}</p>
        </div>
        {metrics.length > 0 ? (
          <div className="grid grid-cols-1 gap-2 min-[420px]:grid-cols-2">
            {metrics.map((metric) => {
              const tone = metric.tone ?? 'neutral';
              return (
                <div key={metric.label} className={`rounded-lg border px-2.5 py-2 ${toneClass[tone]}`}>
                  <p className="flex items-center gap-1.5 text-meta font-medium">
                    {severityIcon[tone]}
                    {metric.label}
                  </p>
                  <p className="mt-1 fortuna-metric">{metric.value}</p>
                </div>
              );
            })}
          </div>
        ) : null}
        <div className="mt-auto flex flex-col gap-2 pt-1 sm:flex-row sm:flex-wrap">
          <Button type="button" onClick={onAction} className="w-full sm:w-auto">
            {actionLabel}
            <ArrowRight className="ml-1.5 h-4 w-4" aria-hidden />
          </Button>
          {secondaryAction ? (
            <div className="w-full sm:w-auto [&>*]:w-full sm:[&>*]:w-auto">{secondaryAction}</div>
          ) : null}
        </div>
      </div>
    </div>
  </section>
);
