import React from 'react';
import { Loader2 } from 'lucide-react';

interface PageLoadingProps {
  message?: string;
  className?: string;
}

/** Unified loading state: spinner + message. Use for full-page or section loading. */
export const PageLoading: React.FC<PageLoadingProps> = ({
  message = 'Loading…',
  className = '',
}) => (
  <div
    className={`flex flex-col items-center justify-center gap-4 py-12 px-4 ${className}`}
    role="status"
    aria-live="polite"
  >
    <Loader2 className="w-10 h-10 text-brand animate-spin" aria-hidden />
    <p className="text-sm text-muted">{message}</p>
  </div>
);
