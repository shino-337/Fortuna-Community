import { useCallback, useState } from 'react';
import { api } from '../../lib/api';
import type {
  AttackChain,
  AttackPath,
  AttackPathGraphData,
  AttackPathSummary,
  Cluster,
  Insight,
  InsightsSummary,
  Notification,
  PipelineHealth,
  PodCapabilitySummaryCapability,
  PodCapabilityTrendPoint,
  PodWithRisk,
  UnifiedRiskScore,
} from '../../types';
import { deriveUnifiedRiskLevelFromScore } from '../../lib/severity';
import type { DashboardDataLoadPolicy, DashboardSectionLoadState } from './types';

const DEFAULT_LOAD_POLICY: DashboardDataLoadPolicy = {
  attackBundle: false,
  pipelineHealth: false,
  pce: false,
  entryPods: false,
};

export interface DashboardStats {
  clusters: number;
  insights: number;
  critical: number;
  pods: number;
  agents: number;
  affectedPodCount: number;
  resolved24h: number;
  clusterName?: string;
}

export interface UseDashboardDataResult {
  stats: DashboardStats;
  insightsSummary: InsightsSummary | null;
  clusters: Cluster[];
  topRisks: Insight[];
  notifications: Notification[];
  pceSummary: PodCapabilitySummaryCapability[];
  pceTrend: PodCapabilityTrendPoint[];
  trendDays: number;
  setTrendDays: (d: number) => void;
  threatVelocity: { date: string; critical: number; high: number; medium: number; low: number }[];
  topRiskyPods: UnifiedRiskScore[];
  exploitedCapCount: number;
  attackPathCount: number;
  attackPathSummary: AttackPathSummary | null;
  pipelineHealth: PipelineHealth | null;
  entryPods: PodWithRisk[];
  attackChains: AttackChain[];
  primitivePaths: AttackPath[];
  attackGraphData: AttackPathGraphData;
  coreReady: boolean;
  initialError: string | null;
  partialErrors: string[];
  sectionState: Record<'risks' | 'attack' | 'pipeline' | 'clusters', DashboardSectionLoadState>;
  refresh: () => Promise<void>;
}

const EMPTY_GRAPH: AttackPathGraphData = { nodes: [], links: [] };

const INITIAL_STATS: DashboardStats = {
  clusters: 0,
  insights: 0,
  critical: 0,
  pods: 0,
  agents: 0,
  affectedPodCount: 0,
  resolved24h: 0,
};

export function useDashboardData(
  selectedClusterId: string | null | undefined,
  sinceMinutes: number | undefined,
  loadPolicy: DashboardDataLoadPolicy = DEFAULT_LOAD_POLICY,
): UseDashboardDataResult {
  const [stats, setStats] = useState<DashboardStats>(INITIAL_STATS);
  const [insightsSummary, setInsightsSummary] = useState<InsightsSummary | null>(null);
  const [clusters, setClusters] = useState<Cluster[]>([]);
  const [topRisks, setTopRisks] = useState<Insight[]>([]);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [pceSummary, setPceSummary] = useState<PodCapabilitySummaryCapability[]>([]);
  const [pceTrend, setPceTrend] = useState<PodCapabilityTrendPoint[]>([]);
  const [trendDays, setTrendDays] = useState(7);
  const [threatVelocity, setThreatVelocity] = useState<
    { date: string; critical: number; high: number; medium: number; low: number }[]
  >([]);
  const [topRiskyPods, setTopRiskyPods] = useState<UnifiedRiskScore[]>([]);
  const [exploitedCapCount, setExploitedCapCount] = useState(0);
  const [attackPathCount, setAttackPathCount] = useState(0);
  const [attackPathSummary, setAttackPathSummary] = useState<AttackPathSummary | null>(null);
  const [pipelineHealth, setPipelineHealth] = useState<PipelineHealth | null>(null);
  const [entryPods, setEntryPods] = useState<PodWithRisk[]>([]);
  const [attackChains, setAttackChains] = useState<AttackChain[]>([]);
  const [primitivePaths, setPrimitivePaths] = useState<AttackPath[]>([]);
  const [attackGraphData, setAttackGraphData] = useState<AttackPathGraphData>(EMPTY_GRAPH);
  const [coreReady, setCoreReady] = useState(false);
  const [initialError, setInitialError] = useState<string | null>(null);
  const [partialErrors, setPartialErrors] = useState<string[]>([]);
  const [sectionState, setSectionState] = useState<
    Record<'risks' | 'attack' | 'pipeline' | 'clusters', DashboardSectionLoadState>
  >({
    risks: 'idle',
    attack: 'idle',
    pipeline: 'idle',
    clusters: 'idle',
  });

  const refresh = useCallback(async () => {
    setInitialError(null);
    const errors: string[] = [];
    setSectionState({ risks: 'loading', attack: 'loading', pipeline: 'loading', clusters: 'loading' });

    try {
      const [statsResult, summaryResult] = await Promise.allSettled([
        api.getStats(selectedClusterId ?? undefined, sinceMinutes, 'all'),
        api.getInsightsSummary(selectedClusterId ?? undefined, sinceMinutes),
      ]);

      if (statsResult.status === 'fulfilled') {
        setStats(statsResult.value);
      } else {
        setStats(INITIAL_STATS);
        errors.push('Summary KPIs could not be loaded; dashboard is running in degraded mode.');
      }

      if (summaryResult.status === 'fulfilled' && summaryResult.value) {
        setInsightsSummary(summaryResult.value);
      } else {
        errors.push('Risk severity summary could not be loaded.');
      }

      setCoreReady(true);

      const [
        clustersResult,
        risksResult,
        notesResult,
        threatResult,
        pceResult,
        pceTrendResult,
        podsResult,
        attackBundleResult,
      ] = await Promise.allSettled([
        api.getClustersStats(),
        api.getRisks({
          page: 1,
          pageSize: 50,
          clusterId: selectedClusterId ?? undefined,
          sinceMinutes,
          withScores: 1,
        }),
        api.getNotifications(),
        api.getThreatVelocity(trendDays, selectedClusterId ?? undefined, 'all'),
        loadPolicy.pce
          ? api.getPceSummaryByCapability({ clusterId: selectedClusterId ?? undefined })
          : Promise.resolve([] as PodCapabilitySummaryCapability[]),
        loadPolicy.pce
          ? api.getPceTrend(trendDays, { clusterId: selectedClusterId ?? undefined })
          : Promise.resolve([] as PodCapabilityTrendPoint[]),
        loadPolicy.entryPods
          ? api.getPods({ cluster: selectedClusterId ?? undefined, page: 1, pageSize: 100, sortBy: 'risk_desc' })
          : Promise.resolve({ pods: [] as PodWithRisk[], total: 0 }),
        loadPolicy.attackBundle
          ? api.getAttackPathsBundle(selectedClusterId ?? undefined)
          : Promise.resolve(null),
      ]);

      if (clustersResult.status === 'fulfilled') {
        setClusters(clustersResult.value);
      } else {
        errors.push('Cluster inventory could not be loaded.');
      }

      if (risksResult.status === 'fulfilled') {
        const { insights, total } = risksResult.value;
        const unifiedLevel = (r: Insight) =>
          (r.finalLevel || deriveUnifiedRiskLevelFromScore(r.score) || '').toLowerCase();
        setTopRisks(insights.filter((r) => unifiedLevel(r) === 'critical').slice(0, 4));
        if (!(summaryResult.status === 'fulfilled' && summaryResult.value)) {
          const bySev = { critical: 0, high: 0, medium: 0, low: 0 };
          insights.forEach((r) => {
            const s = unifiedLevel(r);
            if (s in bySev) (bySev as Record<string, number>)[s]++;
          });
          setInsightsSummary({
            total: total ?? 0,
            critical: bySev.critical,
            high: bySev.high,
            medium: bySev.medium,
            low: bySev.low,
          });
        }
      } else {
        errors.push('Risk findings could not be loaded.');
      }

      if (notesResult.status === 'fulfilled') setNotifications(notesResult.value.slice(0, 3));
      else errors.push('Notifications could not be loaded.');
      if (threatResult.status === 'fulfilled') setThreatVelocity(threatResult.value);
      if (pceResult.status === 'fulfilled') setPceSummary(pceResult.value.slice(0, 5));
      if (pceTrendResult.status === 'fulfilled') setPceTrend(pceTrendResult.value);
      if (loadPolicy.entryPods) {
        if (podsResult.status === 'fulfilled') setEntryPods(podsResult.value.pods || []);
        else errors.push('Pod inventory for entry points could not be loaded.');
      } else {
        setEntryPods([]);
      }

      if (loadPolicy.attackBundle) {
        if (attackBundleResult.status === 'fulfilled' && attackBundleResult.value) {
          const bundle = attackBundleResult.value;
          const paths = bundle.paths || [];
          setAttackGraphData(bundle.graph);
          setAttackChains(bundle.chains || []);
          setPrimitivePaths(paths);
          setAttackPathSummary(bundle.summary ?? null);
          setAttackPathCount(bundle.summary?.totalPaths ?? paths.length);
        } else {
          setAttackPathSummary(null);
          setAttackPathCount(0);
          errors.push('Attack path bundle could not be loaded.');
        }
      } else {
        setAttackGraphData(EMPTY_GRAPH);
        setAttackChains([]);
        setPrimitivePaths([]);
        setAttackPathSummary(null);
        setAttackPathCount(0);
      }

      const [topPodsResult, pipelineResult] = await Promise.allSettled([
        api.getTopRiskyPods(1000, selectedClusterId ?? undefined),
        loadPolicy.pipelineHealth ? api.getPipelineHealth() : Promise.resolve(null),
      ]);

      if (topPodsResult.status === 'fulfilled') setTopRiskyPods(topPodsResult.value);
      else errors.push('Top risky workloads could not be loaded.');

      if (loadPolicy.pipelineHealth) {
        if (pipelineResult.status === 'fulfilled' && pipelineResult.value) {
          const ph = pipelineResult.value;
          setPipelineHealth(ph);
          setExploitedCapCount(ph.layer2.exploitedCapCount ?? 0);
        } else {
          errors.push('Pipeline health could not be loaded.');
        }
      } else {
        setPipelineHealth(null);
        setExploitedCapCount(0);
      }

      setPartialErrors(errors);
      setSectionState({
        risks: risksResult.status === 'fulfilled' ? 'ready' : 'error',
        attack: !loadPolicy.attackBundle
          ? 'skipped'
          : attackBundleResult.status === 'fulfilled' && attackBundleResult.value
            ? 'ready'
            : 'error',
        pipeline: !loadPolicy.pipelineHealth
          ? 'skipped'
          : pipelineResult.status === 'fulfilled' && pipelineResult.value
            ? 'ready'
            : 'error',
        clusters: clustersResult.status === 'fulfilled' ? 'ready' : 'error',
      });
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Unable to load dashboard data';
      setInitialError(message);
      setPartialErrors([message]);
      setCoreReady(true);
      setSectionState({ risks: 'error', attack: 'error', pipeline: 'error', clusters: 'error' });
    }
  }, [selectedClusterId, sinceMinutes, trendDays, loadPolicy]);

  return {
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
  };
}
