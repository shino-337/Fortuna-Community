import React from 'react';
import clsx from 'clsx';

export type CardVariant = 'default' | 'primary' | 'secondary' | 'ghost' | 'panel';

const variantPadding = {
  default: { header: 'px-6 py-4', content: 'p-6' },
  primary: { header: 'px-6 py-4', content: 'p-6' },
  secondary: { header: 'px-4 py-3', content: 'p-4' },
  ghost: { header: 'px-4 py-3', content: 'p-4' },
  panel: { header: 'px-3 py-2', content: 'p-3' },
} as const;

const interactiveShell = 'transition-colors duration-150 motion-reduce:transition-none';

const variantShell: Record<CardVariant, string> = {
  default:
    `bg-surface/80 rounded-xl border border-border/80 shadow-md hover:border-border ${interactiveShell} min-w-0`,
  primary:
    `bg-surface/80 rounded-xl border border-border/80 shadow-md hover:border-border ${interactiveShell} min-w-0`,
  secondary:
    `bg-surface/80 rounded-xl border border-border/80 shadow-md hover:border-border ${interactiveShell} min-w-0`,
  ghost: 'bg-transparent rounded-xl border border-border/50 min-w-0 shadow-none',
  panel: 'bg-surface/80 rounded-xl border border-border/80 shadow-md min-w-0',
};

interface CardProps {
  children: React.ReactNode;
  className?: string;
  contentClassName?: string;
  title?: string;
  description?: string;
  actions?: React.ReactNode;
  variant?: CardVariant;
}

export const Card: React.FC<CardProps> = ({
  children,
  className = '',
  contentClassName,
  title,
  description,
  actions,
  variant = 'default',
}) => {
  const v = variant === 'panel' ? 'panel' : variant;
  const pad = variantPadding[v];
  const shell = variantShell[v];
  const classNameHasPadding = /(?:^|\s)(?:p|px|py|pt|pr|pb|pl)-/.test(className);
  const resolvedContentClassName = contentClassName ?? (classNameHasPadding ? 'p-0' : pad.content);

  return (
    <div className={clsx(shell, className)}>
      {(title || description || actions) && (
        <div
          className={`${pad.header} flex min-w-0 flex-col gap-3 border-b border-border/60 sm:flex-row sm:items-start sm:justify-between`}
        >
          <div className="min-w-0 flex-1">
            {title && <h3 className="text-card-title text-text">{title}</h3>}
            {description && (
              <p className="mt-1.5 max-w-[68ch] text-body leading-relaxed text-muted">{description}</p>
            )}
          </div>
          {actions && <div className="flex w-full min-w-0 flex-wrap items-center gap-2 sm:w-auto sm:shrink-0 sm:justify-end">{actions}</div>}
        </div>
      )}
      <div className={clsx(resolvedContentClassName)}>{children}</div>
    </div>
  );
};
