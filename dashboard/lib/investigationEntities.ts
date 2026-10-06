import type { GroupedScenario } from './attackPathNarrative';
import type { Insight } from '../types';
import type { InvestigationEntity } from '../store/investigationStore';

export function findingInvestigationEntity(insight: Insight): Omit<InvestigationEntity, 'id' | 'pinnedAt'> {
  const label = insight.title?.trim() || insight.id;
  const primaryResource = insight.affectedResources?.[0];
  return {
    type: 'finding',
    label,
    href: `#/risks/${encodeURIComponent(insight.id)}`,
    meta: {
      insightId: String(insight.id),
      severity: String(insight.severityHint ?? insight.severity ?? ''),
      riskLevel: String(insight.finalLevel ?? ''),
      namespace: String(primaryResource?.namespace ?? ''),
      pod: String(primaryResource?.kind === 'Pod' ? primaryResource.name ?? '' : ''),
    },
  };
}

/** A scenario pinned to a case; the link reopens that path in its cluster. */
export function attackPathInvestigationEntity(
  scenario: GroupedScenario,
  where: { podUid?: string | null; clusterId?: string | null; pathId?: string | null } = {},
): Omit<InvestigationEntity, 'id' | 'pinnedAt'> {
  const params = new URLSearchParams();
  if (where.podUid) params.set('podUid', where.podUid);
  if (where.clusterId) params.set('clusterId', where.clusterId);
  if (where.pathId) params.set('path', where.pathId);
  const qs = params.toString();
  return {
    type: 'attack_path',
    label: scenario.headline || scenario.type || 'Attack scenario',
    href: `#/attack-paths${qs ? `?${qs}` : ''}`,
    meta: {
      confidence: scenario.confidence,
      variants: String(scenario.variants.length),
      maxStrength: scenario.maxStrength.toFixed(2),
    },
  };
}

export function podInvestigationEntity(pod: {
  uid: string;
  name: string;
  namespace?: string;
  clusterId?: string;
}): Omit<InvestigationEntity, 'id' | 'pinnedAt'> {
  const clusterQuery = pod.clusterId ? `?clusterId=${encodeURIComponent(pod.clusterId)}` : '';
  return {
    type: 'pod',
    label: `${pod.namespace ?? 'default'}/${pod.name}`,
    href: `#/resources/pods/uid/${encodeURIComponent(pod.uid)}${clusterQuery}`,
    meta: { uid: pod.uid, clusterId: pod.clusterId ?? '' },
  };
}
