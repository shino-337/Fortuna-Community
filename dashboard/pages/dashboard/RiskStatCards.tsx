import React from 'react';
import { ShieldAlert } from 'lucide-react';
import type { InsightsSummary } from '../../types';
import { ScopedMetric } from '../../design-system/components/ScopedMetric';
import { useOperationalContext } from '../../hooks/useOperationalContext';
import type { MetricTrust } from '../../lib/metricSemantics';

export interface RiskStatCardsProps {
  insightsSummary: InsightsSummary | null;
  statsCritical: number;
  clusterName?: string | null;
  metricTrust?: MetricTrust;
}

export const RiskStatCards: React.FC<RiskStatCardsProps> = ({
  insightsSummary,
  statsCritical,
  clusterName,
  metricTrust = 'exact',
}) => {
  const { ownership, telemetry } = useOperationalContext();

  const sev = (key: 'critical' | 'high' | 'medium' | 'low') =>
    Number(insightsSummary?.riskLevelCounts?.[key] ?? insightsSummary?.[key] ?? (key === 'critical' ? statsCritical : 0));

  const levels: Array<{ key: 'critical' | 'high' | 'medium' | 'low'; label: string }> = [
    { key: 'critical', label: 'Critical' },
    { key: 'high', label: 'High' },
    { key: 'medium', label: 'Medium' },
    { key: 'low', label: 'Low' },
  ];

  return (
    <div className="grid grid-cols-1 gap-3 min-[420px]:grid-cols-2 xl:grid-cols-4">
      {levels.map(({ key, label }) => (
        <ScopedMetric
          key={key}
          compact
          label={label}
          value={sev(key)}
          ownership={ownership}
          telemetry={telemetry}
          trust={metricTrust}
          clusterName={clusterName}
          icon={<ShieldAlert className="w-5 h-5" aria-hidden />}
        />
      ))}
    </div>
  );
};
