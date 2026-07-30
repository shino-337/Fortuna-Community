import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { api } from '../lib/api';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { Card } from '../design-system/components/Card';
import { PartialLoadBanner } from './dashboard/PartialLoadBanner';
import { DashboardSubNav } from './dashboard/DashboardSubNav';
import { DashboardHeroMetrics } from './dashboard/DashboardHeroMetrics';
import { DashboardTrendChart } from './dashboard/DashboardTrendChart';
import { RiskStatCards } from './dashboard/RiskStatCards';
import { EntryPointsSection } from './dashboard/EntryPointsSection';
import { useDashboardData } from './dashboard/useDashboardData';
import type { DashboardMode, EntryScenario } from './dashboard/types';
import { usePersona } from '../hooks/usePersona';
import { useOperationalContext } from '../hooks/useOperationalContext';
import { getDashboardComposition } from '../lib/dashboardComposition';
import type { GraphSemanticMode } from '../lib/persona';
import { PersonaDashboardStrip } from './dashboard/PersonaDashboardStrip';
import {
  buildDashboardLoadPolicy,
  getPersonaWidgetCandidates,
  isDashboardSectionVisible,
  isDashboardWidgetVisible,
} from '../lib/dashboardComposition';
import type { DashboardDataLoadPolicy } from './dashboard/types';
import { SemanticEmptyState } from '../design-system/components/SemanticEmptyState';
import { FilterBar } from '../design-system/components/FilterBar';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PAGE_TITLES } from '../lib/pageTitles';
import { Section } from '../design-system/layouts/Section';
import { Badge } from '../design-system/components/Badge';
import { PageEmpty, PageLoading } from '../design-system/components/PageStatus';
import { AttackPathGraph } from '../components/AttackPathGraph';
import { GraphVisibilityOverlay } from '../components/GraphVisibilityOverlay';
import { GraphSemanticBanner } from '../components/GraphSemanticBanner';
import { useFeatureVisibility } from '../hooks/useVisibility';
import { IncidentModeBanner } from '../components/IncidentModeBanner';
import { IncidentPriorityStrip } from '../components/IncidentPriorityStrip';
import { RuntimeThreatStrip } from '../components/RuntimeThreatStrip';
import { useIncidentMode } from '../hooks/useIncidentMode';
import { useGraphTrustContext } from '../hooks/useGraphTrustContext';
import { ShieldAlert, Boxes, ArrowRight, Shield, Info, Target, Zap, Check, GitBranch, Eye, PanelRight, X, KeyRound, Route } from 'lucide-react';
import {
  Cluster,
  Insight,
  Notification,
  PodCapabilitySummaryCapability,
  UnifiedRiskScore,
  PodWithRisk,
  AttackChain,
  AttackPath,
  AttackPathGraphData,
  AttackStep,
  RuntimeSignal,
  PodRuntimeSecurityEvent,
  MitreCoverageItem,
} from '../types';
import { useClusterStore } from '../store/clusterStore';
import { useTimeWindowStore } from '../store/timeWindowStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';
import { deriveUnifiedRiskLevelFromScore, getSeverityBadgeClass } from '../lib/severity';
import { STAT_LABELS } from '../constants/labels';
import { getClusterDisplayName } from '../lib/clusterDisplay';
import { reconcileAttackGraphEntryExit } from '../lib/attackPathGraphTopology';
import { UI_TD_COMPACT, UI_TH, UI_TR, UI_TABLE } from '../lib/tableChrome';
import {
  topContributingDimension, scoreForPod, riskBandFrom, riskLabelUpper,
  riskLabelTitle, rankImpact, rankRisk, CAPABILITY_KNOWLEDGE,
  EDGE_TO_TECHNIQUE_CATEGORIES, IMPACT_RANK, RISK_RANK,
  type RiskBand,
} from '../lib/riskScoring';

/** Dashboard-specific types */
type RuntimeStatus = 'confirmed' | 'inferred' | 'not_observed';

interface RuntimeConfirmationItem {
  id: string;
  label: string;
  status: RuntimeStatus;
  confidence?: number;
  source?: string;
}

function formatPct(value?: number): string {
  if (value == null || Number.isNaN(value)) return '-';
  const normalized = value <= 1 ? value * 100 : value;
  return `${Math.round(normalized)}%`;
}

function normalizeNodeType(type?: string): string {
  return String(type || '')
    .replace(/([a-z])([A-Z])/g, '$1_$2')
    .toLowerCase()
    .replace(/-/g, '_');
}

function nodeTypeLabel(type?: string): string {
  const map: Record<string, string> = {
    pod: 'Pod',
    serviceaccount: 'ServiceAccount',
    service_account: 'ServiceAccount',
    clusterrolebinding: 'ClusterRoleBinding',
    cluster_role_binding: 'ClusterRoleBinding',
    rolebinding: 'RoleBinding',
    role_binding: 'RoleBinding',
    clusterrole: 'ClusterRole',
    cluster_role: 'ClusterRole',
    role: 'Role',
    node: 'Node',
    secret: 'Secret',
  };
  const normalized = normalizeNodeType(type);
  return map[normalized] || (type ? String(type).replace(/_/g, ' ') : 'Resource');
}

function nodeDisplayName(node: AttackPath['nodes'][number], index: number): string {
  const name = String(node.properties?.name || '').trim();
  const type = normalizeNodeType(node.type);
  if ((type === 'clusterrole' || type === 'cluster_role') && name.toLowerCase().includes('cluster-admin')) {
    return 'ClusterRole (cluster-admin)';
  }
  if (name && !name.startsWith('uid://')) return name;
  if (index === 0 && type === 'pod') return 'Pod';
  return nodeTypeLabel(node.type);
}

function pathLabelFromPrimitive(path?: AttackPath, chain?: AttackChain): string {
  if (path?.nodes?.length) {
    const labels = path.nodes.map((node, index) => {
      const type = normalizeNodeType(node.type);
      if (type === 'serviceaccount' || type === 'service_account') return 'SA';
      if (type === 'clusterrole' || type === 'cluster_role') {
        const name = String(node.properties?.name || '').trim();
        return name.toLowerCase().includes('cluster-admin') ? 'cluster-admin' : 'ClusterRole';
      }
      if (type === 'clusterrolebinding' || type === 'cluster_role_binding') return 'ClusterRoleBinding';
      if (type === 'rolebinding' || type === 'role_binding') return 'RoleBinding';
      if (type === 'node') return 'Node';
      return index === 0 ? 'Pod' : nodeTypeLabel(node.type);
    });
    return labels.filter((label, index) => index === 0 || label !== labels[index - 1]).join(' -> ');
  }
  if (chain?.steps?.some((step) => String(step.technique_id).includes('CLUSTER_ADMIN'))) {
    return 'Pod -> SA -> ClusterRoleBinding -> cluster-admin';
  }
  if (chain?.steps?.some((step) => String(step.technique_id).includes('HOST') || String(step.technique_id).includes('ESCAPE'))) {
    return 'Pod -> Node';
  }
  return 'Path unavailable';
}

function getTechniqueStepIndex(edgeType: string, techniques?: AttackStep[]): number | undefined {
  if (!techniques?.length) return undefined;
  const categories = EDGE_TO_TECHNIQUE_CATEGORIES[String(edgeType || '').toUpperCase()];
  if (!categories) return undefined;
  for (let i = 0; i < techniques.length; i += 1) {
    if (categories.includes(techniques[i].technique_id)) return i + 1;
  }
  return undefined;
}

function buildGraphDataFromPrimitivePaths(paths: AttackPath[], chain?: AttackChain): AttackPathGraphData {
  const nodeMap = new Map<string, AttackPathGraphData['nodes'][number]>();
  const linkMap = new Map<string, AttackPathGraphData['links'][number]>();
  const pathSignalFromTotalRisk = (totalRisk: number) => {
    if (totalRisk >= 9) return 'critical';
    if (totalRisk >= 7) return 'high';
    if (totalRisk >= 4) return 'medium';
    return 'low';
  };
  const addPathId = (target: { pathIds?: string[] }, pathId: string) => {
    const next = new Set(target.pathIds ?? []);
    next.add(pathId);
    target.pathIds = Array.from(next);
  };

  paths.forEach((path) => {
    const pathId = path.path_id || `path-${nodeMap.size}`;
    path.nodes.forEach((node, index) => {
      const existing = nodeMap.get(node.id);
      const isStart = index === 0;
      const isEnd = index === path.nodes.length - 1;
      if (!existing) {
        nodeMap.set(node.id, {
          id: node.id,
          label: nodeDisplayName(node, index),
          type: normalizeNodeType(node.type),
          risk: pathSignalFromTotalRisk(path.total_risk),
          isStart,
          isEnd,
          stepIndex: index + 1,
          pathIds: [pathId],
        });
      } else {
        if (isStart) existing.isStart = true;
        if (isEnd) existing.isEnd = true;
        if (existing.stepIndex == null || existing.stepIndex > index + 1) existing.stepIndex = index + 1;
        addPathId(existing, pathId);
      }
    });

    path.edges.forEach((edge) => {
      const key = `${edge.source}->${edge.target}`;
      const mappedStepIdx = getTechniqueStepIndex(edge.type, chain?.steps);
      const existing = linkMap.get(key);
      if (!existing) {
        linkMap.set(key, {
          source: edge.source,
          target: edge.target,
          type: edge.type,
          value: path.total_risk,
          stepIndex: mappedStepIdx,
          pathIds: [pathId],
        });
      } else if (existing.stepIndex == null && mappedStepIdx != null) {
        existing.stepIndex = mappedStepIdx;
        addPathId(existing, pathId);
      } else {
        addPathId(existing, pathId);
      }
    });
  });

  return reconcileAttackGraphEntryExit({ nodes: Array.from(nodeMap.values()), links: Array.from(linkMap.values()) });
}

function filterGraphDataForChain(graphData: AttackPathGraphData, chain?: AttackChain): AttackPathGraphData {
  if (!chain) return graphData;
  const ids = new Set([
    chain.source_id,
    chain.target_id,
    chain.final_target,
    ...(chain.involved_resources || []),
  ].filter((value): value is string => Boolean(value && String(value).trim())));
  if (ids.size === 0) return graphData;
  const links = graphData.links.filter((link) => ids.has(link.source) || ids.has(link.target));
  const linkedNodeIds = new Set<string>();
  links.forEach((link) => {
    linkedNodeIds.add(link.source);
    linkedNodeIds.add(link.target);
  });
  ids.forEach((id) => linkedNodeIds.add(id));
  const nodes = graphData.nodes.filter((node) => linkedNodeIds.has(node.id));
  return nodes.length > 0 ? reconcileAttackGraphEntryExit({ nodes, links }) : graphData;
}

function findChainForPod(pod: PodWithRisk, chains: AttackChain[]): AttackChain | undefined {
  const ids = new Set<string>([
    ...(pod.riskSignals?.chainIds ?? []),
    pod.riskSignals?.maxImpactSourceChainId ?? '',
  ].filter(Boolean));
  if (ids.size > 0) {
    const direct = chains.find((chain) => ids.has(chain.chain_id) || (chain.id != null && ids.has(chain.id)));
    if (direct) return direct;
  }
  return chains.find((chain) => chain.source_id === pod.uid || chain.involved_resources?.includes(pod.uid));
}

function buildEntryScenarios(pods: PodWithRisk[], chains: AttackChain[], paths: AttackPath[]): EntryScenario[] {
  const pathById = new Map(paths.filter((path) => path.path_id).map((path) => [path.path_id!, path]));
  return pods
    .filter((pod) => pod.riskSignals?.isEntryPoint || pod.riskSignals?.hasAttackPath)
    .map((pod) => {
      const chain = findChainForPod(pod, chains);
      const chainPaths = chain?.paths?.map((id) => pathById.get(id)).filter(Boolean) as AttackPath[] | undefined;
      const selectedPaths = chainPaths && chainPaths.length > 0 ? chainPaths : [];
      const score = scoreForPod(pod);
      return {
        key: pod.uid,
        pod,
        chain,
        paths: selectedPaths,
        pathLabel: pathLabelFromPrimitive(selectedPaths[0], chain),
        riskLevel: riskBandFrom(pod.finalLevel, score),
        score,
        runtimeConfirmed:
          chain?.steps?.some((step) => step.runtime_observed) ||
          Boolean((chain?.mitre_summary?.runtime_events_matched ?? 0) > 0 || (chain?.mitre_summary?.observing_pod_count ?? 0) > 0),
      };
    })
    .sort((a, b) => {
      const aHas = a.pod.riskSignals?.hasAttackPath ? 1 : 0;
      const bHas = b.pod.riskSignals?.hasAttackPath ? 1 : 0;
      if (aHas !== bHas) return bHas - aHas;
      const impactDiff = rankImpact(b.pod.riskSignals?.maxImpact ?? b.chain?.impact) - rankImpact(a.pod.riskSignals?.maxImpact ?? a.chain?.impact);
      if (impactDiff !== 0) return impactDiff;
      const riskDiff =
        rankRisk(b.pod.finalLevel, b.score) -
        rankRisk(a.pod.finalLevel, a.score);
      if (riskDiff !== 0) return riskDiff;
      return (b.score ?? 0) - (a.score ?? 0);
    });
}

function buildSoWhat(chain?: AttackChain, pod?: PodWithRisk): string {
  const finalTarget = String(chain?.final_target || chain?.target_id || pod?.riskSignals?.maxImpact || '').trim().toLowerCase();
  const objective = String(chain?.objective || '').toUpperCase();
  if (finalTarget.includes('cluster-admin') || objective.includes('CLUSTER')) {
    return 'SO WHAT: attacker can turn this workload into cluster-wide control.';
  }
  if (objective.includes('NODE') || chain?.steps?.some((step) => step.technique_id.includes('HOST') || step.technique_id.includes('ESCAPE'))) {
    return 'SO WHAT: attacker can break out to the node and expand blast radius.';
  }
  if (objective.includes('LATERAL') || chain?.steps?.some((step) => step.technique_id.includes('LATERAL') || step.technique_id.includes('NETWORK'))) {
    return 'SO WHAT: attacker can move from this pod toward reachable workloads.';
  }
  return 'SO WHAT: attacker can escalate beyond the original pod boundary.';
}

function parseEvidence(raw: unknown): Record<string, unknown> {
  if (!raw) return {};
  if (typeof raw === 'string') {
    try {
      const parsed = JSON.parse(raw);
      return parsed && typeof parsed === 'object' ? parsed as Record<string, unknown> : {};
    } catch {
      return {};
    }
  }
  return typeof raw === 'object' ? raw as Record<string, unknown> : {};
}

function signalText(signal: RuntimeSignal): string {
  const evidence = parseEvidence(signal.evidence);
  return [
    signal.signalType,
    signal.category,
    evidence.target,
    evidence.source,
    evidence.source_rule,
    evidence.syscall,
    evidence.capability,
  ]
    .filter(Boolean)
    .join(' ')
    .toLowerCase();
}

function eventText(event: PodRuntimeSecurityEvent): string {
  return [
    event.sourceRule,
    event.signal,
    event.eventType,
    event.syscall,
    event.targetPath,
    event.capability,
    event.mitreTechnique,
  ]
    .filter(Boolean)
    .join(' ')
    .toLowerCase();
}

function runtimeSourceFromSignal(signal?: RuntimeSignal): string | undefined {
  if (!signal) return undefined;
  const evidence = parseEvidence(signal.evidence);
  const source = evidence.source_rule || evidence.sourceRule || evidence.rule || evidence.source || evidence.syscall || signal.signalType;
  return String(source || '').trim() || undefined;
}

function maxConfidence(signals: RuntimeSignal[], events: PodRuntimeSecurityEvent[]): number | undefined {
  const values = [
    ...signals.map((signal) => signal.confidence),
    ...events.map((event) => event.confidence).filter((value): value is number => typeof value === 'number'),
  ].filter((value) => Number.isFinite(value));
  return values.length ? Math.max(...values) : undefined;
}

function deriveRuntimeConfirmations(
  chain: AttackChain | undefined,
  signals: RuntimeSignal[],
  events: PodRuntimeSecurityEvent[],
): RuntimeConfirmationItem[] {
  const k8sSignals = signals.filter((signal) => {
    const text = signalText(signal);
    return text.includes('k8s') || text.includes('kubernetes') || text.includes('serviceaccount') || text.includes('token');
  });
  const k8sEvents = events.filter((event) => {
    const text = eventText(event);
    return text.includes('k8s') || text.includes('kubernetes') || text.includes('apiserver') || text.includes('serviceaccount') || text.includes('token');
  });
  const k8sStepObserved =
    chain?.steps?.some((step) =>
      step.runtime_observed &&
      (step.technique_id.includes('TOKEN') || step.technique_id.includes('KUBELET') || step.technique_id.includes('API')),
    ) ?? false;

  const shellSignals = signals.filter((signal) => {
    const text = signalText(signal);
    return text.includes('shell') || text.includes('exec') || text.includes('t1059');
  });
  const shellEvents = events.filter((event) => {
    const text = eventText(event);
    return text.includes('exec') || text.includes('/bin/sh') || text.includes('bash') || text.includes('shell');
  });

  const lateralSignals = signals.filter((signal) => {
    const text = signalText(signal);
    return text.includes('lateral') || text.includes('network_queue') || text.includes('network');
  });
  const lateralEvents = events.filter((event) => {
    const text = eventText(event);
    return text.includes('lateral') || text.includes('connect') || text.includes('network');
  });

  const k8sStatus: RuntimeStatus =
    k8sSignals.length > 0 || k8sEvents.length > 0 || k8sStepObserved
      ? 'confirmed'
      : chain?.steps?.some((step) => step.output_caps.includes('SA_TOKEN') || step.technique_id.includes('TOKEN'))
        ? 'inferred'
        : 'not_observed';
  const shellStatus: RuntimeStatus =
    shellSignals.length > 0 || shellEvents.length > 0
      ? 'confirmed'
      : chain?.steps?.some((step) => step.technique_id.includes('ESCAPE') || step.technique_id.includes('EXEC'))
        ? 'inferred'
        : 'not_observed';
  const lateralStatus: RuntimeStatus =
    lateralSignals.length > 0 || lateralEvents.length > 0
      ? 'confirmed'
      : chain?.steps?.some((step) => step.technique_id.includes('LATERAL') || step.technique_id.includes('NETWORK'))
        ? 'inferred'
        : 'not_observed';

  return [
    {
      id: 'k8s-api',
      label: 'K8s API access detected',
      status: k8sStatus,
      confidence: maxConfidence(k8sSignals, k8sEvents),
      source: k8sEvents[0]?.sourceRule ? `Falco rule "${k8sEvents[0].sourceRule}"` : runtimeSourceFromSignal(k8sSignals[0]),
    },
    {
      id: 'shell-exec',
      label: 'Shell execution detected',
      status: shellStatus,
      confidence: maxConfidence(shellSignals, shellEvents),
      source: shellEvents[0]?.sourceRule ? `Falco rule "${shellEvents[0].sourceRule}"` : runtimeSourceFromSignal(shellSignals[0]),
    },
    {
      id: 'lateral',
      label: lateralStatus === 'not_observed' ? 'No lateral movement observed' : 'Lateral movement signal detected',
      status: lateralStatus,
      confidence: maxConfidence(lateralSignals, lateralEvents),
      source: lateralEvents[0]?.sourceRule ? `Falco rule "${lateralEvents[0].sourceRule}"` : runtimeSourceFromSignal(lateralSignals[0]),
    },
  ];
}

function capabilitiesFromChain(chain?: AttackChain): Array<{ id: string; explanation: string; riskPoints?: number }> {
  if (!chain?.steps?.length) return [];
  const caps = new Set<string>();
  chain.steps.forEach((step) => {
    step.output_caps?.forEach((cap) => caps.add(cap));
    const id = String(step.technique_id || '').toUpperCase();
    if (id.includes('TOKEN')) caps.add('SA_TOKEN');
    if (id.includes('CLUSTER_ADMIN')) caps.add('CLUSTER_ADMIN');
    if (id.includes('HOST') || id.includes('ESCAPE')) caps.add('NODE_SHELL_ACCESS');
  });
  return Array.from(caps)
    .filter((id) => id !== 'CONTAINER_ACCESS')
    .map((id) => ({
      id,
      explanation: CAPABILITY_KNOWLEDGE[id]?.explanation || id.replace(/_/g, ' ').toLowerCase(),
      riskPoints: CAPABILITY_KNOWLEDGE[id]?.riskPoints,
    }));
}

function statusClass(status: RuntimeStatus): string {
  if (status === 'confirmed') return 'border-emerald-500/30 bg-emerald-500/10 text-emerald-300';
  if (status === 'inferred') return 'border-amber-500/35 bg-amber-500/10 text-amber-200';
  return 'border-border bg-surface/40 text-muted';
}

function coverageBadgeClass(item: MitreCoverageItem): string {
  if (item.status === 'not_covered') return 'border-l-[3px] border-l-error/80 bg-error/25 text-error';
  if (item.status === 'inferred') return 'border-l-[3px] border-l-amber-500/70 bg-amber-950/20 text-amber-100';
  if (item.status === 'observed') return 'border-l-[3px] border-l-emerald-500/70 bg-emerald-950/20 text-emerald-100';
  return 'border-l-[3px] border-l-border bg-surface/30 text-muted';
}

function selectedStepForNode(node: AttackPathGraphData['nodes'][number] | undefined, chain?: AttackChain): AttackStep | undefined {
  if (!node || !chain?.steps?.length) return undefined;
  const index = typeof node.stepIndex === 'number' ? Math.max(0, node.stepIndex - 1) : 0;
  return chain.steps[index] ?? chain.steps[Math.max(0, index - 1)] ?? chain.steps[0];
}

const MetricTile: React.FC<{ label: string; value: string; hint: string }> = ({ label, value, hint }) => (
  <div className="rounded-lg border border-border/70 bg-surface/30 px-3 py-2" title={hint}>
    <p className="text-micro text-muted-2 uppercase tracking-wide">{label}</p>
    <p className="mt-0.5 text-body font-semibold text-text tabular-nums">{value}</p>
  </div>
);

const NodeEvidenceBlock: React.FC<{
  node?: AttackPathGraphData['nodes'][number];
  chain?: AttackChain;
}> = ({ node, chain }) => {
  const step = selectedStepForNode(node, chain);
  if (!node) {
    return (
      <div className="rounded-lg border border-border bg-surface/30 p-3">
        <p className="text-caption text-muted">Capability flow, MITRE mapping, and runtime grounding.</p>
      </div>
    );
  }
  return (
    <div className="rounded-lg border border-border bg-surface/30 p-3">
      <div className="flex items-center gap-2 mb-3">
        <PanelRight className="w-4 h-4 text-brand" />
        <div className="min-w-0">
          <p className="text-body font-semibold text-text truncate">{node.label}</p>
          <p className="text-caption text-muted">{nodeTypeLabel(String(node.type))}</p>
        </div>
      </div>
      {step ? (
        <div className="space-y-3 text-caption">
          <div>
            <p className="text-micro text-muted-2 uppercase tracking-wide mb-1">Capability in / out</p>
            <p className="text-muted">
              In: <span className="text-text font-mono">{step.input_caps?.length ? step.input_caps.join(', ') : '-'}</span>
            </p>
            <p className="text-muted">
              Out: <span className="text-text font-mono">{step.output_caps?.length ? step.output_caps.join(', ') : '-'}</span>
            </p>
          </div>
          <div>
            <p className="text-micro text-muted-2 uppercase tracking-wide mb-1">MITRE</p>
            <div className="flex flex-wrap gap-1">
              {(step.mitre_techniques || []).length > 0 ? (
                step.mitre_techniques?.map((mt) => (
                  <span key={mt.id} className="rounded border border-amber-500/35 bg-amber-500/10 px-1.5 py-0.5 text-micro font-mono text-amber-200">
                    {mt.id}
                  </span>
                ))
              ) : (
                <span className="rounded border border-border bg-surface/50 px-1.5 py-0.5 text-micro font-mono text-muted">
                  {step.technique_id}
                </span>
              )}
            </div>
          </div>
          <div>
            <p className="text-micro text-muted-2 uppercase tracking-wide mb-1">Runtime evidence</p>
            <div className="flex flex-wrap items-center gap-1.5">
              {step.runtime_observed ? (
                <span className="inline-flex items-center gap-1 rounded border border-emerald-500/35 bg-emerald-500/10 px-2 py-0.5 text-micro text-emerald-300">
                  <Eye className="w-3 h-3" /> observed
                </span>
              ) : (
                <span className="rounded border border-border bg-surface/50 px-2 py-0.5 text-micro text-muted">not observed</span>
              )}
              {step.runtime_grounding_score != null ? (
                <span className="rounded border border-sky-500/35 bg-sky-500/10 px-2 py-0.5 text-micro text-sky-200">
                  {formatPct(step.runtime_grounding_score)} grounded
                </span>
              ) : null}
            </div>
          </div>
        </div>
      ) : (
        <p className="text-caption text-muted">No step-level enrichment for this node.</p>
      )}
    </div>
  );
};

const AttackDetailContent: React.FC<{
  scenario?: EntryScenario;
  graphData: AttackPathGraphData;
  selectedNode?: AttackPathGraphData['nodes'][number];
  onNodeClick: (nodeId: string) => void;
  runtimeItems: RuntimeConfirmationItem[];
  capabilities: Array<{ id: string; explanation: string; riskPoints?: number }>;
  gaps: MitreCoverageItem[];
  correlationPrecision?: number;
  avgGrounding?: number;
  runtimeInfluenceRatio?: number;
  semanticMode?: GraphSemanticMode;
}> = ({
  scenario,
  graphData,
  selectedNode,
  onNodeClick,
  runtimeItems,
  capabilities,
  gaps,
  correlationPrecision,
  avgGrounding,
  runtimeInfluenceRatio,
  semanticMode = 'exploitability',
}) => {
  const graphVisibility = useFeatureVisibility('attack_paths');
  const { graphSemanticMode: incidentGraphMode } = useIncidentMode({
    runtimeConfirmed: scenario?.runtimeConfirmed,
  });
  const graphTrust = useGraphTrustContext('attack_paths', incidentGraphMode ?? semanticMode, {
    runtimeConfirmedNodeIds: scenario?.runtimeConfirmed && scenario.pod.uid ? [scenario.pod.uid] : undefined,
  });

  if (!scenario) {
    return <PageEmpty title="No entry point selected" description="No attack path detail is available in this scope." className="py-8" />;
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <Route className="w-4 h-4 text-brand" />
            <h3 className="text-section-title font-semibold text-text truncate">{scenario.pod.name}</h3>
            <Badge severity={scenario.riskLevel} uppercase>{scenario.riskLevel}</Badge>
            {scenario.runtimeConfirmed ? (
              <span className="inline-flex items-center gap-1 text-micro px-2 py-0.5 rounded border border-emerald-500/30 bg-emerald-500/10 text-emerald-300">
                <Eye className="w-3 h-3" /> Runtime confirmed
              </span>
            ) : null}
          </div>
          <p className="mt-1 text-caption text-muted">Path: <span className="text-text">{scenario.pathLabel}</span></p>
          <p className="mt-1 text-caption text-muted leading-snug">{buildSoWhat(scenario.chain, scenario.pod)}</p>
          {scenario.pod.riskSignals?.summary ? <p className="mt-1 text-caption text-muted leading-snug">WHY: {scenario.pod.riskSignals.summary}</p> : null}
        </div>
        {scenario.score != null ? (
          <div className="text-right">
            <p className="text-micro text-muted-2 uppercase tracking-wide">Score</p>
            <p className="text-xl font-bold text-text tabular-nums">{Math.round(scenario.score)}<span className="text-caption text-muted">/100</span></p>
          </div>
        ) : null}
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[minmax(0,1fr),320px] gap-4">
        <div className="rounded-lg border border-border bg-base/40 overflow-hidden">
          <GraphSemanticBanner mode={semanticMode} className="m-2 mb-0" />
          <GraphVisibilityOverlay
            semanticState={graphVisibility.semanticState}
            reason={graphVisibility.reason}
            className="mx-2"
          />
          <AttackPathGraph
            data={graphData}
            className="h-[420px] w-full"
            onNodeClick={onNodeClick}
            semanticMode={incidentGraphMode ?? semanticMode}
            graphTrust={graphTrust}
          />
        </div>
        <NodeEvidenceBlock node={selectedNode} chain={scenario.chain} />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
        <div className="rounded-lg border border-border bg-surface/30 p-3">
          <p className="text-micro text-muted-2 uppercase tracking-wide mb-2">Capabilities Acquired</p>
          {capabilities.length === 0 ? (
            <p className="text-caption text-muted">No path capability outputs found.</p>
          ) : (
            <div className="space-y-2">
              {capabilities.map((capability) => (
                <div key={capability.id} className="rounded-lg border border-border/70 bg-base/30 px-3 py-2">
                  <div className="flex items-center gap-2">
                    <KeyRound className="w-3.5 h-3.5 text-brand" />
                    <span className="font-mono text-caption font-semibold text-text">{capability.id}</span>
                  </div>
                  <p className="mt-1 text-caption text-muted leading-snug">{capability.explanation}</p>
                  {capability.riskPoints != null ? <p className="mt-1 text-micro text-muted-2">CKDB risk points: {capability.riskPoints.toFixed(2)}</p> : null}
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="rounded-lg border border-border bg-surface/30 p-3">
          <p className="text-micro text-muted-2 uppercase tracking-wide mb-2">Runtime Confirmation</p>
          <div className="space-y-2">
            {runtimeItems.map((item) => (
              <div key={item.id} className={`rounded-lg border px-3 py-2 ${statusClass(item.status)}`}>
                <p className="text-caption font-semibold text-text">{item.label}</p>
                <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-micro">
                  <span>Status: {item.status.replace('_', ' ')}</span>
                  {item.confidence != null ? <span>confidence: {item.confidence.toFixed(2)}</span> : null}
                  {item.source ? <span>source: {item.source}</span> : null}
                </div>
              </div>
            ))}
          </div>
          {scenario.chain?.steps?.length ? (
            <div className="mt-3 border-t border-border/60 pt-3">
              <p className="text-micro text-muted-2 uppercase tracking-wide mb-2">Step Runtime</p>
              <div className="space-y-1.5">
                {scenario.chain.steps.map((step, index) => (
                  <div key={`${step.technique_id}-${index}`} className="rounded-lg border border-border/70 bg-base/25 px-2.5 py-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-mono text-meta font-semibold text-text">{step.technique_id}</span>
                      {step.runtime_observed ? (
                        <span className="inline-flex items-center gap-1 rounded border border-emerald-500/35 bg-emerald-500/10 px-1.5 py-0.5 text-micro text-emerald-300">
                          <Eye className="w-3 h-3" /> observed
                        </span>
                      ) : (
                        <span className="rounded border border-border bg-surface/50 px-1.5 py-0.5 text-micro text-muted">not observed</span>
                      )}
                      {step.runtime_grounding_score != null ? (
                        <span className="rounded border border-sky-500/35 bg-sky-500/10 px-1.5 py-0.5 text-micro text-sky-200">
                          {formatPct(step.runtime_grounding_score)} grounded
                        </span>
                      ) : null}
                    </div>
                    {step.mitre_techniques?.length ? (
                      <div className="mt-1 flex flex-wrap gap-1">
                        {step.mitre_techniques.map((mt) => (
                          <span key={mt.id} className="rounded border border-amber-500/35 bg-amber-500/10 px-1.5 py-0.5 text-micro font-mono text-amber-200">
                            {mt.id}
                          </span>
                        ))}
                      </div>
                    ) : null}
                  </div>
                ))}
              </div>
            </div>
          ) : null}
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
        <div className="rounded-lg border border-border bg-surface/30 p-3">
          <p className="text-micro text-muted-2 uppercase tracking-wide mb-2">Detection Gaps</p>
          {gaps.length === 0 ? (
            <p className="text-caption text-muted">No not-covered or inferred MITRE gaps on this path.</p>
          ) : (
            <div className="overflow-x-auto rounded-lg border border-border/60">
              <table className={UI_TABLE}>
                <thead>
                  <tr>
                    <th className={UI_TH}>MITRE</th>
                    <th className={`${UI_TH} w-24`}>Priority</th>
                    <th className={UI_TH}>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {gaps.map((gap) => (
                    <tr key={`${gap.mitre_id}-${gap.status}`} className={`${UI_TR} ${coverageBadgeClass(gap)}`}>
                      <td className={`${UI_TD_COMPACT} font-mono font-semibold text-text`}>{gap.mitre_id}</td>
                      <td className={`${UI_TD_COMPACT} font-semibold`}>{gap.priority ?? 'LOW'}</td>
                      <td className={UI_TD_COMPACT}>{gap.status.replace(/_/g, ' ')}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        <div className="rounded-lg border border-border bg-surface/30 p-3">
          <p className="text-micro text-muted-2 uppercase tracking-wide mb-2">Advanced Metrics</p>
          <div className="space-y-2">
            <div className="rounded-lg border border-border/70 bg-base/25 p-2">
              <p className="mb-2 text-micro text-muted-2 uppercase tracking-wide">Runtime-window metrics</p>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                <MetricTile
                  label="Correlation precision"
                  value={correlationPrecision != null ? correlationPrecision.toFixed(2) : '-'}
                  hint="Runtime MITRE events matched to path MITRE events in the backend correlation window."
                />
                <MetricTile
                  label="Avg grounding"
                  value={avgGrounding != null ? avgGrounding.toFixed(2) : '-'}
                  hint="Average step-level runtime grounding score across this chain."
                />
              </div>
            </div>
            <div className="rounded-lg border border-border/70 bg-base/25 p-2">
              <p className="mb-2 text-micro text-muted-2 uppercase tracking-wide">Score snapshot metric</p>
              <MetricTile
                label="Runtime influence ratio"
                value={runtimeInfluenceRatio != null ? runtimeInfluenceRatio.toFixed(2) : '-'}
                hint="Runtime threat dimension divided by latest unified score when available."
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export const Dashboard: React.FC = () => {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const selectedClusterId = useClusterStore((s) => s.selectedClusterId);
  const timeWindowMinutes = useTimeWindowStore((s) => s.valueMinutes);
  const sinceMinutes = timeWindowMinutes > 0 ? timeWindowMinutes : undefined;

  const { id: personaId, profile } = usePersona();
  const { user: opUser, ownership, telemetry } = useOperationalContext();
  const { active: incidentActive, graphSemanticMode: incidentGraphMode } = useIncidentMode();
  const bootstrapLoadPolicy = useMemo(
    () => buildDashboardLoadPolicy(opUser, getPersonaWidgetCandidates(personaId)),
    [opUser, personaId],
  );

  const {
    stats,
    insightsSummary,
    clusters,
    topRisks,
    notifications,
    pceSummary,
    pceTrend,
    trendDays,
    setTrendDays,
    threatVelocity,
    topRiskyPods,
    exploitedCapCount,
    attackPathCount,
    attackPathSummary,
    pipelineHealth,
    entryPods,
    attackChains,
    primitivePaths,
    attackGraphData,
    coreReady,
    initialError,
    partialErrors,
    sectionState,
    refresh,
  } = useDashboardData(selectedClusterId, sinceMinutes, bootstrapLoadPolicy);

  const dashComposition = useMemo(
    () =>
      getDashboardComposition(personaId, opUser, ownership, telemetry, {
        hasOpenFindings:
          Number(insightsSummary?.riskLevelCounts?.critical ?? insightsSummary?.critical ?? stats.critical ?? 0) > 0 ||
          Number(insightsSummary?.riskLevelCounts?.high ?? insightsSummary?.high ?? 0) > 0,
        hasAttackPaths: attackPathCount > 0,
      }),
    [personaId, opUser, ownership, telemetry, insightsSummary, stats.critical, attackPathCount],
  );

  const [dashboardMode, setDashboardMode] = useState<DashboardMode>(dashComposition.defaultMode);

  useEffect(() => {
    setDashboardMode(dashComposition.defaultMode);
  }, [dashComposition.defaultMode, personaId]);
  const [selectedEntryUid, setSelectedEntryUid] = useState<string | null>(null);
  const [panelOpen, setPanelOpen] = useState(false);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const [runtimeOnly, setRuntimeOnly] = useState(false);
  const [criticalOnly, setCriticalOnly] = useState(false);
  const [runtimeSignals, setRuntimeSignals] = useState<RuntimeSignal[]>([]);
  const [runtimeEvents, setRuntimeEvents] = useState<PodRuntimeSecurityEvent[]>([]);
  const [sevDelta, setSevDelta] = useState<{ c: number; h: number; m: number; l: number } | null>(null);
  const prevSevRef = useRef<{ c: number; h: number; m: number; l: number } | null>(null);
  const scenarioPanelRef = useRef<HTMLElement>(null);
  const [isRefreshing, setIsRefreshing] = useState(false);

  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.STATS_CLUSTERS));
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);

  const fetchData = useCallback(async () => {
    if (coreReady) setIsRefreshing(true);
    await refresh();
    setIsRefreshing(false);
  }, [coreReady, refresh]);

  usePolling(fetchData, intervalMs, { refreshTrigger });

  useEffect(() => {
    if (coreReady) void refresh();
  }, [trendDays]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    prevSevRef.current = null;
    setSevDelta(null);
  }, [selectedClusterId, sinceMinutes]);

  useEffect(() => {
    if (!insightsSummary) return;
    const c = Number(insightsSummary.riskLevelCounts?.critical ?? insightsSummary.critical ?? 0);
    const h = Number(insightsSummary.riskLevelCounts?.high ?? insightsSummary.high ?? 0);
    const m = Number(insightsSummary.riskLevelCounts?.medium ?? insightsSummary.medium ?? 0);
    const l = Number(insightsSummary.riskLevelCounts?.low ?? insightsSummary.low ?? 0);
    const curr = { c, h, m, l };
    const prev = prevSevRef.current;
    if (prev) {
      setSevDelta({
        c: curr.c - prev.c,
        h: curr.h - prev.h,
        m: curr.m - prev.m,
        l: curr.l - prev.l,
      });
    } else {
      setSevDelta(null);
    }
    prevSevRef.current = curr;
  }, [insightsSummary]);

  /** Build /risks URL with current scope (cluster + time) so Risk Center shows same data as Dashboard Security Risks. */
  const risksUrl = useMemo(() => {
    const params = new URLSearchParams();
    if (selectedClusterId?.trim()) params.set('clusterId', selectedClusterId.trim());
    if (sinceMinutes != null && sinceMinutes > 0) params.set('sinceMinutes', String(sinceMinutes));
    const qs = params.toString();
    return qs ? `/risks?${qs}` : '/risks';
  }, [selectedClusterId, sinceMinutes]);

  // Build day labels for fallback when API returns empty (chart still shows axis)
  const trendDayLabels = useMemo(() => {
    const out: string[] = [];
    for (let i = trendDays - 1; i >= 0; i--) {
      const d = new Date();
      d.setDate(d.getDate() - i);
      out.push(d.toISOString().slice(0, 10));
    }
    return out;
  }, [trendDays]);

  /** Merged data for Velocity (risks) + PCE trend: one chart, two lines by date */
  const trendChartData = useMemo(() => {
    const byDate: Record<string, { risk: number; pce: number }> = {};
    trendDayLabels.forEach((d) => { byDate[d] = { risk: 0, pce: 0 }; });
    threatVelocity.forEach((p) => {
      const d = p.date;
      if (!byDate[d]) byDate[d] = { risk: 0, pce: 0 };
      byDate[d].risk = (p.critical ?? 0) + (p.high ?? 0) + (p.medium ?? 0) + (p.low ?? 0);
    });
    pceTrend.forEach((p) => {
      const d = p.date;
      if (!byDate[d]) byDate[d] = { risk: 0, pce: 0 };
      byDate[d].pce = (p.critical ?? 0) + (p.high ?? 0) + (p.medium ?? 0) + (p.low ?? 0);
    });
    return trendDayLabels.map((date) => ({ name: date, ...byDate[date] })).sort((a, b) => a.name.localeCompare(b.name));
  }, [trendDayLabels, threatVelocity, pceTrend]);

  const riskEntityRows = useMemo(
    () =>
      topRiskyPods.slice(0, 5).map((pod) => ({
        pod,
        dim: topContributingDimension(pod.dimensions),
        level: deriveUnifiedRiskLevelFromScore(pod.totalScore) ?? 'low',
      })),
    [topRiskyPods],
  );

  const highRiskPodCount = useMemo(
    () =>
      topRiskyPods.filter((p) => {
        const lvl = (deriveUnifiedRiskLevelFromScore(p.totalScore) || '').toLowerCase();
        return lvl === 'high' || lvl === 'critical';
      }).length,
    [topRiskyPods],
  );
  const activeFindingsCount = Number(insightsSummary?.total ?? stats.insights ?? 0);
  const criticalRiskFindingCount = Number(
    insightsSummary?.riskLevelCounts?.critical ?? insightsSummary?.critical ?? stats.critical ?? 0,
  );
  const criticalAttackPathCount = Number(attackPathSummary?.criticalPaths ?? 0);

  const clusterList = selectedClusterId ? clusters.filter((c) => c.id === selectedClusterId) : clusters;
  const dashboardRoleMeta = useMemo(() => {
    if (personaId === 'admin') {
      return {
        chartTitle: 'Platform exposure trend',
        chartDescription: 'Findings, exposed capabilities, and platform health for full administrative scope.',
        focusLabel: 'Admin focus',
        focusText: 'Full feature and data visibility. Use this view to verify platform posture, worker health, and RBAC drift.',
        primaryAction: 'Open monitoring',
        primaryRoute: '/monitoring',
        secondaryAction: 'Review policies',
        secondaryRoute: '/policies',
        riskTitle: 'Risk controls',
      };
    }
    if (personaId === 'operator') {
      return {
        chartTitle: 'Triage exposure trend',
        chartDescription: 'Operational findings and attack-path exposure that need queue and response decisions.',
        focusLabel: 'Operator focus',
        focusText: 'Prioritize active findings, entry points, and runtime-confirmed paths without administrative noise.',
        primaryAction: 'Open attack paths',
        primaryRoute: '/attack-paths',
        secondaryAction: 'Open triage',
        secondaryRoute: risksUrl,
        riskTitle: 'Triage queue',
      };
    }
    return {
      chartTitle: 'My exposure trend',
      chartDescription: 'Visible findings and capability exposure for the scope allowed by your role.',
      focusLabel: 'Viewer focus',
      focusText: 'Read-only posture and evidence. Actions are hidden when the role cannot execute them.',
      primaryAction: 'Open risks',
      primaryRoute: risksUrl,
      secondaryAction: 'Open attack paths',
      secondaryRoute: '/attack-paths',
      riskTitle: 'Exposure summary',
    };
  }, [personaId, risksUrl]);

  const dashboardScopeText = selectedClusterId ? stats.clusterName ?? selectedClusterId : 'All clusters';
  const dashboardTimeText = sinceMinutes ? `Last ${sinceMinutes} min` : 'All time';

  const clusterStatsRow = (
    <div className="rounded-lg border border-border/60 bg-surface/20 px-3 py-2.5 flex flex-wrap gap-x-6 gap-y-3 text-caption">
      <button type="button" className="text-left hover:text-brand transition-colors" onClick={() => navigate('/clusters')}>
        <span className="text-typo-micro block">{STAT_LABELS.CLUSTERS}</span>
        <span className="font-mono font-semibold text-body text-text">{stats.clusters}</span>
      </button>
      <button type="button" className="text-left hover:text-brand transition-colors" onClick={() => navigate('/resources')}>
        <span className="text-typo-micro block">
          {selectedClusterId ? `Pods (${stats.clusterName ?? 'cluster'})` : STAT_LABELS.PODS}
        </span>
        <span className="font-mono font-semibold text-body text-text">{stats.pods}</span>
      </button>
      <button type="button" className="text-left hover:text-brand transition-colors" onClick={() => navigate('/monitoring')}>
        <span className="text-typo-micro block">{STAT_LABELS.AGENTS}</span>
        <span className="font-mono font-semibold text-body text-text">{stats.agents}</span>
      </button>
      <button type="button" className="text-left hover:text-brand transition-colors" onClick={() => navigate(risksUrl)}>
        <span className="text-typo-micro block">Active findings</span>
        <span className="font-mono font-semibold text-body text-text">{activeFindingsCount}</span>
      </button>
      <button type="button" className="text-left hover:text-brand transition-colors" onClick={() => navigate('/resources')}>
        <span className="text-typo-micro block">High / critical scored pods</span>
        <span className="font-mono font-semibold text-body text-text">{highRiskPodCount}</span>
      </button>
    </div>
  );

  const clusterHealthCard = (
    <Card className="p-0 overflow-hidden border-border/60 shadow-none" title="Cluster health" variant="secondary">
      <div className="divide-y divide-border/80">
        {clusterList.length === 0 && (
          <div className="p-3">
            <PageEmpty title="No clusters" description="Sync clusters in Core." className="py-3" />
          </div>
        )}
        {clusterList.map((cluster) => (
          <div
            key={cluster.id}
            role="button"
            tabIndex={0}
            className="px-3 py-2.5 hover:bg-surface-2/40 transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 focus-visible:ring-inset"
            onClick={() => navigate(`/clusters/${cluster.id}`)}
            onKeyDown={(event) => {
              if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault();
                navigate(`/clusters/${cluster.id}`);
              }
            }}
          >
            <div className="flex justify-between items-center mb-1.5">
              <div className="flex items-center min-w-0">
                <div
                  className={`w-1.5 h-1.5 rounded-full mr-2 shrink-0 ${
                    cluster.healthScore > 80 ? 'bg-emerald-500' : cluster.healthScore > 50 ? 'bg-yellow-500' : 'bg-red-500'
                  }`}
                />
                <span className="truncate text-body font-medium text-text">{getClusterDisplayName(cluster)}</span>
              </div>
              <span
                className={`text-micro px-1.5 py-0.5 rounded-full font-bold shrink-0 ${
                  cluster.healthScore > 80
                    ? 'text-emerald-400 bg-emerald-500/10'
                    : cluster.healthScore > 50
                      ? 'text-yellow-400 bg-yellow-500/10'
                      : 'text-red-400 bg-red-500/10'
                }`}
              >
                {cluster.healthScore}%
              </span>
            </div>
            <div className="w-full bg-surface-2 rounded-full h-1 overflow-hidden">
              <div
                className={`h-1 rounded-full ${
                  cluster.healthScore > 80 ? 'bg-emerald-500' : cluster.healthScore > 50 ? 'bg-yellow-500' : 'bg-red-500'
                }`}
                style={{ width: `${cluster.healthScore}%` }}
              />
            </div>
          </div>
        ))}
      </div>
    </Card>
  );

  const entryScenarios = useMemo(
    () => buildEntryScenarios(entryPods, attackChains, primitivePaths),
    [entryPods, attackChains, primitivePaths],
  );

  const filteredEntryScenarios = useMemo(
    () =>
      entryScenarios.filter((scenario) => {
        if (runtimeOnly && !scenario.runtimeConfirmed) return false;
        if (criticalOnly && scenario.riskLevel !== 'critical') return false;
        return true;
      }),
    [criticalOnly, entryScenarios, runtimeOnly],
  );

  useEffect(() => {
    const podUid = searchParams.get('podUid');
    if (!podUid || entryScenarios.length === 0) return;
    const match = entryScenarios.find((s) => s.pod.uid === podUid);
    if (match) {
      setSelectedEntryUid(match.key);
      setPanelOpen(true);
      setDashboardMode('full');
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          next.delete('podUid');
          return next;
        },
        { replace: true }
      );
    }
  }, [entryScenarios, searchParams, setSearchParams]);

  const selectedScenario = useMemo(
    () => entryScenarios.find((scenario) => scenario.key === selectedEntryUid) || entryScenarios[0],
    [entryScenarios, selectedEntryUid],
  );

  useEffect(() => {
    setSelectedNodeId(null);
  }, [selectedScenario?.key]);

  useEffect(() => {
    const uid = selectedScenario?.pod.uid;
    if (!uid) {
      setRuntimeSignals([]);
      setRuntimeEvents([]);
      return;
    }
    let cancelled = false;
    Promise.allSettled([
      api.getRuntimeSignalsByPod(uid, { sinceMinutes: sinceMinutes ?? 1440, limit: 80 }),
      api.getPodRuntimeSecurityEvents(uid, 80),
    ]).then(([signalsResult, eventsResult]) => {
      if (cancelled) return;
      setRuntimeSignals(signalsResult.status === 'fulfilled' ? signalsResult.value : []);
      setRuntimeEvents(eventsResult.status === 'fulfilled' ? eventsResult.value : []);
    });
    return () => {
      cancelled = true;
    };
  }, [selectedScenario?.pod.uid, sinceMinutes]);

  const selectedGraphData = useMemo(() => {
    if (selectedScenario?.paths.length) return buildGraphDataFromPrimitivePaths(selectedScenario.paths, selectedScenario.chain);
    return filterGraphDataForChain(attackGraphData, selectedScenario?.chain);
  }, [attackGraphData, selectedScenario]);

  const selectedNode = useMemo(
    () => selectedGraphData.nodes.find((node) => node.id === selectedNodeId),
    [selectedGraphData.nodes, selectedNodeId],
  );

  const selectedRuntimeItems = useMemo(
    () => deriveRuntimeConfirmations(selectedScenario?.chain, runtimeSignals, runtimeEvents),
    [runtimeEvents, runtimeSignals, selectedScenario?.chain],
  );

  const selectedCapabilities = useMemo(() => capabilitiesFromChain(selectedScenario?.chain), [selectedScenario?.chain]);

  const selectedGaps = useMemo(
    () =>
      (selectedScenario?.chain?.mitre_coverage || [])
        .filter((item) => item.status === 'not_covered' || item.status === 'inferred')
        .sort((a, b) => rankRisk(b.priority) - rankRisk(a.priority)),
    [selectedScenario?.chain],
  );

  const selectedAvgGrounding = useMemo(() => {
    const values = (selectedScenario?.chain?.steps || [])
      .map((step) => step.runtime_grounding_score)
      .filter((value): value is number => typeof value === 'number' && Number.isFinite(value));
    if (!values.length) return undefined;
    return values.reduce((sum, value) => sum + value, 0) / values.length;
  }, [selectedScenario?.chain]);

  const selectedRuntimeInfluenceRatio = useMemo(() => {
    const uid = selectedScenario?.pod.uid;
    if (!uid) return undefined;
    const scoreRow = topRiskyPods.find((row) => row.resourceUid === uid);
    const score = scoreForPod(scoreRow || selectedScenario.pod);
    if (!score || !scoreRow?.dimensions) return undefined;
    return scoreRow.dimensions.runtimeThreat / Math.max(score, 1);
  }, [selectedScenario, topRiskyPods]);

  const openScenarioPanel = useCallback((scenario: EntryScenario) => {
    setSelectedEntryUid(scenario.key);
    setPanelOpen(true);
  }, []);

  useEffect(() => {
    if (!panelOpen) return;
    const root = scenarioPanelRef.current;
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        setPanelOpen(false);
        return;
      }
      if (event.key !== 'Tab' || !root) return;
      const focusable = Array.from(root.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      )).filter((el) => !el.hasAttribute('disabled') && el.offsetParent !== null);
      if (!focusable.length) return;
      const active = document.activeElement as HTMLElement | null;
      if (!active || !root.contains(active)) {
        event.preventDefault();
        focusable[0].focus();
        return;
      }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && active === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && active === last) {
        event.preventDefault();
        first.focus();
      }
    };
    window.addEventListener('keydown', onKeyDown, true);
    const t = window.setTimeout(() => {
      root?.querySelector<HTMLElement>('button, [href], input, select, textarea')?.focus();
    }, 0);
    return () => {
      window.clearTimeout(t);
      window.removeEventListener('keydown', onKeyDown, true);
      previousFocus?.focus();
    };
  }, [panelOpen]);

  const dashboardLoadMessages = useMemo(() => {
    const msgs = [...(initialError ? [`Dashboard shell is degraded: ${initialError}`] : []), ...partialErrors];
    return msgs.filter((m) => {
      if (m.includes('Attack path bundle') && !bootstrapLoadPolicy.attackBundle) return false;
      if (m.includes('Pipeline health') && !bootstrapLoadPolicy.pipelineHealth) return false;
      if (m.includes('Pod inventory for entry points') && !bootstrapLoadPolicy.entryPods) return false;
      return true;
    });
  }, [initialError, partialErrors, bootstrapLoadPolicy]);

  if (!coreReady && !initialError) return <PageLoading message="Loading dashboard…" className="min-h-[50dvh]" />;

  if (!dashComposition.pageVisible) {
    return (
      <PageLayout title={PAGE_TITLES.dashboard} description="Cluster-scoped KPIs and drill-downs.">
        <SemanticEmptyState
          state={dashComposition.pageSemantic.semanticState}
          reason={dashComposition.pageSemantic.reason}
          className="min-h-[50dvh]"
        />
      </PageLayout>
    );
  }

  return (
    <PageLayout
      title={PAGE_TITLES.dashboard}
      description="Operational KPIs and drill-downs. Risk findings follow the header cluster and time window; inventory, capability, and graph data follow cluster scope."
      actions={
        <div className="flex items-center gap-2">
          {isRefreshing ? (
            <span className="text-caption text-muted-2" role="status" aria-live="polite">
              Refreshing…
            </span>
          ) : null}
          <Button variant="secondary" onClick={() => navigate(risksUrl)}>View All Risks</Button>
        </div>
      }
    >
      <div className="space-y-6">
      <IncidentModeBanner />
      <RuntimeThreatStrip />
      <IncidentPriorityStrip
        criticalCount={criticalRiskFindingCount}
        attackPathCount={attackPathCount}
        hasOpenFindings={activeFindingsCount > 0}
        telemetryDegraded={dashboardLoadMessages.length > 0}
      />
      <PartialLoadBanner
        messages={dashboardLoadMessages}
        title={initialError ? 'Dashboard is running in degraded mode' : undefined}
        actions={
          <>
            <Button variant="secondary" size="sm" onClick={() => void fetchData()}>
              Retry dashboard data
            </Button>
            <Button variant="ghost" size="sm" onClick={() => navigate('/monitoring')}>
              Open Monitoring
            </Button>
          </>
        }
      />
      {isDashboardWidgetVisible(dashComposition, 'persona_strip') ? <PersonaDashboardStrip /> : null}
      <DashboardSubNav mode={dashboardMode} personaId={personaId} widgets={dashComposition.widgets} />

      {isDashboardSectionVisible(personaId, 'exposure', dashboardMode, dashComposition.widgets) ? (
        <div id="exposure" className="scroll-mt-24 rounded-lg border border-border bg-surface/35 p-3 sm:p-4">
            <div className="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(19rem,0.38fr)]">
              <div className="min-w-0">
                <Section title={dashboardRoleMeta.chartTitle} description={dashboardRoleMeta.chartDescription}>
                  <DashboardTrendChart
                    data={trendChartData}
                    trendDays={trendDays}
                    onTrendDaysChange={setTrendDays}
                    loading={sectionState.risks === 'loading'}
                  />
                </Section>
              </div>
              <aside className="grid content-start gap-3">
                <div className="rounded-lg border border-border/70 bg-base/25 p-3">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <p className="text-micro uppercase tracking-wide text-muted-2">{dashboardRoleMeta.focusLabel}</p>
                      <h3 className="mt-1 text-card-title font-semibold text-text">{dashboardScopeText}</h3>
                    </div>
                    <Badge variant="info" uppercase={false}>{dashboardTimeText}</Badge>
                  </div>
                  <p className="mt-2 text-caption leading-relaxed text-muted">{dashboardRoleMeta.focusText}</p>
                  <dl className="mt-3 grid grid-cols-2 gap-2 text-caption">
                    <div className="rounded-md border border-border/60 bg-surface/35 px-2.5 py-2">
                      <dt className="text-micro uppercase tracking-wide text-muted-2">Findings</dt>
                      <dd className="mt-0.5 font-mono text-body font-semibold text-text">{activeFindingsCount}</dd>
                    </div>
                    <div className="rounded-md border border-border/60 bg-surface/35 px-2.5 py-2">
                      <dt className="text-micro uppercase tracking-wide text-muted-2">Attack paths</dt>
                      <dd className="mt-0.5 font-mono text-body font-semibold text-text">{attackPathCount}</dd>
                    </div>
                    <div className="rounded-md border border-border/60 bg-surface/35 px-2.5 py-2">
                      <dt className="text-micro uppercase tracking-wide text-muted-2">Pods</dt>
                      <dd className="mt-0.5 font-mono text-body font-semibold text-text">{stats.pods}</dd>
                    </div>
                    <div className="rounded-md border border-border/60 bg-surface/35 px-2.5 py-2">
                      <dt className="text-micro uppercase tracking-wide text-muted-2">High pods</dt>
                      <dd className="mt-0.5 font-mono text-body font-semibold text-text">{highRiskPodCount}</dd>
                    </div>
                  </dl>
                  <div className="mt-3 flex flex-wrap gap-2">
                    <Button variant="secondary" size="sm" onClick={() => navigate(dashboardRoleMeta.primaryRoute)}>
                      {dashboardRoleMeta.primaryAction}
                    </Button>
                    <Button variant="ghost" size="sm" onClick={() => navigate(dashboardRoleMeta.secondaryRoute)}>
                      {dashboardRoleMeta.secondaryAction}
                    </Button>
                  </div>
                </div>
                <div className="rounded-lg border border-border/70 bg-base/20 p-3">
                  <h3 className="text-section-title font-semibold text-text">Platform snapshot</h3>
                  <div className="mt-2">{clusterStatsRow}</div>
                  {(exploitedCapCount > 0 || attackPathCount > 0) ? (
                    <div className="mt-3 flex flex-col gap-2">
                      {exploitedCapCount > 0 && (
                        <div className="flex items-center justify-between rounded-md border border-error/20 bg-error/10 px-3 py-2.5 text-meta">
                          <div className="flex items-center gap-2">
                            <Zap className="h-3 w-3 text-error shrink-0" />
                            <span>
                              <strong className="text-error">{exploitedCapCount}</strong> exploited signal(s)
                            </span>
                          </div>
                          <Button variant="ghost" size="sm" onClick={() => navigate('/capabilities')} className="!h-6 !py-0 !text-micro !text-error">
                            View
                          </Button>
                        </div>
                      )}
                      {attackPathCount > 0 && (
                          <div className="flex items-center justify-between rounded-md border border-warning-border bg-warning-background px-3 py-2.5 text-meta">
                            <div className="flex items-center gap-2">
                              <Target className="h-3 w-3 text-warning-foreground shrink-0" />
                              <span>
                                <strong className="text-warning-foreground">{attackPathCount}</strong> attack path(s)
                              </span>
                            </div>
                            <Button variant="ghost" size="sm" onClick={() => navigate('/attack-paths')} className="!h-6 !py-0 !text-micro !text-warning-foreground">
                            View
                            </Button>
                          </div>
                        )}
                      </div>
                    ) : null}
                  {personaId === 'admin' ? (
                    <div className="mt-3 max-h-48 min-w-0 overflow-y-auto rounded-lg border border-border/60">
                      {clusterHealthCard}
                    </div>
                  ) : null}
                  <div className="mt-3 rounded-lg border border-border/80 bg-surface/45 p-3">
                    <h3 className="text-section-title font-semibold text-text">Operational checks</h3>
                    <p className="mt-1 text-meta text-muted">Worker health, RBAC drift, and policy sync.</p>
                    <Button variant="ghost" size="sm" onClick={() => navigate('/monitoring')} className="mt-1.5 !h-7 !text-meta !text-violet-400">
                      Workers <ArrowRight size={11} className="ml-0.5" />
                    </Button>
                  </div>
                </div>
              </aside>
            </div>
        </div>
      ) : null}

      {!dashComposition.hideRuntimeToggles ? (
        <FilterBar
          embedded
          toggles={[
            {
              id: 'runtime',
              label: 'Runtime confirmed',
              icon: <Eye className="h-3.5 w-3.5" />,
              active: runtimeOnly,
              activeTone: 'success',
              onClick: () => setRuntimeOnly((v) => !v),
            },
            {
              id: 'critical',
              label: 'Critical only',
              icon: <ShieldAlert className="h-3.5 w-3.5" />,
              active: criticalOnly,
              activeTone: 'danger',
              onClick: () => setCriticalOnly((v) => !v),
            },
          ]}
        />
      ) : null}
      <Section
        id="risk-overview"
        className="scroll-mt-24"
        title={dashboardRoleMeta.riskTitle}
        description={
          sinceMinutes && sinceMinutes > 0
            ? `Unified risk levels, last ${sinceMinutes >= 60 ? `${Math.round(sinceMinutes / 60)} h` : `${sinceMinutes} min`}`
            : 'Unified risk levels follow header cluster scope'
        }
        actions={
          <div className="flex flex-col items-end gap-1">
            <Button variant="ghost" size="sm" onClick={() => navigate(risksUrl)}>
              Risk roster <ArrowRight size={14} className="ml-1" />
            </Button>
          </div>
        }
      >
        <div className="space-y-4">
        {isDashboardWidgetVisible(dashComposition, 'hero_metrics') ? (
        <DashboardHeroMetrics
          criticalCount={criticalRiskFindingCount}
          attackPathCount={attackPathCount}
          affectedWorkloads={Number(stats.affectedPodCount ?? 0)}
          clusterName={stats.clusterName}
          onCriticalClick={() => navigate(`${risksUrl}${risksUrl.includes('?') ? '&' : '?'}finalLevel=critical`)}
          onPathsClick={() => navigate('/attack-paths')}
          onWorkloadsClick={() => navigate(risksUrl)}
        />
        ) : null}

        {isDashboardWidgetVisible(dashComposition, 'risk_stats') ? (
        <RiskStatCards
          insightsSummary={insightsSummary}
          statsCritical={stats.critical}
          clusterName={stats.clusterName}
        />
          ) : null}
          </div>
        </Section>

          {isDashboardWidgetVisible(dashComposition, 'entry_points') ? (
        <Section id="entry-points" className="scroll-mt-24" title="Top entry points" description={`Prioritized attack paths (${filteredEntryScenarios.length})`}>
          <div className="rounded-xl border border-border/70 bg-surface/25 p-3 space-y-2">
            <h3 className="flex items-center gap-2 text-body font-semibold text-text">
              <Target className="w-4 h-4 text-brand shrink-0" />
              Top Entry Points
                <span className="ml-auto rounded border border-border/70 bg-surface/50 px-2 py-0.5 text-caption font-semibold text-muted">
                  {Math.min(filteredEntryScenarios.length, 10)}
                </span>
            </h3>
            <EntryPointsSection
              scenarios={filteredEntryScenarios}
              onOpenPanel={openScenarioPanel}
              onOpenPod={(uid) => navigate(`/resources/pods/uid/${encodeURIComponent(uid)}`)}
              onOpenAttackPaths={(uid) => navigate(`/attack-paths?podUid=${encodeURIComponent(uid)}`)}
            />
          </div>
        </Section>
        ) : null}

        {dashboardMode === 'full' ? (
        <Section id="extended-risk-intelligence" className="scroll-mt-24">
          <details open={personaId === 'operator'} className="group rounded-lg border border-border/80 bg-surface/25 open:shadow-sm">
            <summary className="flex cursor-pointer list-none items-center justify-between gap-2 px-3 py-2.5 text-caption font-semibold text-text [&::-webkit-details-marker]:hidden">
              <span>Extended risk intelligence</span>
            </summary>
          <div className="flex flex-col gap-3 border-t border-border/60 px-3 pb-3 pt-3">
            <div className="flex flex-col gap-2 rounded-lg border border-border/70 bg-surface/20 px-3 py-2.5 md:flex-row md:items-center">
              <GitBranch className="h-4 w-4 shrink-0 text-orange-300" />
              <div className="min-w-0 flex-1 text-caption leading-snug text-muted">
                <span className="font-medium text-text">Quick Actions</span>
                <span className="mx-2 text-muted-2">|</span>
                <span>
                  <strong className="text-orange-200">{criticalAttackPathCount}</strong> critical path(s)
                </span>
                <span className="mx-2 text-muted-2">|</span>
                <span>{filteredEntryScenarios.length} prioritized entry point(s)</span>
              </div>
              <Button variant="secondary" size="sm" className="h-8 !text-caption shrink-0" onClick={() => navigate('/attack-paths')}>
                Open attack paths
              </Button>
            </div>

            <div className="rounded-xl border border-border/70 bg-surface/25 p-3">
              <h3 className="mb-2 flex items-center gap-2 text-section-title font-semibold text-text">
                <Target className="h-4 w-4 shrink-0 text-brand" />
                Top risk targets
                  <span className="ml-auto rounded border border-border/70 bg-surface/50 px-2 py-0.5 text-caption font-semibold text-muted">
                    V3 pods
                  </span>
              </h3>
              {riskEntityRows.length === 0 ? (
                <p className="text-caption text-muted-2">
                  No V3-scored pods in this scope. Run the unified scorer or select a cluster with risk_scores v3 rows.
                </p>
              ) : (
            <div className="overflow-x-auto -mx-0.5 rounded-lg border border-border/50">
              <table className={`${UI_TABLE} min-w-[36rem] table-fixed`}>
                <thead>
                  <tr>
                    <th className={`${UI_TH} w-[26%]`}>Workload</th>
                    <th className={`${UI_TH} w-[18%]`}>Namespace</th>
                    <th className={`${UI_TH} w-[12%]`}>Level</th>
                    <th className={UI_TH}>Primary signal (V3)</th>
                    <th className={`${UI_TH} w-[10%] text-right`}>Score</th>
                  </tr>
                </thead>
                <tbody>
                  {riskEntityRows.map(({ pod, dim, level }) => (
                    <tr
                      key={pod.resourceUid}
                      role="link"
                      tabIndex={0}
                      className={`${UI_TR} cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 focus-visible:ring-inset`}
                      onClick={() => navigate(`/resources/pods/uid/${encodeURIComponent(pod.resourceUid)}`)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' || event.key === ' ') {
                          event.preventDefault();
                          navigate(`/resources/pods/uid/${encodeURIComponent(pod.resourceUid)}`);
                        }
                      }}
                    >
                      <td className={`${UI_TD_COMPACT} truncate font-semibold text-body`} title={pod.resourceName || pod.resourceUid}>
                        {pod.resourceName || pod.resourceUid}
                      </td>
                      <td className={`${UI_TD_COMPACT} truncate font-mono text-meta text-muted-2`} title={pod.namespace || ''}>
                        {pod.namespace || '—'}
                      </td>
                      <td className={UI_TD_COMPACT}>
                        <span
                          className={`inline-block rounded border px-1.5 py-0.5 text-micro font-bold capitalize ${getSeverityBadgeClass(level as 'critical' | 'high' | 'medium' | 'low')}`}
                        >
                          {level}
                        </span>
                      </td>
                      <td className={`${UI_TD_COMPACT} min-w-0`}>
                        <span className="line-clamp-2 text-muted-2">
                          <span className="font-medium text-text">{dim.short}</span>{' '}
                          <span className="font-mono text-text/90">
                            {dim.value}
                            <span className="text-muted-2">/15</span>
                          </span>
                          {dim.short === 'Capability' && pceSummary[0] ? (
                            <span className="text-muted-2"> · fleet: {pceSummary[0].capabilityId}</span>
                          ) : null}
                        </span>
                      </td>
                      <td className={`${UI_TD_COMPACT} text-right font-mono text-body font-semibold tabular-nums text-text`}>
                        {Math.round(pod.totalScore)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
              )}
              {pceSummary.length > 0 && (
                <div className="mt-3 border-t border-border/60 pt-2">
                  <p className="mb-1.5 text-micro uppercase tracking-wide text-muted-2">Fleet capability hotspots</p>
                  <div className="flex flex-wrap gap-1.5">
                    {pceSummary.slice(0, 6).map((row: PodCapabilitySummaryCapability) => (
                      <button
                        key={`${row.capabilityId}-${row.severity}`}
                        type="button"
                        onClick={() => navigate('/capabilities')}
                        className="rounded-md border border-border/70 bg-surface/40 px-2 py-0.5 text-micro transition-colors hover:border-brand/40 hover:text-brand"
                      >
                        {row.capabilityId} <span className="text-muted-2">({row.count})</span>
                      </button>
                    ))}
                  </div>
                </div>
              )}
            </div>

            <div className="flex items-start gap-3">
              <Shield className="mt-0.5 h-4 w-4 shrink-0 text-red-500" />
              <div className="min-w-0 flex-1">
                <h3 className="mb-1.5 text-caption font-bold text-text">Critical risk findings</h3>
                {topRisks.length === 0 ? (
                  <p className="flex items-center gap-1.5 text-caption text-emerald-400/90">
                    <Check className="h-3.5 w-3.5 shrink-0" />
                    No critical risks detected in current scope.
                  </p>
                ) : (
                  <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                    {topRisks.map((risk) => {
                      const finalLevel = risk.finalLevel || deriveUnifiedRiskLevelFromScore(risk.score);
                      return (
                        <button
                          type="button"
                          key={risk.id}
                          className="w-full cursor-pointer rounded-lg border border-border bg-surface p-3 text-left transition-all hover:border-brand/30 hover:bg-surface-2/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70"
                          onClick={() => navigate(`/risks/${risk.id}`)}
                        >
                          <div className="mb-1 flex items-start justify-between gap-2">
                            <span className="text-body font-bold tabular-nums text-red-400" title="Risk score (0-100)">
                              {risk.score != null ? risk.score : '—'}
                              {risk.score != null && <span className="ml-0.5 font-normal text-micro text-muted-2">/100</span>}
                            </span>
                            {finalLevel ? (
                              <Badge severity={finalLevel.toLowerCase()} uppercase={false}>
                                {finalLevel.toLowerCase()}
                              </Badge>
                            ) : (
                              <span className="rounded border border-border bg-surface-2/60 px-2 py-0.5 text-micro font-semibold text-muted">
                                no score
                              </span>
                            )}
                          </div>
                          <h4 className="line-clamp-2 text-body font-semibold leading-snug text-text">{risk.title}</h4>
                        </button>
                      );
                    })}
                  </div>
                )}
              </div>
            </div>

            <div className="flex flex-col gap-2 rounded-lg border border-orange-500/25 bg-orange-950/15 px-3 py-2.5 sm:flex-row sm:items-center">
              <GitBranch className="h-4 w-4 shrink-0 text-orange-300" />
              <div className="min-w-0 flex-1 text-caption leading-snug text-muted">
                <span className="font-medium text-text">Attack narrative — </span>
                {attackPathCount > 0 ? (
                  <>
                    {criticalAttackPathCount > 0 ? (
                      <span>
                        <strong className="text-orange-200">{criticalAttackPathCount}</strong> critical chain(s) ·{' '}
                      </span>
                    ) : null}
                    <span>
                      {attackPathCount} stored path{attackPathCount !== 1 ? 's' : ''} (workload → exposure).
                    </span>
                  </>
                ) : (
                  <span>No multi-hop attack paths recorded in Layer 3 for this scope yet.</span>
                )}
              </div>
              <Button variant="secondary" size="sm" className="h-7 shrink-0 !text-caption" onClick={() => navigate('/attack-paths')}>
                Attack paths
              </Button>
              </div>
            </div>
          </details>
        </Section>
          ) : null}

        {isDashboardSectionVisible(personaId, 'cluster-health', dashboardMode, dashComposition.widgets) ? (
        <Section id="cluster-health" className="scroll-mt-24" title="Cluster health" description="Pods, agents, and health for the header scope.">
          {clusterStatsRow}
          <div className="max-h-52 min-w-0 overflow-y-auto rounded-lg border border-border/60">{clusterHealthCard}</div>
        </Section>
      ) : null}

      {stats.clusters === 0 && stats.pods === 0 && activeFindingsCount === 0 && (
        <div className="rounded-lg border border-border bg-surface/50 px-4 py-3 text-body text-muted">
          <span className="font-medium text-text">No data yet.</span> Ensure you are logged in, Core is running, and the dashboard can reach the API.
        </div>
      )}

      {dashboardMode === 'full' && isDashboardSectionVisible(personaId, 'attack-analysis', dashboardMode, dashComposition.widgets) && (
        <details id="attack-analysis" open className="group scroll-mt-24 overflow-hidden rounded-xl border border-border/80 bg-surface/30">
          <summary className="flex cursor-pointer list-none items-center justify-between gap-2 px-4 py-3 text-caption font-semibold text-text hover:bg-surface-2/40 [&::-webkit-details-marker]:hidden">
            <span>Attack path analysis</span>
            <span className="font-normal text-meta text-muted-2">Selected entry graph, runtime, capabilities</span>
          </summary>
          <div className="border-t border-border/80 p-4 md:p-5">
            <AttackDetailContent
              scenario={selectedScenario}
              graphData={selectedGraphData}
              selectedNode={selectedNode}
              onNodeClick={setSelectedNodeId}
              runtimeItems={selectedRuntimeItems}
              capabilities={selectedCapabilities}
              gaps={selectedGaps}
              correlationPrecision={selectedScenario?.chain?.mitre_summary?.correlation_precision}
              avgGrounding={selectedAvgGrounding}
              runtimeInfluenceRatio={selectedRuntimeInfluenceRatio}
              semanticMode={profile.graphMode}
            />
          </div>
        </details>
      )}

      {isDashboardSectionVisible(personaId, 'activity', dashboardMode, dashComposition.widgets) && (
        <details id="activity" className="group scroll-mt-24 overflow-hidden rounded-xl border border-border/80 bg-surface/30">
          <summary className="flex cursor-pointer list-none items-center justify-between gap-2 px-4 py-3 text-caption font-semibold text-text hover:bg-surface-2/40 [&::-webkit-details-marker]:hidden">
            <span>Recent activity</span>
            <span className="font-normal text-meta text-muted-2">Notifications</span>
          </summary>
          <div className="border-t border-border/80 p-4">
            {notifications.length === 0 ? (
              <p className="flex items-center gap-2 py-1 text-caption text-muted-2">
                <Info className="h-3.5 w-3.5 shrink-0 opacity-70" />
                No notifications in the recent window.
              </p>
            ) : (
              <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
                {notifications.map((note) => (
                  <div key={note.id} className="flex items-start gap-2 rounded-lg border border-border/70 bg-surface/40 p-2.5">
                    <div
                      className={`shrink-0 rounded-md p-1 ${
                        note.type === 'error'
                          ? 'bg-red-500/10 text-red-400'
                          : note.type === 'success'
                            ? 'bg-emerald-500/10 text-emerald-400'
                            : 'bg-blue-500/10 text-blue-400'
                      }`}
                    >
                      {note.type === 'error' ? <ShieldAlert size={12} /> : <Info size={12} />}
                    </div>
                    <div className="min-w-0">
                      <h4 className="line-clamp-1 text-caption font-semibold text-text">{note.title}</h4>
                      <p className="mt-0.5 line-clamp-2 text-meta text-muted">{note.message}</p>
                      <span className="mt-1 block text-micro text-muted-2">{note.timestamp}</span>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </details>
      )}

      </div>

      {panelOpen && selectedScenario && (
        <div className="pointer-events-none fixed inset-0 z-modal">
          <button
            type="button"
            aria-label="Close attack path detail"
            className="absolute inset-0 bg-black/35 pointer-events-auto"
            onClick={() => setPanelOpen(false)}
          />
          <aside
            ref={scenarioPanelRef}
            role="dialog"
            aria-modal="true"
            aria-label="Attack path detail"
            className="pointer-events-auto fixed inset-x-0 bottom-0 max-h-[92dvh] overflow-y-auto overscroll-y-contain rounded-t-xl border border-border bg-base shadow-2xl md:inset-y-0 md:left-auto md:right-0 md:h-full md:max-h-none md:w-[min(92vw,780px)] md:rounded-none md:rounded-l-xl"
          >
            <div className="sticky top-0 z-10 flex items-center justify-between gap-3 border-b border-border bg-base/95 px-4 py-3 backdrop-blur">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <PanelRight className="w-4 h-4 text-brand shrink-0" />
                  <h2 className="text-body font-semibold text-text truncate">Attack Path Detail</h2>
                </div>
                <p className="mt-0.5 text-caption text-muted truncate">{selectedScenario.pod.name}</p>
              </div>
              <button
                type="button"
                aria-label="Close panel"
                className="inline-flex min-h-10 min-w-10 items-center justify-center rounded-lg border border-border bg-surface/50 text-muted hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70"
                onClick={() => setPanelOpen(false)}
              >
                <X className="w-4 h-4" />
              </button>
            </div>
            <div className="p-4">
              <AttackDetailContent
                scenario={selectedScenario}
                graphData={selectedGraphData}
                selectedNode={selectedNode}
                onNodeClick={setSelectedNodeId}
                runtimeItems={selectedRuntimeItems}
                capabilities={selectedCapabilities}
                gaps={selectedGaps}
                correlationPrecision={selectedScenario.chain?.mitre_summary?.correlation_precision}
                avgGrounding={selectedAvgGrounding}
                runtimeInfluenceRatio={selectedRuntimeInfluenceRatio}
                semanticMode={profile.graphMode}
              />
            </div>
          </aside>
        </div>
      )}
    </PageLayout>
  );
};
