import { useMemo } from 'react';
import type { GraphTrustContext } from '../lib/graphTrustSemantics';
import type { GraphSemanticMode } from '../lib/persona';
import { useOperationalContext } from './useOperationalContext';
import { useFeatureVisibility } from './useVisibility';
import { useIncidentMode } from './useIncidentMode';
import { useOperationalMaterialization } from './useOperationalMaterialization';
import { businessContextFromScope } from '../lib/businessContext';

/**
 * Hook that builds the graph trust context for attack paths or network activity views.
 * @param feature - Which view to build: 'attack_paths' (default) or 'network_activity'
 * @param semanticModeOverride - Optional override for the graph semantic mode
 * @param opts.clustered - Whether to return clustered node sets
 * @param opts.runtimeConfirmedNodeIds - Node IDs confirmed at runtime (used for trust scoring)
 */
export function useGraphTrustContext(
  feature: 'attack_paths' | 'network_activity' = 'attack_paths',
  semanticModeOverride?: GraphSemanticMode,
  opts?: { clustered?: boolean; runtimeConfirmedNodeIds?: string[] },
): GraphTrustContext {
  const { ownership, telemetry } = useOperationalContext();
  const visibility = useFeatureVisibility(feature);
  const { graphSemanticMode: incidentGraphMode } = useIncidentMode();
  const { graphMode: materializedGraphMode } = useOperationalMaterialization();
  const business = useMemo(
    () => businessContextFromScope(ownership.operationalScope),
    [ownership.operationalScope],
  );

  return useMemo(
    () => ({
      semanticMode: semanticModeOverride ?? incidentGraphMode ?? materializedGraphMode,
      visibilityState: visibility.semanticState,
      telemetry,
      clustered: opts?.clustered,
      crownJewelIds: new Set(business.crownJewels),
      runtimeConfirmedNodeIds: opts?.runtimeConfirmedNodeIds
        ? new Set(opts.runtimeConfirmedNodeIds)
        : undefined,
    }),
    [
      semanticModeOverride,
      incidentGraphMode,
      materializedGraphMode,
      visibility.semanticState,
      telemetry,
      opts?.clustered,
      opts?.runtimeConfirmedNodeIds,
      business.crownJewels,
    ],
  );
}
