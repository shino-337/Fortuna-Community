import React from 'react';
import { TrendingUp } from 'lucide-react';
import { useIncidentMode } from '../hooks/useIncidentMode';

/** Status bar shown when an incident is escalated — indicates containment and runtime paths are prioritized. */
export const EscalationStatusBar: React.FC = () => {
  const { context, active } = useIncidentMode();
  if (!active || !context.escalated) return null;

  return (
    <div className="flex items-center gap-2 text-caption text-amber-200/90 px-1">
      <TrendingUp className="w-3.5 h-3.5 shrink-0" aria-hidden />
      <span>Escalation active — containment and runtime paths are prioritized.</span>
    </div>
  );
};
