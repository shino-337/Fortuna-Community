import { useMemo } from 'react';
import { useLocation } from 'react-router-dom';
import { usePermUser } from './usePermUser';
import {
  resolvePersona,
  workflowPhaseFromPath,
  workflowPhaseLabel,
  type PersonaId,
  type PersonaProfile,
  type WorkflowPhase,
} from '../lib/persona';
import { getDashboardComposition, type DashboardComposition } from '../lib/dashboardComposition';
import { buildPersonaNavigation, type PersonaNavSection } from '../lib/personaNavigation';
import { useClusters } from './useClusters';
import { useClusterStore } from '../store/clusterStore';
import { buildOwnershipContext } from '../lib/ownershipContext';
import { DEFAULT_TELEMETRY_CONTEXT } from '../lib/telemetryContext';
import { can, P } from '../lib/permissions';

export function usePersona(): {
  id: PersonaId;
  profile: PersonaProfile;
  navigation: PersonaNavSection[];
  dashboard: DashboardComposition;
  workflowPhase: WorkflowPhase;
  workflowLabel: string;
} {
  const user = usePermUser();
  const location = useLocation();
  const { clusters } = useClusters();
  const selectedClusterId = useClusterStore((s) => s.selectedClusterId);

  return useMemo(() => {
    const { id, profile } = resolvePersona(user);
    const workflowPhase = workflowPhaseFromPath(location.pathname);
    const ownership = buildOwnershipContext(user, clusters, selectedClusterId);
    const telemetry = {
      ...DEFAULT_TELEMETRY_CONTEXT,
      graphPolicyLimited:
        user != null && can(user, P.graphReadSummary) && !can(user, P.graphReadPaths),
    };
    return {
      id,
      profile,
      navigation: buildPersonaNavigation(id),
      dashboard: getDashboardComposition(id, user, ownership, telemetry),
      workflowPhase,
      workflowLabel: workflowPhaseLabel(workflowPhase),
    };
  }, [user, location.pathname, clusters, selectedClusterId]);
}
