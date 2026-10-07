import React, { useCallback, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { ChevronRight, CircleDashed, Eye, HelpCircle, Wrench, X } from 'lucide-react';
import clsx from 'clsx';
import type { AttackPath, AttackPathNode, AttackStep } from '../../types';
import {
  buildFixRecommendations,
  scenarioEnds,
  scenarioMaxRisk,
  scenarioPathIds,
  type GroupedScenario,
  type SharedFix,
} from '../../lib/attackPathNarrative';
import {
  ATTACK_PATH_LANE_HINTS,
  ATTACK_PATH_LANE_LABELS,
  attackPathConfidenceLane,
  type AttackPathConfidenceLane,
} from '../../lib/attackPathConfidence';
import { getSeverityBadgeClass, pathRiskLevel, type SeverityLevel } from '../../lib/severity';
import { findingsForResourcePath, networkForPodPath } from '../../lib/entityLinks';
import {
  NODE_KIND_LABEL,
  TARGET_GROUP_LABEL,
  nodeDisplayName,
  nodeKind,
  pathHops,
  targetGroupOf,
  type TargetGroup,
} from '../../lib/attackPathSteps';
import { AttackChainDiagram } from './AttackChainDiagram';

const LANE_ICON: Record<AttackPathConfidenceLane, React.ReactNode> = {
  observed: <Eye className="h-3.5 w-3.5" aria-hidden />,
  inferred: <CircleDashed className="h-3.5 w-3.5" aria-hidden />,
  theoretical: <HelpCircle className="h-3.5 w-3.5" aria-hidden />,
};

/** Evidence in neutral colours, so it never reads as a severity. */
export function ConfidenceTag({ scenario }: { scenario: GroupedScenario }) {
  const lane = attackPathConfidenceLane(scenario);
  return (
    <span className="inline-flex items-center gap-1 text-caption text-muted" title={ATTACK_PATH_LANE_HINTS[lane]}>
      {LANE_ICON[lane]}
      {ATTACK_PATH_LANE_LABELS[lane]}
    </span>
  );
}

export function LevelBadge({ risk }: { risk: number | null }) {
  if (risk === null) return <span className="rounded border border-border px-1.5 py-0.5 text-meta text-muted">No score</span>;
  const level: SeverityLevel = pathRiskLevel(risk);
  return (
    <span className={clsx('inline-flex shrink-0 items-center gap-1 rounded px-1.5 py-0.5 text-meta font-semibold uppercase', getSeverityBadgeClass(level))}>
      {level}
      <span className="font-mono normal-case opacity-90">{risk.toFixed(1)}</span>
    </span>
  );
}

const LEVEL_BAR: Record<SeverityLevel, string> = {
  critical: 'bg-red-500',
  high: 'bg-orange-500',
  medium: 'bg-yellow-400',
  low: 'bg-slate-400',
};

export type RankedFix = SharedFix & { paths: number; maxRisk: number | null; firstScenarioKey: string };

/** Ranks shared fixes by how many paths they cut, then by the highest path level they cut. */
export function rankSharedFixes(fixes: SharedFix[], scenarios: GroupedScenario[], pathById: Map<string, AttackPath>): RankedFix[] {
  const byKey = new Map(scenarios.map((s) => [s.key, s]));
  return fixes
    .map((fix) => {
      const ids = new Set<string>();
      let maxRisk: number | null = null;
      let firstScenarioKey = fix.scenarioKeys[0];
      for (const key of fix.scenarioKeys) {
        const s = byKey.get(key);
        if (!s) continue;
        scenarioPathIds(s).forEach((id) => ids.add(id));
        const risk = scenarioMaxRisk(s, pathById);
        if (risk !== null && (maxRisk === null || risk > maxRisk)) {
          maxRisk = risk;
          firstScenarioKey = key;
        }
      }
      return { ...fix, paths: Math.max(ids.size, fix.scenarioKeys.length), maxRisk, firstScenarioKey };
    })
    .sort((a, b) => b.paths - a.paths || (b.maxRisk ?? -1) - (a.maxRisk ?? -1));
}

/** Fewest fixes that together cut every critical and high scenario (greedy set cover). */
export function fixesToCutUrgent(ranked: RankedFix[], scenarios: GroupedScenario[], pathById: Map<string, AttackPath>): { fixes: number; uncovered: number; urgent: number } {
  const urgent = new Set(
    scenarios
      .filter((s) => {
        const risk = scenarioMaxRisk(s, pathById);
        return risk !== null && risk >= 7;
      })
      .map((s) => s.key),
  );
  const left = new Set(urgent);
  let fixes = 0;
  while (left.size > 0) {
    let best: RankedFix | null = null;
    let bestGain = 0;
    for (const f of ranked) {
      const gain = f.scenarioKeys.filter((k) => left.has(k)).length;
      if (gain > bestGain) {
        best = f;
        bestGain = gain;
      }
    }
    if (!best) break;
    best.scenarioKeys.forEach((k) => left.delete(k));
    fixes += 1;
  }
  return { fixes, uncovered: left.size, urgent: urgent.size };
}

/** Scenario → what it ends at. */
export function scenarioTargetGroup(s: GroupedScenario, pathById: Map<string, AttackPath>): TargetGroup {
  for (const id of scenarioPathIds(s)) {
    const nodes = pathById.get(id)?.nodes ?? [];
    if (nodes.length > 1) return targetGroupOf(nodes[nodes.length - 1]);
  }
  return targetGroupOf(null, s.finalTarget);
}

/** Three numbers that answer "how exposed are we": urgent scenarios, what they reach, fixes needed. */
export const PostureStrip: React.FC<{
  scenarios: GroupedScenario[];
  pathById: Map<string, AttackPath>;
  ranked: RankedFix[];
  targetFilter: TargetGroup | '';
  onTarget: (group: TargetGroup | '') => void;
  onUrgent: () => void;
  onFixes: () => void;
}> = ({ scenarios, pathById, ranked, targetFilter, onTarget, onUrgent, onFixes }) => {
  const targets = useMemo(() => {
    const counts = new Map<TargetGroup, number>();
    for (const s of scenarios) {
      const g = scenarioTargetGroup(s, pathById);
      counts.set(g, (counts.get(g) ?? 0) + 1);
    }
    return [...counts.entries()].sort((a, b) => b[1] - a[1]);
  }, [scenarios, pathById]);
  const cover = useMemo(() => fixesToCutUrgent(ranked, scenarios, pathById), [ranked, scenarios, pathById]);
  const tile = 'flex min-w-0 flex-col gap-1 rounded-xl border border-border bg-surface/45 px-4 py-3 text-left';
  return (
    <section aria-label="Exposure" className="grid grid-cols-1 gap-3 sm:grid-cols-3">
      <button type="button" onClick={onUrgent} className={clsx(tile, 'hover:border-brand')}>
        <span className="text-caption text-muted">Critical or high scenarios</span>
        <span className="text-section-title font-semibold tabular-nums text-text">
          {cover.urgent.toLocaleString()}
          <span className="ml-1 text-body font-normal text-muted">of {scenarios.length.toLocaleString()}</span>
        </span>
      </button>
      <div className={tile}>
        <span className="text-caption text-muted">What they reach</span>
        <span className="flex flex-wrap gap-1.5">
          {targets.map(([group, n]) => (
            <button
              key={group}
              type="button"
              aria-pressed={targetFilter === group}
              onClick={() => onTarget(targetFilter === group ? '' : group)}
              className={clsx(
                'inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-caption',
                targetFilter === group ? 'border-brand bg-brand/15 text-text' : 'border-border text-text hover:border-brand',
              )}
            >
              {TARGET_GROUP_LABEL[group]}
              <span className="tabular-nums text-muted">{n}</span>
            </button>
          ))}
        </span>
      </div>
      <button type="button" onClick={onFixes} className={clsx(tile, 'hover:border-brand')} disabled={cover.urgent === 0}>
        <span className="text-caption text-muted">Fixes that cut every critical or high scenario</span>
        <span className="text-section-title font-semibold tabular-nums text-text">
          {cover.urgent === 0 ? '—' : cover.fixes.toLocaleString()}
          {cover.uncovered > 0 ? (
            <span className="ml-1 text-body font-normal text-muted">+ {cover.uncovered} with no fix proposed</span>
          ) : null}
        </span>
      </button>
    </section>
  );
};

/** Fixes ranked by how many paths each cuts, drawn as bars; choosing one narrows the list to those paths. */
export const ChokePoints: React.FC<{
  ranked: RankedFix[];
  activeFix: string;
  onFilter: (fixLabel: string) => void;
}> = ({ ranked, activeFix, onFilter }) => {
  if (ranked.length === 0) return null;
  const shown = ranked.slice(0, 5);
  const max = Math.max(...shown.map((f) => f.paths), 1);
  return (
    <section id="choke-points" aria-labelledby="choke-points-title" className="scroll-mt-4 rounded-xl border border-border bg-surface/45 p-3">
      <h2 id="choke-points-title" className="mb-1 flex items-center gap-2 text-body font-semibold text-text">
        <Wrench className="h-4 w-4 text-brand" aria-hidden /> Fix one thing, cut several paths
      </h2>
      <p className="mb-2 text-caption text-muted">Bar = paths the fix cuts, coloured by the highest level among them. Choose a fix to see its paths.</p>
      <ol className="flex flex-col gap-1">
        {shown.map((fix) => {
          const level = fix.maxRisk === null ? null : pathRiskLevel(fix.maxRisk);
          const active = activeFix === fix.label;
          return (
            <li key={fix.label}>
              <button
                type="button"
                aria-pressed={active}
                onClick={() => onFilter(active ? '' : fix.label)}
                className={clsx(
                  'grid w-full grid-cols-1 gap-x-4 gap-y-1 rounded-lg px-2 py-2 text-left md:grid-cols-[minmax(0,1fr)_minmax(8rem,16rem)] md:items-center',
                  active ? 'bg-brand/10' : 'hover:bg-surface-2/60',
                )}
              >
                <span className="flex min-w-0 items-start gap-2">
                  <LevelBadge risk={fix.maxRisk} />
                  <span className="min-w-0 text-body text-text">{fix.label}</span>
                </span>
                <span className="flex items-center gap-2">
                  <span className="h-2 flex-1 rounded-full bg-border/40">
                    <span
                      className={clsx('block h-2 rounded-full', level ? LEVEL_BAR[level] : 'bg-muted')}
                      style={{ width: `${Math.max(8, (fix.paths / max) * 100)}%` }}
                    />
                  </span>
                  <span className="w-16 shrink-0 text-right text-caption tabular-nums text-text">
                    {fix.paths} {fix.paths === 1 ? 'path' : 'paths'}
                  </span>
                </span>
              </button>
            </li>
          );
        })}
      </ol>
    </section>
  );
};

/** Shown above the list while a fix filter is on. */
export function FixFilterChip({ label, count, onClear }: { label: string; count: number; onClear: () => void }) {
  return (
    <div className="flex items-start gap-2 rounded-lg border border-brand/40 bg-brand/5 px-3 py-2 text-caption text-text">
      <span className="min-w-0 flex-1">
        {count} {count === 1 ? 'scenario' : 'scenarios'} cut by: <span className="font-semibold">{label}</span>
      </span>
      <button type="button" onClick={onClear} aria-label="Clear fix filter" className="shrink-0 text-muted hover:text-text">
        <X className="h-4 w-4" aria-hidden />
      </button>
    </div>
  );
}

/** One row per scenario: level, entry → target, confidence. */
export const AttackPathList: React.FC<{
  scenarios: GroupedScenario[];
  pathById: Map<string, AttackPath>;
  selectedKey: string | null;
  onSelect: (key: string) => void;
}> = ({ scenarios, pathById, selectedKey, onSelect }) => (
  <ul aria-label="Attack paths" className="divide-y divide-border/60 overflow-hidden rounded-xl border border-border bg-surface/30">
    {scenarios.map((s) => {
      const { entry, target } = scenarioEnds(s, pathById);
      const selected = s.key === selectedKey;
      const variants = s.variants.length;
      return (
        <li key={s.key}>
          <button
            type="button"
            aria-current={selected ? 'true' : undefined}
            onClick={() => onSelect(s.key)}
            className={clsx(
              'flex w-full items-start gap-3 px-3 py-2.5 text-left transition-colors',
              selected ? 'bg-brand/10' : 'hover:bg-surface-2/60',
            )}
          >
            <LevelBadge risk={scenarioMaxRisk(s, pathById)} />
            <span className="min-w-0 flex-1">
              <span className="block truncate text-body font-medium text-text">{s.headline}</span>
              <span className="block truncate text-caption text-muted">
                {nodeDisplayName(entry) || 'Unknown entry'} → {target || 'target'}
              </span>
              <span className="mt-0.5 flex flex-wrap items-center gap-x-3 text-caption text-muted">
                <ConfidenceTag scenario={s} />
                {variants > 1 ? <span>{variants} entry points</span> : null}
              </span>
            </span>
            <ChevronRight className={clsx('mt-1 h-4 w-4 shrink-0', selected ? 'text-brand' : 'text-muted')} aria-hidden />
          </button>
        </li>
      );
    })}
  </ul>
);

function Section({ title, children, action }: { title: string; children: React.ReactNode; action?: React.ReactNode }) {
  return (
    <section className="flex flex-col gap-2">
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-meta font-semibold uppercase tracking-wider text-muted">{title}</h3>
        {action}
      </div>
      {children}
    </section>
  );
}

/** The selected scenario: how to break it, the chain, every hop on it, and the evidence. */
export const AttackPathDetail: React.FC<{
  scenario: GroupedScenario;
  pathById: Map<string, AttackPath>;
  clusterId: string | null;
  allowedRoutes: string[];
  hrefFor: (node: AttackPathNode) => string | null;
  /** The force-directed graph, kept as an alternative view. */
  graph: React.ReactNode;
  addToCase: React.ReactNode;
  highlightedStepIndex: number | null;
  onStepClick: (index: number) => void;
  extraEvidence?: React.ReactNode;
}> = ({ scenario, pathById, clusterId, allowedRoutes, hrefFor, graph, addToCase, highlightedStepIndex, onStepClick, extraEvidence }) => {
  const [view, setView] = useState<'chain' | 'graph'>('chain');
  const [pathIndex, setPathIndex] = useState(0);
  const chain = scenario.representativeChain;
  const paths = useMemo(
    () =>
      scenarioPathIds(scenario)
        .map((id) => pathById.get(id))
        .filter((p): p is AttackPath => Boolean(p))
        .sort((a, b) => b.total_risk - a.total_risk),
    [scenario, pathById],
  );
  const steps = useMemo(() => (Array.isArray(chain.steps) ? chain.steps : []), [chain]);
  const fixes = buildFixRecommendations(chain);
  const evidence = Array.isArray(chain.evidence) ? chain.evidence : [];
  const assumptions = Array.isArray(chain.assumptions) ? chain.assumptions : [];
  const activePath = paths[Math.min(pathIndex, paths.length - 1)];
  const stepsFor = useCallback(
    (path: AttackPath): AttackStep[] => {
      const own = scenario.variants.find((v) => Array.isArray(v.paths) && v.paths.includes(path.path_id));
      return Array.isArray(own?.steps) ? own.steps : steps;
    },
    [scenario, steps],
  );
  const hops = activePath ? pathHops(activePath, steps, stepsFor(activePath)) : [];
  const canFindings = allowedRoutes.includes('/risks');
  const canNetwork = allowedRoutes.includes('/network-activity');

  return (
    <article aria-labelledby="attack-path-title" className="flex min-w-0 flex-col gap-5 rounded-xl border border-border bg-surface/45 p-4">
      <header className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-3">
          <LevelBadge risk={scenarioMaxRisk(scenario, pathById)} />
          <ConfidenceTag scenario={scenario} />
          {paths.length > 1 ? <span className="text-caption text-muted">{paths.length} paths</span> : null}
          <span className="ml-auto">{addToCase}</span>
        </div>
        <h2 id="attack-path-title" className="text-section-title font-semibold text-text">
          {scenario.headline}
        </h2>
        <p className="text-body text-muted">{scenario.narrative}</p>
      </header>

      {fixes.length > 0 ? (
        <Section title="Fix">
          <ol className="flex flex-col gap-1.5">
            {fixes.map((fix, i) => (
              <li key={fix.label} className="flex items-start gap-3 text-body text-text">
                <span className="w-5 shrink-0 text-right font-mono text-caption text-muted">{i + 1}</span>
                <span className="min-w-0 flex-1">{fix.label}</span>
              </li>
            ))}
          </ol>
        </Section>
      ) : null}

      <div className="hidden sm:block">
        <Section
          title="Chain"
          action={
            <div role="group" aria-label="Chain view" className="flex gap-1">
              {(['chain', 'graph'] as const).map((v) => (
                <button
                  key={v}
                  type="button"
                  aria-pressed={view === v}
                  onClick={() => setView(v)}
                  className={clsx(
                    'rounded-md px-2 py-0.5 text-caption font-semibold',
                    view === v ? 'bg-brand/15 text-text' : 'text-muted hover:text-text',
                  )}
                >
                  {v === 'chain' ? 'Diagram' : 'Graph'}
                </button>
              ))}
            </div>
          }
        >
          {view === 'chain' ? (
            <AttackChainDiagram paths={paths} steps={steps} stepsFor={stepsFor} highlightedStepIndex={highlightedStepIndex} onStepClick={onStepClick} hrefFor={hrefFor} />
          ) : (
            graph
          )}
        </Section>
      </div>

      {hops.length > 0 ? (
        <Section title="Step by step">
          {paths.length > 1 ? (
            <div role="group" aria-label="Path" className="flex flex-wrap gap-1.5">
              {paths.map((p, i) => (
                <button
                  key={p.path_id ?? i}
                  type="button"
                  aria-pressed={i === pathIndex}
                  onClick={() => setPathIndex(i)}
                  className={clsx(
                    'rounded-md border px-2 py-0.5 text-caption',
                    i === pathIndex ? 'border-brand bg-brand/15 text-text' : 'border-border text-muted hover:text-text',
                  )}
                >
                  From {nodeDisplayName(p.nodes?.[0])}
                </button>
              ))}
            </div>
          ) : null}
          <ol className="flex flex-col">
            {hops.map((hop, i) => {
              const kind = nodeKind(hop.node);
              const href = hrefFor(hop.node);
              const ref = { uid: hop.node.id, clusterId };
              const active = hop.step !== undefined && highlightedStepIndex === hop.step;
              return (
                <li key={`${hop.node.id}-${i}`} className="relative flex gap-3 pb-3 last:pb-0">
                  {i < hops.length - 1 ? <span className="absolute left-[0.6875rem] top-6 h-full w-px bg-border" aria-hidden /> : null}
                  <button
                    type="button"
                    disabled={hop.step === undefined}
                    onClick={() => hop.step !== undefined && onStepClick(hop.step)}
                    aria-pressed={active}
                    aria-label={hop.step !== undefined ? `Highlight step ${hop.step}` : undefined}
                    className={clsx(
                      'relative z-10 mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full border font-mono text-meta font-semibold',
                      active
                        ? 'border-brand bg-brand text-white'
                        : hop.step !== undefined
                          ? 'border-border bg-surface text-text hover:border-brand'
                          : 'border-border bg-base text-muted',
                    )}
                  >
                    {hop.step ?? '·'}
                  </button>
                  <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                    {hop.action ? (
                      <span className="flex flex-wrap items-center gap-2 text-body text-text">
                        {hop.action}
                        {hop.observed ? (
                          <span className="rounded border border-emerald-500/40 px-1.5 text-meta text-emerald-300">seen at runtime</span>
                        ) : null}
                      </span>
                    ) : null}
                    <span className="flex flex-wrap items-center gap-x-3 gap-y-0.5 text-caption">
                      <span className="text-muted">{NODE_KIND_LABEL[kind] ?? kind}</span>
                      {href ? (
                        <Link to={href} className="min-w-0 truncate font-medium text-text hover:text-brand hover:underline">
                          {nodeDisplayName(hop.node)}
                        </Link>
                      ) : (
                        <span className="min-w-0 truncate font-medium text-text">{nodeDisplayName(hop.node)}</span>
                      )}
                      {kind === 'pod' && canFindings ? (
                        <Link to={findingsForResourcePath(ref)} className="font-semibold text-brand hover:underline">
                          Findings
                        </Link>
                      ) : null}
                      {kind === 'pod' && canNetwork ? (
                        <Link
                          to={networkForPodPath(ref, String(hop.node.properties?.namespace || '') || undefined)}
                          className="font-semibold text-brand hover:underline"
                        >
                          Network
                        </Link>
                      ) : null}
                    </span>
                  </div>
                </li>
              );
            })}
          </ol>
        </Section>
      ) : null}

      {evidence.length > 0 || assumptions.length > 0 || extraEvidence || paths.length > 0 ? (
        <details className="group rounded-lg border border-border">
          <summary className="cursor-pointer select-none px-3 py-2 text-caption font-semibold text-muted hover:text-text">Evidence and assumptions</summary>
          <div className="flex flex-col gap-3 border-t border-border px-3 py-3 text-body">
            {evidence.length > 0 ? (
              <ul className="list-disc pl-5 text-text">
                {evidence.slice(0, 12).map((e, i) => (
                  <li key={i}>
                    {e.detail || e.source_ref}
                    {e.source_ref && e.detail ? <span className="ml-1 font-mono text-caption text-muted">{e.source_ref}</span> : null}
                  </li>
                ))}
              </ul>
            ) : null}
            {assumptions.length > 0 ? (
              <div>
                <p className="mb-1 text-caption font-semibold text-muted">Assumes</p>
                <ul className="list-disc pl-5 text-text">
                  {assumptions.map((a) => (
                    <li key={a.id}>{a.description}</li>
                  ))}
                </ul>
              </div>
            ) : null}
            {paths.length > 0 ? (
              <p className="font-mono text-caption text-muted">Path ids: {paths.map((p) => p.path_id).filter(Boolean).join(', ')}</p>
            ) : null}
            {extraEvidence}
          </div>
        </details>
      ) : null}
    </article>
  );
};
