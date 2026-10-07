import React from 'react';
import { fullTime, relativeTime } from '../lib/time';

/** Relative time ("3 h ago") with the full date and time on hover. */
export const When: React.FC<{ iso?: string | null; className?: string }> = ({ iso, className }) => (
  <time dateTime={iso ?? undefined} title={fullTime(iso)} className={`whitespace-nowrap ${className ?? ''}`}>
    {relativeTime(iso)}
  </time>
);
