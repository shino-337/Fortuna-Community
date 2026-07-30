import React from 'react';
import { BatteryWarning } from 'lucide-react';
import { useOperationalContextGraph } from '../hooks/useOperationalContextGraph';

/** Strip showing responder fatigue — burnout risk and queue pressure. */
export const OperationalFatigueStrip: React.FC = () => {
  const { fatigue } = useOperationalContextGraph();

  if (fatigue.level === 'low') return null;

  return (
    <div
      className={`mb-3 rounded-lg border px-3 py-2 text-caption ${
        fatigue.level === 'critical'
          ? 'border-rose-500/35 bg-rose-500/10'
          : fatigue.level === 'high'
            ? 'border-amber-500/30 bg-amber-500/10'
            : 'border-border/80 bg-surface/40'
      }`}
      role="status"
    >
      <p className="font-semibold text-text flex items-center gap-2">
        <BatteryWarning className="w-4 h-4 shrink-0" />
        Responder fatigue: {fatigue.level}
      </p>
      <p className="text-muted mt-1">{fatigue.burnoutHeuristic}</p>
      <p className="text-meta text-muted-2 mt-1">
        Overload risk {Math.round(fatigue.overloadRisk * 100)}% · Queue pressure{' '}
        {Math.round(fatigue.queuePressure * 100)}%
      </p>
      {fatigue.recommendations.length > 0 ? (
        <ul className="mt-2 text-meta text-muted list-disc pl-4">
          {fatigue.recommendations.slice(0, 2).map((r) => (
            <li key={r}>{r}</li>
          ))}
        </ul>
      ) : null}
    </div>
  );
};
