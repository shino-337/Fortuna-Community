import React from 'react';
import { AlertTriangle, ArrowRight } from 'lucide-react';
import { Link } from 'react-router-dom';
import { useIncidentMode } from '../hooks/useIncidentMode';

/** Banner shown when an incident is active — provides quick access to the investigation workspace. */
export const IncidentModeBanner: React.FC = () => {
  const { active, label, context } = useIncidentMode();

  if (!active) return null;

  return (
    <div
      className="mb-3 flex flex-wrap items-center justify-between gap-2 rounded-lg border border-red-500/35 bg-red-500/10 px-3 py-2 text-caption"
      role="status"
      aria-live="polite"
    >
      <div className="flex items-center gap-2 min-w-0">
        <AlertTriangle className="w-4 h-4 shrink-0 text-red-300" aria-hidden />
        <span className="font-semibold text-red-100">{label}</span>
        {context.activeCaseTitle ? (
          <span className="text-muted truncate">— {context.activeCaseTitle}</span>
        ) : null}
      </div>
      <Link
        to={context.activeCaseId ? `/investigation?case=${context.activeCaseId}` : '/investigation'}
        className="inline-flex items-center gap-1 text-brand hover:text-brand/90 font-semibold shrink-0"
      >
        Open workspace <ArrowRight className="w-3.5 h-3.5" />
      </Link>
    </div>
  );
};
