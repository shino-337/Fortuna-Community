import React, { useMemo } from 'react';
import clsx from 'clsx';
import { ChevronRight, Route } from 'lucide-react';
import type { AttackPath } from '../types';
import type { GroupedScenario } from '../lib/attackPathNarrative';
import { attackPathConfidenceLane, ATTACK_PATH_LANE_LABELS } from '../lib/attackPathConfidence';
import { getSeverityTextClass } from '../lib/severity';

/**
 * Renders a ranked list of attack scenarios with their priority paths.
 */
export const AttackPathPriorityList: React.FC<{
  /** Ranked scenarios sorted by maxStrength descending */
  scenarios: GroupedScenario[];
  /** Paths associated with each scenario's variants */
  paths: AttackPath[];
  /** Currently selected scenario key, or null if none */
  selectedScenarioKey: string | null;
  /** Currently selected path ID within a scenario, or null to collapse */
  selectedPathId: string | null;
  /** Callback when user selects a different scenario */
  onSelectScenario: (key: string) => void;
  /** Callback when user expands/collapses paths for the active scenario */
  onSelectPath: (pathId: string | null) => void;
  /** Opens the full attack path graph view */
  onOpenGraph: () => void;
}> = ({
  scenarios,
  paths,
  selectedScenarioKey,
  selectedPathId,
  onSelectScenario,
  onSelectPath,
  onOpenGraph,
}) => {
  const ranked = useMemo(
    () => [...scenarios].sort((a, b) => b.maxStrength - a.maxStrength),
    [scenarios],
  );

  const pathsForScenario = (scenario: GroupedScenario) => {
    const ids = new Set<string>();
    (Array.isArray(scenario.variants) ? scenario.variants : []).forEach((v) =>
      (Array.isArray(v.paths) ? v.paths : []).forEach((id) => ids.add(id)),
    );
    return (Array.isArray(paths) ? paths : []).filter((p) => p.path_id && ids.has(p.path_id));
  };

  if (ranked.length === 0) return null;

  return (
    <section className="rounded-lg border border-border bg-surface/40 p-4 space-y-3" aria-labelledby="attack-path-priority-heading">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 id="attack-path-priority-heading" className="text-body font-semibold text-text flex items-center gap-2">
            <Route className="w-4 h-4 text-brand" aria-hidden />
            Priority attack chains
          </h2>
          <p className="text-caption text-muted mt-1 max-w-3xl">
            Path-first review: ranked by chain strength. Expand a chain, then open the graph for exact nodes and edges.
          </p>
        </div>
        <button
          type="button"
          onClick={onOpenGraph}
          className="text-caption font-semibold text-brand hover:underline shrink-0"
        >
          Open graph view
        </button>
      </div>

      <ol className="space-y-2 list-none m-0 p-0">
        {ranked.map((scenario, index) => {
          const lane = attackPathConfidenceLane(scenario);
          const active = (selectedScenarioKey ?? ranked[0]?.key) === scenario.key;
          const chainPaths = pathsForScenario(scenario);
          return (
            <li
              key={scenario.key}
              className={clsx(
                'rounded-lg border transition-colors',
                active ? 'border-brand/50 bg-brand/5' : 'border-border bg-base/30',
              )}
            >
              <button
                type="button"
                className="w-full text-left px-3 py-3 flex flex-wrap items-start gap-2"
                onClick={() => onSelectScenario(scenario.key)}
                aria-expanded={active}
              >
                <span className="text-micro font-bold text-muted tabular-nums w-6 shrink-0">#{index + 1}</span>
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2 mb-1">
                    <span className={`text-micro font-semibold uppercase ${getSeverityTextClass(scenario.confidence)}`}>
                      {ATTACK_PATH_LANE_LABELS[lane]}
                    </span>
                    <span className="text-micro text-muted tabular-nums">
                      strength {(scenario.maxStrength * 100).toFixed(0)}%
                    </span>
                    <span className="text-micro text-muted">
                      {chainPaths.length} path{chainPaths.length === 1 ? '' : 's'}
                    </span>
                  </div>
                  <p className="text-body font-medium text-text">{scenario.headline}</p>
                  <p className="text-caption text-muted mt-1 line-clamp-2">{scenario.narrative}</p>
                </div>
                <ChevronRight
                  className={clsx('w-4 h-4 shrink-0 text-muted transition-transform', active && 'rotate-90 text-brand')}
                  aria-hidden
                />
              </button>
              {active && chainPaths.length > 0 ? (
                <ul className="border-t border-border/80 divide-y divide-border/60 max-h-48 overflow-y-auto">
                  {chainPaths.slice(0, 12).map((path) => (
                    <li key={path.path_id}>
                      <button
                        type="button"
                        onClick={() => onSelectPath(path.path_id === selectedPathId ? null : path.path_id ?? null)}
                        className={clsx(
                          'w-full text-left px-4 py-2 text-caption transition-colors',
                          selectedPathId === path.path_id
                            ? 'bg-brand/10 text-text'
                            : 'text-muted hover:bg-surface/80 hover:text-text',
                        )}
                      >
                        <span className="font-mono text-micro text-muted mr-2">{path.path_id}</span>
                        {path.description.slice(0, 140)}
                        {path.description.length > 140 ? '…' : ''}
                      </button>
                    </li>
                  ))}
                  {chainPaths.length > 12 ? (
                    <li className="px-4 py-2 text-meta text-muted">+{chainPaths.length - 12} more paths in this scenario</li>
                  ) : null}
                </ul>
              ) : null}
            </li>
          );
        })}
      </ol>
    </section>
  );
};
