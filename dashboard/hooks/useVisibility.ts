import { useMemo } from 'react';
import type { FeatureId } from '../lib/featureRegistry';
import { FEATURE_REGISTRY } from '../lib/featureRegistry';
import {
  evaluateVisibility,
  isNavVisible,
  type VisibilityInput,
  type VisibilityResult,
} from '../lib/visibilityEngine';
import { useOperationalContext } from './useOperationalContext';
import type { TelemetryContext } from '../lib/telemetryContext';

export function useFeatureVisibility(
  feature: FeatureId,
  opts?: {
    dataEmpty?: boolean;
    ownershipEmpty?: boolean;
    telemetry?: Partial<TelemetryContext>;
  },
): VisibilityResult {
  const { user, personaId, ownership, telemetry: baseTelemetry } = useOperationalContext(opts?.telemetry);
  return useMemo(
    () =>
      evaluateVisibility({
        feature,
        user,
        personaId,
        ownership,
        telemetry: { ...baseTelemetry, ...opts?.telemetry },
        dataEmpty: opts?.dataEmpty,
        ownershipEmpty: opts?.ownershipEmpty,
      }),
    [
      feature,
      user,
      personaId,
      ownership,
      baseTelemetry,
      opts?.dataEmpty,
      opts?.ownershipEmpty,
      opts?.telemetry,
    ],
  );
}

export function useVisibilityMap(): Record<FeatureId, VisibilityResult> {
  const ctx = useOperationalContext();
  return useMemo(() => {
    const map = {} as Record<FeatureId, VisibilityResult>;
    (Object.keys(FEATURE_REGISTRY) as FeatureId[]).forEach((id) => {
      map[id] = evaluateVisibility({
        feature: id,
        user: ctx.user,
        personaId: ctx.personaId,
        ownership: ctx.ownership,
        telemetry: ctx.telemetry,
      });
    });
    return map;
  }, [ctx]);
}

export function useNavFeatureVisibility(feature: FeatureId): boolean {
  const { user, personaId, ownership, telemetry } = useOperationalContext();
  return useMemo(
    () => isNavVisible({ feature, user, personaId, ownership, telemetry }),
    [feature, user, personaId, ownership, telemetry],
  );
}
