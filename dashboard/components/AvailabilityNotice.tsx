import React from 'react';
import { AlertTriangle, RefreshCw, Wrench } from 'lucide-react';
import type { AvailabilityIssue } from '../lib/api';
import { Button } from './ui/Button';

export const AvailabilityNotice: React.FC<{
  issue: AvailabilityIssue;
  onRetry?: () => void;
  className?: string;
}> = ({ issue, onRetry, className = '' }) => {
  const retryable = issue.retryable;
  return (
    <div
      role={retryable ? 'status' : 'alert'}
      className={`rounded-lg border px-4 py-3 ${
        retryable
          ? 'border-amber-500/35 bg-amber-500/10 text-amber-100'
          : 'border-red-500/35 bg-red-500/10 text-red-100'
      } ${className}`}
    >
      <div className="flex items-start gap-3">
        {retryable ? (
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-300" aria-hidden />
        ) : (
          <Wrench className="mt-0.5 h-4 w-4 shrink-0 text-red-300" aria-hidden />
        )}
        <div className="min-w-0 flex-1">
          <p className="text-body font-semibold">{issue.title}</p>
          <p className="mt-1 text-caption opacity-90">{issue.description}</p>
          {issue.code ? (
            <p className="mt-1 font-mono text-meta opacity-70">Code: {issue.code}</p>
          ) : null}
        </div>
        {retryable && onRetry ? (
          <Button variant="secondary" size="sm" onClick={onRetry}>
            <RefreshCw className="mr-1.5 h-3.5 w-3.5" aria-hidden /> Retry
          </Button>
        ) : null}
      </div>
    </div>
  );
};
