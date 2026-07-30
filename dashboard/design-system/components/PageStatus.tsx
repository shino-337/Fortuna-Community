import React from 'react';
import clsx from 'clsx';
import { AlertCircle, Inbox, Loader2 } from 'lucide-react';

export type PageEmptyVariant = 'default' | 'compact' | 'withImage';

export interface PageEmptyProps {
  title?: string;
  description?: string;
  className?: string;
  icon?: React.ReactNode;
  action?: React.ReactNode;
  variant?: PageEmptyVariant;
  /** Used when variant="withImage" */
  image?: React.ReactNode;
}

export const PageEmpty: React.FC<PageEmptyProps> = ({
  title = 'No data',
  description,
  className = '',
  icon,
  action,
  variant = 'default',
  image,
}) => {
  const compact = variant === 'compact';
  const showImage = variant === 'withImage' && image;

  return (
    <div
      className={clsx(
        'flex flex-col items-center justify-center px-4 text-center text-text',
        compact ? 'gap-1 py-4' : 'gap-2 py-8',
        className
      )}
    >
      {showImage ? (
        <div className="mb-2 shrink-0">{image}</div>
      ) : (
        (icon ?? (
          <Inbox
            className={clsx('text-muted-2 opacity-60', compact ? 'h-8 w-8' : 'h-10 w-10')}
            aria-hidden
          />
        ))
      )}
      <p className={clsx('font-medium text-muted', compact ? 'text-caption' : 'text-body')}>{title}</p>
      {description ? (
        <p className={clsx('max-w-sm text-muted-2', compact ? 'text-caption' : 'text-caption font-normal')}>
          {description}
        </p>
      ) : null}
      {action ? <div className={compact ? 'mt-1' : 'mt-2'}>{action}</div> : null}
    </div>
  );
};


export interface PageErrorProps {
  title?: string;
  description?: string;
  className?: string;
  action?: React.ReactNode;
}

export const PageError: React.FC<PageErrorProps> = ({
  title = "Could not load data",
  description,
  className = "",
  action,
}) => (
  <div
    className={clsx("flex flex-col items-center justify-center gap-2 px-4 py-8 text-center text-text", className)}
    role="alert"
  >
    <AlertCircle className="h-10 w-10 text-amber-400" aria-hidden />
    <p className="text-body font-medium text-text">{title}</p>
    {description ? <p className="max-w-sm text-caption text-muted-2">{description}</p> : null}
    {action ? <div className="mt-2">{action}</div> : null}
  </div>
);

export interface PageLoadingProps {
  message?: string;
  className?: string;
}

/** Shared spinner animation for page or section loading states */
export const PageLoading: React.FC<PageLoadingProps> = ({
  message = 'Loading…',
  className = '',
}) => (
  <div
    className={clsx('flex flex-col items-center justify-center gap-4 px-4 py-12', className)}
    role="status"
    aria-live="polite"
  >
    <Loader2 className="h-10 w-10 animate-spin text-brand motion-reduce:animate-none" aria-hidden />
    <p className="text-body text-muted">{message}</p>
  </div>
);
