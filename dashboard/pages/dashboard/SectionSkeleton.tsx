import React from 'react';
import clsx from 'clsx';

export const SectionSkeleton: React.FC<{ className?: string; rows?: number }> = ({
  className,
  rows = 3,
}) => (
  <div
    className={clsx('animate-pulse space-y-3 rounded-xl border border-border/60 bg-surface/20 p-4', className)}
    aria-hidden
  >
    <div className="h-4 w-1/3 rounded bg-surface-2" />
    <div className="grid grid-cols-1 gap-3 min-[420px]:grid-cols-2 lg:grid-cols-4">
      {[0, 1, 2, 3].map((i) => (
        <div key={i} className="h-16 rounded-lg bg-surface-2/80" />
      ))}
    </div>
    {Array.from({ length: rows }).map((_, i) => (
      <div key={i} className="h-10 rounded-lg bg-surface-2/60" />
    ))}
  </div>
);
