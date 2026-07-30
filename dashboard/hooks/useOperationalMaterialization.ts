import { useMemo } from 'react';
import { usePermUser } from './usePermUser';
import { resolvePersona } from '../lib/persona';
import { useClusters } from './useClusters';
import { useClusterStore } from '../store/clusterStore';
import { buildOwnershipContext } from '../lib/ownershipContext';
import { DEFAULT_TELEMETRY_CONTEXT } from '../lib/telemetryContext';
import { can, P } from '../lib/permissions';
import {
  materializeOperationalPlane,
  type MaterializedOperationalPlane,
} from '../lib/routeMaterialization';
import { buildMissionNavigation, type MissionNavSection } from '../lib/personaMissionNavigation';

export function useOperationalMaterialization(): MaterializedOperationalPlane & {
  navigation: MissionNavSection[];
} {
  const user = usePermUser();
  const { id: personaId } = resolvePersona(user);
  const shouldLoadClusters = personaId !== 'user_admin' && can(user, P.inventoryRead);
  const { clusters } = useClusters({ enabled: shouldLoadClusters });
  const selectedClusterId = useClusterStore((s) => s.selectedClusterId);

  return useMemo(() => {
    const ownership = buildOwnershipContext(user, clusters, selectedClusterId);
    const telemetry = {
      ...DEFAULT_TELEMETRY_CONTEXT,
      graphPolicyLimited:
        user != null && can(user, P.graphReadSummary) && !can(user, P.graphReadPaths),
    };

    const canReadInvestigations = can(user, P.investigationsRead);
    const signals = {
      hasOpenFindings: true,
      hasAttackPaths: true,
      hasAssignedCases: canReadInvestigations,
      hasActiveRemediation: false,
    };

    const plane = materializeOperationalPlane({
      personaId,
      user,
      ownership,
      telemetry,
      signals,
    });

    const navigation = buildMissionNavigation(personaId, plane);

    return { ...plane, navigation };
  }, [user, personaId, clusters, selectedClusterId]);
}
