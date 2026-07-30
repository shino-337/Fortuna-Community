import { useEffect, useMemo } from 'react';
import { useTimeWindowStore } from '../store/timeWindowStore';
import { useClusterStore } from '../store/clusterStore';
import { useOperationalMaterialization } from './useOperationalMaterialization';
import { buildDashboardLoadPolicy, type DashboardWidgetId } from '../lib/dashboardComposition';
import { useDashboardData } from '../pages/dashboard/useDashboardData';
import { usePermUser } from './usePermUser';

function widgetsFromSections(sections: string[]): DashboardWidgetId[] {
  const widgets = new Set<DashboardWidgetId>(['persona_strip', 'activity_feed']);
  const has = (k: string) => sections.includes(k);

  if (has('exposure_summary') || has('triage_queue') || has('platform_integrity')) {
    widgets.add('hero_metrics');
    widgets.add('risk_stats');
  }
  if (has('attack_chains') || has('exposure_summary')) {
    widgets.add('entry_points');
  }
  if (has('telemetry_degradation') || has('telemetry_reliability') || has('operational_oversight')) {
    widgets.add('telemetry_health');
  }
  if (has('cluster_posture') || has('platform_integrity')) {
    widgets.add('cluster_health');
  }
  if (has('scoped_trends') || has('governance_exposure')) {
    widgets.add('exposure_trend');
  }
  if (has('attack_chains')) {
    widgets.add('attack_analysis');
  }
  return [...widgets];
}

export function usePersonaDashboardData() {
  const user = usePermUser();
  const selectedClusterId = useClusterStore((s) => s.selectedClusterId);
  const timeWindowMinutes = useTimeWindowStore((s) => s.valueMinutes);
  const sinceMinutes = timeWindowMinutes > 0 ? timeWindowMinutes : undefined;
  const plane = useOperationalMaterialization();

  const loadPolicy = useMemo(
    () => buildDashboardLoadPolicy(user, widgetsFromSections(plane.dashboardSections)),
    [user, plane.dashboardSections],
  );

  const data = useDashboardData(selectedClusterId, sinceMinutes, loadPolicy);

  useEffect(() => {
    void data.refresh();
  }, [data.refresh]);

  return {
    ...data,
    sections: plane.dashboardSections,
    shellVariant: plane.shellVariant,
    identityLabel: plane.identityLabel,
  };
}
