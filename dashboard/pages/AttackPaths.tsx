import React, { useEffect, useState, useCallback, useMemo } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PageError, PageLoading } from '../design-system/components/PageStatus';
import { SemanticEmptyState } from '../design-system/components/SemanticEmptyState';
import { Button } from '../components/ui/Button';
import { AttackPathGraph } from '../components/AttackPathGraph';
import { api, isApiError } from '../lib/api';
import type { AttackPathGraphData, AttackPathSummary, AttackPath, AttackChain } from '../types';
import { useClusterStore } from '../store/clusterStore';
import {
  AlertTriangle,
  Shield,
  Network,
  ChevronRight,
  ChevronDown,
  RefreshCw,
  Target,
  Layers,
  Zap,
  Wrench,
  Crosshair,
} from 'lucide-react';
import { PrimaryScenarioHero } from '../components/PrimaryScenarioHero';
import { AttackStepsTimeline } from '../components/AttackStepsTimeline';
import { deriveUnifiedRiskLevelFromScore, getSeverityTextClass } from '../lib/severity';
import {
  groupChains,
  groupPrimitivePaths,
  chainTypeHumanLabel,
  buildWhyItMattersHuman,
  weakLinksFromPath,
  strengthSemantic,
  impactSemantic,
  buildFixRecommendations,
  variantLabel,
  type GroupedScenario,
} from '../lib/attackPathNarrative';
import { AttackPathConfidenceLanes } from '../components/AttackPathConfidenceLanes';
import { attackPathConfidenceLane, type AttackPathConfidenceLane } from '../lib/attackPathConfidence';
import { PinToInvestigationButton } from '../components/PinToInvestigationButton';
import { attackPathInvestigationEntity } from '../lib/investigationEntities';
import { AttackPathPriorityList } from '../components/AttackPathPriorityList';
import { GraphSemanticBanner } from '../components/GraphSemanticBanner';
import { GraphSemanticLegend } from '../components/GraphSemanticLegend';
import { GraphVisibilityOverlay } from '../components/GraphVisibilityOverlay';
import { useFeatureVisibility } from '../hooks/useVisibility';
import { useIncidentMode } from '../hooks/useIncidentMode';
import { useGraphTrustContext } from '../hooks/useGraphTrustContext';
import { PAGE_TITLES } from '../lib/pageTitles';
import type { GraphSemanticMode } from '../lib/persona';
import { useCan } from '../hooks/usePermUser';
import { P } from '../lib/permissions';
import type { SemanticVisibilityState } from '../lib/visibilityEngine';

type AttackPathIssueKind = 'unauthenticated' | 'forbidden' | 'cluster_scope' | 'load_failed';

type AttackPathIssue = {
  kind: AttackPathIssueKind;
  title: string;
  description: string;
  detail?: string;
  actionLabel: string;
};

function classifyAttackPathIssue(error: unknown, fallbackTitle = 'Could not load attack paths'): AttackPathIssue {
  if (isApiError(error)) {
    if (error.status === 401) {
      return {
        kind: 'unauthenticated',
        title: 'Session is not active',
        description: 'Sign in again to load attack-path data.',
        detail: error.body?.code ? `Core returned ${error.body.code}.` : 'Core rejected the current JWT.',
        actionLabel: 'Go to login',
      };
    }
    if (error.status === 403 && error.body?.reason === 'cluster_scope') {
      const cluster = error.body.required_cluster_id;
      return {
        kind: 'cluster_scope',
        title: 'Cluster outside your scope',
        description: cluster
          ? `Your account is not scoped for ${cluster}. Select an allowed cluster or ask an administrator to update your scope.`
          : 'Your account is not scoped for this attack-path query.',
        detail: 'Required scope: cluster access.',
        actionLabel: 'Refresh',
      };
    }
    if (error.status === 403) {
      const required = error.body?.required_permission ?? error.body?.required_permissions?.join(', ');
      return {
        kind: 'forbidden',
        title: 'Permission required',
        description: required
          ? `Attack Analysis requires ${required}. Ask an administrator to update your Fortuna role or permissions.`
          : 'Your account does not include permission for attack-path APIs.',
        detail: 'Core returned 403 forbidden.',
        actionLabel: 'Refresh',
      };
    }
    return {
      kind: 'load_failed',
      title: fallbackTitle,
      description: error.status >= 500
        ? 'Core returned a server error while building attack-path data.'
        : error.message || 'The attack-path request failed.',
      detail: `HTTP ${error.status}`,
      actionLabel: 'Retry',
    };
  }
  return {
    kind: 'load_failed',
    title: fallbackTitle,
    description: error instanceof Error ? error.message : 'The request failed before attack-path data was returned.',
    actionLabel: 'Retry',
  };
}

function shouldStopAttackPathFallback(error: unknown): boolean {
  return isApiError(error) && (error.status === 401 || error.status === 403);
}

/* ─── helpers ─────────────────────────────────────────────── */

function compactText(value: string | undefined | null, max = 140): string {
  const text = String(value ?? '').replace(/\s+/g, ' ').trim();
  if (!text) return 'No detail provided.';
  return text.length > max ? `${text.slice(0, Math.max(0, max - 1)).trimEnd()}…` : text;
}

function blastRadiusLabel(value: number): { label: string; className: string } {
  if (value >= 0.75) return { label: 'Critical', className: 'text-red-300' };
  if (value >= 0.5) return { label: 'High', className: 'text-orange-300' };
  if (value >= 0.25) return { label: 'Medium', className: 'text-amber-300' };
  return { label: 'Limited', className: 'text-muted' };
}

function exploitRequirementSummary(chain: AttackChain): Array<{ label: string; value: string }> {
  const steps = Array.isArray(chain.steps) ? chain.steps : [];
  const requiresNetwork = steps.some((s) =>
    String(s.technique_id || '').toUpperCase().includes('NETWORK') ||
    (Array.isArray(s.input_caps) ? s.input_caps : []).some((c) => String(c).toUpperCase().includes('NETWORK')),
  );
  const requiresNode = steps.some((s) =>
    String(s.technique_id || '').toUpperCase().includes('NODE') ||
    (Array.isArray(s.input_caps) ? s.input_caps : []).some((c) => String(c).toUpperCase().includes('NODE')),
  );
  const softOrGap =
    Boolean(chain.capability_validation?.soft_mode) ||
    (Array.isArray(chain.capability_validation?.gaps) && chain.capability_validation.gaps.length > 0);
  const scope = String(chain.impact_scope || chain.objective || '').toUpperCase();
  const privilegeScope = scope.includes('CLUSTER')
    ? 'Cluster-wide'
    : scope.includes('NODE')
      ? 'Node-level'
      : scope.includes('NAMESPACE')
        ? 'Namespace'
        : 'Workload';

  return [
    { label: 'Network', value: requiresNetwork ? (softOrGap ? 'Inferred' : 'Required') : 'Not primary' },
    { label: 'Precondition', value: requiresNode ? 'Node access' : softOrGap ? 'Validate reachability' : 'Workload foothold' },
    { label: 'Scope', value: privilegeScope },
  ];
}

function topItems(items: unknown[], max = 3): string[] {
  return items.map((x) => String(x || '').trim()).filter(Boolean).slice(0, max);
}

function riskLabel(score: number): string {
  if (score >= 9) return 'critical';
  if (score >= 7) return 'high';
  if (score >= 4) return 'medium';
  return 'low';
}

const EDGE_TO_TECHNIQUE_CATEGORIES: Record<string, string[]> = {
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

function getTechniqueStepIndex(edgeType: string, techniques?: any[]): number | undefined {
  if (!techniques || techniques.length === 0) return undefined;
  const categories = EDGE_TO_TECHNIQUE_CATEGORIES[edgeType.toUpperCase()];
  if (!categories) return undefined;
  
  for (let i = 0; i < techniques.length; i++) {
    if (categories.includes(techniques[i].technique_id)) {
      return i + 1; // 1-based index matching Technical Steps panel
    }
  }
  return undefined;
}

function buildGraphDataFromPrimitivePaths(paths: AttackPath[], techniques?: any[]): AttackPathGraphData {
  const nodeMap = new Map<string, AttackPathGraphData['nodes'][0]>();
  const linkMap = new Map<string, AttackPathGraphData['links'][0]>();
  const riskFromScore = (totalRisk: number) => {
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
  
  for (const p of paths) {
    const pathId = p.path_id || `path-${nodeMap.size}`;
    const pathNodes = Array.isArray(p.nodes) ? p.nodes : [];
    const pathEdges = Array.isArray(p.edges) ? p.edges : [];
    const nodeCount = pathNodes.length;
    const firstNode = pathNodes[0];
    if (firstNode?.id) {
      const entryId = `path-entry:${pathId}`;
      nodeMap.set(entryId, {
        id: entryId,
        label: pathId,
        type: 'path_entry',
        risk: riskFromScore(p.total_risk),
        isStart: true,
        isEnd: false,
        stepIndex: 0,
        pathId,
        pathIds: [pathId],
      });
      linkMap.set(`${entryId}->${firstNode.id}`, {
        source: entryId,
        target: firstNode.id,
        type: 'PATH_ENTRY',
        value: p.total_risk,
        pathIds: [pathId],
      });
    }
    pathNodes.forEach((n, idx) => {
      const isEnd = idx === nodeCount - 1;
      
      const existing = nodeMap.get(n.id);
      if (!existing) {
        nodeMap.set(n.id, {
          id: n.id,
          label: String(n.properties?.name ?? n.id),
          type: String(n.type || '').toLowerCase(),
          risk: riskFromScore(p.total_risk),
          isStart: false,
          isEnd,
          stepIndex: idx + 1, // 1-based index
          pathIds: [pathId],
        });
      } else {
        if (isEnd) existing.isEnd = true;
        if (existing.stepIndex === undefined || idx + 1 < existing.stepIndex) {
          existing.stepIndex = idx + 1; // Keep the minimum step index encountered
        }
        addPathId(existing, pathId);
      }
    });

    pathEdges.forEach((e) => {
      // Deduplicate edges entirely by source->target to avoid overlapping lines
      const k = `${e.source}->${e.target}`;
      const existing = linkMap.get(k);
      
      const mappedStepIdx = getTechniqueStepIndex(e.type, techniques);
      const isMetaEdge = e.type === 'HAS_ATTACK_STEP' || e.type === 'CAN_STEAL_CREDENTIALS';
      
      if (!existing) {
        linkMap.set(k, {
          source: e.source,
          target: e.target,
          type: e.type,
          value: p.total_risk,
          stepIndex: mappedStepIdx,
          pathIds: [pathId],
        });
      } else {
        // If the existing one is a meta-edge and the new one isn't, replace the type
        if ((existing.type === 'HAS_ATTACK_STEP' || existing.type === 'CAN_STEAL_CREDENTIALS') && !isMetaEdge) {
          existing.type = e.type;
        }
        
        // Use the newly mapped step index if the existing one doesn't have one
        if (existing.stepIndex === undefined && mappedStepIdx !== undefined) {
          existing.stepIndex = mappedStepIdx;
        }
        addPathId(existing, pathId);
      }
    });
  }
  
  return { nodes: Array.from(nodeMap.values()), links: Array.from(linkMap.values()) };
}

function AttackPathIssuePanel({
  issue,
  onAction,
  compact = false,
}: {
  issue: AttackPathIssue;
  onAction: () => void;
  compact?: boolean;
}) {
  if (issue.kind === 'load_failed') {
    return (
      <div className={compact ? '' : 'flex min-h-[420px] items-center justify-center px-4'}>
        <PageError
          title={issue.title}
          description={`${issue.description}${issue.detail ? ` ${issue.detail}` : ''}`}
          className={compact ? 'py-4' : undefined}
          action={
            <Button variant="secondary" size="sm" type="button" onClick={onAction}>
              {issue.actionLabel}
            </Button>
          }
        />
      </div>
    );
  }

  const state: SemanticVisibilityState =
    issue.kind === 'unauthenticated'
      ? 'no_permission'
      : issue.kind === 'cluster_scope'
        ? 'no_scope'
        : 'no_permission';

  return (
    <div className={compact ? '' : 'flex min-h-[420px] items-center justify-center px-4'}>
      <SemanticEmptyState
        state={state}
        title={issue.title}
        reason={`${issue.description}${issue.detail ? ` ${issue.detail}` : ''}`}
        compact={compact}
        action={
          <Button variant="secondary" size="sm" type="button" onClick={onAction}>
            {issue.actionLabel}
          </Button>
        }
      />
    </div>
  );
}

/* ─── main component ──────────────────────────────────────── */

export const AttackPaths: React.FC = () => {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const { selectedClusterId } = useClusterStore();
  const canOpenInventory = useCan(P.inventoryRead);
  const graphVisibility = useFeatureVisibility('attack_paths');
  const { graphSemanticMode } = useIncidentMode();
  const attackGraphMode: GraphSemanticMode = graphSemanticMode ?? 'exploitability';
  const graphTrust = useGraphTrustContext('attack_paths', attackGraphMode);

  const [graphData, setGraphData] = useState<AttackPathGraphData>({ nodes: [], links: [] });
  const [summary, setSummary] = useState<AttackPathSummary | null>(null);
  const [chains, setChains] = useState<AttackChain[]>([]);
  const [primitivePaths, setPrimitivePaths] = useState<AttackPath[]>([]);
  const [selectedScenarioKey, setSelectedScenarioKey] = useState<string | null>(null);
  const [graphExpanded, setGraphExpanded] = useState(true);
  const [showLowConfidenceScenarios, setShowLowConfidenceScenarios] = useState(true);
  const [confidenceLane, setConfidenceLane] = useState<AttackPathConfidenceLane | 'all'>('all');
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [dataIssue, setDataIssue] = useState<AttackPathIssue | null>(null);
  const [actionNotice, setActionNotice] = useState<AttackPathIssue | null>(null);

  const podUidParam = searchParams.get('podUid') || '';
  const [selectedPodPaths, setSelectedPodPaths] = useState<AttackPath[]>([]);
  const [selectedPodLoading, setSelectedPodLoading] = useState(false);
  const [selectedPodIssue, setSelectedPodIssue] = useState<AttackPathIssue | null>(null);

  const [hoveredPathId, setHoveredPathId] = useState<string | null>(null);
  const [clickedPathId, setClickedPathId] = useState<string | null>(null);
  /** 1-based — aligns with graph link `stepIndex` when mapped from technique categories */
  const [highlightedStepIndex, setHighlightedStepIndex] = useState<number | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setDataIssue(null);
    try {
      const bundle = await api.getAttackPathsBundleStrict(selectedClusterId || undefined);
      if (bundle) {
        setGraphData(bundle.graph);
        setSummary(bundle.summary);
        setChains(bundle.chains);
        setPrimitivePaths(bundle.paths ?? []);
      } else {
        const [graph, sum, chs] = await Promise.all([
          api.getAttackPathsGraphStrict(),
          api.getAttackPathsSummaryStrict(selectedClusterId || undefined),
          api.getAttackChainsStrict(selectedClusterId || undefined),
        ]);
        setGraphData(graph);
        setSummary(sum);
        setChains(chs);
        setPrimitivePaths([]);
      }
      setDataIssue(null);
      setActionNotice(null);
    } catch (err) {
      if (shouldStopAttackPathFallback(err)) {
        setGraphData({ nodes: [], links: [] });
        setSummary(null);
        setChains([]);
        setPrimitivePaths([]);
        setDataIssue(classifyAttackPathIssue(err));
      } else {
        try {
          const [graph, sum, chs] = await Promise.all([
            api.getAttackPathsGraphStrict(),
            api.getAttackPathsSummaryStrict(selectedClusterId || undefined),
            api.getAttackChainsStrict(selectedClusterId || undefined),
          ]);
          setGraphData(graph);
          setSummary(sum);
          setChains(chs);
          setPrimitivePaths([]);
          setDataIssue(null);
        } catch (fallbackErr) {
          setGraphData({ nodes: [], links: [] });
          setSummary(null);
          setChains([]);
          setPrimitivePaths([]);
          setDataIssue(classifyAttackPathIssue(fallbackErr));
        }
      }
    } finally {
      setLoading(false);
    }
  }, [selectedClusterId]);

  useEffect(() => { fetchData(); }, [fetchData]);

  useEffect(() => {
    if (!podUidParam) { setSelectedPodPaths([]); setSelectedPodIssue(null); return; }
    setSelectedPodLoading(true);
    setSelectedPodIssue(null);
    api.getAttackPathsForPodStrict(podUidParam)
      .then((paths) => {
        setSelectedPodPaths(paths);
        setSelectedPodIssue(null);
      })
      .catch((err) => {
        setSelectedPodPaths([]);
        setSelectedPodIssue(classifyAttackPathIssue(err, 'Could not load pod attack paths'));
      })
      .finally(() => setSelectedPodLoading(false));
  }, [podUidParam]);

  const handleRefresh = async () => {
    setRefreshing(true);
    setSelectedScenarioKey(null);
    setHoveredPathId(null);
    setClickedPathId(null);
    setHighlightedStepIndex(null);
    await fetchData();
    setRefreshing(false);
  };

  const handleIssueAction = (issue: AttackPathIssue) => {
    if (issue.kind === 'unauthenticated') {
      navigate('/login');
      return;
    }
    void handleRefresh();
  };

  const focusPathOnGraph = useCallback((pathId: string | null | undefined) => {
    setClickedPathId(pathId || null);
    if (pathId) {
      setGraphExpanded(true);
      window.setTimeout(() => {
        document.getElementById('graph-section')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
      }, 0);
    }
  }, []);

  const handleNodeClick = useCallback((nodeId: string, type: string) => {
    if (type === 'path_entry' || nodeId.startsWith('path-entry:')) {
      focusPathOnGraph(nodeId.replace(/^path-entry:/, ''));
      return;
    }
    if (type !== 'pod') return;
    if (!canOpenInventory) {
      setActionNotice({
        kind: 'forbidden',
        title: 'Permission required',
        description: 'Opening pod details requires inventory.read.',
        detail: 'Attack-path data remains visible in this view.',
        actionLabel: 'Refresh',
      });
      return;
    }
    navigate(`/resources/pods/uid/${encodeURIComponent(nodeId)}`);
  }, [canOpenInventory, focusPathOnGraph, navigate]);

  const clearPodUidFilter = useCallback(() => {
    const next = new URLSearchParams(searchParams);
    next.delete('podUid');
    setSearchParams(next, { replace: true });
  }, [searchParams, setSearchParams]);

  /* ─── Phase 1: Group chains into scenarios ───────────────── */
  const groupedScenarios = useMemo(() => {
    const grouped = groupChains(chains, primitivePaths);
    const chainedPathIds = new Set<string>();
    for (const chain of Array.isArray(chains) ? chains : []) {
      for (const pathId of Array.isArray(chain.paths) ? chain.paths : []) {
        if (pathId) chainedPathIds.add(pathId);
      }
    }
    const orphanPaths = primitivePaths.filter((path) => path.path_id && !chainedPathIds.has(path.path_id));
    const orphanScenarios = groupPrimitivePaths(orphanPaths).map((scenario) => ({
      ...scenario,
      key: `standalone::${scenario.key}`,
    }));
    return [...grouped, ...orphanScenarios].sort((a, b) => b.maxStrength - a.maxStrength);
  }, [chains, primitivePaths]);
  const lowConfidenceScenarioCount = useMemo(
    () => groupedScenarios.filter((s) => s.confidence === 'low' || s.maxStrength < 0.12).length,
    [groupedScenarios],
  );

  const visibleScenarios = useMemo(() => {
    let list = groupedScenarios;
    if (!showLowConfidenceScenarios) {
      list = list.filter((s) => s.confidence !== 'low' && s.maxStrength >= 0.12);
    }
    if (confidenceLane !== 'all') {
      list = list.filter((s) => attackPathConfidenceLane(s) === confidenceLane);
    }
    return list;
  }, [groupedScenarios, showLowConfidenceScenarios, confidenceLane]);

  const totalPathCount = summary?.totalPaths ?? primitivePaths.length;
  const visiblePathCount = useMemo(() => {
    const ids = new Set<string>();
    for (const scenario of visibleScenarios) {
      for (const variant of Array.isArray(scenario.variants) ? scenario.variants : []) {
        for (const pathId of Array.isArray(variant.paths) ? variant.paths : []) {
          if (pathId) ids.add(pathId);
        }
      }
    }
    return ids.size;
  }, [visibleScenarios]);

  const hiddenCount = groupedScenarios.length - visibleScenarios.length;

  const currentScenario =
    visibleScenarios.find((s) => s.key === selectedScenarioKey) || visibleScenarios[0];
  const scenarioOrdinal = useMemo(() => {
    if (!currentScenario) return 1;
    const idx = visibleScenarios.findIndex((s) => s.key === currentScenario.key);
    return idx >= 0 ? idx + 1 : 1;
  }, [visibleScenarios, currentScenario]);

  const graphForView = useMemo(() => {
    if (primitivePaths.length === 0) return graphData;

    if (podUidParam && selectedPodPaths.length > 0) {
      return buildGraphDataFromPrimitivePaths(selectedPodPaths, visibleScenarios[0]?.representativeChain?.steps);
    }

    if (!selectedScenarioKey) {
      const primaryScenario = visibleScenarios[0];
      return buildGraphDataFromPrimitivePaths(primitivePaths, primaryScenario?.representativeChain?.steps);
    }
    
    const scenario = groupedScenarios.find((s) => s.key === selectedScenarioKey);
    if (!scenario) return buildGraphDataFromPrimitivePaths(primitivePaths);
    
    const allPathIds = new Set<string>();
    for (const v of scenario.variants) for (const pid of Array.isArray(v.paths) ? v.paths : []) allPathIds.add(pid);
    const subset = primitivePaths.filter((p) => allPathIds.has(p.path_id || ''));
    
    if (subset.length === 0) return buildGraphDataFromPrimitivePaths(primitivePaths, scenario.representativeChain.steps);
    return buildGraphDataFromPrimitivePaths(subset, scenario.representativeChain.steps);
  }, [selectedScenarioKey, groupedScenarios, graphData, primitivePaths, podUidParam, selectedPodPaths, visibleScenarios]);

  useEffect(() => {
    setHighlightedStepIndex(null);
  }, [selectedScenarioKey]);

  const { highlightedNodeIds, highlightedEdgeKeys } = useMemo(() => {
    const targetPathId = hoveredPathId || clickedPathId;
    if (!targetPathId) return { highlightedNodeIds: undefined, highlightedEdgeKeys: undefined };
    const path = primitivePaths.find(p => p.path_id === targetPathId);
    if (!path) return { highlightedNodeIds: undefined, highlightedEdgeKeys: undefined };

    const nodes = new Set<string>();
    const edges = new Set<string>();
    const entryId = `path-entry:${targetPathId}`;
    nodes.add(entryId);
    (Array.isArray(path.nodes) ? path.nodes : []).forEach(n => nodes.add(n.id));
    const firstNodeId = Array.isArray(path.nodes) ? path.nodes[0]?.id : '';
    if (firstNodeId) edges.add(`${entryId}->${firstNodeId}`);
    (Array.isArray(path.edges) ? path.edges : []).forEach(e => edges.add(`${e.source}->${e.target}`));
    return { highlightedNodeIds: nodes, highlightedEdgeKeys: edges };
  }, [hoveredPathId, clickedPathId, primitivePaths]);

  const focusNodeIds = useMemo(() => {
    if (!clickedPathId) return undefined;
    const path = primitivePaths.find((p) => p.path_id === clickedPathId);
    if (!path) return undefined;
    const nodes = new Set<string>([`path-entry:${clickedPathId}`]);
    (Array.isArray(path.nodes) ? path.nodes : []).forEach((node) => nodes.add(node.id));
    return nodes;
  }, [clickedPathId, primitivePaths]);

  /* ─── render ─────────────────────────────────────────────── */

  return (
    <PageLayout
      title={PAGE_TITLES.attackAnalysis}
      description="Identify critical vulnerabilities and visualize potential attack vectors."
      actions={
        <Button variant="secondary" size="sm" onClick={() => void handleRefresh()} isLoading={refreshing}>
          <RefreshCw size={16} className="mr-2 shrink-0" />
          Refresh
        </Button>
      }
    >
      {loading ? (
        <PageLoading message="Loading attack paths..." className="min-h-[420px]" />
      ) : dataIssue ? (
        <AttackPathIssuePanel issue={dataIssue} onAction={() => handleIssueAction(dataIssue)} />
      ) : visibleScenarios.length > 0 ? (
        <div className="space-y-4">
          {actionNotice ? (
            <div className="rounded-lg border border-warning-border bg-warning-background px-3 py-2 text-caption text-muted" role="status">
              <div className="flex flex-wrap items-start justify-between gap-2">
                <div className="min-w-0">
                  <p className="font-semibold text-text">{actionNotice.title}</p>
                  <p className="mt-0.5">{actionNotice.description}</p>
                </div>
                <button
                  type="button"
                  onClick={() => setActionNotice(null)}
                  className="shrink-0 text-caption font-semibold text-text hover:text-brand"
                >
                  Dismiss
                </button>
              </div>
            </div>
          ) : null}
          {podUidParam && (
            <div className="rounded-lg border border-brand/30 bg-surface/10 p-3">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <p className="text-caption font-semibold text-brand">Pod filter</p>
                  <p className="text-body text-text mt-1 font-mono break-all">podUid={podUidParam}</p>
                  <p className="text-caption text-muted mt-1">
                    {selectedPodLoading
                      ? 'Loading paths for this pod…'
                      : selectedPodIssue
                        ? selectedPodIssue.title
                        : `${selectedPodPaths.length} path(s) returned for this workload.`}
                  </p>
                </div>
                <div className="flex flex-wrap gap-2 shrink-0">
                  {canOpenInventory ? (
                    <Link
                      to={`/resources/pods/uid/${encodeURIComponent(podUidParam)}`}
                      className="text-caption px-3 py-1.5 rounded-lg border border-border text-text hover:bg-surface-2"
                    >
                      Open pod
                    </Link>
                  ) : (
                    <button
                      type="button"
                      disabled
                      title="Requires inventory.read"
                      className="text-caption px-3 py-1.5 rounded-lg border border-border text-muted-2 opacity-70"
                    >
                      Open pod
                    </button>
                  )}
                  <button
                    type="button"
                    onClick={clearPodUidFilter}
                    className="text-caption px-3 py-1.5 rounded-lg border border-border text-muted hover:text-text hover:border-muted"
                  >
                    Clear filter
                  </button>
                </div>
              </div>
              {!selectedPodLoading && selectedPodIssue ? (
                <div className="mt-4 rounded-lg border border-border/70 bg-surface/40 px-3 py-2">
                  <AttackPathIssuePanel
                    issue={selectedPodIssue}
                    onAction={() => handleIssueAction(selectedPodIssue)}
                    compact
                  />
                </div>
              ) : null}
              {!selectedPodLoading && selectedPodPaths.length > 0 && (
                <div className="mt-4 max-h-48 overflow-y-auto rounded-lg border border-border bg-base/40 divide-y divide-border/80">
                  {selectedPodPaths.map((p) => (
                    <button
                      key={p.path_id || p.description}
                      type="button"
                      onClick={() => {
                        focusPathOnGraph(p.path_id);
                      }}
                      className="w-full text-left px-3 py-2 text-meta text-muted hover:bg-surface/80 hover:text-text transition-colors"
                    >
                      <span className="text-muted mr-2">{p.path_id}</span>
                      {p.description.slice(0, 120)}
                      {p.description.length > 120 ? '…' : ''}
                    </button>
                  ))}
                </div>
              )}
              <div className="mt-4 border-t border-border/80 pt-4">
                <p className="text-caption font-semibold text-muted mb-2">Runtime attack steps (this pod)</p>
                <AttackStepsTimeline podUid={podUidParam} />
              </div>
            </div>
          )}

          <div className="grid grid-cols-2 gap-2 md:grid-cols-3 xl:grid-cols-6">
            <StatCard icon={<Target size={14} className="text-brand" />} label="Scenario groups" value={visibleScenarios.length} accent="yellow" />
            <StatCard icon={<Layers size={14} className="text-red-400" />} label="Visible paths" value={`${visiblePathCount}/${totalPathCount || '—'}`} />
            <StatCard icon={<AlertTriangle size={14} className="text-red-400" />} label="Critical" value={summary?.criticalPaths ?? '—'} accent="red" />
            <StatCard icon={<Zap size={14} className="text-orange-400" />} label="High" value={summary?.highPaths ?? '—'} accent="orange" />
            <StatCard icon={<Shield size={14} className="text-yellow-400" />} label="Medium" value={summary?.mediumPaths ?? '—'} accent="yellow" />
            <StatCard icon={<Network size={14} className="text-muted" />} label="Low confidence" value={lowConfidenceScenarioCount} accent={lowConfidenceScenarioCount > 0 ? 'yellow' : undefined} />
          </div>

          <div className="flex flex-col gap-3 rounded-lg border border-border bg-surface/45 p-3 lg:flex-row lg:items-center lg:justify-between">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <span className="text-caption font-semibold text-muted">Confidence</span>
              {(['all', 'confirmed', 'probable', 'theoretical'] as const).map((lane) => (
                <button
                  key={lane}
                  type="button"
                  onClick={() => setConfidenceLane(lane)}
                  className={`rounded-md border px-2.5 py-1 text-caption font-semibold transition-colors ${
                    confidenceLane === lane
                      ? 'border-brand/60 bg-brand/15 text-brand'
                      : 'border-border bg-base/40 text-muted hover:text-text'
                  }`}
                >
                  {lane === 'all' ? `All (${groupedScenarios.length})` : `${lane[0].toUpperCase()}${lane.slice(1)}`}
                </button>
              ))}
              {lowConfidenceScenarioCount > 0 && (
                <button
                  type="button"
                  onClick={() => setShowLowConfidenceScenarios((v) => !v)}
                  className="rounded-md border border-border bg-base/40 px-2.5 py-1 text-caption font-semibold text-muted transition-colors hover:text-text"
                >
                  {showLowConfidenceScenarios ? 'Hide low-confidence' : `Show hidden (${hiddenCount})`}
                </button>
              )}
            </div>
            <p className="text-caption text-muted">
              Showing <span className="font-semibold text-text">{visiblePathCount}</span> of{' '}
              <span className="font-semibold text-text">{totalPathCount}</span> paths across{' '}
              <span className="font-semibold text-text">{visibleScenarios.length}</span> scenario group(s).
            </p>
            <div className="inline-flex w-fit max-w-full rounded-lg border border-border bg-base/60 p-1">
              <button
                type="button"
                onClick={() => setGraphExpanded(true)}
                className={`rounded-md px-3 py-1.5 text-caption font-semibold transition-colors sm:px-4 ${
                  graphExpanded ? 'bg-surface-2 text-text shadow-sm' : 'text-muted hover:text-text'
                }`}
              >
                Decision context
              </button>
              <button
                type="button"
                onClick={() => setGraphExpanded(false)}
                className={`rounded-md px-3 py-1.5 text-caption font-semibold transition-colors sm:px-4 ${
                  !graphExpanded ? 'bg-surface-2 text-text shadow-sm' : 'text-muted hover:text-text'
                }`}
              >
                Technical evidence
              </button>
            </div>
          </div>

          <section id="graph-section" className="min-w-0">
            <div className="rounded-xl border border-border bg-surface/45 p-3 shadow-sm">
              <div className="mb-3 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                <div className="min-w-0">
                  <h2 className="flex items-center gap-2 text-body font-semibold text-text">
                    <Layers size={16} className="text-brand" /> Attack path graph
                  </h2>
                  <p className="mt-1 text-caption text-muted">
                    {graphForView.nodes.length} nodes · {graphForView.links.length} edges · {selectedScenarioKey ? 'scenario focus' : 'full graph'} · exploitability
                  </p>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  {clickedPathId && (
                    <button
                      type="button"
                      onClick={() => setClickedPathId(null)}
                      className="rounded-md border border-brand/30 bg-brand/10 px-2.5 py-1.5 text-caption font-semibold text-brand transition-colors hover:bg-brand/20"
                    >
                      Clear path focus
                    </button>
                  )}
                  {highlightedStepIndex !== null && (
                    <button
                      type="button"
                      onClick={() => setHighlightedStepIndex(null)}
                      className="rounded-md border border-border bg-surface/80 px-2.5 py-1.5 text-caption font-semibold text-text transition-colors hover:bg-surface-2"
                    >
                      Clear step highlight
                    </button>
                  )}
                  <div className="inline-flex rounded-lg border border-border bg-base/60 p-1">
                    <button
                      type="button"
                      onClick={() => setSelectedScenarioKey(selectedScenarioKey || visibleScenarios[0].key)}
                      className={`rounded-md px-3 py-1.5 text-caption font-semibold transition-colors ${
                        selectedScenarioKey !== null ? 'bg-surface-2 text-text shadow-sm' : 'text-muted hover:text-text'
                      }`}
                    >
                      Scenario focus
                    </button>
                    <button
                      type="button"
                      onClick={() => setSelectedScenarioKey(null)}
                      className={`rounded-md px-3 py-1.5 text-caption font-semibold transition-colors ${
                        selectedScenarioKey === null ? 'bg-surface-2 text-text shadow-sm' : 'text-muted hover:text-text'
                      }`}
                    >
                      Full graph
                    </button>
                  </div>
                </div>
              </div>

              <GraphVisibilityOverlay
                semanticState={graphVisibility.semanticState}
                reason={graphVisibility.reason}
                className="mb-3"
              />
              <div className="relative overflow-hidden rounded-xl border border-border bg-base">
                <AttackPathGraph
                  data={graphForView}
                  onNodeClick={handleNodeClick}
                  highlightedNodeIds={highlightedNodeIds}
                  highlightedEdgeKeys={highlightedEdgeKeys}
                  focusNodeIds={focusNodeIds}
                  highlightedStepIndex={highlightedStepIndex}
                  semanticMode={attackGraphMode}
                  graphTrust={graphTrust}
                  className="h-[min(46dvh,500px)] min-h-[24rem] w-full"
                />
              </div>
              <GraphSemanticLegend mode={attackGraphMode} className="mt-3" />
            </div>
          </section>

          {graphExpanded ? (
            <div className="space-y-4 animate-in fade-in duration-150">
              {currentScenario && (
                <div className="space-y-2">
                  <PrimaryScenarioHero
                    scenario={currentScenario}
                    scenarioOrdinal={scenarioOrdinal}
                    visibleScenarioCount={visibleScenarios.length}
                    highlightedStepIndex={highlightedStepIndex}
                    onStepClick={(n) =>
                      setHighlightedStepIndex((prev) => (prev === n ? null : n))
                    }
                    onOpenTechnical={() => setGraphExpanded(false)}
                    onVisualize={() => {
                      setSelectedScenarioKey(currentScenario.key);
                      document.getElementById('graph-section')?.scrollIntoView({ behavior: 'smooth' });
                    }}
                  />
                  <PinToInvestigationButton
                    entity={attackPathInvestigationEntity(currentScenario, podUidParam)}
                  />
                </div>
              )}

              {visibleScenarios.length > 0 && (
                <AttackPathPriorityList
                  scenarios={visibleScenarios}
                  paths={primitivePaths}
                  selectedScenarioKey={selectedScenarioKey}
                  selectedPathId={clickedPathId}
                  onSelectScenario={(key) => {
                    setSelectedScenarioKey(key);
                    setGraphExpanded(false);
                  }}
                  onSelectPath={focusPathOnGraph}
                  onOpenGraph={() => {
                    setGraphExpanded(true);
                    document.getElementById('graph-section')?.scrollIntoView({ behavior: 'smooth' });
                  }}
                />
              )}

              {visibleScenarios.length > 0 && (
                <section>
                  <h2 className="text-micro font-bold text-muted mb-3 uppercase tracking-widest flex items-center gap-2">
                    <Target size={12} /> All scenarios
                  </h2>
                  <div className="grid grid-cols-1 gap-4">
                    {visibleScenarios.map((s) => {
                      const activeKey = selectedScenarioKey ?? visibleScenarios[0]?.key;
                      const active = s.key === activeKey;
                      return (
                        <ScenarioCard
                          key={s.key}
                          scenario={s}
                          paths={primitivePaths}
                          active={active}
                          alwaysExpanded={s.key === visibleScenarios[0]?.key}
                          onToggle={() => setSelectedScenarioKey(s.key)}
                          onHoverPathId={(id) => setHoveredPathId(id)}
                        />
                      );
                    })}
                  </div>
                </section>
              )}
            </div>
          ) : (
            <section className="animate-in slide-in-from-right-4 duration-500">
               <div className="flex flex-wrap items-center justify-between gap-3 mb-4">
                 <h2 className="text-base font-semibold text-text flex items-center gap-2">
                    <Shield size={18} className="text-muted" />
                    Technical Evidence & Path Variants
                 </h2>
                 {visibleScenarios.length > 1 && (
                   <div className="flex flex-wrap gap-2">
                     {visibleScenarios.map((s) => (
                       <button
                         key={s.key}
                         type="button"
                         onClick={() => setSelectedScenarioKey(s.key)}
                         className={`px-2 py-1 rounded border text-micro font-semibold transition-colors ${
                           (selectedScenarioKey ?? visibleScenarios[0]?.key) === s.key
                             ? 'border-brand bg-brand/15 text-brand'
                             : 'border-border text-muted hover:border-border'
                         }`}
                       >
                         {s.headline.slice(0, 48)}{s.headline.length > 48 ? '…' : ''}
                       </button>
                     ))}
                   </div>
                 )}
               </div>
               <div className="flex flex-col xl:flex-row gap-6 xl:items-start">
                  <div className="w-full xl:w-[min(100%,380px)] xl:shrink-0 space-y-6">
                      {(() => {
                      const techScenario = visibleScenarios.find(s => s.key === selectedScenarioKey) || visibleScenarios[0];
                      const techVariants = Array.isArray(techScenario.variants) ? techScenario.variants : [];
                      return (
                          <div className="bg-surface/50 border border-border rounded-xl p-4 xl:sticky xl:top-4 xl:max-h-[min(80dvh,560px)] xl:flex xl:flex-col">
                             <h3 className="text-caption font-bold text-muted mb-4 uppercase tracking-wider shrink-0">Affected Entities ({techVariants.length})</h3>
                             <div className="space-y-2 overflow-y-auto min-h-0 flex-1 pr-1">
                               {techVariants.map((v, i) => (
                                 <div key={i} className="px-3 py-2 rounded bg-base/50 border border-border/80 text-meta text-text font-mono">
                                   <div className="flex flex-wrap items-center gap-2 mb-1">
                                     <span className={`text-micro px-1.5 py-0.5 rounded border shrink-0 ${getSeverityTextClass(v.confidence)} border-current/30`}>
                                       {String(v.confidence || 'n/a').toUpperCase()}
                                     </span>
                                     <span className="text-micro text-muted tabular-nums">str {(v.chain_strength ?? 0).toFixed(2)}</span>
                                   </div>
                                   <span className="break-all" title={v.final_target}>{compactText(v.final_target, 96)}</span>
                                   <div className="mt-1 text-micro text-muted">{(Array.isArray(v.paths) ? v.paths : []).length} unique paths</div>
                                 </div>
                               ))}
                             </div>
                          </div>
                        );
                      })()}
                  </div>

                  <div className="flex-1 min-w-0 flex flex-col min-h-0">
                    {(() => {
                      const techScenario = visibleScenarios.find(s => s.key === selectedScenarioKey) || visibleScenarios[0];
                      const techVariants = Array.isArray(techScenario.variants) ? techScenario.variants : [];
                      const scenarioPaths = primitivePaths.filter((p) =>
                        techVariants.some((v) => (Array.isArray(v.paths) ? v.paths : []).includes(p.path_id || '')),
                      );
                      return (
                        <div className="space-y-3 flex flex-col min-h-0">
                          <h3 className="text-caption font-bold text-muted uppercase tracking-wider shrink-0">Path Details ({scenarioPaths.length})</h3>
                          <div className="grid grid-cols-1 gap-3 max-h-[min(70dvh,720px)] overflow-y-auto overscroll-contain pr-1">
                            {scenarioPaths.map((path, idx) => (
                               <PathCard
                                 key={path.path_id || idx}
                                 path={path}
                                 isSelected={clickedPathId === path.path_id}
                                 onSelect={() => path.path_id && focusPathOnGraph(path.path_id === clickedPathId ? null : path.path_id)}
                                 onPointerEnter={() => path.path_id && setHoveredPathId(path.path_id)}
                                 onPointerLeave={() => setHoveredPathId(null)}
                               />
                            ))}
                          </div>
                        </div>
                      );
                    })()}
                  </div>
               </div>
            </section>
          )}
        </div>
      ) : (
        <div className="flex flex-col items-center justify-center min-h-[420px] text-muted">
          <Shield size={48} className="text-muted-2 mb-4" />
          <p>No attack scenarios detected.</p>
        </div>
      )}
    </PageLayout>
  );
};

/* ─── sub-components ──────────────────────────────────────── */

const StatCard: React.FC<{
  icon: React.ReactNode; label: string; value: string | number; loading?: boolean; accent?: string;
}> = ({ icon, label, value, loading, accent }) => (
  <div className="bg-surface/70 border border-border rounded-lg p-2.5 sm:p-3 min-h-[4.5rem] sm:min-h-[5rem] flex flex-col justify-between">
    <div className="flex items-start gap-1.5 mb-0.5 min-w-0">
      <span className="shrink-0 mt-0.5">{icon}</span>
      <span className="text-caption text-muted uppercase tracking-wide leading-tight line-clamp-2">{label}</span>
    </div>
    {loading ? (
      <div className="h-6 sm:h-7 w-14 sm:w-16 bg-surface-2 rounded animate-pulse mt-1" />
    ) : (
      <p className={`text-lg sm:text-xl md:text-2xl font-bold tabular-nums leading-tight truncate ${accent === 'red' ? 'text-red-400' : accent === 'orange' ? 'text-orange-400' : accent === 'yellow' ? 'text-yellow-400' : 'text-text'}`}>
        {value}
      </p>
    )}
  </div>
);

/* ─── ScenarioCard (grouped, Phase 1–4) ──────────────────── */

const ScenarioCard: React.FC<{
  scenario: GroupedScenario;
  paths: AttackPath[];
  active: boolean;
  alwaysExpanded?: boolean;
  onToggle: () => void;
  /** Highlights first primitive path on the graph when hovering the card */
  onHoverPathId: (pathId: string | null) => void;
}> = ({ scenario, paths, active, alwaysExpanded = false, onToggle, onHoverPathId }) => {
  const [expanded, setExpanded] = useState(false);
  const [detailsOpen, setDetailsOpen] = useState(alwaysExpanded);
  useEffect(() => {
    if (alwaysExpanded) setDetailsOpen(true);
  }, [alwaysExpanded]);
  const chain = scenario.representativeChain;
  const story = chain.story;
  const headline = scenario.headline || story?.headline;
  const narrative = scenario.narrative || story?.narrative;
  const impactText = story?.impact_text
    ? story.impact_text.split('\n')
    : Array.isArray(scenario.impactStatements)
      ? scenario.impactStatements
      : [];
  const chainSteps = Array.isArray(chain.steps) ? chain.steps : [];
  const variants = Array.isArray(scenario.variants) ? scenario.variants : [];
  const scenarioPaths = Array.isArray(paths) ? paths : [];
  const evidenceSummary = Array.isArray(story?.evidence_summary) ? story!.evidence_summary! : [];
  const assumptionSummary = Array.isArray(story?.assumption_summary) ? story!.assumption_summary! : [];
  const evidenceBrief = topItems(evidenceSummary, 3);
  const assumptionBrief = topItems(assumptionSummary, 2);
  const impactBrief = topItems(impactText, 3);
  const fixes = buildFixRecommendations(chain).slice(0, 3);
  const requirements = exploitRequirementSummary(chain);
  
  // Parse confidence
  const confRaw = chain.confidence_raw ?? 0;
  const realismVal = chain.realism ?? 0;
  const blastVal = typeof chain.blast_radius === 'number' ? Math.max(0, Math.min(1, chain.blast_radius)) : 0;
  const blast = blastRadiusLabel(blastVal);
  const runtimeEventsSeen = chain.mitre_summary?.runtime_events_considered ?? 0;
  const runtimeEventsMatched = chain.mitre_summary?.runtime_events_matched ?? 0;
  const runtimeObserved = runtimeEventsMatched > 0 || (chain.mitre_summary?.runtime_mitre_distinct ?? 0) > 0;
  const runtimeScoped = runtimeEventsSeen > 0 || (chain.mitre_summary?.observing_pod_count ?? 0) > 0;
  const softOrGap =
    Boolean(chain.capability_validation?.soft_mode) ||
    (Array.isArray(chain.capability_validation?.gaps) && chain.capability_validation.gaps.length > 0);

  const firstPathId = (Array.isArray(chain.paths) ? chain.paths : [])[0] ?? null;
  const showDetails = alwaysExpanded || detailsOpen;

  return (
    <div
      className={`rounded-lg border p-4 transition-colors ${active ? 'border-brand/70 bg-surface/20 ring-1 ring-brand/40' : 'border-border bg-surface/70 hover:border-border'}`}
      onPointerEnter={() => firstPathId && onHoverPathId(firstPathId)}
      onPointerLeave={() => onHoverPathId(null)}
    >
      {/* Header */}
      <div className="mb-3">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
          <button type="button" onClick={onToggle} className="min-w-0 flex-1 text-left">
            <h3 className="text-base font-semibold text-text leading-snug">{headline}</h3>
          </button>
          {alwaysExpanded ? (
            <span className="shrink-0 rounded border border-brand/30 bg-brand/10 px-2 py-1 text-micro font-semibold text-brand">
              Highest priority · full detail
            </span>
          ) : (
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                setDetailsOpen((v) => !v);
              }}
              className="inline-flex shrink-0 items-center gap-1 rounded border border-border bg-base/60 px-2.5 py-1.5 text-caption font-semibold text-text transition-colors hover:bg-surface-2"
              aria-expanded={showDetails}
            >
              {showDetails ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
              {showDetails ? 'Hide details' : 'Show details'}
            </button>
          )}
        </div>
        <div className="flex items-center gap-4 mt-1.5 text-caption">
          <span className="flex items-center gap-1.5">
            <span className="text-muted">Confidence:</span>
            <span className={`font-semibold ${getSeverityTextClass(chain.confidence)}`}>
              {chain.confidence.toUpperCase()} {confRaw > 0 ? `(${(confRaw * 100).toFixed(0)}%)` : ''}
            </span>
          </span>
          <span className="flex items-center gap-1.5">
            <span className="text-muted">Exploit cost:</span>
            <span className="text-text font-medium">{chain.exploit_cost || 'N/A'}</span>
          </span>
        </div>
        <div className="mt-2 flex flex-wrap gap-1.5">
          <span
            title={runtimeObserved ? 'Runtime events matched path techniques.' : runtimeScoped ? 'Runtime events exist but do not align to path techniques.' : 'Grounded by Kubernetes config and graph relations.'}
            className={`rounded border px-1.5 py-0.5 text-micro font-medium ${
              runtimeObserved
                ? 'border-emerald-500/40 bg-emerald-500/10 text-emerald-300'
                : runtimeScoped
                  ? 'border-amber-500/40 bg-amber-500/10 text-amber-200'
                  : 'border-border bg-surface-2/80 text-muted'
            }`}
          >
            {runtimeObserved ? 'Runtime observed' : runtimeScoped ? 'Runtime no match' : 'Config evidence'}
          </span>
          {softOrGap && (
            <span
              title="Capability continuity uses advisory soft mode or has missing pre-state."
              className="rounded border border-indigo-500/35 bg-indigo-500/10 px-1.5 py-0.5 text-micro font-medium text-indigo-200"
            >
              Inferred/soft
            </span>
          )}
        </div>
        {!showDetails && (
          <p className="mt-2 text-caption text-muted">
            Detail hidden. Show details to view evidence, blast radius, exploit requirements, immediate actions, impact, and technical steps.
          </p>
        )}
      </div>

      {showDetails && (
        <>
          {/* 1. Evidence */}
          {evidenceBrief.length > 0 && (
            <div className="mt-3">
              <p className="text-caption text-text font-semibold mb-1">Evidence</p>
              <ul className="space-y-1">
                {evidenceBrief.map((ev, i) => (
                  <li key={i} className="flex items-start gap-1.5 text-caption text-muted">
                    <span className="text-muted mt-0.5 shrink-0">•</span>
                    <span>{compactText(ev, 128)}</span>
                  </li>
                ))}
              </ul>
              {evidenceSummary.length > evidenceBrief.length ? (
                <p className="mt-1 text-meta text-muted-2">+{evidenceSummary.length - evidenceBrief.length} more signal(s) in technical details</p>
              ) : null}
            </div>
          )}

          {/* 2. Assumptions */}
          {assumptionBrief.length > 0 && (
            <div className="mt-3">
              <p className="text-caption text-text font-semibold mb-1">Assumptions</p>
              <ul className="space-y-1">
                {assumptionBrief.map((as, i) => (
                  <li key={i} className="flex items-start gap-1.5 text-caption text-amber-200/80">
                    <span className="text-amber-500/50 mt-0.5 shrink-0">•</span>
                    <span>{compactText(as, 128)}</span>
                  </li>
                ))}
              </ul>
              {assumptionSummary.length > assumptionBrief.length ? (
                <p className="mt-1 text-meta text-muted-2">+{assumptionSummary.length - assumptionBrief.length} more assumption(s)</p>
              ) : null}
            </div>
          )}

          {/* 3. Story */}
          <div className="mt-4 rounded border border-border/80 bg-base/50 p-3">
            <p className="text-body text-text leading-relaxed">{compactText(narrative, 220)}</p>
          </div>

      {/* 4. Operational summary */}
      <div className="mt-3 grid grid-cols-1 gap-2 md:grid-cols-3">
        <div className="rounded border border-border/70 bg-base/40 p-2">
          <p className="text-micro uppercase tracking-wide text-muted">Blast radius</p>
          <p className={`text-caption font-semibold ${blast.className}`}>
            {blast.label} <span className="font-mono text-muted">({(blastVal * 100).toFixed(0)}%)</span>
          </p>
        </div>
        <div className="rounded border border-border/70 bg-base/40 p-2">
          <p className="text-micro uppercase tracking-wide text-muted">Exploit realism</p>
          <p className="text-caption font-semibold text-text">
            {(realismVal * 100).toFixed(0)}% <span className="font-normal text-muted">confidence-adjusted</span>
          </p>
        </div>
        <div className="rounded border border-border/70 bg-base/40 p-2">
          <p className="text-micro uppercase tracking-wide text-muted">Exploit cost</p>
          <p className="text-caption font-semibold text-text">{chain.exploit_cost || 'N/A'}</p>
        </div>
      </div>

      <div className="mt-3 grid grid-cols-1 gap-3 lg:grid-cols-2">
        <div className="rounded border border-border/80 bg-surface/40 p-3">
          <p className="mb-2 flex items-center gap-1.5 text-caption font-semibold text-text">
            <Crosshair size={13} className="text-indigo-300" /> Exploit requirements
          </p>
          <div className="grid grid-cols-1 gap-1.5 min-[480px]:grid-cols-3">
            {requirements.map((item) => (
              <div key={item.label} className="rounded border border-border/60 bg-base/40 px-2 py-1.5">
                <p className="text-micro uppercase tracking-wide text-muted">{item.label}</p>
                <p className="text-caption font-medium text-text">{item.value}</p>
              </div>
            ))}
          </div>
        </div>

        <div className="rounded border border-red-500/25 bg-red-950/10 p-3">
          <p className="mb-2 flex items-center gap-1.5 text-caption font-semibold text-text">
            <Wrench size={13} className="text-red-300" /> Immediate actions
          </p>
          <ol className="space-y-1.5">
            {fixes.map((fix, i) => (
              <li key={`${fix.label}-${i}`} className="flex items-start gap-2 text-caption text-text">
                <span className="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-full bg-red-500/15 text-micro font-bold text-red-300">
                  {i + 1}
                </span>
                <span>
                  {compactText(fix.label, 112)}
                  <span className="ml-1 font-mono text-micro text-red-300">{fix.priority}</span>
                </span>
              </li>
            ))}
          </ol>
        </div>
      </div>

      {/* 5. Impact */}
      {impactBrief.length > 0 && (
        <div className="mt-3">
          <p className="text-caption text-text font-semibold mb-1">Impact</p>
          <ul className="text-caption text-muted space-y-0.5">
            {impactBrief.map((s, i) => (
              <li key={i} className="flex items-start gap-1.5">
                <span className="text-red-400 mt-0.5 shrink-0">→</span>
                <span>{compactText(s.replace(/^[•\-\*]\s*/, ''), 132)}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* 6. Steps (Technical Details) */}
      {chainSteps.length > 0 && (
        <details className="mt-3 group">
          <summary className="cursor-pointer text-caption font-medium text-brand hover:text-brand/90 flex items-center gap-1 list-none [&::-webkit-details-marker]:hidden">
            <ChevronRight size={14} className="group-open:rotate-90 transition-transform shrink-0" />
            Technical Steps ({chainSteps.length})
          </summary>
          <div className="mt-2 ml-1 border-l-2 border-border pl-3 space-y-2">
            {chainSteps.slice(0, 6).map((step, i) => (
              <div key={i} className="text-caption">
                <div className="flex justify-between items-start">
                  <p className="text-text font-medium">
                    <span className="text-muted mr-1">{i + 1}.</span> {step.name}
                  </p>
                  <span className="text-muted font-mono text-micro">{step.technique_id}</span>
                </div>
                <div className="flex gap-3 mt-0.5 text-muted text-micro">
                  <span>Realism: {(step.realism * 100).toFixed(0)}%</span>
                  <span>Cost: {step.cost}</span>
                </div>
              </div>
            ))}
            {chainSteps.length > 6 ? (
              <p className="text-meta text-muted-2">+{chainSteps.length - 6} more step(s)</p>
            ) : null}
          </div>
        </details>
      )}

      {/* Confidence Breakdown UI (Phase 6.5) */}
      {(confRaw > 0 || realismVal > 0) && (
        <details className="mt-2 group">
          <summary className="cursor-pointer text-meta font-medium text-muted hover:text-muted flex items-center gap-1 list-none [&::-webkit-details-marker]:hidden">
            <ChevronRight size={12} className="group-open:rotate-90 transition-transform shrink-0" />
            Scoring Breakdown
          </summary>
          <div className="mt-2 ml-4 p-2 rounded bg-base/30 border border-border/50 text-meta text-muted">
            <p>Detection Confidence: <span className="text-text">{(confRaw * 100).toFixed(1)}%</span></p>
            <p>Exploit Realism: <span className="text-text">{(realismVal * 100).toFixed(1)}%</span></p>
            <p className="mt-1 text-muted italic">Scores are prioritization factors from technique properties and environment signals; validate assumptions before treating a chain as confirmed.</p>
          </div>
        </details>
      )}

          {/* Variants (Phase 1 dedup) */}
          {variants.length > 1 && (
            <div className="mt-3">
              <button
                type="button"
                onClick={(e) => { e.stopPropagation(); setExpanded((v) => !v); }}
                className="flex items-center gap-1 text-caption text-muted hover:text-text"
              >
                {expanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
                {variants.length} variant{variants.length !== 1 ? 's' : ''} (different source pods/paths)
              </button>
              {expanded && (
                <ul className="mt-2 ml-4 text-meta text-muted space-y-1">
                  {variants.map((v, i) => (
                    <li key={i} className="font-mono">
                      {variantLabel(v, scenarioPaths.filter((p) => (Array.isArray(v.paths) ? v.paths : []).includes(p.path_id || '')))}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}
        </>
      )}
    </div>
  );
};

/* ─── PathCard (Technical evidence detail) ────────────────── */

const PathCard: React.FC<{
  path: AttackPath;
  isSelected?: boolean;
  onSelect?: () => void;
  onPointerEnter?: () => void;
  onPointerLeave?: () => void;
}> = ({ path, isSelected, onSelect, onPointerEnter, onPointerLeave }) => {
  const level = deriveUnifiedRiskLevelFromScore(path.total_risk * 10) || riskLabel(path.total_risk);
  const whyItMatters = buildWhyItMattersHuman(path);
  const weakLinks = weakLinksFromPath(path);
  const x = path.explainability;
  const pathNodes = Array.isArray(path.nodes) ? path.nodes : [];
  const capabilities = Array.isArray(x?.evidence?.capabilities) ? x.evidence.capabilities : [];
  const attackSteps = Array.isArray(x?.evidence?.attack_steps) ? x.evidence.attack_steps : [];
  const keyCapabilities = topItems(capabilities, 3);
  const keyAttackSteps = topItems(attackSteps, 3);
  const keyWeakLinks = topItems(weakLinks, 3);

  return (
    <div
      role={onSelect ? 'button' : undefined}
      tabIndex={onSelect ? 0 : undefined}
      onClick={onSelect}
      onKeyDown={onSelect ? (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onSelect(); } } : undefined}
      onPointerEnter={onPointerEnter}
      onPointerLeave={onPointerLeave}
      className={`bg-surface/70 border rounded-lg p-3 sm:p-4 transition-colors text-left w-full ${
        isSelected ? 'border-brand ring-1 ring-brand/40 shadow-lg shadow-black/20' : 'border-border hover:border-border'
      } ${onSelect ? 'cursor-pointer' : ''}`}
    >
      {/* Severity + semantic labels (Phase 6) */}
      <div className="flex flex-wrap items-center gap-2 text-caption mb-2">
        <span className={`font-semibold ${getSeverityTextClass(level)}`}>{level}</span>
        <span className="rounded border border-border bg-base/40 px-2 py-0.5 text-muted">{strengthSemantic(x?.strength ?? path.total_risk / 10)}</span>
        <span className="rounded border border-border bg-base/40 px-2 py-0.5 text-muted">{impactSemantic(path)}</span>
        {path.path_id ? <span className="ml-auto font-mono text-micro text-muted">{path.path_id}</span> : null}
      </div>
      <p className="mb-2 text-caption text-text leading-relaxed">{compactText(path.description, 150)}</p>

      {/* Hop chain */}
      <div className="flex items-center gap-1 text-caption text-muted overflow-x-auto pb-1">
        {pathNodes.slice(0, 7).map((n, i) => (
          <React.Fragment key={n.id}>
            <span className="px-2 py-0.5 bg-surface-2 border border-border rounded whitespace-nowrap">
              <span className="text-muted">{n.type}: </span>
              <span className="text-text">{(n.properties.name as string) || n.id.slice(0, 12)}</span>
            </span>
            {i < Math.min(pathNodes.length, 7) - 1 && <ChevronRight size={12} className="text-muted-2 shrink-0" />}
          </React.Fragment>
        ))}
        {pathNodes.length > 7 ? <span className="shrink-0 text-meta text-muted-2">+{pathNodes.length - 7} hops</span> : null}
      </div>

      {/* Why it matters (impact-driven) */}
      <div className="mt-2 rounded border border-border bg-base/50 p-2">
        <p className="text-caption text-text font-semibold mb-1">Why it matters</p>
        <p className="text-caption text-muted leading-relaxed">{compactText(whyItMatters, 170)}</p>
        {keyWeakLinks.length > 0 && (
          <ul className="mt-2 text-caption text-amber-200/90 list-disc list-inside space-y-0.5">
            {keyWeakLinks.map((w) => (<li key={w}>{compactText(w, 120)}</li>))}
          </ul>
        )}
      </div>

      {/* Technical details (collapsed, Phase 6) */}
      {x && (
        <details className="mt-3 group rounded-lg border border-border bg-base/40">
          <summary className="cursor-pointer px-2 py-1.5 text-caption font-medium text-muted hover:text-text list-none flex items-center gap-1 [&::-webkit-details-marker]:hidden">
            <ChevronRight size={14} className="text-muted group-open:rotate-90 transition-transform shrink-0" />
            Technical details (for analysts)
          </summary>
          <div className="px-2 pb-2 pt-1 grid grid-cols-1 md:grid-cols-2 gap-3 text-caption border-t border-border/80">
            <div className="rounded border border-border/80 bg-base/50 p-2">
              <p className="text-text font-semibold mb-1">Path overview</p>
              <p className="text-muted">Class: <span className="text-text font-mono">{x.class}</span></p>
              <p className="text-muted">Strength: <span className="text-text">{strengthSemantic(x.strength)}</span></p>
              <p className="text-muted">Hops: <span className="text-text">{path.length}</span></p>
            </div>
            <div className="rounded border border-border/80 bg-base/50 p-2">
              <p className="text-text font-semibold mb-1">Feasibility</p>
              <p className="text-muted">Network: <span className="text-text">{x.feasibility.network_decision}</span></p>
              <p className="text-muted">Reason: <span className="text-text font-mono">{x.feasibility.reason}</span></p>
            </div>
            <div className="rounded border border-border/80 bg-base/50 p-2 md:col-span-2">
              <p className="text-text font-semibold mb-1">Evidence signals</p>
              <p className="text-muted">Capabilities: <span className="text-text">{keyCapabilities.join(', ') || '—'}</span></p>
              <p className="text-muted">Attack steps: <span className="text-text">{keyAttackSteps.join(', ') || '—'}</span></p>
            </div>
          </div>
        </details>
      )}
    </div>
  );
};
