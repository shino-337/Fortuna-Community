import type {
  AttackChain,
  AttackPath,
  AttackPathGraphData,
  AttackStep,
  MitreCoverageItem,
  PodRuntimeSecurityEvent,
  PodWithRisk,
  RuntimeSignal,
  UnifiedRiskScore,
} from '../../types';
import { deriveUnifiedRiskLevelFromScore } from '../../lib/severity';
import { reconcileAttackGraphEntryExit } from '../../lib/attackPathGraphTopology';
import type { EntryScenario, RiskBand, RuntimeConfirmationItem, RuntimeStatus } from './types';

export function topContributingDimension(dim: UnifiedRiskScore['dimensions']): { short: string; value: number } {
  const pairs: [string, number][] = [
    ['Vuln', dim.vulnerability ?? 0],
    ['Capability', dim.capabilityExposure ?? 0],
    ['Attack path', dim.attackPath ?? 0],
    ['RBAC', dim.rbacPolicy ?? 0],
    ['Runtime', dim.runtimeThreat ?? 0],
    ['Exposure', dim.exposure ?? 0],
    ['Blast', dim.blastRadius ?? 0],
  ];
  let best = pairs[0];
  for (const p of pairs) {
    if (p[1] > best[1]) best = p;
  }
  return { short: best[0], value: best[1] };
}

export const IMPACT_RANK: Record<string, number> = {
  CRITICAL: 4,
  HIGH: 3,
  MEDIUM: 2,
  LOW: 1,
};

export const RISK_RANK: Record<string, number> = {
  CRITICAL: 4,
  HIGH: 3,
  MEDIUM: 2,
  LOW: 1,
};

export const CAPABILITY_KNOWLEDGE: Record<string, { explanation: string; riskPoints: number }> = {
  SA_TOKEN: { explanation: 'Can access the Kubernetes API with service account credentials.', riskPoints: 1.2 },
  SERVICE_ACCOUNT_ACCESS: { explanation: 'Can use Kubernetes service account credentials.', riskPoints: 1.2 },
  CLUSTER_ADMIN: { explanation: 'Full cluster takeover through cluster-wide control.', riskPoints: 2 },
  NODE_SHELL_ACCESS: { explanation: 'Can interact with host or node filesystem/process space.', riskPoints: 1.5 },
  HOST_ACCESS: { explanation: 'Can touch host-level surfaces from the workload.', riskPoints: 1.5 },
  NETWORK_ACCESS: { explanation: 'Can reach another workload or control plane endpoint.', riskPoints: 0.65 },
};

export const EDGE_TO_TECHNIQUE_CATEGORIES: Record<string, string[]> = {
  ESC_HOSTPATH_NODE: ['ESCAPE_HOSTPATH'],
  ESC_HOSTPID: ['ESCAPE_HOSTPID'],
  ESC_PRIV_POD: ['ESCAPE_PRIVILEGED'],
  CONTAINER_ESCAPE: ['ESCAPE_HOSTPATH', 'ESCAPE_RUNTIME', 'ESCAPE_PROC_ROOT', 'ESCAPE_PRIVILEGED', 'ESCAPE_HOSTPID'],
  HOST_ACCESS: ['ESCAPE_HOSTPATH', 'ESCAPE_RUNTIME', 'ESCAPE_PROC_ROOT', 'ESCAPE_PRIVILEGED', 'ESCAPE_HOSTPID'],
  LATERAL_MOVE: ['LATERAL_NETWORK'],
  NETWORK_REACH: ['LATERAL_NETWORK'],
  NETWORK_REACH_SOFT: ['LATERAL_NETWORK'],
  SERVICE_ACCOUNT_ACCESS: ['KUBELET_TOKEN_HARVEST', 'RUNTIME_TOKEN_HARVEST', 'SA_TOKEN_REUSE'],
  RBAC_BINDING: ['RBAC_PRIV_ESC', 'CLUSTER_ADMIN_ESC'],
  GRANTS_ROLE: ['RBAC_PRIV_ESC', 'CLUSTER_ADMIN_ESC'],
  CAN_STEAL_CREDENTIALS: ['KUBELET_API_PROBE', 'KUBELET_TOKEN_HARVEST', 'RUNTIME_TOKEN_HARVEST'],
};

export function scoreForPod(pod: PodWithRisk | UnifiedRiskScore): number | undefined {
  const raw = 'totalScore' in pod ? pod.totalScore : undefined;
  const value =
    typeof raw === 'number'
      ? raw
      : 'unifiedScore' in pod && typeof pod.unifiedScore === 'number'
        ? pod.unifiedScore
        : undefined;
  return value != null && Number.isFinite(value) ? value : undefined;
}

export function finalRiskBandFromScore(score?: number): RiskBand {
  return deriveUnifiedRiskLevelFromScore(score) ?? 'low';
}

export function riskBandFrom(value?: string, score?: number): RiskBand {
  const raw = String(value || '').toLowerCase();
  if (raw === 'critical' || raw === 'high' || raw === 'medium' || raw === 'low') return raw;
  return finalRiskBandFromScore(score);
}

export function riskLabelUpper(value?: string, score?: number): string {
  return riskBandFrom(value, score).toUpperCase();
}

export function riskLabelTitle(value?: string, score?: number): string {
  const label = riskBandFrom(value, score);
  return label.charAt(0).toUpperCase() + label.slice(1);
}

export function rankImpact(value?: string): number {
  return IMPACT_RANK[String(value || '').toUpperCase()] ?? 0;
}

export function rankRisk(value?: string, score?: number): number {
  return RISK_RANK[riskLabelUpper(value, score)] ?? 0;
}

export function formatPct(value?: number): string {
  if (value == null || Number.isNaN(value)) return '-';
  const normalized = value <= 1 ? value * 100 : value;
  return `${Math.round(normalized)}%`;
}

export function normalizeNodeType(type?: string): string {
  return String(type || '')
    .replace(/([a-z])([A-Z])/g, '$1_$2')
    .toLowerCase()
    .replace(/-/g, '_');
}

export function nodeTypeLabel(type?: string): string {
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

export function nodeDisplayName(node: AttackPath['nodes'][number], index: number): string {
  const name = String(node.properties?.name || '').trim();
  const type = normalizeNodeType(node.type);
  if ((type === 'clusterrole' || type === 'cluster_role') && name.toLowerCase().includes('cluster-admin')) {
    return 'ClusterRole (cluster-admin)';
  }
  if (name && !name.startsWith('uid://')) return name;
  if (index === 0 && type === 'pod') return 'Pod';
  return nodeTypeLabel(node.type);
}

export function pathLabelFromPrimitive(path?: AttackPath, chain?: AttackChain): string {
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

export function getTechniqueStepIndex(edgeType: string, techniques?: AttackStep[]): number | undefined {
  if (!techniques?.length) return undefined;
  const categories = EDGE_TO_TECHNIQUE_CATEGORIES[String(edgeType || '').toUpperCase()];
  if (!categories) return undefined;
  for (let i = 0; i < techniques.length; i += 1) {
    if (categories.includes(techniques[i].technique_id)) return i + 1;
  }
  return undefined;
}

export function buildGraphDataFromPrimitivePaths(paths: AttackPath[], chain?: AttackChain): AttackPathGraphData {
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

export function filterGraphDataForChain(graphData: AttackPathGraphData, chain?: AttackChain): AttackPathGraphData {
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

export function findChainForPod(pod: PodWithRisk, chains: AttackChain[]): AttackChain | undefined {
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

export function buildEntryScenarios(pods: PodWithRisk[], chains: AttackChain[], paths: AttackPath[]): EntryScenario[] {
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

export function buildSoWhat(chain?: AttackChain, pod?: PodWithRisk): string {
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

export function parseEvidence(raw: unknown): Record<string, unknown> {
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

export function signalText(signal: RuntimeSignal): string {
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

export function eventText(event: PodRuntimeSecurityEvent): string {
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

export function runtimeSourceFromSignal(signal?: RuntimeSignal): string | undefined {
  if (!signal) return undefined;
  const evidence = parseEvidence(signal.evidence);
  const source = evidence.source_rule || evidence.sourceRule || evidence.rule || evidence.source || evidence.syscall || signal.signalType;
  return String(source || '').trim() || undefined;
}

export function maxConfidence(signals: RuntimeSignal[], events: PodRuntimeSecurityEvent[]): number | undefined {
  const values = [
    ...signals.map((signal) => signal.confidence),
    ...events.map((event) => event.confidence).filter((value): value is number => typeof value === 'number'),
  ].filter((value) => Number.isFinite(value));
  return values.length ? Math.max(...values) : undefined;
}

export function deriveRuntimeConfirmations(
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

export function capabilitiesFromChain(chain?: AttackChain): Array<{ id: string; explanation: string; riskPoints?: number }> {
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

export function statusClass(status: RuntimeStatus): string {
  if (status === 'confirmed') return 'border-emerald-500/30 bg-emerald-500/10 text-emerald-300';
  if (status === 'inferred') return 'border-amber-500/35 bg-amber-500/10 text-amber-200';
  return 'border-border bg-surface/40 text-muted';
}

export function coverageBadgeClass(item: MitreCoverageItem): string {
  if (item.status === 'not_covered') return 'border-l-[3px] border-l-red-500/80 bg-red-950/25 text-red-200';
  if (item.status === 'inferred') return 'border-l-[3px] border-l-amber-500/70 bg-amber-950/20 text-amber-100';
  if (item.status === 'observed') return 'border-l-[3px] border-l-emerald-500/70 bg-emerald-950/20 text-emerald-100';
  return 'border-l-[3px] border-l-border bg-surface/30 text-muted';
}

export function selectedStepForNode(node: AttackPathGraphData['nodes'][number] | undefined, chain?: AttackChain): AttackStep | undefined {
  if (!node || !chain?.steps?.length) return undefined;
  const index = typeof node.stepIndex === 'number' ? Math.max(0, node.stepIndex - 1) : 0;
  return chain.steps[index] ?? chain.steps[Math.max(0, index - 1)] ?? chain.steps[0];
}
