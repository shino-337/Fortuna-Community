import React from 'react';
import clsx from 'clsx';

/** Max width 90rem (1440px at 16px root): Fortuna page canvas */
export interface PageContainerProps {
  children: React.ReactNode;
  className?: string;
  /** Fill remaining main height; use with PageLayout fillHeight */
  fillHeight?: boolean;
}

export const PageContainer: React.FC<PageContainerProps> = ({ children, className, fillHeight }) => (
  <div
    className={clsx(
      'mx-auto flex w-full max-w-[90rem] flex-1 min-h-0 flex-col px-4 pb-8 pt-4 sm:px-6 sm:pb-10 sm:pt-5 lg:px-8 lg:pb-10 lg:pt-6',
      fillHeight ? 'h-full' : '',
      className
    )}
  >
    {children}
  </div>
);

export interface PageHeaderProps {
  title: string;
  description?: string;
  actions?: React.ReactNode;
  /** Smaller title row (e.g. network topology) */
  compact?: boolean;
}

export const PageHeader: React.FC<PageHeaderProps> = ({ title, description, actions, compact }) => compact ? (
  <div
    className="flex min-w-0 flex-col justify-between gap-4 md:flex-row md:items-start md:gap-3"
  >
    <div className="min-w-0 flex-1">
      <h1
        className="max-w-[72ch] break-words font-bold text-text [text-wrap:balance] text-section-title"
      >
        {title}
      </h1>
      {description ? (
        <p className="text-muted mt-1 max-w-[72ch] text-caption leading-snug">
          {description}
        </p>
      ) : null}
    </div>
    {actions ? (
      <div
        className="flex w-full min-w-0 flex-wrap items-center gap-2 md:w-auto md:shrink-0 md:justify-end md:gap-2"
      >
        {actions}
      </div>
    ) : null}
  </div>
) : (
  <div className="grid min-w-0 gap-x-4 gap-y-1 border-b border-border/60 pb-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-start">
    <div className="min-w-0">
      <h1 className="max-w-[72ch] break-words font-bold text-text [text-wrap:balance] text-page-title">
        {title}
      </h1>
    </div>
    {actions ? (
      <div className="flex w-full min-w-0 flex-wrap items-center gap-2 md:w-auto md:shrink-0 md:justify-end md:gap-3">
        {actions}
      </div>
    ) : null}
    {description ? (
      <p className="text-muted mt-1 fortuna-prose-compact md:col-span-2">
        {description}
      </p>
    ) : null}
  </div>
);
