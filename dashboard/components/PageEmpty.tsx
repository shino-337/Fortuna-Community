import React from 'react';
import { Inbox } from 'lucide-react';

interface PageEmptyProps {
  title?: string;
  description?: string;
  className?: string;
  icon?: React.ReactNode;
}

/** Unified empty state: icon + title + description. */
export const PageEmpty: React.FC<PageEmptyProps> = ({
  title = 'No data',
  description,
  className = '',
  icon,
}) => (
  <div
    className={`flex flex-col items-center justify-center gap-3 py-12 px-4 text-center ${className}`}
  >
    {icon ?? <Inbox className="w-12 h-12 text-muted-2 opacity-60" aria-hidden />}
    <p className="text-sm font-medium text-muted">{title}</p>
    {description && <p className="text-xs text-muted-2 max-w-sm">{description}</p>}
  </div>
);
