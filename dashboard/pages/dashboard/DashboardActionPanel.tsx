import React from 'react';
import type { LucideIcon } from 'lucide-react';

export const DashboardActionPanel: React.FC<{
  icon: LucideIcon;
  title: string;
  description: string;
  action?: React.ReactNode;
  tone?: 'neutral' | 'warning' | 'brand';
  className?: string;
}> = ({ icon: Icon, title, description, action, tone = 'neutral', className = '' }) => {
  const toneClass = {
    neutral: 'border-border bg-surface/30 text-brand',
    warning: 'border-amber-500/30 bg-amber-500/5 text-amber-200',
    brand: 'border-brand/25 bg-brand/5 text-brand',
  }[tone];

  return (
    <section
      className={`flex flex-col gap-3 rounded-lg border p-4 sm:flex-row sm:items-center sm:justify-between ${toneClass} ${className}`.trim()}
    >
      <div className="flex min-w-0 items-start gap-3">
        <div className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-current/25 bg-base/25">
          <Icon className="h-5 w-5" aria-hidden />
        </div>
        <div className="min-w-0">
          <h2 className="text-card-title text-text">{title}</h2>
          <p className="mt-1 max-w-[68ch] text-caption font-normal text-muted">{description}</p>
        </div>
      </div>
      {action ? (
        <div className="flex w-full shrink-0 sm:ml-4 sm:w-auto sm:justify-end [&>*]:w-full sm:[&>*]:w-auto">
          {action}
        </div>
      ) : null}
    </section>
  );
};
