import React from 'react';
import clsx from 'clsx';
import { RefreshCw } from 'lucide-react';
import { formatDateTime } from '../lib/display';

interface DataFreshnessProps {
  updatedAt?: Date | null;
  loading?: boolean;
  error?: string | null;
  staleAfterMs?: number;
  className?: string;
}

export const DataFreshness: React.FC<DataFreshnessProps> = ({
  updatedAt,
  loading,
  error,
  staleAfterMs,
  className,
}) => {
  const isStale = Boolean(updatedAt && staleAfterMs && Date.now() - updatedAt.getTime() > staleAfterMs);
  const label = error
    ? error
    : loading
      ? 'Refreshing data...'
      : updatedAt
        ? `${isStale ? 'Stale' : 'Updated'} ${formatDateTime(updatedAt.toISOString())}`
        : 'Not refreshed yet';

  return (
    <p
      className={clsx(
        'inline-flex min-h-8 items-center gap-2 rounded-lg border border-border bg-base/40 px-3 py-1.5 text-caption text-muted',
        isStale && !error && 'border-warning/30 bg-warning/10 text-warning',
        error && 'border-warning/30 bg-warning/10 text-warning',
        className,
      )}
      role={error || isStale ? 'status' : undefined}
      aria-live="polite"
    >
      {loading ? <RefreshCw className="h-3.5 w-3.5 animate-spin motion-reduce:animate-none" aria-hidden /> : null}
      {label}
    </p>
  );
};
