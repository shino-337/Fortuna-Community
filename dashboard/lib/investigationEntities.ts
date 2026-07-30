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
      severity: String(insight.severityHint ?? insight.severity ?? ''),
      riskLevel: String(insight.finalLevel ?? ''),
      namespace: String(primaryResource?.namespace ?? ''),
      pod: String(primaryResource?.kind === 'Pod' ? primaryResource.name ?? '' : ''),
    },
  };
}

export function attackPathInvestigationEntity(
  scenario: GroupedScenario,
  podUid?: string | null,
): Omit<InvestigationEntity, 'id' | 'pinnedAt'> {
  const qs = podUid ? `?podUid=${encodeURIComponent(podUid)}` : '';
  return {
    type: 'attack_path',
    label: scenario.headline || scenario.type || 'Attack scenario',
    href: `#/attack-paths${qs}`,
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
}): Omit<InvestigationEntity, 'id' | 'pinnedAt'> {
  return {
    type: 'pod',
    label: `${pod.namespace ?? 'default'}/${pod.name}`,
    href: `#/resources/pods/uid/${encodeURIComponent(pod.uid)}`,
    meta: { uid: pod.uid },
  };
}
