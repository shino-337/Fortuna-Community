import React from 'react';
import { Construction } from 'lucide-react';

interface ComingSoonProps {
  /** Feature name (e.g. "Attack Analysis") */
  title?: string;
  /** Short description of what will be available */
  description?: string;
  className?: string;
}

/** Placeholder for features not yet implemented. */
export const ComingSoon: React.FC<ComingSoonProps> = ({
  title = 'Coming soon',
  description = 'This feature is under development and will be available in a future release.',
  className = '',
}) => (
  <div
    className={`flex flex-col items-center justify-center gap-4 py-16 px-6 text-center rounded-xl border border-border bg-surface/50 ${className}`}
  >
    <div className="w-14 h-14 rounded-xl bg-brand/10 border border-brand/30 flex items-center justify-center">
      <Construction className="w-7 h-7 text-brand" aria-hidden />
    </div>
    <h2 className="text-section-title text-text">{title}</h2>
    <p className="text-body text-muted max-w-md">{description}</p>
  </div>
);
