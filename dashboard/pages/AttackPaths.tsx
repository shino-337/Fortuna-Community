import React, { useEffect, useState, useCallback, useMemo, useRef } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PageError, PageLoading } from '../design-system/components/PageStatus';
import { SemanticEmptyState } from '../design-system/components/SemanticEmptyState';
import { Button } from '../components/ui/Button';
import { AttackPathGraph } from '../components/AttackPathGraph';
import { api, isApiError } from '../lib/api';
import type { AttackPathGraphData, AttackPathSummary, AttackPath, AttackChain, AttackPathNode, AttackStep } from '../types';
import { useClusterStore } from '../store/clusterStore';
import { RefreshCw, Shield, X } from 'lucide-react';
import clsx from 'clsx';
import { AttackStepsTimeline } from '../components/AttackStepsTimeline';
import { pathRiskLevel, type SeverityLevel } from '../lib/severity';
import { scenarioEnds } from '../lib/attackPathNarrative';
import {
  groupChains,
  groupPrimitivePaths,
  fixesAcrossScenarios,
  scenarioMaxRisk,
  scenarioPathIds,
  type GroupedScenario,
} from '../lib/attackPathNarrative';
import { ATTACK_PATH_LANE_LABELS, attackPathConfidenceLane, type AttackPathConfidenceLane } from '../lib/attackPathConfidence';
import { AddToCaseButton } from '../components/AddToCaseButton';
import { findingsForResourcePath, identityDetailPath } from '../lib/entityLinks';
import { attackPathInvestigationEntity } from '../lib/investigationEntities';
import {
  AttackPathDetail,
  AttackPathList,
  ChokePoints,
  FixFilterChip,
  PostureStrip,
  rankSharedFixes,
  scenarioTargetGroup,
} from '../components/attack-paths/AttackPathWorkspace';
import { AttackPathOverview } from '../components/attack-paths/AttackPathOverview';
import { nodeHref, techniqueStepIndex, TARGET_GROUP_LABEL, type TargetGroup } from '../lib/attackPathSteps';
import { GraphVisibilityOverlay } from '../components/GraphVisibilityOverlay';
import { useFeatureVisibility } from '../hooks/useVisibility';
import { useIncidentMode } from '../hooks/useIncidentMode';
import { useGraphTrustContext } from '../hooks/useGraphTrustContext';
import { useOperationalMaterialization } from '../hooks/useOperationalMaterialization';
import { PAGE_TITLES } from '../lib/pageTitles';
import type { GraphSemanticMode } from '../lib/persona';
import { useCan } from '../hooks/usePermUser';
import { P } from '../lib/permissions';
import type { SemanticVisibilityState } from '../lib/visibilityEngine';
import { podDetailPath } from '../lib/podRoute';

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
          ? `Attack Paths requires ${required}. Ask an administrator to update your Fortuna role or permissions.`
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
  // 502 is produced locally by strict response validation. Falling back after a
  // malformed successful payload would erase the protocol failure and could
  // reinterpret incomplete data as a valid empty/partial attack-path state.
  return isApiError(error) && (error.status === 401 || error.status === 403 || error.status === 502);
}

/* ─── helpers ─────────────────────────────────────────────── */





const riskLabel = pathRiskLevel;

function buildGraphDataFromPrimitivePaths(paths: AttackPath[], techniques?: AttackStep[]): AttackPathGraphData {
  const nodeMap = new Map<string, AttackPathGraphData['nodes'][0]>();
  const linkMap = new Map<string, AttackPathGraphData['links'][0]>();
  const riskFromScore = riskLabel;
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
        label: 'start',
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
      
      const mappedStepIdx = techniqueStepIndex(e.type, techniques);
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
  const { selectedClusterId, setSelectedClusterId } = useClusterStore();
  const canOpenInventory = useCan(P.inventoryRead);
  const { allowedRoutes } = useOperationalMaterialization();
  const graphVisibility = useFeatureVisibility('attack_paths');
  const { graphSemanticMode } = useIncidentMode();
  const attackGraphMode: GraphSemanticMode = graphSemanticMode ?? 'exploitability';
  const graphTrust = useGraphTrustContext('attack_paths', attackGraphMode);

  const [graphData, setGraphData] = useState<AttackPathGraphData>({ nodes: [], links: [] });
  const [summary, setSummary] = useState<AttackPathSummary | null>(null);
  const [chains, setChains] = useState<AttackChain[]>([]);
  const [primitivePaths, setPrimitivePaths] = useState<AttackPath[]>([]);
  const [selectedScenarioKey, setSelectedScenarioKey] = useState<string | null>(null);
  const [confidenceLane, setConfidenceLane] = useState<AttackPathConfidenceLane | 'all'>('all');
  // 'urgent' = critical or high, set from the posture strip.
  const [levelFilter, setLevelFilter] = useState<SeverityLevel | 'urgent' | ''>('');
  const [targetFilter, setTargetFilter] = useState<TargetGroup | ''>('');
  const [namespaceFilter, setNamespaceFilter] = useState('');
  const [fixFilter, setFixFilter] = useState('');
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [dataIssue, setDataIssue] = useState<AttackPathIssue | null>(null);
  const [actionNotice, setActionNotice] = useState<AttackPathIssue | null>(null);
  const dataRequestSequence = useRef(0);
  const podRequestSequence = useRef(0);

  const podUidParam = searchParams.get('podUid') || '';
  const clusterIdParam = searchParams.get('clusterId') || '';
  const effectiveClusterId = clusterIdParam || selectedClusterId || '';
  const podClusterIdParam = effectiveClusterId;
  const [selectedPodPaths, setSelectedPodPaths] = useState<AttackPath[]>([]);
  const [selectedPodLoading, setSelectedPodLoading] = useState(false);
  const [selectedPodIssue, setSelectedPodIssue] = useState<AttackPathIssue | null>(null);

  /** 1-based — aligns with graph link `stepIndex` when mapped from technique categories */
  const [highlightedStepIndex, setHighlightedStepIndex] = useState<number | null>(null);

  // Apply a deep-linked ?clusterId= to the global selector when the URL changes. This must not depend on
  // selectedClusterId: otherwise every header cluster change is immediately reverted to the URL value.
  useEffect(() => {
    if (clusterIdParam) setSelectedClusterId(clusterIdParam);
  }, [clusterIdParam, setSelectedClusterId]);

  // Once the user picks another cluster in the header, the URL param is stale; drop it so the new scope wins.
  const clusterSyncMountedRef = useRef(false);
  useEffect(() => {
    if (!clusterSyncMountedRef.current) {
      clusterSyncMountedRef.current = true;
      return;
    }
    if (!clusterIdParam || selectedClusterId === clusterIdParam) return;
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      next.delete('clusterId');
      return next;
    }, { replace: true });
  }, [selectedClusterId]); // eslint-disable-line react-hooks/exhaustive-deps

  const fetchData = useCallback(async () => {
    const sequence = ++dataRequestSequence.current;
    setLoading(true);
    setDataIssue(null);
    try {
      const bundle = await api.getAttackPathsBundleStrict(effectiveClusterId || undefined);
      if (sequence !== dataRequestSequence.current) return;
      if (bundle) {
        setGraphData(bundle.graph);
        setSummary(bundle.summary);
        setChains(bundle.chains);
        setPrimitivePaths(bundle.paths ?? []);
      } else {
        const [graph, sum, chs] = await Promise.all([
          api.getAttackPathsGraphStrict(effectiveClusterId || undefined),
          api.getAttackPathsSummaryStrict(effectiveClusterId || undefined),
          api.getAttackChainsStrict(effectiveClusterId || undefined),
        ]);
        if (sequence !== dataRequestSequence.current) return;
        setGraphData(graph);
        setSummary(sum);
        setChains(chs);
        setPrimitivePaths([]);
      }
      if (sequence !== dataRequestSequence.current) return;
      setDataIssue(null);
      setActionNotice(null);
    } catch (err) {
      if (sequence !== dataRequestSequence.current) return;
      if (shouldStopAttackPathFallback(err)) {
        setGraphData({ nodes: [], links: [] });
        setSummary(null);
        setChains([]);
        setPrimitivePaths([]);
        setDataIssue(classifyAttackPathIssue(err));
      } else {
        try {
          const [graph, sum, chs] = await Promise.all([
            api.getAttackPathsGraphStrict(effectiveClusterId || undefined),
            api.getAttackPathsSummaryStrict(effectiveClusterId || undefined),
            api.getAttackChainsStrict(effectiveClusterId || undefined),
          ]);
          if (sequence !== dataRequestSequence.current) return;
          setGraphData(graph);
          setSummary(sum);
          setChains(chs);
          setPrimitivePaths([]);
          setDataIssue(null);
        } catch (fallbackErr) {
          if (sequence !== dataRequestSequence.current) return;
          setGraphData({ nodes: [], links: [] });
          setSummary(null);
          setChains([]);
          setPrimitivePaths([]);
          setDataIssue(classifyAttackPathIssue(fallbackErr));
        }
      }
    } finally {
      if (sequence === dataRequestSequence.current) setLoading(false);
    }
  }, [effectiveClusterId]);

  useEffect(() => {
    void fetchData();
    return () => { dataRequestSequence.current++; };
  }, [fetchData]);

  useEffect(() => {
    const sequence = ++podRequestSequence.current;
    if (!podUidParam) {
      setSelectedPodPaths([]);
      setSelectedPodIssue(null);
      setSelectedPodLoading(false);
      return () => { podRequestSequence.current++; };
    }
    setSelectedPodLoading(true);
    setSelectedPodIssue(null);
    api.getAttackPathsForPodStrict(podUidParam, podClusterIdParam || undefined)
      .then((paths) => {
        if (sequence !== podRequestSequence.current) return;
        setSelectedPodPaths(paths);
        setSelectedPodIssue(null);
      })
      .catch((err) => {
        if (sequence !== podRequestSequence.current) return;
        setSelectedPodPaths([]);
        setSelectedPodIssue(classifyAttackPathIssue(err, 'Could not load pod attack paths'));
      })
      .finally(() => {
        if (sequence === podRequestSequence.current) setSelectedPodLoading(false);
      });
    return () => { podRequestSequence.current++; };
  }, [podUidParam, podClusterIdParam]);

  const handleRefresh = async () => {
    setRefreshing(true);
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

  const pathById = useMemo(() => {
    const m = new Map<string, AttackPath>();
    for (const p of primitivePaths) if (p.path_id) m.set(p.path_id, p);
    return m;
  }, [primitivePaths]);

  // ?podUid= narrows every section to the scenarios that run through that pod.
  const podPathIds = useMemo(() => new Set(selectedPodPaths.map((p) => p.path_id).filter(Boolean) as string[]), [selectedPodPaths]);
  const podScopedScenarios = useMemo(() => {
    if (!podUidParam || selectedPodLoading || selectedPodIssue) return groupedScenarios;
    return groupedScenarios.filter((s) => scenarioPathIds(s).some((id) => podPathIds.has(id)));
  }, [groupedScenarios, podUidParam, selectedPodLoading, selectedPodIssue, podPathIds]);

  const scopedPaths = useMemo(
    () => (podUidParam && !selectedPodIssue ? selectedPodPaths : primitivePaths),
    [podUidParam, selectedPodIssue, selectedPodPaths, primitivePaths],
  );

  // The list shows scenarios, so the level buttons count scenarios too: a scenario's level is its highest path.
  const scenarioLevel = useCallback(
    (s: GroupedScenario): SeverityLevel | null => {
      const risk = scenarioMaxRisk(s, pathById);
      return risk === null ? null : pathRiskLevel(risk);
    },
    [pathById],
  );

  const laneCounts = useMemo(() => {
    const counts: Record<AttackPathConfidenceLane, number> = { observed: 0, inferred: 0, theoretical: 0 };
    for (const s of podScopedScenarios) counts[attackPathConfidenceLane(s)] += 1;
    return counts;
  }, [podScopedScenarios]);

  const laneScenarios = useMemo(
    () => (confidenceLane === 'all' ? podScopedScenarios : podScopedScenarios.filter((s) => attackPathConfidenceLane(s) === confidenceLane)),
    [podScopedScenarios, confidenceLane],
  );

  const levelCounts = useMemo(() => {
    const counts: Record<SeverityLevel, number> = { critical: 0, high: 0, medium: 0, low: 0 };
    for (const s of laneScenarios) {
      const level = scenarioLevel(s);
      if (level) counts[level] += 1;
    }
    return counts;
  }, [laneScenarios, scenarioLevel]);

  // Every filter but the fix filter: the choke points are ranked over these, so choosing a fix never hides the others.
  const filteredScenarios = useMemo(() => {
    let list = laneScenarios;
    if (levelFilter === 'urgent') list = list.filter((s) => ['critical', 'high'].includes(scenarioLevel(s) ?? ''));
    else if (levelFilter) list = list.filter((s) => scenarioLevel(s) === levelFilter);
    if (targetFilter) list = list.filter((s) => scenarioTargetGroup(s, pathById) === targetFilter);
    if (namespaceFilter) list = list.filter((s) => String(scenarioEnds(s, pathById).entry?.properties?.namespace || '') === namespaceFilter);
    return [...list].sort(
      (a, b) => (scenarioMaxRisk(b, pathById) ?? -1) - (scenarioMaxRisk(a, pathById) ?? -1) || b.maxStrength - a.maxStrength,
    );
  }, [laneScenarios, levelFilter, targetFilter, namespaceFilter, scenarioLevel, pathById]);

  const rankedFixes = useMemo(
    () => rankSharedFixes(fixesAcrossScenarios(filteredScenarios), filteredScenarios, pathById),
    [filteredScenarios, pathById],
  );
  const postureFixes = useMemo(
    () => rankSharedFixes(fixesAcrossScenarios(podScopedScenarios), podScopedScenarios, pathById),
    [podScopedScenarios, pathById],
  );
  const activeFix = rankedFixes.find((f) => f.label === fixFilter) ?? null;

  const visibleScenarios = useMemo(
    () => (activeFix ? filteredScenarios.filter((s) => activeFix.scenarioKeys.includes(s.key)) : filteredScenarios),
    [filteredScenarios, activeFix],
  );

  const currentScenario = visibleScenarios.find((s) => s.key === selectedScenarioKey) || visibleScenarios[0];

  // ?path= (attack-path alerts, Inventory) opens the scenario that contains that path, once per link.
  const pathParam = searchParams.get('path') || '';
  const appliedPathParamRef = useRef('');
  useEffect(() => {
    if (!pathParam || appliedPathParamRef.current === pathParam || groupedScenarios.length === 0) return;
    const scenario = groupedScenarios.find((s) => s.variants.some((c) => Array.isArray(c.paths) && c.paths.includes(pathParam)));
    appliedPathParamRef.current = pathParam;
    if (scenario) setSelectedScenarioKey(scenario.key);
  }, [pathParam, groupedScenarios]);

  const selectScenario = useCallback(
    (key: string) => {
      setSelectedScenarioKey(key);
      const scenario = groupedScenarios.find((s) => s.key === key);
      const firstPath = scenario ? scenarioPathIds(scenario)[0] : '';
      if (firstPath) {
        appliedPathParamRef.current = firstPath;
        const next = new URLSearchParams(searchParams);
        next.set('path', firstPath);
        setSearchParams(next, { replace: true });
      }
      // On narrow screens the detail sits below the list.
      if (window.matchMedia?.('(max-width: 1279px)').matches) {
        window.setTimeout(() => document.getElementById('attack-path-detail')?.scrollIntoView({ behavior: 'smooth', block: 'start' }), 0);
      }
    },
    [groupedScenarios, searchParams, setSearchParams],
  );

  useEffect(() => {
    setHighlightedStepIndex(null);
  }, [currentScenario?.key]);

  const graphForView = useMemo(() => {
    if (primitivePaths.length === 0) return graphData;
    if (!currentScenario) return buildGraphDataFromPrimitivePaths(primitivePaths);
    const steps = currentScenario.representativeChain?.steps;
    const ids = new Set(scenarioPathIds(currentScenario));
    const subset = primitivePaths.filter((p) => ids.has(p.path_id || ''));
    return buildGraphDataFromPrimitivePaths(subset.length > 0 ? subset : primitivePaths, steps);
  }, [currentScenario, graphData, primitivePaths]);

  // Step numbers belong to one scenario; the whole-cluster graph carries none.
  const fullGraph = useMemo(
    () => (primitivePaths.length === 0 ? graphData : buildGraphDataFromPrimitivePaths(scopedPaths.length > 0 ? scopedPaths : primitivePaths)),
    [graphData, primitivePaths, scopedPaths],
  );

  const view = searchParams.get('view') === 'overview' ? 'overview' : 'paths';
  const setView = useCallback(
    (v: 'paths' | 'overview') => {
      const next = new URLSearchParams(searchParams);
      if (v === 'overview') next.set('view', 'overview');
      else next.delete('view');
      setSearchParams(next, { replace: true });
    },
    [searchParams, setSearchParams],
  );

  const hrefFor = useCallback(
    (node: AttackPathNode) => nodeHref(node, effectiveClusterId || null, canOpenInventory),
    [effectiveClusterId, canOpenInventory],
  );

  const handleNodeClick = useCallback(
    (nodeId: string, type: string) => {
      if (type === 'path_entry' || nodeId.startsWith('path-entry:')) {
        const pathId = nodeId.replace(/^path-entry:/, '');
        const scenario = groupedScenarios.find((s) => scenarioPathIds(s).includes(pathId));
        if (scenario) selectScenario(scenario.key);
        return;
      }
      if (type !== 'pod' && type !== 'service_account') return;
      if (!canOpenInventory) {
        setActionNotice({
          kind: 'forbidden',
          title: 'Permission required',
          description: 'Opening pods and identities requires inventory.read.',
          actionLabel: 'Refresh',
        });
        return;
      }
      navigate(
        type === 'pod'
          ? podDetailPath(nodeId, effectiveClusterId || undefined)
          : identityDetailPath({ uid: nodeId, clusterId: effectiveClusterId || null }),
      );
    },
    [canOpenInventory, effectiveClusterId, groupedScenarios, navigate, selectScenario],
  );

  const podLabel = useMemo(() => {
    if (!podUidParam) return '';
    for (const p of [...selectedPodPaths, ...primitivePaths]) {
      const n = (Array.isArray(p.nodes) ? p.nodes : []).find((x) => x.id === podUidParam);
      if (n) {
        const ns = String(n.properties?.namespace || '');
        return `${ns ? `${ns}/` : ''}${String(n.properties?.name || podUidParam)}`;
      }
    }
    return podUidParam;
  }, [podUidParam, selectedPodPaths, primitivePaths]);

  const filtersActive = Boolean(levelFilter) || confidenceLane !== 'all' || Boolean(targetFilter) || Boolean(namespaceFilter) || Boolean(fixFilter);
  const resetFilters = () => {
    setLevelFilter('');
    setConfidenceLane('all');
    setTargetFilter('');
    setNamespaceFilter('');
    setFixFilter('');
  };
  const scrollTo = (id: string) =>
    window.setTimeout(() => document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' }), 0);
  // Alerts and the summary count paths; say how many paths sit behind the scenarios listed.
  const pathTotal = scopedPaths.length > 0 ? scopedPaths.length : podUidParam ? 0 : summary?.totalPaths ?? 0;

  /* ─── render ─────────────────────────────────────────────── */

  return (
    <PageLayout
      title={PAGE_TITLES.attackAnalysis}
      actions={
        <Button variant="secondary" size="sm" onClick={() => void handleRefresh()} isLoading={refreshing} aria-label="Refresh">
          <RefreshCw size={16} className="shrink-0" />
        </Button>
      }
    >
      {loading ? (
        <PageLoading message="Loading attack paths..." className="min-h-[420px]" />
      ) : dataIssue ? (
        <AttackPathIssuePanel issue={dataIssue} onAction={() => handleIssueAction(dataIssue)} />
      ) : groupedScenarios.length === 0 ? (
        <div className="flex min-h-[320px] flex-col items-center justify-center gap-2 text-muted">
          <Shield size={40} className="text-muted-2" />
          <p>No attack paths{effectiveClusterId ? ` in ${effectiveClusterId}` : ''}.</p>
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          {actionNotice ? (
            <div className="flex items-start justify-between gap-2 rounded-lg border border-warning-border bg-warning-background px-3 py-2 text-caption" role="status">
              <p className="text-text">
                <span className="font-semibold">{actionNotice.title}.</span> {actionNotice.description}
              </p>
              <button type="button" onClick={() => setActionNotice(null)} className="shrink-0 font-semibold text-text hover:text-brand">
                Dismiss
              </button>
            </div>
          ) : null}

          {podUidParam ? (
            <div className="flex flex-wrap items-center gap-x-4 gap-y-2 rounded-lg border border-brand/30 bg-brand/5 px-3 py-2 text-body">
              <span className="text-text">
                From pod <span className="font-semibold">{podLabel}</span>
              </span>
              <span className="text-caption text-muted">
                {selectedPodLoading
                  ? 'Loading…'
                  : selectedPodIssue
                    ? selectedPodIssue.title
                    : `${selectedPodPaths.length} ${selectedPodPaths.length === 1 ? 'path starts' : 'paths start'} at this pod`}
              </span>
              <span className="ml-auto flex flex-wrap items-center gap-3 text-caption font-semibold">
                {canOpenInventory ? (
                  <Link to={podDetailPath(podUidParam, podClusterIdParam)} className="text-brand hover:underline">
                    Open pod
                  </Link>
                ) : null}
                <Link to={findingsForResourcePath({ uid: podUidParam, clusterId: podClusterIdParam || null })} className="text-brand hover:underline">
                  Findings
                </Link>
                <button type="button" onClick={clearPodUidFilter} className="text-muted hover:text-text">
                  Show all paths
                </button>
              </span>
            </div>
          ) : null}

          <PostureStrip
            scenarios={podScopedScenarios}
            pathById={pathById}
            ranked={postureFixes}
            targetFilter={targetFilter}
            onTarget={(g) => {
              setTargetFilter(g);
              setView('paths');
            }}
            onUrgent={() => {
              setLevelFilter((cur) => (cur === 'urgent' ? '' : 'urgent'));
              setView('paths');
            }}
            onFixes={() => {
              resetFilters();
              setView('paths');
              scrollTo('choke-points');
            }}
          />

          <div className="flex flex-wrap items-center gap-2">
            <div role="group" aria-label="View" className="flex rounded-lg border border-border p-0.5">
              {(['paths', 'overview'] as const).map((v) => (
                <button
                  key={v}
                  type="button"
                  aria-pressed={view === v}
                  onClick={() => setView(v)}
                  className={clsx(
                    'min-h-8 rounded-md px-3 text-caption font-semibold',
                    view === v ? 'bg-brand/15 text-text' : 'text-muted hover:text-text',
                  )}
                >
                  {v === 'paths' ? 'Paths' : 'Overview'}
                </button>
              ))}
            </div>
            <div role="group" aria-label="Path level" className="flex flex-wrap gap-2">
              {(['critical', 'high', 'medium', 'low'] as const).map((lv) => {
                const pressed = levelFilter === lv || (levelFilter === 'urgent' && (lv === 'critical' || lv === 'high'));
                return (
                  <button
                    key={lv}
                    type="button"
                    aria-pressed={pressed}
                    onClick={() => setLevelFilter((cur) => (cur === lv ? '' : lv))}
                    className={clsx(
                      'inline-flex min-h-9 items-center gap-2 rounded-lg border px-3 text-caption font-semibold transition-colors',
                      pressed ? 'border-brand bg-brand/15 text-text' : 'border-border bg-base/40 text-muted hover:text-text',
                    )}
                  >
                    <span className={clsx('h-2 w-2 rounded-full', LEVEL_DOT[lv])} aria-hidden />
                    {lv[0].toUpperCase() + lv.slice(1)}
                    <span className="tabular-nums text-text">{levelCounts[lv].toLocaleString()}</span>
                  </button>
                );
              })}
            </div>
            <select
              value={confidenceLane}
              onChange={(e) => setConfidenceLane(e.target.value as AttackPathConfidenceLane | 'all')}
              aria-label="Evidence"
              className="min-h-9 rounded-lg border border-border bg-base px-3 text-caption text-text"
            >
              <option value="all">Any evidence</option>
              {(['observed', 'inferred', 'theoretical'] as const).map((lane) => (
                <option key={lane} value={lane}>
                  {ATTACK_PATH_LANE_LABELS[lane]} ({laneCounts[lane]})
                </option>
              ))}
            </select>
            {targetFilter ? (
              <FilterTag label={`Reaches ${TARGET_GROUP_LABEL[targetFilter]}`} onClear={() => setTargetFilter('')} />
            ) : null}
            {namespaceFilter ? <FilterTag label={`Starts in ${namespaceFilter}`} onClear={() => setNamespaceFilter('')} /> : null}
            {filtersActive ? (
              <button type="button" onClick={resetFilters} className="text-caption font-semibold text-muted hover:text-text">
                Reset filters
              </button>
            ) : null}
            <span className="ml-auto text-caption text-muted">
              {podScopedScenarios.length.toLocaleString()} {podScopedScenarios.length === 1 ? 'scenario' : 'scenarios'}
              {pathTotal > 0 ? ` · ${pathTotal.toLocaleString()} ${pathTotal === 1 ? 'path' : 'paths'}` : ''}
            </span>
          </div>

          <GraphVisibilityOverlay semanticState={graphVisibility.semanticState} reason={graphVisibility.reason} />

          {view === 'overview' ? (
            <AttackPathOverview
              scenarios={filteredScenarios}
              pathById={pathById}
              onNamespace={(ns) => {
                setNamespaceFilter(ns);
                setView('paths');
              }}
              onTarget={(g) => {
                setTargetFilter(g);
                setView('paths');
              }}
              fullGraph={
                <div className="relative overflow-hidden rounded-lg border border-border bg-base">
                  <AttackPathGraph
                    data={fullGraph}
                    onNodeClick={handleNodeClick}
                    highlightedStepIndex={null}
                    semanticMode={attackGraphMode}
                    graphTrust={graphTrust}
                    showTrustOverlay={false}
                    className="h-[min(60dvh,560px)] min-h-[20rem] w-full"
                  />
                </div>
              }
            />
          ) : visibleScenarios.length === 0 ? (
            <div className="flex min-h-[200px] flex-col items-center justify-center gap-3 rounded-xl border border-border text-muted">
              <p>{podUidParam && !filtersActive ? 'No attack path starts at this pod.' : 'No attack paths match these filters.'}</p>
              {filtersActive ? (
                <Button variant="secondary" size="sm" onClick={resetFilters}>
                  Reset filters
                </Button>
              ) : null}
            </div>
          ) : (
            <>
              <ChokePoints ranked={rankedFixes} activeFix={activeFix?.label ?? ''} onFilter={setFixFilter} />

              <div className="grid min-w-0 grid-cols-1 gap-4 xl:grid-cols-[minmax(0,26rem)_minmax(0,1fr)] xl:items-start">
                <div className="flex min-w-0 flex-col gap-2 xl:sticky xl:top-4 xl:max-h-[calc(100dvh-6rem)] xl:overflow-y-auto">
                  {activeFix ? <FixFilterChip label={activeFix.label} count={visibleScenarios.length} onClear={() => setFixFilter('')} /> : null}
                  <AttackPathList
                    scenarios={visibleScenarios}
                    pathById={pathById}
                    selectedKey={currentScenario?.key ?? null}
                    onSelect={selectScenario}
                  />
                </div>
                {currentScenario ? (
                  <div id="attack-path-detail" className="min-w-0 scroll-mt-4">
                    <AttackPathDetail
                      key={currentScenario.key}
                      scenario={currentScenario}
                      pathById={pathById}
                      clusterId={effectiveClusterId || null}
                      allowedRoutes={allowedRoutes}
                      hrefFor={hrefFor}
                      highlightedStepIndex={highlightedStepIndex}
                      onStepClick={(n) => setHighlightedStepIndex((prev) => (prev === n ? null : n))}
                      addToCase={
                        <AddToCaseButton
                          entity={attackPathInvestigationEntity(currentScenario, {
                            podUid: podUidParam,
                            clusterId: effectiveClusterId || null,
                            pathId: scenarioPathIds(currentScenario)[0],
                          })}
                          clusterId={effectiveClusterId || null}
                        />
                      }
                      graph={
                        <div className="relative overflow-hidden rounded-lg border border-border bg-base">
                          <AttackPathGraph
                            data={graphForView}
                            onNodeClick={handleNodeClick}
                            highlightedStepIndex={highlightedStepIndex}
                            semanticMode={attackGraphMode}
                            graphTrust={graphTrust}
                            showTrustOverlay={false}
                            className="h-[min(42dvh,380px)] min-h-[18rem] w-full"
                          />
                        </div>
                      }
                      extraEvidence={
                        podUidParam ? (
                          <div>
                            <p className="mb-1 text-caption font-semibold text-muted">Runtime steps seen on this pod</p>
                            <AttackStepsTimeline podUid={podUidParam} clusterId={podClusterIdParam || undefined} />
                          </div>
                        ) : undefined
                      }
                    />
                  </div>
                ) : null}
              </div>
            </>
          )}
        </div>
      )}
    </PageLayout>
  );
};

const LEVEL_DOT: Record<SeverityLevel, string> = {
  critical: 'bg-red-500',
  high: 'bg-orange-500',
  medium: 'bg-yellow-400',
  low: 'bg-slate-400',
};

function FilterTag({ label, onClear }: { label: string; onClear: () => void }) {
  return (
    <span className="inline-flex min-h-9 items-center gap-1 rounded-lg border border-brand/40 bg-brand/5 pl-3 pr-1 text-caption text-text">
      {label}
      <button type="button" onClick={onClear} aria-label={`Clear: ${label}`} className="rounded p-1 text-muted hover:text-text">
        <X size={14} aria-hidden />
      </button>
    </span>
  );
}
