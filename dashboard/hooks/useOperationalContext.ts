import { useMemo } from 'react';
import { useClusterStore } from '../store/clusterStore';
import { usePermUser } from './usePermUser';
import { usePersona } from './usePersona';
import { buildOwnershipContext, type OwnershipContext } from '../lib/ownershipContext';
import { DEFAULT_TELEMETRY_CONTEXT, type TelemetryContext } from '../lib/telemetryContext';
import { useClusters } from './useClusters';
import { can, P } from '../lib/permissions';

/** @deprecated Prefer useOperationalContextGraph for unified cognition. */
export function useOperationalContext(telemetryOverride?: Partial<TelemetryContext>): {
  user: ReturnType<typeof usePermUser>;
  personaId: ReturnType<typeof usePersona>['id'];
  profile: ReturnType<typeof usePersona>['profile'];
  ownership: OwnershipContext;
  telemetry: TelemetryContext;
} {
  const user = usePermUser();
  const { id: personaId, profile } = usePersona();
  const { clusters } = useClusters();
  const selectedClusterId = useClusterStore((s) => s.selectedClusterId);

  const ownership = useMemo(
    () => buildOwnershipContext(user, clusters, selectedClusterId),
    [user, clusters, selectedClusterId],
  );

  const telemetry = useMemo<TelemetryContext>(() => {
    const base = { ...DEFAULT_TELEMETRY_CONTEXT, ...telemetryOverride };
    base.graphPolicyLimited =
      telemetryOverride?.graphPolicyLimited ??
      (user != null && can(user, P.graphReadSummary) && !can(user, P.graphReadPaths));
    return base;
  }, [telemetryOverride, user]);

  return { user, personaId, profile, ownership, telemetry };
}
