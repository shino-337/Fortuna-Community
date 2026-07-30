import React from 'react';
import { Link } from 'react-router-dom';
import {
  Flame,
  Network,
  ShieldAlert,
  Zap,
  AlertTriangle,
  Crosshair,
  Wrench,
  ChevronRight,
  ExternalLink,
  Eye,
} from 'lucide-react';
import type { GroupedScenario } from '../lib/attackPathNarrative';
import { buildRootCauses, buildFixRecommendations } from '../lib/attackPathNarrative';

function objectiveDisplayLabel(objective?: string): string {
  const o = String(objective || '').toUpperCase();
  const map: Record<string, string> = {
    CLUSTER_TAKEOVER: 'Cluster takeover',
    CLUSTER_PRIVILEGE_ESCALATION: 'Cluster privilege escalation',
    NODE_COMPROMISE: 'Node compromise',
    WORKLOAD_CONTROL: 'Workload control',
    SECRET_EXFIL: 'Secret exposure',
    DATA_EXFILTRATION: 'Data exfiltration',
    LIMITED_RBAC_IMPACT: 'Limited RBAC impact',
    STORAGE_ABUSE: 'Storage abuse',
    LATERAL_MOVEMENT: 'Lateral movement',
  };
  if (map[o]) return map[o];
  return objective
    ? objective.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
    : 'Attack scenario';
}

function confidenceChipClass(conf: string): string {
  switch (String(conf || '').toLowerCase()) {
    case 'high':
      return 'text-brand';
    case 'medium':
      return 'text-amber-400';
    default:
      return 'text-muted';
  }
}

function effortChipClass(costVal: number): string {
  if (costVal <= 3) return 'text-emerald-400';
  if (costVal <= 6) return 'text-amber-400';
  return 'text-red-400';
}

function impactScopeBadge(chain: GroupedScenario['representativeChain']): { label: string; clusterWide: boolean } {
  const scope = String(chain.impact_scope || '').toUpperCase();
  if (scope === 'CLUSTER')
    return { label: 'CLUSTER-WIDE IMPACT', clusterWide: true };
  if (scope === 'NODE')
    return { label: 'NODE-LEVEL IMPACT', clusterWide: false };
  if (scope === 'NAMESPACE')
    return { label: 'NAMESPACE IMPACT', clusterWide: false };
  const obj = String(chain.objective || '').toUpperCase();
  if (obj.includes('CLUSTER') || obj.includes('TAKEOVER'))
    return { label: 'CLUSTER-WIDE IMPACT', clusterWide: true };
  return { label: 'ESCALATION RISK', clusterWide: false };
}

function privilegeScopeLabel(chain: GroupedScenario['representativeChain']): string {
  const steps = Array.isArray(chain.steps) ? chain.steps : [];
  const ids = steps.map((s) => String(s.technique_id || '').toUpperCase());
  if (ids.some((id) => id.includes('CLUSTER_ADMIN'))) return 'Toward cluster-admin scope';
  if (ids.some((id) => id.includes('RBAC'))) return 'RBAC / binding abuse path';
  if (ids.some((id) => id.includes('NODE') || id.includes('HOST'))) return 'Host / node surface';
  return 'Workload foothold onward';
}

function evidencePostureBadges(chain: GroupedScenario['representativeChain']): Array<{
  label: string;
  className: string;
  title?: string;
}> {
  const summary = chain.mitre_summary;
  const eventsSeen = summary?.runtime_events_considered ?? 0;
  const eventsMatched = summary?.runtime_events_matched ?? 0;
  const runtimeObserved = eventsMatched > 0 || (summary?.runtime_mitre_distinct ?? 0) > 0;
  const runtimeScoped = eventsSeen > 0 || (summary?.observing_pod_count ?? 0) > 0;
  const softMode = Boolean(chain.capability_validation?.soft_mode);
  const hasGaps = Array.isArray(chain.capability_validation?.gaps) && chain.capability_validation.gaps.length > 0;

  const badges: Array<{ label: string; className: string; title?: string }> = [];
  if (runtimeObserved) {
    badges.push({
      label: 'Runtime observed',
      className: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-300',
      title: 'Runtime events matched at least one path MITRE technique.',
    });
  } else if (runtimeScoped) {
    badges.push({
      label: 'Runtime seen, no match',
      className: 'border-amber-500/40 bg-amber-500/10 text-amber-200',
      title: 'Runtime events exist for scoped pods but did not match the path techniques.',
    });
  } else {
    badges.push({
      label: 'Config evidence',
      className: 'border-border bg-surface-2/80 text-text',
      title: 'This chain is currently grounded by Kubernetes configuration and graph relations.',
    });
  }
  if (softMode || hasGaps) {
    badges.push({
      label: hasGaps ? 'Capability gap' : 'Soft chain',
      className: 'border-indigo-500/35 bg-indigo-500/10 text-indigo-200',
      title: 'Capability continuity is advisory; validate inferred preconditions before remediation.',
    });
  }
  return badges;
}

/** Hero card for the highest-priority scenario — headline, narrative, root causes, fixes, and visualization controls. */
export const PrimaryScenarioHero: React.FC<{
  scenario: GroupedScenario;
  scenarioOrdinal: number;
  visibleScenarioCount: number;
  onVisualize: () => void;
  /** Opens Technical Evidence tab (parent switches tab + may scroll). */
  onOpenTechnical?: () => void;
  /** 1-based attack step aligned with graph edge stepIndex where mapped. */
  highlightedStepIndex?: number | null;
  onStepClick?: (stepNumber: number) => void;
}> = ({
  scenario,
  scenarioOrdinal,
  visibleScenarioCount,
  onVisualize,
  onOpenTechnical,
  highlightedStepIndex,
  onStepClick,
}) => {
  const chain = scenario.representativeChain;
  const story = chain.story;
  const headline = scenario.headline || story?.headline;

  const costVal = chain.exploit_cost || 0;
  const cost = costVal <= 3 ? 'LOW' : costVal <= 6 ? 'MODERATE' : 'HIGH';
  const confidence = chain.confidence.toUpperCase();
  const rootCauses = buildRootCauses(chain);
  const fixes = buildFixRecommendations(chain);
  const matchedMitreIds = Array.isArray(chain.mitre_summary?.matched_mitre_ids)
    ? chain.mitre_summary.matched_mitre_ids
    : [];
  const mitreCoverage = Array.isArray(chain.mitre_coverage) ? chain.mitre_coverage : [];
  const capabilityGaps = Array.isArray(chain.capability_validation?.gaps)
    ? chain.capability_validation.gaps
    : [];

  const involvedPods = new Set<string>();
  const involvedNodes = new Set<string>();
  const involvedNamespaces = new Set<string>();

  const variants = Array.isArray(scenario.variants) ? scenario.variants : [];
  variants.forEach((v) => {
    if (v.final_target) {
      involvedPods.add(v.final_target);
      if (v.final_target.includes('.')) {
        const parts = v.final_target.split('.');
        if (parts.length > 1) involvedNamespaces.add(parts[1]);
      }
    }
    if (Array.isArray(v.variant_nodes)) v.variant_nodes.forEach((n) => involvedNodes.add(n));
  });

  const podCount = Math.max(involvedPods.size, 1);
  const nodeCount = Math.max(involvedNodes.size, 1);
  const nsCount = Math.max(involvedNamespaces.size, 1);

  const heuristicBlast = Math.min(0.9, podCount * 0.1 + nodeCount * 0.2 + nsCount * 0.3);
  const backendBlast = typeof chain.blast_radius === 'number' ? chain.blast_radius : NaN;
  const blastValue =
    Number.isFinite(backendBlast) && backendBlast > 0 ? Math.min(1, backendBlast) : heuristicBlast;

  const steps = Array.isArray(chain.steps) ? chain.steps : [];
  const requiresNetwork = steps.some(
    (s) => s.technique_id?.includes('NETWORK') || (Array.isArray(s.input_caps) ? s.input_caps : []).includes('NETWORK_ACCESS'),
  );
  const exploitType = rootCauses.length > 0 ? 'Misconfiguration' : 'Vulnerability';

  const objectiveLabel = objectiveDisplayLabel(chain.objective);
  const impactBadge = impactScopeBadge(chain);
  const privilegeLabel = privilegeScopeLabel(chain);
  const validity = chain.validity;
  const evidenceBadges = evidencePostureBadges(chain);

  const titleLine =
    scenarioOrdinal === 1 && visibleScenarioCount >= 1
      ? 'Highest priority scenario'
      : `Scenario ${scenarioOrdinal} of ${visibleScenarioCount}`;

  const scrollToGraph = () => {
    document.getElementById('graph-section')?.scrollIntoView({ behavior: 'smooth' });
  };

  return (
    <div className="mb-5 rounded-xl border border-error/30 bg-error/10 p-5 sm:p-6 relative overflow-hidden shadow-xl shadow-error/10">
      <div className="absolute top-0 left-0 w-1.5 h-full bg-error"></div>

      <div className="flex flex-wrap items-center justify-between gap-3 mb-3">
        <div className="flex items-center gap-2">
          <Flame size={18} className="text-red-500 animate-pulse shrink-0" />
          <h2 className="text-section-title font-bold text-error">{titleLine}</h2>
        </div>
        <div className="flex flex-wrap items-center gap-x-1 gap-y-1 text-micro text-muted font-mono bg-base/50 px-2.5 py-1 rounded-full border border-border max-w-full">
          <span className="shrink-0">Risk drivers:</span>
          <span className="text-red-400">{objectiveLabel}</span>
          <span className="text-muted-2">+</span>
          <span className={confidenceChipClass(chain.confidence)}>{confidence} confidence</span>
          <span className="text-muted-2">+</span>
          <span className={effortChipClass(costVal)}>{cost} effort ({costVal})</span>
        </div>
      </div>

      <h1 className="text-xl sm:text-2xl font-extrabold text-text mb-2 leading-tight">{headline}</h1>

      <p className="text-text text-body leading-relaxed mb-4 max-w-3xl">{scenario.narrative}</p>

      <div className="flex flex-wrap items-center gap-2 mb-5">
        <span className="px-2.5 py-0.5 rounded border border-border bg-surface-2/80 text-meta font-semibold text-text">
          <span className="text-brand mr-1">Confidence:</span> {confidence}
        </span>
        <span className="px-2.5 py-0.5 rounded border border-border bg-surface-2/80 text-meta font-semibold text-text">
          <span className="text-amber-400 mr-1">Difficulty:</span> {cost}
        </span>
        <span
          className={`px-2.5 py-0.5 rounded border text-meta font-semibold flex items-center gap-1 ${
            impactBadge.clusterWide
              ? 'border-red-900/50 bg-red-900/20 text-red-400'
              : 'border-border bg-surface-2/80 text-text'
          }`}
        >
          <ShieldAlert size={12} /> {impactBadge.label}
        </span>
        {evidenceBadges.map((badge) => (
          <span
            key={badge.label}
            title={badge.title}
            className={`px-2.5 py-0.5 rounded border text-meta font-semibold ${badge.className}`}
          >
            {badge.label}
          </span>
        ))}
      </div>

      {validity && validity.is_valid === false && (
        <div className="mb-5 rounded-lg border border-amber-500/35 bg-amber-950/20 px-3 py-2.5">
          <p className="text-micro font-semibold uppercase tracking-wider text-amber-300 flex items-center gap-1.5">
            <AlertTriangle size={13} className="shrink-0" />
            Validation gate: {validity.severity || 'review required'}
          </p>
          <p className="mt-1 text-caption text-amber-100/85">
            {validity.reason || 'This chain needs analyst validation before remediation is treated as confirmed.'}
          </p>
        </div>
      )}

      {validity && validity.is_valid !== false && validity.severity && (
        <div className="mb-5 rounded-lg border border-sky-500/25 bg-sky-950/15 px-3 py-2.5">
          <p className="text-micro font-semibold uppercase tracking-wider text-sky-300">
            Validation: {validity.severity}
          </p>
          {validity.reason && <p className="mt-1 text-caption text-sky-100/80">{validity.reason}</p>}
        </div>
      )}

      {/* Compact horizontal attack steps — click highlights matching edges on graph */}
      {steps.length > 0 && (
        <div className="mb-5">
          <h3 className="text-meta font-semibold text-muted mb-2 uppercase tracking-wider flex items-center gap-1.5">
            <Zap size={14} className="text-brand shrink-0" /> Attack progression
          </h3>
          <div className="flex gap-2 overflow-x-auto pb-1 -mx-0.5 px-0.5 scroll-smooth [scrollbar-width:thin]">
            {steps.map((step, i) => {
              const n = i + 1;
              const active = highlightedStepIndex === n;
              return (
                <button
                  key={i}
                  type="button"
                  onClick={() => {
                    onStepClick?.(n);
                    scrollToGraph();
                  }}
                  className={`shrink-0 text-left rounded-lg border px-2.5 py-2 max-w-[220px] transition-colors ${
                    active
                      ? 'border-brand bg-brand/15 ring-1 ring-brand/40'
                      : 'border-border bg-surface/60 hover:border-border hover:bg-surface'
                  }`}
                >
                  <div className="flex items-center gap-1 mb-0.5 flex-wrap">
                    <span className="text-micro font-bold text-brand tabular-nums">{n}</span>
                    <span className="text-nano text-muted font-mono truncate">{step.technique_id}</span>
                    {step.runtime_grounding_score != null && (
                      <span
                        title="Step-level runtime grounding: chain events matching this step’s MITRE ids ÷ all chain events (7d)"
                        className="text-nano font-mono text-sky-400/90 tabular-nums"
                      >
                        {(step.runtime_grounding_score * 100).toFixed(0)}% gr
                      </span>
                    )}
                    {step.runtime_observed && (
                      <span title="Runtime signal matched this step’s MITRE id" className="inline-flex items-center text-emerald-400">
                        <Eye size={12} aria-hidden />
                      </span>
                    )}
                  </div>
                  <p className="text-caption text-text font-medium leading-snug line-clamp-2">{step.name}</p>
                  {(Array.isArray(step.mitre_techniques) ? step.mitre_techniques : []).length > 0 && (
                    <div className="flex flex-wrap gap-1 mt-1.5">
                      {(Array.isArray(step.mitre_techniques) ? step.mitre_techniques : []).map((mt) => (
                        <a
                          key={`${step.technique_id}-${mt.id}`}
                          href={mt.url || `https://attack.mitre.org/techniques/${mt.id.replace(/\./g, '/')}/`}
                          target="_blank"
                          rel="noopener noreferrer"
                          onClick={(e) => e.stopPropagation()}
                          className="inline-flex items-center gap-0.5 rounded border border-amber-500/40 bg-amber-500/10 px-1 py-0.5 text-nano font-mono text-amber-200/90 hover:bg-amber-500/20"
                        >
                          {mt.id}
                          <ExternalLink size={9} className="opacity-70" />
                        </a>
                      ))}
                    </div>
                  )}
                </button>
              );
            })}
          </div>
          <p className="text-micro text-muted mt-1.5">Tap a step to emphasize mapped edges in the graph.</p>
        </div>
      )}

      {chain.mitre_summary && chain.mitre_summary.path_mitre_distinct > 0 && (
        <div className="mb-5 rounded-lg border border-amber-500/25 bg-amber-950/10 px-3 py-2.5">
          <p className="text-micro font-semibold text-amber-400/90 uppercase tracking-wider mb-1.5">
            MITRE coverage (path vs runtime)
          </p>
          <div className="flex flex-wrap gap-x-4 gap-y-1 text-meta text-text">
            <span>
              Alignment:{' '}
              <span className="text-text font-semibold tabular-nums">
                {(chain.mitre_summary.alignment_ratio * 100).toFixed(0)}%
              </span>
            </span>
            <span className="text-muted">
              Path techniques: {chain.mitre_summary.path_mitre_distinct} · Runtime (pods):{' '}
              {chain.mitre_summary.runtime_mitre_distinct} · Pods scoped: {chain.mitre_summary.observing_pod_count}
            </span>
            {typeof chain.mitre_summary.correlation_precision === 'number' && (
              <span className="text-muted">
                Precision:{' '}
                <span className="text-amber-200/90 tabular-nums">
                  {(chain.mitre_summary.correlation_precision * 100).toFixed(0)}%
                </span>
              </span>
            )}
            {(chain.mitre_summary.runtime_events_considered ?? 0) > 0 && (
              <span className="text-muted">
                Events (7d): {chain.mitre_summary.runtime_events_matched ?? 0} matched /{' '}
                {chain.mitre_summary.runtime_events_considered} seen
              </span>
            )}
          </div>
          {matchedMitreIds.length > 0 && (
            <div className="flex flex-wrap gap-1 mt-2">
              {matchedMitreIds.map((id) => (
                <span
                  key={id}
                  className="rounded border border-emerald-500/40 bg-emerald-500/10 px-1.5 py-0.5 text-micro font-mono text-emerald-300"
                >
                  ✓ {id}
                </span>
              ))}
            </div>
          )}
          {(mitreCoverage.filter((r) => r.status !== 'unknown').length > 0 ||
            mitreCoverage.some((r) => r.status === 'unknown')) && (
            <div className="mt-2 pt-2 border-t border-amber-500/20">
              <p className="text-micro text-muted mb-1">Detection coverage (MITRE)</p>
              <div className="flex flex-wrap gap-1">
                {mitreCoverage
                  .filter((row) => row.status !== 'unknown')
                  .map((row) => (
                    <span
                      key={row.mitre_id}
                      title={`${row.priority ?? ''}${row.priority ? ' · ' : ''}${row.status}${typeof row.confidence === 'number' ? ` · ${(row.confidence * 100).toFixed(0)}%` : ''}`}
                      className={`rounded px-1.5 py-0.5 text-nano font-mono ${
                        row.status === 'observed'
                          ? 'border border-emerald-500/40 bg-emerald-500/10 text-emerald-300'
                          : row.status === 'inferred'
                            ? 'border border-border bg-surface-2/80 text-text'
                            : 'border border-rose-500/35 bg-rose-950/30 text-rose-300/90'
                      }`}
                    >
                      {row.mitre_id}:{row.status}
                    </span>
                  ))}
              </div>
              {mitreCoverage.some((r) => r.status === 'unknown') && (
                <p
                  title="MITRE technique not listed in detection_coverage.yaml — no gap verdict; extend the registry when detectors exist."
                  className="text-micro text-muted-2 mt-1.5 cursor-help"
                >
                  {mitreCoverage.filter((r) => r.status === 'unknown').length} id(s) have unknown
                  detection registry coverage (not counted as gaps).
                </p>
              )}
            </div>
          )}
        </div>
      )}

      {chain.capability_validation && (
        <div className="mb-5 rounded-lg border border-indigo-500/25 bg-indigo-950/10 px-3 py-2.5">
          <p className="text-micro font-semibold text-indigo-300/90 uppercase tracking-wider mb-1">
            Capability continuity (advisory)
          </p>
          <p className="text-meta text-text">
            Score:{' '}
            <span className="text-text font-semibold tabular-nums">
              {(chain.capability_validation.confidence * 100).toFixed(0)}%
            </span>
            {chain.capability_validation.soft_mode && (
              <span className="text-muted ml-2">· soft mode (chains are never dropped)</span>
            )}
          </p>
          {capabilityGaps.length > 0 && (
            <ul className="mt-1.5 text-micro text-amber-200/80 space-y-0.5">
              {capabilityGaps.map((g) => (
                <li key={g} className="font-mono">
                  Missing pre-state: {g}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-5">
        <details className="group rounded-lg bg-surface/60 border border-border open:pb-3">
          <summary className="cursor-pointer list-none px-3 py-2.5 flex items-center justify-between gap-2 text-body font-semibold text-text hover:bg-surface-2/40 rounded-lg">
            <span className="flex items-center gap-2">
              <AlertTriangle size={14} className="text-amber-400 shrink-0" /> Root causes
            </span>
            <ChevronRight size={14} className="text-muted transition-transform duration-150 motion-reduce:transition-none group-open:rotate-90 shrink-0" />
          </summary>
          <ul className="px-3 pb-1 space-y-1.5 border-t border-border/80 pt-2 mt-0">
            {rootCauses.length === 0 ? (
              <li className="text-caption text-muted">No structured root-cause summary for this chain.</li>
            ) : (
              rootCauses.map((c, i) => (
                <li key={i} className="flex items-start gap-2 text-caption text-amber-200/80 leading-snug">
                  <span className="text-amber-500/50 mt-0.5 shrink-0">•</span>
                  <span>{c}</span>
                </li>
              ))
            )}
          </ul>
        </details>

        <details open className="group rounded-lg bg-red-950/20 border border-red-500/30 open:pb-3 relative overflow-hidden">
          <div className="absolute top-0 left-0 w-1 h-full bg-red-500/40 pointer-events-none" />
          <summary className="cursor-pointer list-none px-3 py-2.5 flex items-center justify-between gap-2 text-body font-bold text-text hover:bg-red-950/30 rounded-lg relative">
            <span className="flex items-center gap-2">
              <Wrench size={14} className="text-red-400 shrink-0" /> Immediate actions
            </span>
            <ChevronRight size={14} className="text-muted transition-transform duration-150 motion-reduce:transition-none group-open:rotate-90 shrink-0" />
          </summary>
          <ol className="px-3 pb-1 space-y-2.5 border-t border-error/40 pt-2 mt-0 relative">
            {fixes.map((fix, i) => (
              <li key={i} className="flex items-start gap-2">
                <span className="flex items-center justify-center w-5 h-5 rounded-full bg-error/20 text-error text-micro font-bold shrink-0 mt-0.5">
                  {i + 1}
                </span>
                <div>
                  <p className="text-body text-text">{fix.label}</p>
                  <p className="text-micro text-error font-bold mt-0.5">{fix.priority} PRIORITY</p>
                </div>
              </li>
            ))}
          </ol>
        </details>

        <details className="group rounded-lg bg-surface/60 border border-border open:pb-3 lg:col-span-2">
          <summary className="cursor-pointer list-none px-3 py-2.5 flex items-center justify-between gap-2 text-body font-semibold text-text hover:bg-surface-2/40 rounded-lg">
            <span className="flex items-center gap-2">
              <Crosshair size={14} className="text-indigo-400 shrink-0" /> Exploit requirements
            </span>
            <ChevronRight size={14} className="text-muted transition-transform duration-150 motion-reduce:transition-none group-open:rotate-90 shrink-0" />
          </summary>
          <div className="mt-0 grid grid-cols-1 gap-2 border-t border-border/80 px-3 pb-2 pt-2 min-[420px]:grid-cols-2 lg:grid-cols-4">
            <div className="bg-base/50 p-2 rounded border border-border">
              <p className="text-micro text-muted uppercase tracking-wide">Network access</p>
              <p className="text-caption text-text font-medium">{requiresNetwork ? 'Likely required' : 'Can stay local'}</p>
            </div>
            <div className="bg-base/50 p-2 rounded border border-border">
              <p className="text-micro text-muted uppercase tracking-wide">User interaction</p>
              <p className="text-caption text-text font-medium">Not modeled</p>
            </div>
            <div className="bg-base/50 p-2 rounded border border-border">
              <p className="text-micro text-muted uppercase tracking-wide">Exploit type</p>
              <p className="text-caption text-text font-medium">{exploitType}</p>
            </div>
            <div className="bg-base/50 p-2 rounded border border-border">
              <p className="text-micro text-muted uppercase tracking-wide">Privilege scope</p>
              <p className="text-caption text-text font-medium leading-snug">{privilegeLabel}</p>
            </div>
          </div>
        </details>

        <details className="group rounded-lg bg-surface/60 border border-border open:pb-3 lg:col-span-2">
          <summary className="cursor-pointer list-none px-3 py-2.5 flex items-center justify-between gap-2 text-body font-semibold text-text hover:bg-surface-2/40 rounded-lg">
            <span className="flex items-center gap-2">
              <Network size={14} className="text-emerald-400 shrink-0" /> Blast radius
            </span>
            <ChevronRight size={14} className="text-muted transition-transform duration-150 motion-reduce:transition-none group-open:rotate-90 shrink-0" />
          </summary>
          <div className="px-3 pb-3 pt-2 border-t border-border/80 mt-0">
            <div className="flex items-center gap-3 mb-2">
              <div className="flex-1 h-2 bg-surface-2 rounded-full overflow-hidden">
                <div className="h-full bg-critical transition-[width] duration-150 motion-reduce:transition-none" style={{ width: `${blastValue * 100}%` }} />
              </div>
            </div>
            <p className="text-caption text-muted">
              Model estimate:{' '}
              <span className="text-text font-semibold tabular-nums">{(blastValue * 100).toFixed(0)}%</span>
              {Number.isFinite(backendBlast) && backendBlast > 0 ? (
                <span className="text-muted"> (from path analysis)</span>
              ) : (
                <span className="text-muted"> (fallback from affected entities)</span>
              )}
            </p>
            <p className="text-caption text-muted mt-1">
              Affects <span className="text-text font-medium">{podCount}</span> target line(s),{' '}
              <span className="text-text font-medium">{nsCount}</span> namespace bucket(s),{' '}
              <span className="text-text font-medium">{nodeCount}</span> node bucket(s).
            </p>
          </div>
        </details>
      </div>

      <div className="flex flex-wrap gap-3">
        <button
          type="button"
          onClick={onVisualize}
          className="px-5 py-2 rounded border border-brand/50 bg-brand/10 hover:bg-brand/20 text-brand font-semibold text-body transition-colors duration-150 motion-reduce:transition-none flex items-center gap-2 shadow-lg shadow-brand/5"
        >
          <Network size={16} /> Visualize in graph
        </button>
        {onOpenTechnical && (
          <button
            type="button"
            onClick={onOpenTechnical}
            className="px-5 py-2 rounded border border-border bg-surface-2/80 hover:bg-surface-2 text-text font-semibold text-body transition-colors duration-150 motion-reduce:transition-none flex items-center gap-2"
          >
            Technical evidence
          </button>
        )}
        {/* Navigation bridge: Attack Path → Risk Center */}
        <Link
          to={`/risks/findings?attackPath=true`}
          className="px-5 py-2 rounded border border-amber-500/30 bg-amber-500/5 hover:bg-amber-500/15 text-amber-300 font-semibold text-body transition-colors duration-150 motion-reduce:transition-none flex items-center gap-2"
        >
          <ShieldAlert size={16} /> View related findings
        </Link>
      </div>
    </div>
  );
};
