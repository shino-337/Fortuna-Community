import React from 'react';

export interface PageLayoutProps {
  title: string;
  description?: string;
  actions?: React.ReactNode;
  toolbar?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  constrained?: boolean;
}

export const PageLayout: React.FC<PageLayoutProps> = ({
  title,
  description,
  actions,
  toolbar,
  children,
  className = '',
  constrained = true,
}) => (
  <div className={`space-y-6 animate-in fade-in duration-300 ${className}`}>
    <header className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
      <div className="min-w-0">
        <h1 className="text-2xl font-bold text-text tracking-tight truncate">{title}</h1>
        {description && <p className="text-muted text-sm mt-1 max-w-2xl">{description}</p>}
      </div>
      {actions && <div className="flex items-center gap-2 shrink-0 flex-wrap">{actions}</div>}
    </header>
    {toolbar && (
      <div className="rounded-lg border border-border bg-surface/60 px-4 py-3">{toolbar}</div>
    )}
    <div className={constrained ? 'max-w-full' : ''}>{children}</div>
  </div>
);

export interface PageSectionProps {
  title?: string;
  description?: string;
  actions?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}

export const PageSection: React.FC<PageSectionProps> = ({
  title,
  description,
  actions,
  children,
  className = '',
}) => (
  <section className={`space-y-4 ${className}`}>
    {(title || description || actions) && (
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 mb-4">
      <div>
        {title && (
          <h2 className="text-sm font-semibold text-muted uppercase tracking-wider">{title}</h2>
        )}
        {description && <p className="text-muted-2 text-xs mt-0.5">{description}</p>}
      </div>
        {actions && <div className="flex items-center gap-2">{actions}</div>}
      </div>
    )}
    {children}
  </section>
);
