import React from 'react';
import { ArrowDown, ArrowUp, Minus } from 'lucide-react';
import clsx from 'clsx';

export type StatCardTone = 'default' | 'info' | 'warning' | 'critical' | 'high' | 'medium' | 'success' | 'brand';

const toneClasses: Record<StatCardTone, string> = {
  default: 'bg-surface-2 text-muted',
  info: 'bg-info/15 text-info',
  warning: 'bg-warning/15 text-warning',
  critical: 'bg-critical/15 text-critical',
  high: 'bg-high/15 text-high',
  medium: 'bg-medium/15 text-medium',
  success: 'bg-success/15 text-success',
  brand: 'bg-brand/15 text-brand',
};

interface StatCardProps {
  title: string;
  value: string | number;
  icon: React.ReactNode;
  subtitle?: string;
  trend?: number;
  trendLabel?: string;
  trendDirection?: 'up' | 'down' | 'neutral';
  tone?: StatCardTone;
  /** @deprecated Prefer tone for semantic dashboard colors. */
  color?: string;
  onClick?: () => void;
  ariaLabel?: string;
}

/** Stat card — metric title/value with optional trend and click handler. */
export const StatCard: React.FC<StatCardProps> = ({
  title,
  value,
  icon,
  subtitle,
  trend,
  trendLabel = 'vs last month',
  trendDirection = 'neutral',
  tone = 'default',
  color,
  onClick,
  ariaLabel,
}) => {
  const shellClass = clsx(
    'h-full min-w-0 bg-surface/80 rounded-xl border border-border/80 p-3 sm:p-4 shadow-sm transition-colors duration-150 motion-reduce:transition-none w-full text-left',
    onClick && 'hover:shadow-md hover:border-brand/30 hover:bg-surface-2/60 cursor-pointer focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/50',
  );

  const body = (
    <>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-caption font-medium text-muted leading-snug line-clamp-2">{title}</p>
          <p className="mt-1 break-words fortuna-metric tabular-nums sm:mt-1.5">{value}</p>
        </div>
        <div className={clsx('p-2.5 rounded-lg shrink-0', color ?? toneClasses[tone])}>{icon}</div>
      </div>
      {trend !== undefined && (
        <div className="mt-3 flex items-center text-caption md:text-body">
          <span
            className={clsx(
              'flex items-center font-medium',
              trendDirection === 'up' && 'text-success',
              trendDirection === 'down' && 'text-critical',
              trendDirection === 'neutral' && 'text-muted',
            )}
          >
            {trendDirection === 'up' && <ArrowUp className="w-3 h-3 mr-1" aria-hidden />}
            {trendDirection === 'down' && <ArrowDown className="w-3 h-3 mr-1" aria-hidden />}
            {trendDirection === 'neutral' && <Minus className="w-3 h-3 mr-1" aria-hidden />}
            {Math.abs(trend)}%
          </span>
          <span className="text-muted-2 ml-2">{trendLabel}</span>
        </div>
      )}
      {trend === undefined && subtitle ? (
        <p className="mt-1.5 text-caption md:text-body text-muted-2">{subtitle}</p>
      ) : null}
    </>
  );

  if (onClick) {
    return (
      <button type="button" className={shellClass} onClick={onClick} aria-label={ariaLabel ?? `${title}: ${value}`}>
        {body}
      </button>
    );
  }

  return <div className={shellClass}>{body}</div>;
};
