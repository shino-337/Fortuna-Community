import React from 'react';
import clsx from 'clsx';
import {
  ATTACK_PATH_LANE_LABELS,
  type AttackPathConfidenceLane,
  countAttackPathLanes,
} from '../lib/attackPathConfidence';
import type { GroupedScenario } from '../lib/attackPathNarrative';

const LANE_STYLES: Record<AttackPathConfidenceLane, string> = {
  confirmed: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-200',
  probable: 'border-amber-500/40 bg-amber-500/10 text-amber-100',
  theoretical: 'border-slate-500/40 bg-slate-500/10 text-slate-200',
};

/** Props for the attack path confidence lanes component. */
interface AttackPathConfidenceLanesProps {
  /** Ranked scenarios sorted by maxStrength descending */
  scenarios: GroupedScenario[];
  /** Currently selected lane filter ('all' or one of confirmed/probable/theoretical) */
  activeLane: AttackPathConfidenceLane | 'all';
  /** Callback when user selects a different confidence lane */
  onLaneChange: (lane: AttackPathConfidenceLane | 'all') => void;
  /** Number of scenarios hidden by the currently applied filter, if any */
  hiddenByFilter?: number;
}

/**
 * Renders a collapsible panel showing scenario counts grouped by attack path confidence lane.
 */
export const AttackPathConfidenceLanes: React.FC<AttackPathConfidenceLanesProps> = ({ scenarios, activeLane, onLaneChange, hiddenByFilter = 0 }) => {
  const counts = countAttackPathLanes(scenarios);

  return (
    <div className="rounded-lg border border-border/80 bg-surface/30 p-3 space-y-2">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-caption font-semibold text-text">Scenario confidence lanes</p>
        {hiddenByFilter > 0 ? (
          <span className="text-caption text-amber-200 font-medium">
            {hiddenByFilter} hidden by active filter
          </span>
        ) : null}
      </div>
      <div className="flex flex-wrap gap-2" role="group" aria-label="Filter attack scenarios by confidence lane">
        <button
          type="button"
          onClick={() => onLaneChange('all')}
          className={clsx(
            'rounded-md border px-2.5 py-1 text-caption font-semibold transition-colors',
            activeLane === 'all' ? 'border-brand bg-brand/15 text-brand' : 'border-border text-muted hover:text-text',
          )}
          aria-pressed={activeLane === 'all'}
        >
          All ({scenarios.length})
        </button>
        {(Object.keys(ATTACK_PATH_LANE_LABELS) as AttackPathConfidenceLane[]).map((lane) => (
          <button
            key={lane}
            type="button"
            onClick={() => onLaneChange(lane)}
            className={clsx(
              'rounded-md border px-2.5 py-1 text-caption font-semibold transition-colors',
              LANE_STYLES[lane],
              activeLane === lane && 'ring-1 ring-brand/50',
            )}
            aria-pressed={activeLane === lane}
          >
            {ATTACK_PATH_LANE_LABELS[lane]} ({counts[lane]})
          </button>
        ))}
      </div>
      <p className="text-meta text-muted leading-snug">
        Theoretical and low-strength paths stay visible by default so analysts can validate assumptions before containment decisions.
      </p>
    </div>
  );
};
