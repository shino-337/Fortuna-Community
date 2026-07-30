import React from 'react';
import { Zap } from 'lucide-react';
import { useIncidentMode } from '../hooks/useIncidentMode';

/** Strip showing runtime-confirmed signals — elevated during incident mode. */
export const RuntimeThreatStrip: React.FC<{ runtimeConfirmedCount?: number }> = ({
  runtimeConfirmedCount = 0,
}) => {
  const { active } = useIncidentMode();
  if (!active && runtimeConfirmedCount === 0) return null;

  return (
    <div className="flex items-center gap-2 rounded-lg border border-emerald-500/25 bg-emerald-500/10 px-3 py-2 text-caption">
      <Zap className="w-4 h-4 text-emerald-300 shrink-0" aria-hidden />
      <span className="text-emerald-100">
        {runtimeConfirmedCount > 0
          ? `${runtimeConfirmedCount} runtime-confirmed signal(s) in scope — prioritize exploitability review.`
          : 'Incident mode: runtime findings elevated in navigation and graphs.'}
      </span>
    </div>
  );
};
