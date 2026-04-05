import React from 'react';
import clsx from 'clsx';

export type CardVariant = 'primary' | 'secondary' | 'panel';

const variantPadding = {
  primary: { header: 'px-6 py-4', content: 'p-6' },
  secondary: { header: 'px-4 py-3', content: 'p-4' },
  panel: { header: 'px-3 py-2', content: 'p-3' },
} as const;

interface CardProps {
  children: React.ReactNode;
  className?: string;
  /** Merges with variant default inner padding (e.g. p-0 for full-bleed tables). */
  contentClassName?: string;
  title?: string;
  description?: string;
  actions?: React.ReactNode;
  /** Design system: primary (charts, main blocks), secondary (compact), panel (small). Default primary. */
  variant?: CardVariant;
}

export const Card: React.FC<CardProps> = ({
  children,
  className = '',
  contentClassName,
  title,
  description,
  actions,
  variant = 'primary',
}) => {
  const pad = variantPadding[variant];
  return (
    <div className={clsx('bg-surface rounded-lg border border-border shadow-sm min-w-0', className)}>
      {(title || description || actions) && (
        <div className={`${pad.header} border-b border-border flex justify-between items-start`}>
          <div>
            {title && <h3 className="text-lg font-semibold text-text">{title}</h3>}
            {description && <p className="text-sm text-muted mt-1">{description}</p>}
          </div>
          {actions && <div className="ml-4">{actions}</div>}
        </div>
      )}
      <div className={clsx(pad.content, contentClassName)}>{children}</div>
    </div>
  );
};
