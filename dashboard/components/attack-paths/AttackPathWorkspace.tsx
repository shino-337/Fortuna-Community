import React from 'react';
import { Link } from 'react-router-dom';
import { CheckCircle2, ChevronRight, Eye, HelpCircle, Wrench } from 'lucide-react';
import clsx from 'clsx';
import type { AttackPath, AttackPathNode } from '../../types';
import {
  buildFixRecommendations,
  capabilityHumanSnippet,
  scenarioEnds,
  scenarioMaxRisk,
  scenarioPathIds,
  type GroupedScenario,
  type SharedFix,
} from '../../lib/attackPathNarrative';
import { ATTACK_PATH_LANE_LABELS, attackPathConfidenceLane, type AttackPathConfidenceLane } from '../../lib/attackPathConfidence';
import { getSeverityBadgeClass, pathRiskLevel, type SeverityLevel } from '../../lib/severity';
import { findingsForResourcePath, identityDetailPath, networkForPodPath, podDetailPath } from '../../lib/entityLinks';

const PRIORITY_CLASS: Record<string, string> = {
  CRITICAL: 'text-red-300',
  HIGH: 'text-orange-300',
  MEDIUM: 'text-yellow-300',
  LOW: 'text-muted',
};

const LANE_ICON: Record<AttackPathConfidenceLane, React.ReactNode> = {
  confirmed: <CheckCircle2 className="h-3.5 w-3.5" aria-hidden />,
  probable: <Eye className="h-3.5 w-3.5" aria-hidden />,
  theoretical: <HelpCircle className="h-3.5 w-3.5" aria-hidden />,
};

/** Confidence in neutral colours, so it never reads as a severity. */
export function ConfidenceTag({ scenario }: { scenario: GroupedScenario }) {
  const lane = attackPathConfidenceLane(scenario);
  return (
    <span className="inline-flex items-center gap-1 text-caption text-muted" title="How much of this path is backed by observed evidence">
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

function nodeName(node: AttackPathNode | null | undefined): string {
  if (!node) return '';
  const name = String(node.properties?.name || node.id);
  const clusterScoped = ['node', 'capability', 'cluster_role', 'cluster_role_binding', 'external'].includes(String(node.type || '').toLowerCase());
  const ns = clusterScoped ? '' : String(node.properties?.namespace || '');
  return ns ? `${ns}/${name}` : name;
}

/** Fixes that break the most paths, the first thing to do on this page. */
export const BreakTheseFirst: React.FC<{
  fixes: SharedFix[];
  scenarios: GroupedScenario[];
  onSelect: (scenarioKey: string) => void;
}> = ({ fixes, scenarios, onSelect }) => {
  if (fixes.length === 0) return null;
  const pathsBroken = (fix: SharedFix) => {
    const ids = new Set<string>();
    for (const s of scenarios) if (fix.scenarioKeys.includes(s.key)) scenarioPathIds(s).forEach((id) => ids.add(id));
    return Math.max(ids.size, fix.scenarioKeys.length);
  };
  return (
    <section aria-labelledby="break-first-title" className="rounded-xl border border-border bg-surface/45 p-3">
      <h2 id="break-first-title" className="mb-2 flex items-center gap-2 text-body font-semibold text-text">
        <Wrench className="h-4 w-4 text-brand" aria-hidden /> Break these first
      </h2>
      <ol className="divide-y divide-border/60">
        {fixes.slice(0, 3).map((fix) => (
          <li key={fix.label} className="grid grid-cols-[4rem_minmax(0,1fr)] items-baseline gap-x-3 gap-y-1 py-2 sm:flex sm:items-center">
            <span className={clsx('w-16 shrink-0 text-meta font-semibold uppercase', PRIORITY_CLASS[fix.priority] ?? 'text-muted')}>{fix.priority}</span>
            <span className="min-w-0 text-body text-text sm:flex-1">{fix.label}</span>
            <button
              type="button"
              onClick={() => onSelect(fix.scenarioKeys[0])}
              className="col-start-2 w-fit shrink-0 text-caption font-semibold text-brand hover:underline"
            >
              Breaks {pathsBroken(fix)} {pathsBroken(fix) === 1 ? 'path' : 'paths'}
            </button>
          </li>
        ))}
      </ol>
    </section>
  );
};

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
                {nodeName(entry) || 'Unknown entry'} → {target || 'target'}
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

const KIND_LABEL: Record<string, string> = {
  pod: 'Pod',
  service_account: 'Service account',
  role_binding: 'Role binding',
  cluster_role_binding: 'Cluster role binding',
  role: 'Role',
  cluster_role: 'Cluster role',
  node: 'Node',
  capability: 'Capability',
  external: 'External',
};

/** Every object a scenario's paths pass through, in path order, each linked to its own page. */
function ResourceHops({
  nodes,
  clusterId,
  allowedRoutes,
  canInventory,
}: {
  nodes: AttackPathNode[];
  clusterId: string | null;
  allowedRoutes: string[];
  canInventory: boolean;
}) {
  const canFindings = allowedRoutes.includes('/risks');
  const canNetwork = allowedRoutes.includes('/network-activity');
  return (
    <ul className="divide-y divide-border/60 rounded-lg border border-border">
      {nodes.map((n) => {
        const type = String(n.type || '').toLowerCase();
        const ref = { uid: n.id, clusterId };
        const links: React.ReactNode[] = [];
        if (type === 'pod') {
          if (canFindings) links.push(<Link key="f" to={findingsForResourcePath(ref)} className="hover:underline">Findings</Link>);
          if (canNetwork) links.push(<Link key="n" to={networkForPodPath(ref, String(n.properties?.namespace || '') || undefined)} className="hover:underline">Network</Link>);
        }
        const href =
          type === 'pod' && canInventory
            ? podDetailPath(n.id, clusterId ?? undefined)
            : type === 'service_account' && canInventory
              ? identityDetailPath(ref)
              : null;
        return (
          <li key={n.id} className="flex flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2">
            <span className="w-32 shrink-0 text-caption text-muted">{KIND_LABEL[type] ?? n.type}</span>
            {href ? (
              <Link to={href} className="min-w-0 flex-1 truncate text-body text-text hover:text-brand hover:underline">
                {nodeName(n)}
              </Link>
            ) : (
              <span className="min-w-0 flex-1 truncate text-body text-text">{nodeName(n)}</span>
            )}
            {links.length ? <span className="flex gap-3 text-caption font-semibold text-brand">{links}</span> : null}
          </li>
        );
      })}
    </ul>
  );
}

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

/** The selected scenario: what it reaches, how, how to break it, and every object on the way. */
export const AttackPathDetail: React.FC<{
  scenario: GroupedScenario;
  pathById: Map<string, AttackPath>;
  clusterId: string | null;
  allowedRoutes: string[];
  canOpenInventory: boolean;
  graph: React.ReactNode;
  addToCase: React.ReactNode;
  highlightedStepIndex: number | null;
  onStepClick: (index: number) => void;
  extraEvidence?: React.ReactNode;
}> = ({ scenario, pathById, clusterId, allowedRoutes, canOpenInventory, graph, addToCase, highlightedStepIndex, onStepClick, extraEvidence }) => {
  const chain = scenario.representativeChain;
  const paths = scenarioPathIds(scenario)
    .map((id) => pathById.get(id))
    .filter((p): p is AttackPath => Boolean(p));
  const steps = Array.isArray(chain.steps) ? chain.steps : [];
  const fixes = buildFixRecommendations(chain);
  const evidence = Array.isArray(chain.evidence) ? chain.evidence : [];
  const assumptions = Array.isArray(chain.assumptions) ? chain.assumptions : [];

  const seen = new Set<string>();
  const nodes: AttackPathNode[] = [];
  for (const p of paths) {
    for (const n of Array.isArray(p.nodes) ? p.nodes : []) {
      if (seen.has(n.id)) continue;
      seen.add(n.id);
      nodes.push(n);
    }
  }
  const fallbackSteps =
    steps.length === 0 && paths[0]
      ? (Array.isArray(paths[0].edges) ? paths[0].edges : []).map((e) => capabilityHumanSnippet(e.type))
      : [];

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

      {graph}

      <Section title="How it works">
        {steps.length > 0 ? (
          <ol className="flex flex-col gap-1">
            {steps.map((step, i) => (
              <li key={`${step.technique_id}-${i}`}>
                <button
                  type="button"
                  onClick={() => onStepClick(i + 1)}
                  aria-pressed={highlightedStepIndex === i + 1}
                  className={clsx(
                    'flex w-full items-center gap-3 rounded-md px-2 py-1.5 text-left text-body',
                    highlightedStepIndex === i + 1 ? 'bg-brand/10 text-text' : 'text-text hover:bg-surface-2/60',
                  )}
                >
                  <span className="w-5 shrink-0 text-right font-mono text-caption text-muted">{i + 1}</span>
                  <span className="min-w-0 flex-1">{step.name || capabilityHumanSnippet(step.technique_id)}</span>
                  {step.runtime_observed ? (
                    <span className="shrink-0 rounded border border-emerald-500/40 px-1.5 text-meta text-emerald-300">seen at runtime</span>
                  ) : null}
                </button>
              </li>
            ))}
          </ol>
        ) : fallbackSteps.length > 0 ? (
          <ol className="list-decimal pl-6 text-body text-text">
            {fallbackSteps.map((s, i) => (
              <li key={i}>{s}</li>
            ))}
          </ol>
        ) : (
          <p className="text-body text-muted">No step detail for this path.</p>
        )}
      </Section>

      <Section title="Fix">
        <ol className="flex flex-col gap-1.5">
          {fixes.map((fix) => (
            <li key={fix.label} className="flex items-start gap-3 text-body text-text">
              <span className={clsx('w-16 shrink-0 text-meta font-semibold uppercase', PRIORITY_CLASS[fix.priority] ?? 'text-muted')}>{fix.priority}</span>
              <span className="min-w-0 flex-1">{fix.label}</span>
            </li>
          ))}
        </ol>
      </Section>

      {nodes.length > 0 ? (
        <Section title="On this path">
          <ResourceHops nodes={nodes} clusterId={clusterId} allowedRoutes={allowedRoutes} canInventory={canOpenInventory} />
        </Section>
      ) : null}

      {evidence.length > 0 || assumptions.length > 0 || extraEvidence ? (
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
