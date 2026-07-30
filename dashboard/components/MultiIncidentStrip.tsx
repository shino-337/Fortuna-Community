import React from 'react';
import { Link } from 'react-router-dom';
import { Users, AlertTriangle } from 'lucide-react';
import { useOperationalContextGraph } from '../hooks/useOperationalContextGraph';

/** Strip showing multi-incident load — when multiple cases compete for attention. */
export const MultiIncidentStrip: React.FC = () => {
  const { multiIncident } = useOperationalContextGraph();

  if (multiIncident.level === 'none' || multiIncident.level === 'low') return null;

  return (
    <div
      className={`mb-3 rounded-lg border px-3 py-2 text-caption ${
        multiIncident.level === 'critical'
          ? 'border-red-500/35 bg-red-500/10'
          : 'border-amber-500/30 bg-amber-500/10'
      }`}
      role="status"
    >
      <p className="font-semibold text-text flex items-center gap-2">
        <Users className="w-4 h-4 shrink-0" />
        Multi-incident load: {multiIncident.level}
      </p>
      <p className="text-muted mt-1">{multiIncident.resourceContentionMessage}</p>
      {multiIncident.competingCases.length > 1 ? (
        <ul className="mt-2 space-y-1">
          {multiIncident.competingCases.slice(0, 4).map((c) => (
            <li key={c.id} className="flex items-center justify-between gap-2">
              <Link to={`/investigation?case=${c.id}`} className="text-brand hover:underline truncate">
                {c.title}
              </Link>
              <span className="text-meta text-muted shrink-0">score {c.priorityScore}</span>
            </li>
          ))}
        </ul>
      ) : null}
      {multiIncident.escalationOverload ? (
        <p className="mt-2 flex items-center gap-1 text-amber-200 text-meta">
          <AlertTriangle className="w-3.5 h-3.5" />
          Escalation overload — multiple SLA breaches
        </p>
      ) : null}
    </div>
  );
};
