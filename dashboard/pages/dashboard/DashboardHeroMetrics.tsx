import React from 'react';
import { ShieldAlert, Target, Boxes } from 'lucide-react';
import clsx from 'clsx';
import { getSeverityTextClass } from '../../lib/severity';
import { ScopedMetric } from '../../design-system/components/ScopedMetric';
import { useOperationalContext } from '../../hooks/useOperationalContext';
import type { MetricTrust } from '../../lib/metricSemantics';

export interface DashboardHeroMetricsProps {
  criticalCount: number;
  attackPathCount: number;
  affectedWorkloads: number;
  clusterName?: string | null;
  metricTrust?: MetricTrust;
  onCriticalClick?: () => void;
  onPathsClick?: () => void;
  onWorkloadsClick?: () => void;
}

export const DashboardHeroMetrics: React.FC<DashboardHeroMetricsProps> = ({
  criticalCount,
  attackPathCount,
  affectedWorkloads,
  clusterName,
  metricTrust = 'exact',
  onCriticalClick,
  onPathsClick,
  onWorkloadsClick,
}) => {
  const { ownership, telemetry } = useOperationalContext();

  const tiles = [
    {
      label: 'Critical risk findings',
      value: criticalCount,
      icon: <ShieldAlert className="h-6 w-6" aria-hidden />,
      tone: 'text-red-400',
      onClick: onCriticalClick,
    },
    {
      label: 'Attack paths',
      value: attackPathCount,
      icon: <Target className="h-6 w-6" aria-hidden />,
      tone: 'text-orange-400',
      onClick: onPathsClick,
    },
    {
      label: 'Affected workloads',
      value: affectedWorkloads,
      icon: <Boxes className="h-6 w-6" aria-hidden />,
      tone: 'text-rose-400',
      onClick: onWorkloadsClick,
    },
  ];

  return (
    <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
      {tiles.map((t) => (
        <ScopedMetric
          key={t.label}
          label={t.label}
          value={t.value}
          ownership={ownership}
          telemetry={telemetry}
          trust={metricTrust}
          clusterName={clusterName}
          onClick={t.onClick}
          icon={<div className={clsx('rounded-lg bg-surface-2/60 p-2', t.tone)}>{t.icon}</div>}
          valueClassName={getSeverityTextClass(t.label.includes('Critical') ? 'critical' : undefined) || undefined}
        />
      ))}
    </div>
  );
};
