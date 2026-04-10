import React from 'react';

export interface PageLayoutProps {
  title: string;
  description?: string;
  actions?: React.ReactNode;
  toolbar?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  constrained?: boolean;
  /** Header nhỏ hơn (vd. Network topology — tối đa không gian nội dung phía dưới). */
  compact?: boolean;
  /** Chiếm chiều cao còn lại trong main (flex-1); dùng cùng flex column min-h-0 ở cha. */
  fillHeight?: boolean;
}

export const PageLayout: React.FC<PageLayoutProps> = ({
  title,
  description,
  actions,
  toolbar,
  children,
  className = '',
  constrained = true,
  compact = false,
  fillHeight = false,
}) => (
  <div
    className={
      fillHeight
        ? `flex flex-col gap-2 flex-1 min-h-0 h-full animate-in fade-in duration-300 ${className}`
        : `${compact ? 'space-y-2' : 'space-y-8'} animate-in fade-in duration-300 ${className}`
    }
  >
    <header
      className={`flex flex-col sm:flex-row sm:items-start sm:justify-between ${compact ? 'gap-2' : 'gap-4'}`}
    >
      <div className="min-w-0">
        <h1
          className={`font-bold text-text tracking-tight truncate ${compact ? 'text-lg sm:text-xl' : 'text-2xl'}`}
        >
          {title}
        </h1>
        {description && (
          <p
            className={`text-muted ${compact ? 'text-[11px] mt-0.5 leading-snug max-w-4xl' : 'text-sm mt-1 max-w-2xl'}`}
          >
            {description}
          </p>
        )}
      </div>
      {actions && (
        <div className={`flex items-center gap-2 shrink-0 flex-wrap ${compact ? 'sm:pt-0.5' : ''}`}>
          {actions}
        </div>
      )}
    </header>
    {toolbar && (
      <div className="rounded-lg border border-border bg-surface/60 px-4 py-3">{toolbar}</div>
    )}
    <div
      className={`${constrained ? 'max-w-full' : ''} ${fillHeight ? 'flex-1 flex flex-col min-h-0' : ''}`}
    >
      {children}
    </div>
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
