import type { GroupedScenario } from './attackPathNarrative';

/**
 * How much of a scenario is backed by evidence: observed (a step matched runtime events),
 * inferred (built from configuration only) or theoretical (weak, low-confidence model).
 */
export type AttackPathConfidenceLane = 'observed' | 'inferred' | 'theoretical';

/** True when any step of any variant matched runtime events (server-side, 7-day window). */
export function scenarioRuntimeObserved(scenario: GroupedScenario): boolean {
  for (const chain of Array.isArray(scenario.variants) ? scenario.variants : []) {
    for (const step of Array.isArray(chain.steps) ? chain.steps : []) {
      if (step?.runtime_observed) return true;
    }
  }
  return false;
}

/** Returns the evidence lane for an attack path scenario. */
export function attackPathConfidenceLane(scenario: GroupedScenario): AttackPathConfidenceLane {
  if (scenarioRuntimeObserved(scenario)) return 'observed';
  if (scenario.confidence === 'low' || scenario.maxStrength < 0.12) return 'theoretical';
  return 'inferred';
}

/** Counts scenarios by evidence lane. */
export function countAttackPathLanes(scenarios: GroupedScenario[]): Record<AttackPathConfidenceLane, number> {
  const counts: Record<AttackPathConfidenceLane, number> = {
    observed: 0,
    inferred: 0,
    theoretical: 0,
  };
  for (const scenario of scenarios) {
    counts[attackPathConfidenceLane(scenario)] += 1;
  }
  return counts;
}

/** Human-readable labels for evidence lanes. */
export const ATTACK_PATH_LANE_LABELS: Record<AttackPathConfidenceLane, string> = {
  observed: 'Observed',
  inferred: 'Inferred',
  theoretical: 'Theoretical',
};

export const ATTACK_PATH_LANE_HINTS: Record<AttackPathConfidenceLane, string> = {
  observed: 'A step of this path matched runtime events in the last 7 days',
  inferred: 'Built from configuration; no step has been seen at runtime',
  theoretical: 'Weak or low-confidence path; no step has been seen at runtime',
};
