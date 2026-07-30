import type { GroupedScenario } from './attackPathNarrative';

export type AttackPathConfidenceLane = 'confirmed' | 'probable' | 'theoretical';

/** Returns the confidence lane for an attack path scenario. */
export function attackPathConfidenceLane(scenario: GroupedScenario): AttackPathConfidenceLane {
  if (scenario.confidence === 'low' || scenario.maxStrength < 0.12) return 'theoretical';
  if (scenario.confidence === 'high') return 'confirmed';
  return 'probable';
}

/** Counts scenarios by confidence lane. */
export function countAttackPathLanes(scenarios: GroupedScenario[]): Record<AttackPathConfidenceLane, number> {
  const counts: Record<AttackPathConfidenceLane, number> = {
    confirmed: 0,
    probable: 0,
    theoretical: 0,
  };
  for (const scenario of scenarios) {
    counts[attackPathConfidenceLane(scenario)] += 1;
  }
  return counts;
}

/** Human-readable labels for confidence lanes. */
export const ATTACK_PATH_LANE_LABELS: Record<AttackPathConfidenceLane, string> = {
  confirmed: 'Confirmed',
  probable: 'Probable',
  theoretical: 'Theoretical',
};
