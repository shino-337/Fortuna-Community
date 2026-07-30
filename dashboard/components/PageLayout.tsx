import React from 'react';
import { PageHeader } from '../design-system/layouts/PageContainer';

export interface PageLayoutProps {
  title: string;
  description?: string;
  actions?: React.ReactNode;
  toolbar?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  /** Smaller header (e.g. network topology — maximize space below). */
  compact?: boolean;
  /** Fill remaining main height (flex-1); use with a flex column parent that has min-h-0. */
  fillHeight?: boolean;
}

/** Layout wrapper for pages — provides consistent header/footer and responsive container. */
export const PageLayout: React.FC<PageLayoutProps> = ({
  title,
  description,
  actions,
  toolbar,
  children,
  className = '',
  compact = false,
  fillHeight = false,
}) => (
  <div
    className={
      fillHeight
        ? `flex flex-col gap-2 flex-1 min-h-0 h-full animate-in fade-in duration-150 motion-reduce:animate-none ${className}`
        : `${compact ? 'flex flex-col gap-2' : 'flex flex-col gap-5'} animate-in fade-in duration-150 motion-reduce:animate-none ${className}`
    }
  >
    <PageHeader compact={compact} title={title} description={description} actions={actions} />
    {toolbar && (
      <div className="rounded-lg border border-border bg-surface/60 px-3 py-2.5 sm:px-4 text-body">{toolbar}</div>
    )}
    <div className={`w-full max-w-full ${fillHeight ? 'flex flex-1 min-h-0 flex-col' : ''}`}>
      {children}
    </div>
  </div>
);

/** Section container for page content — provides consistent spacing and optional header. */
export interface PageSectionProps {
  id?: string;
  title?: string;
  description?: string;
  actions?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}

/** A section of a page with an optional heading, subheading, and action buttons. */
export const PageSection: React.FC<PageSectionProps> = ({
  id,
  title,
  description,
  actions,
  children,
  className = '',
}) => (
  <section id={id} className={`flex flex-col gap-5 ${className}`}>
    {(title || description || actions) && (
      <div className="flex min-w-0 flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div className="min-w-0 flex-1">
          {title && <h2 className="text-section-title text-text">{title}</h2>}
          {description && (
            <p className="mt-1 fortuna-prose-compact">{description}</p>
          )}
        </div>
        {actions && <div className="flex w-full min-w-0 flex-wrap items-center gap-2 md:w-auto md:shrink-0 md:justify-end">{actions}</div>}
      </div>
    )}
    <div className="flex min-w-0 flex-col gap-3">{children}</div>
  </section>
);
