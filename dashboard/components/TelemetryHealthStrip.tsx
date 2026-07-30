import React from 'react';
import { Radio } from 'lucide-react';
import { useOperationalContextGraph } from '../hooks/useOperationalContextGraph';

export const TelemetryHealthStrip: React.FC = () => {
  const { telemetryHealth } = useOperationalContextGraph();

  if (!telemetryHealth.overallDegraded) return null;

  return (
    <div className="mb-3 rounded-lg border border-amber-500/25 bg-amber-500/10 px-3 py-2 text-caption" role="note">
      <p className="font-semibold text-amber-100 flex items-center gap-2">
        <Radio className="w-4 h-4 shrink-0" />
        Telemetry dependency impact
      </p>
      <p className="text-muted mt-1">{telemetryHealth.graphConfidenceImpact}</p>
      <ul className="mt-2 text-meta text-muted-2 space-y-0.5">
        {telemetryHealth.pipelines
          .filter((p) => p.healthy === false)
          .map((p) => (
            <li key={p.pipeline}>
              {p.label} — affects {p.impacts.join(', ')}
            </li>
          ))}
      </ul>
    </div>
  );
};
