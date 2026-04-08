import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { api } from '../lib/api';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { Cluster, Insight, InsightsSummaryByClusterItem, PodCapabilityDetail, PodCapabilitySummaryNamespace, PodCapabilitySummarySeverity, PodCapabilityTrendPoint, RuntimeSignal, AuditLog } from '../types';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Tabs } from '../design-system/components/Tabs';
import { Pagination } from '../components/Pagination';
import { Shield, AlertTriangle, Info, CheckCircle, Search, Box, User, ArrowRight, X, ExternalLink, Loader2 } from 'lucide-react';
import { useLocation, useNavigate, useSearchParams, Link } from 'react-router-dom';
import { useClusterStore } from '../store/clusterStore';
import { useTimeWindowStore } from '../store/timeWindowStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';
import { getSeverityBadgeClass, getSeverityTextClass } from '../lib/severity';
import { RISK_CENTER_DESCRIPTION } from '../constants/labels';
import { RuntimeSignalsTable } from '../components/RuntimeSignalsTable';
import { RiskHistogram } from '../components/RiskHistogram';
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import type { RiskHistogramResponse } from '../types';
import { riskListSecondaryLabel } from '../lib/riskDisplay';
import { runtimeSignalVisual } from '../lib/runtimeSignalVisual';
import { formatMinutesHuman } from '../lib/formatDuration';

type TabId = 'risks' | 'pce' | 'reference';

/** Shared table chrome for Risk Center data tables */
const TABLE_HEAD_ROW = 'text-xs uppercase tracking-wide text-slate-400 bg-slate-950/95 backdrop-blur-sm border-b border-slate-800';
const TABLE_BODY_ROW = 'border-b border-slate-800/80 transition-colors hover:bg-slate-800/35';

const RISK_FINDINGS_COLS_KEY = 'fortuna-risk-findings-table-cols-v1';

type RiskFindingsTableCols = {
  type: boolean;
  resource: boolean;
  score: boolean;
  nsCluster: boolean;
  detected: boolean;
  updated: boolean;
};

const defaultRiskFindingsCols: RiskFindingsTableCols = {
  type: true,
  resource: true,
  score: true,
  nsCluster: true,
  detected: true,
  updated: true,
};

function loadRiskFindingsCols(): RiskFindingsTableCols {
  if (typeof window === 'undefined') return { ...defaultRiskFindingsCols };
  try {
    const raw = window.localStorage.getItem(RISK_FINDINGS_COLS_KEY);
    if (!raw) return { ...defaultRiskFindingsCols };
    const o = JSON.parse(raw) as Partial<RiskFindingsTableCols>;
    return { ...defaultRiskFindingsCols, ...o };
  } catch {
    return { ...defaultRiskFindingsCols };
  }
}

export const RiskCenter: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { selectedClusterId, setSelectedClusterId } = useClusterStore();
  const { valueMinutes: timeWindowMinutes, setValueMinutes: setTimeWindowMinutes } = useTimeWindowStore();
  const [searchParams] = useSearchParams();
  const severityFromUrl = searchParams.get('severity');
  const searchFromUrl = searchParams.get('search') ?? '';
  const clusterIdFromUrl = searchParams.get('clusterId');
  const sinceMinutesFromUrl = searchParams.get('sinceMinutes');
  const [activeTab, setActiveTab] = useState<TabId>('risks');
  const [risks, setRisks] = useState<Insight[]>([]);
  const [risksTotal, setRisksTotal] = useState(0);
  const [pceSummary, setPceSummary] = useState<PodCapabilitySummarySeverity[]>([]);
  const [pceDetails, setPceDetails] = useState<PodCapabilityDetail[]>([]);
  const [clusters, setClusters] = useState<Cluster[]>([]);
  const [pceClusterId, setPceClusterId] = useState('');
  const [pceNamespace, setPceNamespace] = useState('');
  const [pceCapabilityId, setPceCapabilityId] = useState('');
  const [pcePodName, setPcePodName] = useState('');
  const [pceSeverityFilter, setPceSeverityFilter] = useState<string>('');
  /** When user clicks a heatmap cell, filter drill-down table by this namespace + severity. */
  const [pceHeatmapFilter, setPceHeatmapFilter] = useState<{ namespace: string; severity: string } | null>(null);
  /** PCE drill-down server pagination (GET /inventory/pod-capabilities returns total). */
  const [pceListPage, setPceListPage] = useState(1);
  const [pceListPageSize, setPceListPageSize] = useState(25);
  const [pceListTotal, setPceListTotal] = useState(0);
  const drawerRef = React.useRef<HTMLDivElement>(null);
  const [filter, setFilter] = useState<'all' | string>(severityFromUrl && ['critical', 'high', 'medium', 'low'].includes(severityFromUrl) ? severityFromUrl : 'all');
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'resolved' | 'acknowledged'>('active');
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedRisk, setSelectedRisk] = useState<Insight | null>(null);
  const [selectedRiskDetail, setSelectedRiskDetail] = useState<Insight | null>(null);
  const [selectedRiskSignals, setSelectedRiskSignals] = useState<RuntimeSignal[]>([]);
  const [selectedRiskCapabilities, setSelectedRiskCapabilities] = useState<PodCapabilityDetail[]>([]);
  const [selectedRiskReferences, setSelectedRiskReferences] = useState<string[]>([]);
  const [selectedRiskLoading, setSelectedRiskLoading] = useState(false);
  const [selectedRiskContext, setSelectedRiskContext] = useState<{
    pods: any[];
    cluster?: { id?: string; name?: string };
    rules: any[];
  } | null>(null);
  const [resolved24h, setResolved24h] = useState<number>(0);
  const [insightsSummary, setInsightsSummary] = useState<{ total: number; critical: number; high: number; medium: number; low: number } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [risksPage, setRisksPage] = useState(1);
  const [risksPageSize, setRisksPageSize] = useState(20);
  const [threatVelocity, setThreatVelocity] = useState<{ date: string; critical: number; high: number; medium: number; low: number }[]>([]);
  const [riskSort, setRiskSort] = useState<'newest' | 'oldest' | 'score_desc' | 'score_asc' | 'severity_desc' | 'title_asc'>('score_desc');
  const [priorityLevel, setPriorityLevel] = useState<string>(''); // P0–P4 filter (Phase 3.1)
  const [pceSort, setPceSort] = useState<'severity_desc' | 'severity_asc' | 'capability_asc' | 'pod_asc' | 'namespace_asc'>('severity_desc');
  /** When user clicks a date on Threat Velocity chart, filter risk table to that day (spec: drill-down). */
  const [selectedChartDate, setSelectedChartDate] = useState<string | null>(null);
  /** Risks by cluster (when scope = all clusters). Phase 2.2. */
  const [risksByCluster, setRisksByCluster] = useState<InsightsSummaryByClusterItem[]>([]);
  /** PCE trend (7 days) and namespace×severity heatmap data. Phase 4. */
  const [pceTrend, setPceTrend] = useState<PodCapabilityTrendPoint[]>([]);
  const [pceHeatmap, setPceHeatmap] = useState<PodCapabilitySummaryNamespace[]>([]);
  const [namespaceFilter, setNamespaceFilter] = useState<string>('');
  const [typeFilter, setTypeFilter] = useState<string>('');
  const [debouncedSearchTerm, setDebouncedSearchTerm] = useState('');
  /** When summary API fails: 'page' = current table page only; 'sample' = first N rows (≤1000); 'exact' = trusted counts */
  const [severityCountTrust, setSeverityCountTrust] = useState<'exact' | 'sample' | 'page'>('exact');
  const [toast, setToast] = useState<{ message: string; variant: 'success' | 'error' } | null>(null);
  const [pceTrendDays, setPceTrendDays] = useState(7);
  const [heatmapShowAll, setHeatmapShowAll] = useState(false);
  const [pageBlocking, setPageBlocking] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [drawerNotice, setDrawerNotice] = useState<string | null>(null);
  const hasLoadedOnceRef = React.useRef(false);

  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [drawerTab, setDrawerTab] = useState<'summary' | 'evidence' | 'pce'>('summary');
  const [drawerAuditLogs, setDrawerAuditLogs] = useState<AuditLog[]>([]);
  const [drawerAuditLoading, setDrawerAuditLoading] = useState(false);
  /** Evidence tab sub-tabs: Runtime Signals | Audit Trail (Phase 4). */
  const [evidenceSubTab, setEvidenceSubTab] = useState<'signals' | 'audit'>('signals');
  const [evidenceAuditLogs, setEvidenceAuditLogs] = useState<AuditLog[]>([]);
  const [evidenceAuditTotal, setEvidenceAuditTotal] = useState(0);
  const [evidenceAuditPage, setEvidenceAuditPage] = useState(1);
  const [evidenceAuditLoading, setEvidenceAuditLoading] = useState(false);
  /** Risk Score Distribution histogram (GET /risk/histogram) */
  const [histogramData, setHistogramData] = useState<RiskHistogramResponse | null>(null);
  const [histogramLoading, setHistogramLoading] = useState(false);
  /** When user clicks a histogram bar, filter findings to this score bin (e.g. 10 = 10–20) */
  const [selectedScoreBin, setSelectedScoreBin] = useState<number | null>(null);
  const [trendDays, setTrendDays] = useState<number>(7);
  /** Recalculate all scores: loading and success message (POST /risk/scores/sync) */
  const [syncScoresLoading, setSyncScoresLoading] = useState(false);
  const [syncScoresMessage, setSyncScoresMessage] = useState<string | null>(null);
  const [riskFindingsCols, setRiskFindingsCols] = useState<RiskFindingsTableCols>(() => loadRiskFindingsCols());
  const [drawerActionBusy, setDrawerActionBusy] = useState<null | 'ack' | 'resolve' | 'dismiss'>(null);
  // Resolve cluster + time: URL from Dashboard link overrides store so Risk Center shows same scope
  const effectiveClusterId = clusterIdFromUrl ?? selectedClusterId ?? undefined;
  const effectiveSinceMinutes = sinceMinutesFromUrl != null ? parseInt(sinceMinutesFromUrl, 10) : timeWindowMinutes;
  const effectiveSinceMinutesNum = Number.isFinite(effectiveSinceMinutes) && effectiveSinceMinutes > 0 ? effectiveSinceMinutes : undefined;

  /** Phase 2 design: /risks = Overview only (KPI, trend, histogram, Quick Links). /risks/findings = full table. */
  const isOverviewPage = location.pathname === '/risks' || location.pathname.replace(/\/$/, '') === '/risks';
  /** Export loading for UX feedback (tooltip 10k, filter warning). */
  const [exportLoading, setExportLoading] = useState(false);
  const [, setSearchParams] = useSearchParams();
  const insightIdFromUrl = searchParams.get('insightId');
  const podUidFromEvidenceUrl = searchParams.get('podUid');

  /** Since minutes for API: if a chart date is selected, use from start of that day to now; else use URL/store. */
  const sinceMinutesForApi = useMemo(() => {
    if (selectedChartDate) {
      const start = new Date(selectedChartDate + 'T00:00:00Z').getTime();
      const now = Date.now();
      return Math.max(0, Math.floor((now - start) / 60000));
    }
    return effectiveSinceMinutesNum ?? (timeWindowMinutes > 0 ? timeWindowMinutes : undefined);
  }, [selectedChartDate, effectiveSinceMinutesNum, timeWindowMinutes]);

  React.useEffect(() => {
    try {
      window.localStorage.setItem(RISK_FINDINGS_COLS_KEY, JSON.stringify(riskFindingsCols));
    } catch {
      /* ignore quota */
    }
  }, [riskFindingsCols]);

  React.useEffect(() => {
    setDrawerActionBusy(null);
  }, [selectedRisk?.id]);

  React.useEffect(() => {
    const id = window.setTimeout(() => setDebouncedSearchTerm(searchTerm.trim()), 400);
    return () => window.clearTimeout(id);
  }, [searchTerm]);

  const clusterLabelById = useMemo(() => {
    const m = new Map<string, string>();
    clusters.forEach((c) => m.set(c.id, c.name || c.id));
    return m;
  }, [clusters]);
  const scopeClusterDisplay = effectiveClusterId
    ? (clusterLabelById.get(effectiveClusterId) ?? effectiveClusterId)
    : null;

  // Sync active tab with route path for Phase 2 routes: /risks/findings, /risks/pce, /risks/evidence
  React.useEffect(() => {
    const path = location.pathname || '';
    if (path.endsWith('/pce')) {
      setActiveTab('pce');
    } else if (path.endsWith('/evidence')) {
      setActiveTab('reference');
    } else {
      setActiveTab('risks');
    }
  }, [location.pathname]);

  // Sync URL -> store so Dashboard link scope is applied (and persisted for next visits)
  React.useEffect(() => {
    if (clusterIdFromUrl?.trim()) setSelectedClusterId(clusterIdFromUrl.trim());
  }, [clusterIdFromUrl, setSelectedClusterId]);
  React.useEffect(() => {
    const m = sinceMinutesFromUrl != null ? parseInt(sinceMinutesFromUrl, 10) : NaN;
    if (Number.isFinite(m) && m >= 0) setTimeWindowMinutes(m);
  }, [sinceMinutesFromUrl, setTimeWindowMinutes]);

  // Sync filter and search from URL (e.g. from global search or deep links)
  React.useEffect(() => {
    if (severityFromUrl && ['critical', 'high', 'medium', 'low'].includes(severityFromUrl)) {
      setFilter(severityFromUrl);
    }
  }, [severityFromUrl]);
  React.useEffect(() => {
    if (searchFromUrl) setSearchTerm(searchFromUrl);
  }, [searchFromUrl]);
  React.useEffect(() => {
    setDebouncedSearchTerm((searchFromUrl ?? '').trim());
  }, [searchFromUrl]);

  React.useEffect(() => {
    if (!toast) return undefined;
    const id = window.setTimeout(() => setToast(null), 4200);
    return () => window.clearTimeout(id);
  }, [toast]);

  const fetchData = useCallback(async () => {
    const isFirst = !hasLoadedOnceRef.current;
    if (!isFirst) setRefreshing(true);
    setError(null);
    const num = (v: unknown) => (typeof v === 'number' ? v : Number(v) || 0);
    const sinceMinutes = sinceMinutesForApi;
    const clusterId = effectiveClusterId ?? selectedClusterId ?? undefined;
    const useScores = riskSort === 'score_desc' || riskSort === 'score_asc' || priorityLevel !== '';
    const risksListParamsBase = {
      severity: filter !== 'all' ? filter : undefined,
      status: statusFilter,
      search: debouncedSearchTerm || undefined,
      clusterId: clusterId ?? undefined,
      namespace: namespaceFilter.trim() || undefined,
      type: typeFilter || undefined,
      sinceMinutes,
      withScores: useScores ? 1 : undefined,
      priorityLevel: priorityLevel || undefined,
      scoreBin: selectedScoreBin ?? undefined,
    };
    const countSeverity = (insights: Insight[]) => {
      const bySev = { critical: 0, high: 0, medium: 0, low: 0 };
      insights.forEach((r: Insight) => {
        const sev = (r.severity || '').toLowerCase();
        if (sev in bySev) (bySev as Record<string, number>)[sev]++;
      });
      return bySev;
    };
    const runPceBlock = async (errors: string[]) => {
      const pceDrillCluster = (pceClusterId || '').trim() || clusterId;
      const pceResults = await Promise.allSettled([
        api.getPceSummaryBySeverity(),
        api.getPceCapabilities({
          clusterId: pceDrillCluster || undefined,
          namespace: pceNamespace.trim() || undefined,
          severity: pceSeverityFilter || undefined,
          podName: pcePodName.trim() || undefined,
          capabilityId: pceCapabilityId.trim() || undefined,
          limit: pceListPageSize,
          offset: (pceListPage - 1) * pceListPageSize,
        }),
        api.getPceTrend(pceTrendDays, { clusterId: clusterId ?? undefined }),
        api.getPceSummaryByNamespace({ clusterId: clusterId ?? undefined }),
      ]);
      const [pceSummaryResult, pceListResult, pceTrendResult, pceHeatmapResult] = pceResults;
      if (pceSummaryResult.status === 'fulfilled') {
        setPceSummary(pceSummaryResult.value);
      } else {
        setPceSummary([]);
        errors.push('PCE summary: ' + (pceSummaryResult.reason?.message || String(pceSummaryResult.reason)));
      }
      if (pceListResult.status === 'fulfilled') {
        const v = pceListResult.value;
        setPceDetails(v.capabilities);
        setPceListTotal(v.total);
      } else {
        setPceDetails([]);
        setPceListTotal(0);
      }
      if (pceTrendResult.status === 'fulfilled') {
        setPceTrend(Array.isArray(pceTrendResult.value) ? pceTrendResult.value : []);
      } else {
        setPceTrend([]);
      }
      if (pceHeatmapResult.status === 'fulfilled') {
        setPceHeatmap(Array.isArray(pceHeatmapResult.value) ? pceHeatmapResult.value : []);
      } else {
        setPceHeatmap([]);
      }
    };
    try {
      const loadRisksHeavy = activeTab === 'risks';
      if (!loadRisksHeavy) {
        const errors: string[] = [];
        const lightResults = await Promise.allSettled([
          api.getInsightsSummary(clusterId ?? undefined, sinceMinutes),
          api.getClusters(),
          api.getStats(clusterId ?? undefined, sinceMinutes, 'all'),
        ]);
        const [summaryResult, clustersResult, statsResult] = lightResults;
        if (clustersResult.status === 'fulfilled') {
          setClusters(clustersResult.value);
        } else {
          setClusters([]);
        }
        if (statsResult.status === 'fulfilled') {
          const st = statsResult.value as { resolved24h?: number };
          setResolved24h(st?.resolved24h ?? 0);
        } else {
          setResolved24h(0);
        }
        if (summaryResult.status === 'fulfilled') {
          setSeverityCountTrust('exact');
          const s = summaryResult.value;
          setInsightsSummary({
            total: num(s?.total),
            critical: num(s?.critical),
            high: num(s?.high),
            medium: num(s?.medium),
            low: num(s?.low),
          });
        } else {
          errors.push('Summary: ' + (summaryResult.reason?.message || String(summaryResult.reason)));
          setInsightsSummary(null);
          setSeverityCountTrust('exact');
        }
        if (activeTab === 'pce') {
          await runPceBlock(errors);
        }
        if (errors.length > 0) setError(errors.join('; '));
      } else {
        const errors: string[] = [];
        const risksPromise = api.getRisks({
          page: risksPage,
          pageSize: risksPageSize,
          ...risksListParamsBase,
        });
        const summaryPromise = api.getInsightsSummary(clusterId ?? undefined, sinceMinutes);
        const threatPromise = api.getThreatVelocity(trendDays, clusterId ?? undefined).catch(() => []);
        const clustersPromise = api.getClusters();
        const statsPromise = api.getStats(clusterId ?? undefined, sinceMinutes, 'all');
        const byClusterPromise = !clusterId ? api.getInsightsSummaryByCluster(sinceMinutes) : Promise.resolve([] as InsightsSummaryByClusterItem[]);
        setHistogramLoading(true);
        const histogramPromise = api
          .getRiskHistogram({ clusterId: clusterId ?? undefined, sinceMinutes })
          .then((r) => {
            setHistogramData(r);
            return r;
          })
          .finally(() => setHistogramLoading(false));
        const coreResults = await Promise.allSettled([
          risksPromise,
          summaryPromise,
          threatPromise,
          clustersPromise,
          statsPromise,
          byClusterPromise,
          histogramPromise,
        ]);
        const [risksResult, summaryResult, threatResult, clustersResult, statsResult, byClusterResult] = coreResults;
        if (threatResult.status === 'fulfilled') {
          setThreatVelocity(threatResult.value);
        } else {
          setThreatVelocity([]);
        }
        if (risksResult.status === 'fulfilled') {
          setRisks(risksResult.value.insights);
          setRisksTotal(risksResult.value.total);
        } else {
          setRisks([]);
          setRisksTotal(0);
          errors.push('Risks: ' + (risksResult.reason?.message || String(risksResult.reason)));
        }
        if (summaryResult.status === 'fulfilled') {
          setSeverityCountTrust('exact');
          const s = summaryResult.value;
          setInsightsSummary({
            total: num(s?.total),
            critical: num(s?.critical),
            high: num(s?.high),
            medium: num(s?.medium),
            low: num(s?.low),
          });
        } else if (risksResult.status === 'fulfilled') {
          const { insights, total } = risksResult.value;
          const totalN = num(total);
          const sampleSize = Math.min(1000, Math.max(1, totalN));
          try {
            const wide = await api.getRisks({
              page: 1,
              pageSize: sampleSize,
              ...risksListParamsBase,
            });
            const bySev = countSeverity(wide.insights);
            const wideTotal = num(wide.total);
            setInsightsSummary({
              total: wideTotal,
              critical: bySev.critical,
              high: bySev.high,
              medium: bySev.medium,
              low: bySev.low,
            });
            setSeverityCountTrust(wide.insights.length >= wideTotal ? 'exact' : 'sample');
          } catch {
            const bySev = countSeverity(insights);
            setInsightsSummary({
              total: totalN,
              critical: bySev.critical,
              high: bySev.high,
              medium: bySev.medium,
              low: bySev.low,
            });
            setSeverityCountTrust('page');
          }
        } else {
          setSeverityCountTrust('exact');
          setInsightsSummary(null);
        }
        if (clustersResult.status === 'fulfilled') {
          setClusters(clustersResult.value);
        } else {
          setClusters([]);
        }
        if (statsResult.status === 'fulfilled') {
          const st = statsResult.value as { resolved24h?: number };
          setResolved24h(st?.resolved24h ?? 0);
        } else {
          setResolved24h(0);
        }
        if (byClusterResult.status === 'fulfilled') {
          setRisksByCluster(Array.isArray(byClusterResult.value) ? byClusterResult.value : []);
        } else {
          setRisksByCluster([]);
        }
        if (errors.length > 0) setError(errors.join('; '));
      }
    } finally {
      hasLoadedOnceRef.current = true;
      setPageBlocking(false);
      setRefreshing(false);
    }
  }, [
    risksPage,
    risksPageSize,
    filter,
    statusFilter,
    debouncedSearchTerm,
    effectiveClusterId,
    sinceMinutesForApi,
    selectedClusterId,
    riskSort,
    priorityLevel,
    selectedScoreBin,
    namespaceFilter,
    typeFilter,
    trendDays,
    activeTab,
    pceTrendDays,
    pceClusterId,
    pceNamespace,
    pceSeverityFilter,
    pcePodName,
    pceCapabilityId,
    pceListPage,
    pceListPageSize,
  ]);

  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.SBOM_RISK_LIST));
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);
  usePolling(fetchData, intervalMs, { refreshTrigger });
  const fetchDataRef = React.useRef(fetchData);
  fetchDataRef.current = fetchData;
  React.useEffect(() => {
    const wsUrl = api.getRisksWsUrl();
    let ws: WebSocket | null = null;
    try {
      ws = new WebSocket(wsUrl);
      ws.onmessage = (e) => {
        try {
          const d = JSON.parse(e.data as string) as { type?: string };
          if (d?.type === 'insights_updated') fetchDataRef.current();
        } catch {
          // ignore
        }
      };
    } catch {
      // fallback to polling only
    }
    return () => {
      ws?.close();
    };
  }, []);
  const isFirstFetch = React.useRef(true);
  React.useEffect(() => {
    if (isFirstFetch.current) {
      isFirstFetch.current = false;
      return;
    }
    fetchData();
  }, [fetchData]);

  React.useEffect(() => { setRisksPage(1); }, [filter, statusFilter, debouncedSearchTerm, timeWindowMinutes, priorityLevel, namespaceFilter, typeFilter]);
  React.useEffect(() => { setSelectedScoreBin(null); }, [effectiveClusterId, effectiveSinceMinutesNum]);

  // Global drawer: open by URL ?insightId= (from Overview/PCE/Evidence deep link)
  React.useEffect(() => {
    if (!insightIdFromUrl) return;
    const id = insightIdFromUrl.trim();
    if (!id) return;
    const inList = risks.find((r) => r.id === id);
    if (inList) {
      setSelectedRisk(inList);
      return;
    }
    api.getInsight(id).then((detail) => {
      if (detail) {
        const asInsight: Insight = {
          id: detail.id,
          title: detail.title ?? '',
          description: detail.description,
          severity: (detail.severity ?? 'medium').toLowerCase() as Insight['severity'],
          score: detail.totalScore != null ? Math.round(Number(detail.totalScore)) : undefined,
          priorityLevel: detail.priorityLevel,
          insightType: (detail.insightType ?? detail.insight_type ?? 'vulnerability') as string,
          status: (detail.status === 'active' ? 'new' : detail.status === 'resolved' ? 'resolved' : 'acknowledged') as Insight['status'],
          timestamp: detail.detectedAt || detail.createdAt,
          clusterId: '',
          clusterName: '',
          affectedResources: detail.resourceUid ? [{ id: detail.resourceUid, name: detail.resourceName, kind: detail.resourceType ?? 'Pod', namespace: detail.resourceNamespace }] : [],
          totalScore: detail.totalScore,
        } as Insight;
        setSelectedRisk(asInsight);
      }
    }).catch(() => {});
  }, [insightIdFromUrl, risks]);

  // Sync drawer open state to URL (so bookmark/share works)
  React.useEffect(() => {
    if (selectedRisk) {
      setSearchParams((prev) => {
        const next = new URLSearchParams(prev);
        next.set('insightId', selectedRisk.id);
        return next;
      }, { replace: true });
    } else {
      setSearchParams((prev) => {
        const next = new URLSearchParams(prev);
        next.delete('insightId');
        return next;
      }, { replace: true });
    }
  }, [selectedRisk, setSearchParams]);

  React.useEffect(() => {
    if (!selectedRisk) {
      setSelectedRiskDetail(null);
      setSelectedRiskSignals([]);
      setSelectedRiskCapabilities([]);
      setSelectedRiskReferences([]);
      setDrawerAuditLogs([]);
      setSelectedRiskContext(null);
      setDrawerTab('summary');
      return;
    }

    let cancelled = false;
    const loadRiskContext = async () => {
      setSelectedRiskLoading(true);
      setDrawerAuditLoading(true);
      try {
        const podUid = selectedRisk.affectedResources?.find((r) => r.kind === 'Pod' && r.id)?.id;
        const [detailRes, signalsRes, capsRes, auditRes, contextRes] = await Promise.allSettled([
          api.getInsight(selectedRisk.id),
          podUid ? api.getRuntimeSignalsByPod(podUid, { limit: 10, sinceMinutes: sinceMinutesForApi }) : Promise.resolve([]),
          podUid ? api.getPodCapabilities(podUid) : Promise.resolve([]),
          api.getAuditLogs({ resource: 'insight', resourceId: selectedRisk.id, page: 1, pageSize: 20 }),
          api.getInsightContext(selectedRisk.id),
        ]);

        const detail = detailRes.status === 'fulfilled' ? detailRes.value : null;
        const signals = signalsRes.status === 'fulfilled' ? signalsRes.value : [];
        const capabilities = capsRes.status === 'fulfilled' ? capsRes.value : [];
        const auditLogs = auditRes.status === 'fulfilled' ? auditRes.value.logs : [];
        const context =
          contextRes.status === 'fulfilled' && contextRes.value
            ? {
                pods: Array.isArray(contextRes.value.pods) ? contextRes.value.pods : [],
                cluster: contextRes.value.cluster as { id?: string; name?: string } | undefined,
                rules: Array.isArray(contextRes.value.rules) ? contextRes.value.rules : [],
              }
            : null;

        const capabilityIds = Array.from(new Set(capabilities.map((c) => c.capabilityId).filter(Boolean))).slice(0, 5);
        const metadataResults = await Promise.allSettled(
          capabilityIds.map((capabilityId) => api.getCapabilityMetadataById(capabilityId))
        );
        const references = Array.from(new Set(
          metadataResults
            .filter((r): r is PromiseFulfilledResult<any> => r.status === 'fulfilled')
            .flatMap((r) => (r.value?.references || []))
            .filter((v: unknown) => typeof v === 'string' && /^https?:\/\//.test(v))
        ));

        if (!cancelled) {
          setSelectedRiskDetail(detail);
          setSelectedRiskSignals(signals);
          setSelectedRiskCapabilities(capabilities);
          setSelectedRiskReferences(references);
          setDrawerAuditLogs(auditLogs);
          setSelectedRiskContext(context);
        }
      } finally {
        if (!cancelled) {
          setSelectedRiskLoading(false);
          setDrawerAuditLoading(false);
        }
      }
    };

    loadRiskContext();
    return () => {
      cancelled = true;
    };
  }, [selectedRisk, sinceMinutesForApi]);

  // Evidence tab: fetch global Audit Trail (resource=insight) when sub-tab is Audit Trail (Phase 4)
  useEffect(() => {
    if (activeTab !== 'reference' || evidenceSubTab !== 'audit') return;
    let cancelled = false;
    setEvidenceAuditLoading(true);
    api.getAuditLogs({ resource: 'insight', page: evidenceAuditPage, pageSize: 20 })
      .then((data) => {
        if (!cancelled) {
          setEvidenceAuditLogs(data.logs);
          setEvidenceAuditTotal(data.total);
        }
      })
      .finally(() => { if (!cancelled) setEvidenceAuditLoading(false); });
    return () => { cancelled = true; };
  }, [activeTab, evidenceSubTab, evidenceAuditPage]);

  const getSeverityIcon = (severity: Insight['severity']) => {
    switch (severity) {
      case 'critical': return <Shield className="w-5 h-5 text-[#B42318]" />;
      case 'high': return <AlertTriangle className="w-5 h-5 text-[#F79009]" />;
      case 'medium': return <Info className="w-5 h-5 text-[#FDB022]" />;
      case 'low': return <CheckCircle className="w-5 h-5 text-[#667085]" />;
      default: return <Info className="w-5 h-5 text-slate-400" title={severity ? String(severity) : 'Unknown severity'} />;
    }
  };

  const summarizePceEvidence = (ev: Record<string, unknown> | undefined): { label: string; title: string } => {
    if (!ev || typeof ev !== 'object' || Object.keys(ev).length === 0) {
      return { label: '', title: '' };
    }
    const full = JSON.stringify(ev);
    const keys = Object.keys(ev).slice(0, 5);
    const label = keys
      .map((k) => {
        const v = ev[k];
        const vs =
          v != null && typeof v === 'object' ? JSON.stringify(v).slice(0, 32) : String(v ?? '').slice(0, 32);
        return `${k}: ${vs}`;
      })
      .join(' · ');
    return { label: label.length > 90 ? `${label.slice(0, 87)}…` : label, title: full };
  };

  React.useEffect(() => {
    setDrawerNotice(null);
  }, [selectedRisk?.id]);

  React.useEffect(() => {
    if (!selectedRisk) return;
    const root = drawerRef.current;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        setSelectedRisk(null);
        return;
      }
      if (e.key !== 'Tab' || !root) return;
      const selectors = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';
      const focusable = Array.from(root.querySelectorAll<HTMLElement>(selectors)).filter(
        (el) => !el.hasAttribute('disabled') && !el.getAttribute('aria-disabled') && el.offsetParent !== null,
      );
      if (focusable.length === 0) return;
      const active = document.activeElement as HTMLElement | null;
      if (!active || !root.contains(active)) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (e.shiftKey) {
        if (active === first) {
          e.preventDefault();
          last.focus();
        }
      } else if (active === last) {
        e.preventDefault();
        first.focus();
      }
    };
    window.addEventListener('keydown', onKeyDown, true);
    const t = window.setTimeout(() => {
      const el = drawerRef.current?.querySelector<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
      );
      el?.focus();
    }, 0);
    return () => {
      window.clearTimeout(t);
      window.removeEventListener('keydown', onKeyDown, true);
    };
  }, [selectedRisk, drawerTab]);

  // Same as Dashboard Security Risks: insights/summary first; fallback to current-page counts so cards always show numbers
  const severityBar = {
    total: Number(insightsSummary?.total ?? risksTotal ?? 0),
    critical: Number(insightsSummary?.critical ?? risks.filter(r => (r.severity || '').toLowerCase() === 'critical').length),
    high: Number(insightsSummary?.high ?? risks.filter(r => (r.severity || '').toLowerCase() === 'high').length),
    medium: Number(insightsSummary?.medium ?? risks.filter(r => (r.severity || '').toLowerCase() === 'medium').length),
    low: Number(insightsSummary?.low ?? risks.filter(r => (r.severity || '').toLowerCase() === 'low').length),
  };

  const severityRank: Record<string, number> = { critical: 4, high: 3, medium: 2, low: 1 };

  const RiskTrendTooltipContent = ({ active, payload, label }: { active?: boolean; payload?: any[]; label?: string }) => {
    if (!active || !payload || !payload.length) return null;
    const row = payload[0]?.payload as { date: string; critical?: number; high?: number; medium?: number; low?: number; risk?: number };
    if (!row) return null;
    const total = row.risk ?? ((row.critical ?? 0) + (row.high ?? 0) + (row.medium ?? 0) + (row.low ?? 0));
    return (
      <div className="bg-slate-900 border border-slate-700 rounded-lg shadow-xl p-3 text-left min-w-[180px]">
        <div className="text-slate-300 font-medium">Date: {label}</div>
        <div className="text-slate-400 text-sm mt-1">
          Total risks: <span className="text-slate-100 font-semibold">{total}</span>
        </div>
        <div className="flex flex-wrap gap-x-3 gap-y-0.5 mt-2 text-xs">
          <span className="text-red-400">Critical: {row.critical ?? 0}</span>
          <span className="text-orange-400">High: {row.high ?? 0}</span>
          <span className="text-yellow-400">Medium: {row.medium ?? 0}</span>
          <span className="text-blue-400">Low: {row.low ?? 0}</span>
        </div>
      </div>
    );
  };
  const sortedRisks = useMemo(() => {
    const out = [...risks];
    out.sort((a, b) => {
      switch (riskSort) {
        case 'oldest':
          return new Date(a.timestamp ?? 0).getTime() - new Date(b.timestamp ?? 0).getTime();
        case 'score_desc':
          return (b.score ?? -1) - (a.score ?? -1);
        case 'score_asc':
          return (a.score ?? Number.MAX_SAFE_INTEGER) - (b.score ?? Number.MAX_SAFE_INTEGER);
        case 'severity_desc':
          return (severityRank[(b.severity || '').toLowerCase()] ?? 0) - (severityRank[(a.severity || '').toLowerCase()] ?? 0);
        case 'title_asc':
          return (a.title || '').localeCompare(b.title || '');
        case 'newest':
        default:
          return new Date(b.timestamp ?? 0).getTime() - new Date(a.timestamp ?? 0).getTime();
      }
    });
    return out;
  }, [risks, riskSort]);

  const sortedPceDetails = useMemo(() => {
    const out = [...pceDetails];
    out.sort((a, b) => {
      const aSev = (a.severity || '').toLowerCase();
      const bSev = (b.severity || '').toLowerCase();
      switch (pceSort) {
        case 'severity_asc':
          return (severityRank[aSev] ?? 0) - (severityRank[bSev] ?? 0);
        case 'capability_asc':
          return (a.capabilityId || '').localeCompare(b.capabilityId || '');
        case 'pod_asc':
          return (a.podName || a.podUid || '').localeCompare(b.podName || b.podUid || '');
        case 'namespace_asc':
          return (a.namespace || '').localeCompare(b.namespace || '');
        case 'severity_desc':
        default:
          return (severityRank[bSev] ?? 0) - (severityRank[aSev] ?? 0);
      }
    });
    return out;
  }, [pceDetails, pceSort]);

  const paginatedRisks = sortedRisks;
  const statusLabelMap: Record<string, string> = {
    new: 'Active',
    acknowledged: 'In review',
    resolved: 'Resolved',
  };

  const findingsTableColCount = useMemo(() => {
    let n = 5; // checkbox, level, finding, workflow, actions
    if (riskFindingsCols.type) n += 1;
    if (riskFindingsCols.resource) n += 1;
    if (riskFindingsCols.score) n += 1;
    if (!effectiveClusterId && riskFindingsCols.nsCluster) n += 1;
    if (riskFindingsCols.detected) n += 1;
    if (riskFindingsCols.updated) n += 1;
    return n;
  }, [riskFindingsCols, effectiveClusterId]);

  if (pageBlocking) {
    return (
      <PageLayout title="Risk Operations" description={RISK_CENTER_DESCRIPTION}>
        <div className="space-y-6 animate-pulse" aria-busy="true" aria-label="Loading Risk Operations">
          <div className="h-10 bg-slate-800/80 rounded-lg border border-slate-800" />
          <div className="h-14 bg-slate-800/60 rounded-lg border border-slate-800" />
          <div className="grid gap-3 sm:grid-cols-3">
            <div className="h-28 bg-slate-800/60 rounded-lg border border-slate-800" />
            <div className="h-28 bg-slate-800/60 rounded-lg border border-slate-800" />
            <div className="h-28 bg-slate-800/60 rounded-lg border border-slate-800" />
          </div>
          <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
            <div className="h-[220px] bg-slate-800/40 rounded-lg border border-slate-800" />
            <div className="h-[220px] bg-slate-800/40 rounded-lg border border-slate-800" />
          </div>
          <div className="h-40 bg-slate-800/40 rounded-lg border border-slate-800" />
        </div>
      </PageLayout>
    );
  }

  const tabs: { id: TabId; label: string }[] = [
    { id: 'risks', label: 'Risk Findings' },
    { id: 'pce', label: 'Capability Exposure' },
    { id: 'reference', label: 'Evidence & References' },
  ];

  return (
    <PageLayout
      title="Risk Operations"
      description={RISK_CENTER_DESCRIPTION}
    >
      <div className="contents" inert={selectedRisk ? true : undefined}>
      {/* Error banner when some APIs failed */}
      {error && (
        <div className="flex items-center justify-between gap-4 p-4 bg-amber-500/10 border border-amber-500/30 rounded-lg text-amber-200">
          <div className="flex items-center gap-2 min-w-0">
            <AlertTriangle className="w-5 h-5 text-amber-500 shrink-0" />
            <span className="text-sm truncate" title={error}>{error}</span>
          </div>
          <Button variant="secondary" size="sm" onClick={() => { setError(null); fetchData(); }}>
            Retry
          </Button>
        </div>
      )}
      {refreshing && (
        <div className="flex items-center gap-2 px-3 py-2 text-xs text-slate-400 border border-slate-800 rounded-lg bg-slate-900/80">
          <Loader2 className="w-3.5 h-3.5 animate-spin shrink-0" />
          Updating data…
        </div>
      )}

      {/* Tabs */}
      <Tabs
        items={tabs}
        value={activeTab}
        onChange={(id) => {
          const tabId = id as TabId;
          if (tabId === 'risks') {
            navigate('/risks/findings');
          } else if (tabId === 'pce') {
            navigate('/risks/pce');
          } else {
            navigate('/risks/evidence');
          }
        }}
      />
      <details className="mt-3 group border border-slate-800 rounded-lg bg-slate-900/50 text-xs text-slate-400">
        <summary className="cursor-pointer select-none px-3 py-2 text-slate-300 font-medium list-none flex items-center gap-2 [&::-webkit-details-marker]:hidden">
          <span className="text-slate-500 group-open:rotate-90 transition-transform inline-block">▸</span>
          About this page
        </summary>
        <div className="px-3 pb-3 pt-0 space-y-2 border-t border-slate-800/80">
          <p>
            <strong className="text-slate-300">Risk Findings</strong> = active security findings.{' '}
            <strong className="text-slate-300">Capability Exposure</strong> = pod capability exposure counts (separate from findings).{' '}
            <strong className="text-slate-300">Evidence &amp; References</strong> = runtime evidence and capability knowledge (no single total).{' '}
            <em>Total findings below applies only to Risk Findings.</em>
          </p>
          <p>
            <span className="text-slate-300 font-medium">Objective:</span> Triage and prioritize active risks in operations.{' '}
            <Link className="text-pink-400 hover:underline" to="/rules">Detection &amp; Policy</Link>
            {' · '}
            <Link className="text-pink-400 hover:underline" to="/capabilities">Capability Knowledge</Link>
          </p>
        </div>
      </details>
      <div className="mt-2 p-3 bg-slate-900/40 border border-slate-800 rounded-lg text-xs text-slate-400 flex flex-wrap gap-x-4 gap-y-1 items-center">
        {activeTab !== 'risks' && (
          <span title="Count of risk findings in current scope. On Risk Findings tab, see Risk Level Overview for the same total.">
            Active findings (Risk): <span className="text-slate-200 font-medium">{severityBar.total}</span>
          </span>
        )}
        <span>
          Scope:{' '}
          <span className="text-slate-200 font-medium" title={effectiveClusterId ?? undefined}>
            {scopeClusterDisplay ? `cluster: ${scopeClusterDisplay}` : 'all clusters'}
          </span>
        </span>
        <span>
          Time window:{' '}
          <span className="text-slate-200 font-medium">{formatMinutesHuman(sinceMinutesForApi)}</span>
        </span>
        {activeTab === 'risks' && (
          <>
            <Button
              variant="secondary"
              size="sm"
              disabled={exportLoading}
              title="Max 10,000 rows. Current filters (cluster, time, severity, status, search) apply."
              onClick={async () => {
                setExportLoading(true);
                try {
                  await api.exportRisksCSV({
                    clusterId: effectiveClusterId ?? undefined,
                    sinceMinutes: sinceMinutesForApi,
                    status: statusFilter,
                    severity: filter !== 'all' ? filter : undefined,
                    search: debouncedSearchTerm || undefined,
                  });
                } catch (e) {
                  setError(String(e instanceof Error ? e.message : e));
                } finally {
                  setExportLoading(false);
                }
              }}
            >
              {exportLoading ? <Loader2 className="w-4 h-4 animate-spin inline mr-1" /> : null}
              Export CSV
            </Button>
            <Button
              variant="secondary"
              size="sm"
              disabled={exportLoading}
              title="Max 10,000 rows. Print-optimized HTML; use browser Print → Save as PDF. Current filters apply."
              onClick={async () => {
                setExportLoading(true);
                try {
                  await api.exportRisksPDF({
                    clusterId: effectiveClusterId ?? undefined,
                    sinceMinutes: sinceMinutesForApi,
                    status: statusFilter,
                    severity: filter !== 'all' ? filter : undefined,
                    search: debouncedSearchTerm || undefined,
                  });
                } catch (e) {
                  setError(String(e instanceof Error ? e.message : e));
                } finally {
                  setExportLoading(false);
                }
              }}
            >
              {exportLoading ? <Loader2 className="w-4 h-4 animate-spin inline mr-1" /> : null}
              Export PDF
            </Button>
          </>
        )}
      </div>

      {/* ----- Risks tab ----- */}
      {activeTab === 'risks' && (
        <div className="space-y-6">
          {/* Risk Trend + Risk Score Distribution (wireframe §1) */}
          <div className="grid gap-4 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)] items-stretch">
            {/* Risk Trend 7 Days (AreaChart) */}
            <div className="bg-surface border border-border rounded-lg p-3 md:p-4 flex flex-col">
              <div className="flex items-center justify-between gap-2 mb-1">
                <h2 className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Risk Trend (Last {trendDays} Days)</h2>
                <div className="flex items-center gap-1 text-[11px] text-slate-400">
                  <span>Range:</span>
                  {[1, 7, 30].map((d) => (
                    <button
                      key={d}
                      type="button"
                      onClick={() => setTrendDays(d)}
                      className={`px-2 py-0.5 rounded-full border ${
                        trendDays === d
                          ? 'border-pink-500 text-pink-300 bg-pink-500/10'
                          : 'border-slate-700 text-slate-400 hover:border-slate-500'
                      }`}
                    >
                      {d === 1 ? '24h' : `${d}d`}
                    </button>
                  ))}
                </div>
              </div>
              <p className="text-[11px] text-slate-500 mb-2 md:mb-3">
                Track whether open risks are increasing or decreasing. Same scope as Total findings (Risk Findings only).{' '}
                {scopeClusterDisplay ? `Scoped to cluster: ${scopeClusterDisplay}.` : 'All clusters.'} Click a point to filter findings from that date.
              </p>
              {threatVelocity.length > 0 ? (
                <div className="w-full h-[220px] md:h-[240px]">
                  <ResponsiveContainer width="100%" height="100%">
                    <AreaChart
                      data={threatVelocity.map((p) => ({
                        ...p,
                        name: p.date,
                        risk: (p.critical ?? 0) + (p.high ?? 0) + (p.medium ?? 0) + (p.low ?? 0),
                      })).sort((a, b) => a.name.localeCompare(b.name))}
                      margin={{ top: 10, right: 10, left: 5, bottom: 0 }}
                      onClick={(state) => {
                        if (state?.activePayload?.[0]?.payload?.name) {
                          setSelectedChartDate(state.activePayload[0].payload.name);
                          setRisksPage(1);
                        }
                      }}
                      style={{ cursor: 'pointer' }}
                    >
                      <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#334155" />
                      <XAxis dataKey="name" axisLine={false} tickLine={false} tick={{ fill: '#94a3b8', fontSize: 11 }} />
                      <YAxis domain={[0, 'auto']} axisLine={false} tickLine={false} tick={{ fill: '#94a3b8', fontSize: 11 }} width={28} />
                      <Tooltip content={<RiskTrendTooltipContent />} cursor={{ stroke: '#64748b', strokeDasharray: '4 4' }} />
                      <Area type="monotone" dataKey="low" stackId="risk" stroke="#3b82f6" fill="#3b82f6" fillOpacity={0.65} name="Low" />
                      <Area type="monotone" dataKey="medium" stackId="risk" stroke="#ca8a04" fill="#ca8a04" fillOpacity={0.65} name="Medium" />
                      <Area type="monotone" dataKey="high" stackId="risk" stroke="#ea580c" fill="#ea580c" fillOpacity={0.65} name="High" />
                      <Area type="monotone" dataKey="critical" stackId="risk" stroke="#dc2626" fill="#dc2626" fillOpacity={0.7} name="Critical" />
                    </AreaChart>
                  </ResponsiveContainer>
                  {selectedChartDate && (
                    <p className="text-xs text-slate-400 mt-2">
                      Showing findings since <span className="font-medium text-pink-400">{selectedChartDate}</span>
                      <button type="button" onClick={() => setSelectedChartDate(null)} className="ml-2 text-pink-400 hover:underline">Clear</button>
                    </p>
                  )}
                </div>
              ) : (
                <p className="text-slate-500 text-sm py-4">No trend data for the selected range.</p>
              )}
            </div>

            {/* Risk Score Distribution (compact card, 1/3 width on desktop) */}
            <div className="bg-surface border border-border rounded-lg p-3 md:p-4 flex flex-col">
              <div className="flex flex-wrap items-center justify-between gap-2 mb-2">
                <h2 className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Risk Score Distribution</h2>
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={syncScoresLoading}
                  title="Recalculate risk scores for all resources with active insights. Use when histogram is empty or after bulk import."
                  onClick={async () => {
                    setSyncScoresLoading(true);
                    setSyncScoresMessage(null);
                    try {
                      const res = await api.syncRiskScores();
                      setSyncScoresMessage(
                        res.resources === 0
                          ? 'No resources to sync.'
                          : `Sync started for ${res.resources} resources. Refreshing in a few seconds…`
                      );
                      if (res.resources > 0) {
                        setTimeout(() => {
                          fetchDataRef.current();
                          setSyncScoresMessage(null);
                        }, 4000);
                      }
                    } catch (e) {
                      setSyncScoresMessage(String(e instanceof Error ? e.message : e));
                    } finally {
                      setSyncScoresLoading(false);
                    }
                  }}
                >
                  {syncScoresLoading ? <Loader2 className="w-4 h-4 animate-spin inline mr-1" /> : null}
                  Recalculate all scores
                </Button>
              </div>
              {(histogramData?.totalFindings === 0 && !histogramLoading) && (
                <p className="text-xs text-amber-400/90 mb-2">
                  No score data yet. Click &quot;Recalculate all scores&quot; to compute risk scores for all resources with findings.
                </p>
              )}
              <p className="text-xs text-slate-500 mb-3">
                Bins 0–10 … 90–100. Stacked by severity. Click a bar to filter the table to that score range. Ref lines: P0 (90), P1 (70).
              </p>
              {syncScoresMessage && (
                <p className="text-xs text-slate-400 mb-2">{syncScoresMessage}</p>
              )}
              {selectedScoreBin != null && (
                <p className="text-xs text-slate-400 mb-2">
                  Filtered to score <span className="font-medium text-pink-400">{selectedScoreBin}–{selectedScoreBin + 10}</span>
                  <button type="button" onClick={() => { setSelectedScoreBin(null); setRisksPage(1); fetchDataRef.current(); }} className="ml-2 text-pink-400 hover:underline">Clear filter</button>
                </p>
              )}
              <RiskHistogram
                data={histogramData}
                loading={histogramLoading}
                compact={isOverviewPage}
                onBinClick={(bin) => {
                  setSelectedScoreBin(bin);
                  setRisksPage(1);
                  if (isOverviewPage) navigate('/risks/findings');
                  fetchDataRef.current();
                }}
                selectedBin={selectedScoreBin}
              />
            </div>
          </div>

          {/* Layer 2 – Risk level overview: totals, velocity, resolved + severity breakdown */}
          <div>
            <h2 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Risk Level Overview (Active Findings)</h2>
            {severityCountTrust !== 'exact' && (
              <div
                className="mb-2 p-2 rounded-lg border border-amber-500/40 bg-amber-500/10 text-amber-100 text-xs"
                title={
                  severityCountTrust === 'sample'
                    ? 'Severity breakdown from first up to 1000 rows matching filters'
                    : 'Severity breakdown from current page only'
                }
              >
                {severityCountTrust === 'sample' ? (
                  <>
                    Summary API unavailable: severity counts use the <strong>first up to 1,000</strong> findings matching your filters. Total count is still the server total.
                  </>
                ) : (
                  <>
                    Summary API unavailable: severity counts reflect the <strong>current table page</strong> only. Total findings still matches the server total.
                  </>
                )}
              </div>
            )}
            <p className="text-xs text-slate-500 mb-3">
              Same scope as &quot;Total findings&quot; above. Severities from GET /insights/summary when available.
            </p>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 mb-4">
              <button
                type="button"
                onClick={() => isOverviewPage && navigate('/risks/findings')}
                className="bg-slate-900 border border-slate-800 rounded-lg p-4 text-left hover:border-slate-600 transition-colors"
                title="Total risk findings in scope"
              >
                <div className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Total</div>
                <div className="mt-1 text-2xl font-bold text-slate-200">{severityBar.total}</div>
                {threatVelocity.length >= 2 && (() => {
                  const sorted = [...threatVelocity].sort((a, b) => a.date.localeCompare(b.date));
                  const first = sorted[0];
                  const last = sorted[sorted.length - 1];
                  const firstTotal = (first?.critical ?? 0) + (first?.high ?? 0) + (first?.medium ?? 0) + (first?.low ?? 0);
                  const lastTotal = (last?.critical ?? 0) + (last?.high ?? 0) + (last?.medium ?? 0) + (last?.low ?? 0);
                  const delta = lastTotal - firstTotal;
                  if (delta === 0) return null;
                  return (
                    <span className={`text-xs font-medium ${delta > 0 ? 'text-red-400' : 'text-emerald-400'}`}>
                      ({delta > 0 ? '+' : ''}{delta} vs start of range)
                    </span>
                  );
                })()}
              </button>
              <div className="bg-slate-900 border border-slate-800 rounded-lg p-4" title="Net change in open risks over the trend range">
                <div className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Velocity</div>
                {threatVelocity.length >= 2 ? (() => {
                  const sorted = [...threatVelocity].sort((a, b) => a.date.localeCompare(b.date));
                  const first = sorted[0];
                  const last = sorted[sorted.length - 1];
                  const firstTotal = (first?.critical ?? 0) + (first?.high ?? 0) + (first?.medium ?? 0) + (first?.low ?? 0);
                  const lastTotal = (last?.critical ?? 0) + (last?.high ?? 0) + (last?.medium ?? 0) + (last?.low ?? 0);
                  const delta = lastTotal - firstTotal;
                  return (
                    <div className="mt-1 flex items-baseline gap-1">
                      <span className={`text-xl font-bold ${delta >= 0 ? 'text-red-400' : 'text-emerald-400'}`}>{delta >= 0 ? '↑' : '↓'}</span>
                      <span className="text-slate-300 font-medium">{Math.abs(delta)}</span>
                      <span className="text-xs text-slate-500">vs range start</span>
                    </div>
                  );
                })() : (
                  <div className="mt-1 text-slate-500 text-sm">—</div>
                )}
              </div>
              <div className="bg-slate-900 border border-slate-800 rounded-lg p-4" title="Insights resolved in the last 24h (GET /dashboard/stats)">
                <div className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Resolved (24h)</div>
                <div className="mt-1 text-2xl font-bold text-emerald-500">{Number(resolved24h ?? 0)}</div>
              </div>
            </div>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              {(['critical', 'high', 'medium', 'low'] as const).map((sev) => (
                <button
                  key={sev}
                  type="button"
                  onClick={() => isOverviewPage ? navigate(`/risks/findings?severity=${sev}`) : setFilter(sev)}
                  className="bg-slate-900 border border-slate-800 p-4 rounded-lg flex flex-col gap-1 text-left hover:border-slate-600 transition-colors"
                  title={
                    severityCountTrust === 'sample'
                      ? `Approximate count (sample up to 1000). View ${sev} findings`
                      : severityCountTrust === 'page'
                        ? `Approximate count (current page). View ${sev} findings`
                        : isOverviewPage
                          ? `View ${sev} findings`
                          : 'Filter by severity'
                  }
                >
                  <span className="text-slate-400 text-sm capitalize">{sev}</span>
                  <span className={`font-bold text-xl ${sev === 'critical' ? 'text-red-500' : sev === 'high' ? 'text-orange-500' : sev === 'medium' ? 'text-yellow-500' : 'text-blue-500'}`}>
                    {severityBar[sev]}
                  </span>
                  {sev === 'critical' && (histogramData?.p0Count ?? 0) > 0 && (
                    <span className="text-[10px] text-slate-500">P0 in histogram: {histogramData!.p0Count}</span>
                  )}
                </button>
              ))}
            </div>
            {/* Risks by cluster (Phase 2.2) – when scope is all clusters */}
            {!effectiveClusterId && risksByCluster.length > 0 && (
              <div className="mt-4">
                <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Risks by cluster</h3>
                <div className="overflow-x-auto border border-slate-800 rounded-lg">
                  <table className="w-full text-sm">
                    <thead className="sticky top-0 z-[1]">
                      <tr className={TABLE_HEAD_ROW}>
                        <th className="text-left px-3 py-2 font-medium">Cluster</th>
                        <th className="text-right px-3 py-2 font-medium">Total</th>
                        <th className="text-right px-3 py-2 text-red-400 font-medium">Critical</th>
                        <th className="text-right px-3 py-2 text-orange-400 font-medium">High</th>
                        <th className="text-right px-3 py-2 text-yellow-400 font-medium">Medium</th>
                        <th className="text-right px-3 py-2 text-blue-400 font-medium">Low</th>
                      </tr>
                    </thead>
                    <tbody>
                      {risksByCluster.map((row) => (
                        <tr key={row.clusterId} className={TABLE_BODY_ROW}>
                          <td className="px-3 py-2">
                            <button
                              type="button"
                              onClick={() => setSelectedClusterId(row.clusterId)}
                              className="text-pink-400 hover:text-pink-300 hover:underline font-medium text-left"
                            >
                              {row.clusterName || row.clusterId}
                            </button>
                          </td>
                          <td className="text-right px-3 py-2 text-slate-200">{row.total}</td>
                          <td className="text-right px-3 py-2 text-red-400">{row.critical}</td>
                          <td className="text-right px-3 py-2 text-orange-400">{row.high}</td>
                          <td className="text-right px-3 py-2 text-yellow-400">{row.medium}</td>
                          <td className="text-right px-3 py-2 text-blue-400">{row.low}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
          </div>

          {/* Overview only: Quick Links + CTA. Findings page: filters + table. */}
          {isOverviewPage ? (
            <>
              <div className="bg-slate-900 border border-slate-800 rounded-lg p-4">
                <h2 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Quick Links</h2>
                <p className="text-xs text-slate-500 mb-3">Jump to detailed views. Drawer can be opened from any page via URL <code className="text-slate-400">?insightId=</code>.</p>
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                  <button
                    type="button"
                    onClick={() => navigate('/risks/findings')}
                    className="flex items-center gap-3 p-4 rounded-lg border border-slate-700 bg-slate-800/50 hover:bg-slate-800 hover:border-pink-500/50 transition-colors text-left"
                  >
                    <Box className="w-10 h-10 text-pink-400 shrink-0" />
                    <div>
                      <span className="font-medium text-slate-200 block">View All Findings</span>
                      <span className="text-xs text-slate-500">Table, filters, bulk actions, export</span>
                    </div>
                    <ArrowRight className="w-5 h-5 text-slate-500 shrink-0 ml-auto" />
                  </button>
                  <button
                    type="button"
                    onClick={() => navigate('/risks/pce')}
                    className="flex items-center gap-3 p-4 rounded-lg border border-slate-700 bg-slate-800/50 hover:bg-slate-800 hover:border-pink-500/50 transition-colors text-left"
                  >
                    <Shield className="w-10 h-10 text-amber-400 shrink-0" />
                    <div>
                      <span className="font-medium text-slate-200 block">Capability Exposure Heatmap</span>
                      <span className="text-xs text-slate-500">Capability exposure by namespace & severity</span>
                    </div>
                    <ArrowRight className="w-5 h-5 text-slate-500 shrink-0 ml-auto" />
                  </button>
                  <button
                    type="button"
                    onClick={() => navigate('/risks/evidence')}
                    className="flex items-center gap-3 p-4 rounded-lg border border-slate-700 bg-slate-800/50 hover:bg-slate-800 hover:border-pink-500/50 transition-colors text-left"
                  >
                    <ExternalLink className="w-10 h-10 text-sky-400 shrink-0" />
                    <div>
                      <span className="font-medium text-slate-200 block">Evidence & References</span>
                      <span className="text-xs text-slate-500">Runtime signals, audit trail</span>
                    </div>
                    <ArrowRight className="w-5 h-5 text-slate-500 shrink-0 ml-auto" />
                  </button>
                </div>
              </div>
              <div className="flex justify-center py-6">
                <Button variant="primary" size="lg" onClick={() => navigate('/risks/findings')}>
                  View all findings ({severityBar.total})
                </Button>
              </div>
            </>
          ) : (
            <>
          {/* Filters: Severity, Status (spec), Search */}
          <div className="flex flex-col md:flex-row gap-4 md:items-center md:justify-between">
            <div className="flex flex-wrap gap-2 items-center">
              <span className="text-xs text-slate-500 uppercase tracking-wider mr-1">Risk Level:</span>
              {['all', 'critical', 'high', 'medium', 'low'].map((sev) => (
                <button
                  key={sev}
                  onClick={() => setFilter(sev)}
                  className={`px-3 py-1.5 rounded-full text-sm font-medium capitalize transition-colors whitespace-nowrap ${
                    filter === sev
                      ? 'bg-pink-600 text-white shadow-md'
                      : 'bg-slate-900 text-slate-400 hover:bg-slate-800 border border-slate-700'
                  }`}
                >
                  {sev}
                </button>
              ))}
              <span className="text-xs text-slate-500 uppercase tracking-wider ml-2 mr-1">Workflow:</span>
              {(['all', 'active', 'resolved', 'acknowledged'] as const).map((st) => (
                <button
                  key={st}
                  onClick={() => setStatusFilter(st)}
                  className={`px-3 py-1.5 rounded-full text-sm font-medium capitalize transition-colors whitespace-nowrap ${
                    statusFilter === st
                      ? 'bg-pink-600 text-white shadow-md'
                      : 'bg-slate-900 text-slate-400 hover:bg-slate-800 border border-slate-700'
                  }`}
                >
                  {st === 'all' ? 'All' : st === 'active' ? 'Active' : st === 'resolved' ? 'Resolved' : 'In review'}
                </button>
              ))}
              <span className="text-xs text-slate-500 uppercase tracking-wider ml-2 mr-1">Namespace:</span>
              <input
                value={namespaceFilter}
                onChange={(e) => setNamespaceFilter(e.target.value)}
                placeholder="All"
                className="px-3 py-1.5 rounded-full text-sm bg-slate-900 text-slate-200 border border-slate-700 focus:outline-none focus:border-pink-500"
              />
              <span className="text-xs text-slate-500 uppercase tracking-wider ml-2 mr-1">Rule type:</span>
              <select
                value={typeFilter}
                onChange={(e) => setTypeFilter(e.target.value)}
                className="px-3 py-1.5 rounded-full text-sm bg-slate-900 text-slate-200 border border-slate-700 focus:outline-none focus:border-pink-500"
              >
                <option value="">All</option>
                <option value="vulnerability">Vulnerability</option>
                <option value="supply_chain_malware">Supply-chain malware</option>
                <option value="rbac_risk">RBAC risk</option>
                <option value="capability">Capability</option>
              </select>
            </div>
            <div className="relative flex-1 md:max-w-xs">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 w-4 h-4" />
              <input
                type="text"
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                placeholder="Search title, CVE, package, pod, namespace..."
                className="pl-9 pr-4 py-2 bg-slate-900 border border-slate-700 rounded-lg text-sm text-white focus:ring-2 focus:ring-pink-500 outline-none w-full placeholder:text-slate-600"
              />
            </div>
            <div className="flex items-center gap-2">
              <span className="text-xs text-slate-500 uppercase tracking-wider whitespace-nowrap">Priority:</span>
              <select
                value={priorityLevel}
                onChange={(e) => setPriorityLevel(e.target.value)}
                className="bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-300 focus:outline-none focus:border-pink-500"
              >
                <option value="">All</option>
                <option value="P0">P0</option>
                <option value="P1">P1</option>
                <option value="P2">P2</option>
                <option value="P3">P3</option>
                <option value="P4">P4</option>
              </select>
            </div>
            <div className="flex items-center gap-2">
              <span className="text-xs text-slate-500 uppercase tracking-wider whitespace-nowrap">Sort:</span>
              <select
                value={riskSort}
                onChange={(e) => setRiskSort(e.target.value as typeof riskSort)}
                className="bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-300 focus:outline-none focus:border-pink-500"
              >
                <option value="newest">Newest first</option>
                <option value="oldest">Oldest first</option>
                <option value="severity_desc">Severity high to low</option>
                <option value="score_desc">Risk score high to low</option>
                <option value="score_asc">Risk score low to high</option>
                <option value="title_asc">Title A-Z</option>
              </select>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-x-3 gap-y-2 text-xs text-slate-400 -mt-1">
            <span className="text-slate-500 uppercase tracking-wider whitespace-nowrap">Table columns:</span>
            {(
              [
                { key: 'type' as const, label: 'Type' },
                { key: 'resource' as const, label: 'Impacted resource' },
                { key: 'score' as const, label: 'Risk score' },
                { key: 'nsCluster' as const, label: 'Namespace / cluster', allClustersOnly: true },
                { key: 'detected' as const, label: 'Detected' },
                { key: 'updated' as const, label: 'Updated' },
              ] satisfies { key: keyof RiskFindingsTableCols; label: string; allClustersOnly?: boolean }[]
            )
              .filter((c) => !c.allClustersOnly || !effectiveClusterId)
              .map((c) => (
                <label key={c.key} className="inline-flex items-center gap-1.5 cursor-pointer select-none">
                  <input
                    type="checkbox"
                    className="h-3.5 w-3.5 rounded border-slate-600 bg-slate-900"
                    checked={riskFindingsCols[c.key]}
                    onChange={() =>
                      setRiskFindingsCols((prev) => ({ ...prev, [c.key]: !prev[c.key] }))
                    }
                  />
                  <span>{c.label}</span>
                </label>
              ))}
          </div>
          <p className="text-xs text-slate-500 -mt-2">
            Tip: click any finding row to open contextual investigation panel without leaving this page.
          </p>

          {/* Layer 3 – Risk Exploration (spec §3.3): core table. Runtime signals only in Reference tab / detail drawer. */}
          {/* Bulk actions toolbar */}
          <div className="flex items-center justify-between text-xs text-slate-400 mb-1">
            <div>
              {selectedIds.size > 0 ? (
                <span>
                  {selectedIds.size} selected.{' '}
                  <button
                    type="button"
                    className="text-pink-400 hover:underline"
                    onClick={() => setSelectedIds(new Set())}
                  >
                    Clear selection
                  </button>
                </span>
              ) : (
                <span>Select findings to run bulk actions.</span>
              )}
            </div>
            <div className="flex gap-2">
              <Button
                size="xs"
                variant="secondary"
                disabled={selectedIds.size === 0}
                onClick={async () => {
                  const ids = Array.from(selectedIds);
                  if (!ids.length) return;
                  if (!window.confirm(`Acknowledge ${ids.length} selected finding(s)?`)) return;
                  await api.bulkInsightsAction({ action: 'acknowledge', insightIds: ids });
                  setSelectedIds(new Set());
                  fetchDataRef.current();
                }}
              >
                Acknowledge selected
              </Button>
              <Button
                size="xs"
                variant="secondary"
                disabled={selectedIds.size === 0}
                onClick={async () => {
                  const ids = Array.from(selectedIds);
                  if (!ids.length) return;
                  if (!window.confirm(`Resolve ${ids.length} selected finding(s)? This marks them resolved.`)) return;
                  await api.bulkInsightsAction({ action: 'resolve', insightIds: ids });
                  setSelectedIds(new Set());
                  fetchDataRef.current();
                }}
              >
                Resolve selected
              </Button>
              <Button
                size="xs"
                variant="secondary"
                disabled={selectedIds.size === 0}
                onClick={async () => {
                  const ids = Array.from(selectedIds);
                  if (!ids.length) return;
                  if (!window.confirm(`Dismiss ${ids.length} selected finding(s)? This may hide them from active triage.`)) return;
                  await api.bulkInsightsAction({ action: 'dismiss', insightIds: ids });
                  setSelectedIds(new Set());
                  fetchDataRef.current();
                }}
              >
                Dismiss selected
              </Button>
            </div>
          </div>

          {/* Risk list – compact table with internal scroll to keep layout consistent */}
          <div className="bg-surface rounded-lg border border-border shadow-sm p-0 overflow-hidden">
            <div className="ui-table-scroll">
              <table className="w-full text-sm">
                <thead className={`${TABLE_HEAD_ROW} sticky top-0 z-10`}>
                  <tr>
                    <th className="text-left px-4 py-3 w-8">
                      <input
                        type="checkbox"
                        className="h-4 w-4 rounded border-slate-600 bg-slate-900"
                        checked={paginatedRisks.length > 0 && paginatedRisks.every((r) => selectedIds.has(r.id))}
                        onChange={(e) => {
                          if (e.target.checked) {
                            const next = new Set(selectedIds);
                            paginatedRisks.forEach((r) => next.add(r.id));
                            setSelectedIds(next);
                          } else {
                            const next = new Set(selectedIds);
                            paginatedRisks.forEach((r) => next.delete(r.id));
                            setSelectedIds(next);
                          }
                        }}
                      />
                    </th>
                    <th className="text-left px-4 py-3 w-10">Level</th>
                    <th className="text-left px-4 py-3">Finding</th>
                    {riskFindingsCols.type && (
                      <th className="text-left px-4 py-3 w-36 hidden sm:table-cell">Type</th>
                    )}
                    {riskFindingsCols.resource && (
                      <th className="text-left px-4 py-3 w-20">Impacted Resource</th>
                    )}
                    {riskFindingsCols.score && (
                      <th className="text-left px-4 py-3 w-20 min-w-[5rem]">Risk Score</th>
                    )}
                    {!effectiveClusterId && riskFindingsCols.nsCluster && (
                      <th className="text-left px-4 py-3 hidden lg:table-cell w-32">Namespace / Cluster</th>
                    )}
                    <th className="text-left px-4 py-3 w-24">Workflow</th>
                    {riskFindingsCols.detected && (
                      <th className="text-left px-4 py-3 w-24 hidden md:table-cell">Detected At</th>
                    )}
                    {riskFindingsCols.updated && (
                      <th className="text-left px-4 py-3 w-24 hidden lg:table-cell">Updated At</th>
                    )}
                    <th className="text-right px-4 py-3 w-32">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {paginatedRisks.length === 0 ? (
                    <tr>
                      <td colSpan={findingsTableColCount} className="px-4 py-8 text-center text-slate-500">
                        No risks match the current filters.
                      </td>
                    </tr>
                  ) : (
                    paginatedRisks.map((risk) => (
                          <tr
                        key={risk.id}
                        className={`${TABLE_BODY_ROW} cursor-pointer`}
                        onClick={() => setSelectedRisk(risk)}
                      >
                        <td className="px-4 py-3" onClick={(e) => e.stopPropagation()}>
                          <input
                            type="checkbox"
                            className="h-4 w-4 rounded border-slate-600 bg-slate-900"
                            checked={selectedIds.has(risk.id)}
                            onChange={(e) => {
                              const next = new Set(selectedIds);
                              if (e.target.checked) {
                                next.add(risk.id);
                              } else {
                                next.delete(risk.id);
                              }
                              setSelectedIds(next);
                            }}
                          />
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex items-center gap-1">
                            {getSeverityIcon(risk.severity)}
                            {(risk.priorityLevel === 'P0' || risk.priorityLevel === 'P1') && (
                              <span className={`text-[10px] font-bold px-1.5 py-0.5 rounded ${risk.priorityLevel === 'P0' ? 'bg-red-500/20 text-red-400' : 'bg-orange-500/20 text-orange-400'}`} title="Priority">
                                {risk.priorityLevel}
                              </span>
                            )}
                          </div>
                        </td>
                        <td className="px-4 py-3">
                          <span className="text-white font-medium">{risk.title}</span>
                          <span className="text-slate-500 ml-1 text-xs font-mono" title={risk.cveId}>
                            ({riskListSecondaryLabel(risk)})
                          </span>
                        </td>
                        {riskFindingsCols.type && (
                          <td className="px-4 py-3 text-slate-400 hidden sm:table-cell text-xs leading-snug">
                            {(() => {
                              const label =
                                risk.insightType === 'vulnerability'
                                  ? 'Vulnerability'
                                  : risk.insightType === 'supply_chain_malware'
                                    ? 'Supply-chain malware'
                                    : risk.insightType === 'rbac'
                                      ? 'Behavior'
                                      : (risk.insightType ?? 'Finding');
                              const src =
                                risk.insightType === 'vulnerability' || risk.insightType === 'supply_chain_malware'
                                  ? 'Static'
                                  : 'Runtime';
                              return (
                                <>
                                  <span className="capitalize block text-slate-300">{label}</span>
                                  <span className="text-[10px] text-slate-500">({src})</span>
                                </>
                              );
                            })()}
                          </td>
                        )}
                        {riskFindingsCols.resource && (
                          <td className="px-4 py-3 text-slate-400 text-xs" onClick={(e) => e.stopPropagation()}>
                            {risk.affectedResources?.length
                              ? risk.affectedResources.length === 1 && risk.affectedResources[0]?.kind && risk.affectedResources[0]?.id
                                ? risk.affectedResources[0].kind === 'Pod'
                                  ? (
                                      <Link
                                        to={`/resources/pods/uid/${encodeURIComponent(risk.affectedResources[0].id)}`}
                                        className="text-pink-400 hover:text-pink-300 hover:underline font-medium"
                                        title="View pod in Resources"
                                      >
                                        1 Pod →
                                      </Link>
                                    )
                                  : risk.affectedResources[0].kind === 'ServiceAccount'
                                    ? (
                                        <Link
                                          to={`/identities/uid/${encodeURIComponent(risk.affectedResources[0].id)}`}
                                          className="text-pink-400 hover:text-pink-300 hover:underline font-medium"
                                          title="View identity"
                                        >
                                          1 ServiceAccount →
                                        </Link>
                                      )
                                      : `1 ${risk.affectedResources[0].kind}`
                                : `${risk.affectedResources.length} resources`
                              : '—'}
                          </td>
                        )}
                        {riskFindingsCols.score && (
                          <td
                            className="px-4 py-3 text-slate-300 align-top"
                            title={
                              [
                                risk.score != null ? `Score ${risk.score}/100` : null,
                                risk.priorityLevel ? `Priority ${risk.priorityLevel}` : null,
                                risk.exploitabilityScore != null ? `Exploitability ${risk.exploitabilityScore.toFixed(1)}` : null,
                                risk.businessImpactScore != null ? `Business impact ${risk.businessImpactScore.toFixed(1)}` : null,
                              ]
                                .filter(Boolean)
                                .join(' · ') || undefined
                            }
                          >
                            {risk.score != null ? (
                              <div className="space-y-1">
                                <div className="font-medium tabular-nums">
                                  {risk.score}/100
                                  {risk.priorityLevel ? <span className="text-slate-500 font-normal"> {risk.priorityLevel}</span> : null}
                                </div>
                                <div
                                  className="h-1 rounded-full bg-slate-800 overflow-hidden max-w-[4.5rem]"
                                  title="Score / 100"
                                >
                                  <div
                                    className={`h-full rounded-full ${risk.score >= 70 ? 'bg-red-500' : risk.score >= 40 ? 'bg-amber-500' : 'bg-emerald-500/80'}`}
                                    style={{ width: `${Math.min(100, Math.max(0, risk.score))}%` }}
                                  />
                                </div>
                              </div>
                            ) : (
                              '—'
                            )}
                          </td>
                        )}
                        {!effectiveClusterId && riskFindingsCols.nsCluster && (
                          <td className="px-4 py-3 text-slate-400 hidden lg:table-cell max-w-[160px]" title={`ns: ${risk.affectedResources?.[0]?.namespace ?? '—'} · cl: ${risk.clusterName ?? (risk.clusterId ? clusterLabelById.get(risk.clusterId) ?? risk.clusterId : '—')}`}>
                            <div className="text-xs text-slate-200 truncate">
                              ns: {risk.affectedResources?.[0]?.namespace ?? '—'}
                            </div>
                            <div className="text-[10px] text-slate-500 truncate">
                              cl: {risk.clusterName ?? (risk.clusterId ? clusterLabelById.get(risk.clusterId) ?? risk.clusterId : '—')}
                            </div>
                          </td>
                        )}
                        <td className="px-4 py-3">
                          <span className="uppercase px-2 py-0.5 bg-slate-800 rounded text-xs text-slate-300">
                            {statusLabelMap[risk.status || ''] ?? (risk.status || 'Unknown')}
                          </span>
                        </td>
                        {riskFindingsCols.detected && (
                          <td className="px-4 py-3 text-slate-500 hidden md:table-cell text-xs">
                            {risk.timestamp ? new Date(risk.timestamp).toLocaleDateString() : '—'}
                          </td>
                        )}
                        {riskFindingsCols.updated && (
                          <td className="px-4 py-3 text-slate-500 hidden lg:table-cell text-xs">
                            {risk.updatedAt ? new Date(risk.updatedAt).toLocaleDateString() : '—'}
                          </td>
                        )}
                        <td className="px-4 py-3 text-right" onClick={(e) => e.stopPropagation()}>
                          <button
                            onClick={() => setSelectedRisk(risk)}
                            className="text-pink-400 hover:text-pink-300 text-sm font-medium mr-2"
                          >
                            Quick view
                          </button>
                          <Button
                            size="sm"
                            variant="secondary"
                            onClick={() => {
                              const podUid = risk.affectedResources?.find((r) => r.kind === 'Pod' && r.id)?.id;
                              const q = new URLSearchParams();
                              q.set('insightId', risk.id);
                              if (podUid) q.set('podUid', podUid);
                              if (risk.clusterId) q.set('clusterId', risk.clusterId);
                              navigate(`/attack-paths?${q.toString()}`);
                            }}
                          >
                            Attack path
                          </Button>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>
          {(paginatedRisks.length > 0 || risksTotal > 0) && (
            <Pagination
              page={risksPage}
              pageSize={risksPageSize}
              total={risksTotal}
              onPageChange={setRisksPage}
              onPageSizeChange={(size) => { setRisksPageSize(size); setRisksPage(1); }}
              pageSizeOptions={[10, 20, 50, 100]}
              itemLabel="findings"
            />
          )}
            </>
          )}
        </div>
      )}

      {/* ----- PCE tab ----- */}
      {activeTab === 'pce' && (
        <div className="space-y-6">
          <div className="flex items-center justify-between gap-4 flex-wrap">
            <h1 className="text-lg font-semibold text-slate-200">Capability Exposure</h1>
            {pceHeatmap.length > 0 && (
              <Button
                variant="secondary"
                size="sm"
                onClick={() => {
                  const header = 'namespace,severity,count';
                  const rows = pceHeatmap.map((r) => `"${String(r.namespace ?? '').replace(/"/g, '""')}","${String(r.severity ?? '').replace(/"/g, '""')}",${r.count}`);
                  const csv = [header, ...rows].join('\n');
                  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' });
                  const url = URL.createObjectURL(blob);
                  const a = document.createElement('a');
                  a.href = url;
                  a.download = `pce-heatmap-${new Date().toISOString().slice(0, 10)}.csv`;
                  a.click();
                  URL.revokeObjectURL(url);
                }}
                title="Export heatmap data (namespace, severity, count) as CSV"
              >
                Export Heatmap
              </Button>
            )}
          </div>
          {/* Capability exposure summary: capability counts, not risk counts */}
          <div className="bg-surface border border-border p-4 rounded-lg">
            <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-1">
              Capability Exposure Summary
            </h2>
            <p className="text-xs text-slate-500 mb-3">Counts by severity from pod capabilities. These numbers are separate from &quot;Total findings&quot; (which counts Risk Findings only).</p>
            {pceSummary.length === 0 ? (
              <p className="text-sm text-slate-500">No capability exposure data available for this scope.</p>
            ) : (
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                {pceSummary.map((row) => (
                  <div key={row.severity} className="bg-slate-950 border border-slate-800 rounded p-3 flex items-center justify-between">
                    <span className={`text-xs uppercase ${getSeverityTextClass(row.severity)}`}>{row.severity}</span>
                    <span className="text-sm font-bold text-slate-200">{row.count}</span>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Capability exposure trend */}
          {pceTrend.length > 0 && (
            <div className="bg-surface border border-border p-4 rounded-lg">
              <div className="flex flex-wrap items-center justify-between gap-2 mb-2">
                <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-wider">Capability Exposure Trend (Last {pceTrendDays === 1 ? '24h' : `${pceTrendDays}d`})</h2>
                <div className="flex items-center gap-1 text-[11px] text-slate-400">
                  <span>Range:</span>
                  {[1, 7, 30].map((d) => (
                    <button
                      key={d}
                      type="button"
                      onClick={() => setPceTrendDays(d)}
                      className={`px-2 py-0.5 rounded-full border ${
                        pceTrendDays === d
                          ? 'border-pink-500 text-pink-300 bg-pink-500/10'
                          : 'border-slate-700 text-slate-400 hover:border-slate-500'
                      }`}
                    >
                      {d === 1 ? '24h' : `${d}d`}
                    </button>
                  ))}
                </div>
              </div>
              <p className="text-xs text-slate-500 mb-3">Capability exposure counts by day. Same scope as current cluster filter.</p>
              <div className="w-full h-[220px]">
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={pceTrend.map((p) => ({ ...p, total: (p.critical ?? 0) + (p.high ?? 0) + (p.medium ?? 0) + (p.low ?? 0) }))}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
                    <XAxis dataKey="date" tick={{ fill: '#94a3b8', fontSize: 11 }} />
                    <YAxis tick={{ fill: '#94a3b8', fontSize: 11 }} />
                    <Tooltip contentStyle={{ backgroundColor: '#1e293b', border: '1px solid #475569' }} labelStyle={{ color: '#cbd5e1' }} />
                    <Area type="monotone" dataKey="critical" stackId="1" stroke="#dc2626" fill="#dc2626" fillOpacity={0.6} name="Critical" />
                    <Area type="monotone" dataKey="high" stackId="1" stroke="#ea580c" fill="#ea580c" fillOpacity={0.6} name="High" />
                    <Area type="monotone" dataKey="medium" stackId="1" stroke="#ca8a04" fill="#ca8a04" fillOpacity={0.6} name="Medium" />
                    <Area type="monotone" dataKey="low" stackId="1" stroke="#16a34a" fill="#16a34a" fillOpacity={0.6} name="Low" />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            </div>
          )}

          {/* PCE Heatmap: namespace × severity – click cell to filter drill-down table (Phase 3) */}
          {pceHeatmap.length > 0 && (() => {
            const byNs: Record<string, { critical: number; high: number; medium: number; low: number }> = {};
            pceHeatmap.forEach((r) => {
              if (!byNs[r.namespace]) byNs[r.namespace] = { critical: 0, high: 0, medium: 0, low: 0 };
              const sev = (r.severity || '').toLowerCase();
              if (sev in byNs[r.namespace]) (byNs[r.namespace] as Record<string, number>)[sev] = r.count;
            });
            const namespaces = Object.keys(byNs).sort();
            const maxCount = Math.max(1, ...namespaces.flatMap((ns) => Object.values(byNs[ns])));
            const handleHeatmapCellClick = async (ns: string, severity: string) => {
              setPceHeatmapFilter({ namespace: ns, severity });
              setPceNamespace(ns);
              setPceSeverityFilter(severity);
              setPceListPage(1);
              const merged =
                (pceClusterId || '').trim() ||
                effectiveClusterId ||
                selectedClusterId ||
                undefined;
              const res = await api.getPceCapabilities({
                clusterId: merged,
                namespace: ns,
                severity,
                limit: pceListPageSize,
                offset: 0,
              });
              setPceDetails(res.capabilities);
              setPceListTotal(res.total);
            };
            return (
              <div className="bg-surface border border-border p-4 rounded-lg">
                <div className="flex flex-wrap items-center justify-between gap-2 mb-2">
                  <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-wider">Exposure by Namespace (heatmap)</h2>
                  {namespaces.length > 30 && (
                    <Button variant="secondary" size="sm" type="button" onClick={() => setHeatmapShowAll((v) => !v)}>
                      {heatmapShowAll ? 'Show first 30 only' : `Show all (${namespaces.length})`}
                    </Button>
                  )}
                </div>
                <p className="text-xs text-slate-500 mb-3">Click a cell with count &gt; 0 to filter the table below by that namespace and severity.</p>
                <div className="overflow-x-auto max-h-[min(70vh,520px)] overflow-y-auto rounded-lg border border-slate-800">
                  <table className="w-full text-sm">
                    <thead className="sticky top-0 z-[1]">
                      <tr className={TABLE_HEAD_ROW}>
                        <th className="text-left py-2 px-3">Namespace</th>
                        <th className="text-right py-2 px-2 text-red-400 font-medium">Critical</th>
                        <th className="text-right py-2 px-2 text-orange-400 font-medium">High</th>
                        <th className="text-right py-2 px-2 text-yellow-500 font-medium">Medium</th>
                        <th className="text-right py-2 px-2 text-green-400 font-medium">Low</th>
                      </tr>
                    </thead>
                    <tbody>
                      {(heatmapShowAll ? namespaces : namespaces.slice(0, 30)).map((ns) => {
                        const row = byNs[ns];
                        const td = (n: number, sev: string, rgba: string) => (
                          <td
                            key={sev}
                            className={`text-right py-1.5 px-2 rounded ${n > 0 ? 'cursor-pointer hover:ring-2 hover:ring-inset hover:ring-white/50' : 'cursor-default opacity-50'}`}
                            style={{ backgroundColor: `rgba(${rgba},${0.15 + (n / maxCount) * 0.75})` }}
                            onClick={() => n > 0 && handleHeatmapCellClick(ns, sev)}
                            title={n > 0 ? `Filter table by ${ns}, ${sev}` : 'No exposures in this cell'}
                          >
                            {n}
                          </td>
                        );
                        return (
                          <tr key={ns} className={TABLE_BODY_ROW}>
                            <td className="py-1.5 px-3 text-slate-200 font-mono text-xs">{ns}</td>
                            {td(row.critical, 'critical', '220,38,38')}
                            {td(row.high, 'high', '234,88,12')}
                            {td(row.medium, 'medium', '202,138,4')}
                            {td(row.low, 'low', '22,163,74')}
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                  {!heatmapShowAll && namespaces.length > 30 && (
                    <p className="text-xs text-slate-500 mt-2">Showing first 30 of {namespaces.length} namespaces. Use &quot;Show all&quot; to expand.</p>
                  )}
                </div>
              </div>
            );
          })()}

          {/* PCE Drill-down */}
          <div className="bg-surface border border-border p-4 rounded-lg">
            <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-2">
              Capability Drill-down by Pod
            </h2>
            <p className="text-xs text-slate-500 mb-2">
              Explore pod-level capabilities that may explain risk findings. Click a heatmap cell above to filter by namespace and severity, or use the filters below.
            </p>
            {pceHeatmapFilter && (
              <div className="mb-3 flex items-center gap-2 flex-wrap">
                <span className="text-xs text-slate-400">Filtered by namespace <strong className="text-slate-200">{pceHeatmapFilter.namespace}</strong>, severity <strong className="text-slate-200">{pceHeatmapFilter.severity}</strong></span>
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={async () => {
                    setPceHeatmapFilter(null);
                    setPceNamespace('');
                    setPceSeverityFilter('');
                    setPceListPage(1);
                    const merged =
                      (pceClusterId || '').trim() ||
                      effectiveClusterId ||
                      selectedClusterId ||
                      undefined;
                    const res = await api.getPceCapabilities({
                      clusterId: merged,
                      limit: pceListPageSize,
                      offset: 0,
                    });
                    setPceDetails(res.capabilities);
                    setPceListTotal(res.total);
                  }}
                >
                  Clear filter
                </Button>
              </div>
            )}
            <div className="grid grid-cols-1 md:grid-cols-6 gap-3 mb-4">
              <div>
                <label className="block text-xs text-slate-500 mb-1">Cluster</label>
                <select
                  value={pceClusterId}
                  onChange={(e) => setPceClusterId(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded px-3 py-2 text-sm text-slate-200"
                >
                  <option value="">All Clusters</option>
                  {clusters.map((c) => (
                    <option key={c.id} value={c.id}>{c.name || c.id}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-xs text-slate-500 mb-1">Namespace</label>
                <input
                  value={pceNamespace}
                  onChange={(e) => setPceNamespace(e.target.value)}
                  placeholder="e.g. default"
                  className="w-full bg-slate-950 border border-slate-800 rounded px-3 py-2 text-sm text-slate-200"
                />
              </div>
              <div>
                <label className="block text-xs text-slate-500 mb-1">Severity</label>
                <select
                  value={pceSeverityFilter}
                  onChange={(e) => setPceSeverityFilter(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded px-3 py-2 text-sm text-slate-200"
                >
                  <option value="">All</option>
                  <option value="critical">Critical</option>
                  <option value="high">High</option>
                  <option value="medium">Medium</option>
                  <option value="low">Low</option>
                </select>
              </div>
              <div>
                <label className="block text-xs text-slate-500 mb-1">Pod name</label>
                <input
                  value={pcePodName}
                  onChange={(e) => setPcePodName(e.target.value)}
                  placeholder="e.g. my-pod"
                  className="w-full bg-slate-950 border border-slate-800 rounded px-3 py-2 text-sm text-slate-200"
                />
              </div>
              <div>
                <label className="block text-xs text-slate-500 mb-1">Capability ID</label>
                <input
                  value={pceCapabilityId}
                  onChange={(e) => setPceCapabilityId(e.target.value)}
                  placeholder="e.g. ESC_PRIV_POD"
                  className="w-full bg-slate-950 border border-slate-800 rounded px-3 py-2 text-sm text-slate-200"
                />
              </div>
              <div className="flex items-end gap-2">
                <Button
                  size="sm"
                  onClick={async () => {
                    setPceListPage(1);
                    const merged =
                      (pceClusterId || '').trim() ||
                      effectiveClusterId ||
                      selectedClusterId ||
                      undefined;
                    const res = await api.getPceCapabilities({
                      clusterId: merged,
                      namespace: pceNamespace.trim() || undefined,
                      severity: pceSeverityFilter || undefined,
                      podName: pcePodName.trim() || undefined,
                      capabilityId: pceCapabilityId.trim() || undefined,
                      limit: pceListPageSize,
                      offset: 0,
                    });
                    setPceDetails(res.capabilities);
                    setPceListTotal(res.total);
                  }}
                >
                  Apply &amp; search
                </Button>
              </div>
            </div>
            <div className="mb-3 flex flex-wrap items-center gap-2">
              <label className="text-xs text-slate-500 uppercase tracking-wider">Sort:</label>
              <select
                value={pceSort}
                onChange={(e) => setPceSort(e.target.value as typeof pceSort)}
                className="bg-slate-950 border border-slate-800 rounded px-3 py-1.5 text-sm text-slate-300 focus:outline-none focus:border-pink-500"
              >
                <option value="severity_desc">Severity high to low</option>
                <option value="severity_asc">Severity low to high</option>
                <option value="capability_asc">Capability A-Z</option>
                <option value="pod_asc">Pod name A-Z</option>
                <option value="namespace_asc">Namespace A-Z</option>
              </select>
              <span className="text-[10px] text-slate-500">Sort applies to the current page of results.</span>
            </div>
            <div className="ui-table-scroll rounded-lg border border-border bg-surface">
              <table className="w-full text-sm">
                <thead className={`${TABLE_HEAD_ROW} sticky top-0 z-10`}>
                  <tr>
                    <th className="text-left px-3 py-2">Pod Name</th>
                    <th className="text-left px-3 py-2 hidden lg:table-cell">Pod UID</th>
                    <th className="text-left px-3 py-2">Namespace</th>
                    <th className="text-left px-3 py-2">Capability</th>
                    <th className="text-left px-3 py-2">Severity</th>
                    <th className="text-left px-3 py-2 max-w-[140px]">Evidence</th>
                    <th className="text-left px-3 py-2 whitespace-nowrap">Last Seen</th>
                  </tr>
                </thead>
                <tbody>
                  {pceDetails.length === 0 ? (
                    <tr>
                      <td colSpan={7} className="px-3 py-4 text-center text-slate-500">No matching capability records. Adjust filters and run again.</td>
                    </tr>
                  ) : (
                    sortedPceDetails.map((row) => {
                      const evSum = summarizePceEvidence(row.evidence);
                      const lastSeen = row.lastSeenAt ? (() => {
                        try {
                          const d = new Date(row.lastSeenAt);
                          const now = Date.now();
                          const diffMs = now - d.getTime();
                          if (diffMs < 60000) return 'Just now';
                          if (diffMs < 3600000) return `${Math.floor(diffMs / 60000)}m ago`;
                          if (diffMs < 86400000) return `${Math.floor(diffMs / 3600000)}h ago`;
                          if (diffMs < 604800000) return `${Math.floor(diffMs / 86400000)}d ago`;
                          return d.toLocaleDateString();
                        } catch {
                          return row.lastSeenAt;
                        }
                      })() : (row.updatedAt ? (() => {
                        try {
                          const d = new Date(row.updatedAt);
                          return d.toLocaleDateString();
                        } catch {
                          return '—';
                        }
                      })() : '—');
                      return (
                        <tr key={`${row.podUid}-${row.capabilityId}`} className={TABLE_BODY_ROW}>
                          <td className="px-3 py-2 text-slate-200 font-medium" title={row.podUid}>{row.podName ?? '—'}</td>
                          <td className="px-3 py-2 text-slate-500 font-mono text-xs truncate max-w-[120px] hidden lg:table-cell" title={row.podUid}>{row.podUid}</td>
                          <td className="px-3 py-2 text-slate-300">{row.namespace}</td>
                          <td className="px-3 py-2 text-slate-200">{row.capabilityId}</td>
                          <td className="px-3 py-2">
                            <span className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase border ${getSeverityBadgeClass(row.severity)}`}>
                              {row.severity}
                            </span>
                          </td>
                          <td className="px-3 py-2 text-slate-400 text-xs max-w-[200px] leading-snug" title={evSum.title || undefined}>
                            {evSum.label ? <span className="line-clamp-2">{evSum.label}</span> : '—'}
                          </td>
                          <td className="px-3 py-2 text-slate-500 text-xs whitespace-nowrap" title={row.lastSeenAt || row.updatedAt || undefined}>{lastSeen}</td>
                        </tr>
                      );
                    })
                  )}
                </tbody>
              </table>
            </div>
            {(pceListTotal > 0 || pceDetails.length > 0) && (
              <Pagination
                page={pceListPage}
                pageSize={pceListPageSize}
                total={pceListTotal}
                onPageChange={setPceListPage}
                onPageSizeChange={(size) => {
                  setPceListPageSize(size);
                  setPceListPage(1);
                }}
                pageSizeOptions={[10, 25, 50, 100]}
                itemLabel="rows"
                className="rounded-b-lg border border-t-0 border-border"
              />
            )}
          </div>
        </div>
      )}

      {/* ----- Evidence & References tab (Phase 4: Runtime Signals | Audit Trail) ----- */}
      {activeTab === 'reference' && (
        <div className="space-y-6">
          <div className="flex gap-2 border-b border-slate-800 pb-2">
            <button
              type="button"
              onClick={() => setEvidenceSubTab('signals')}
              className={`px-4 py-2 rounded-t text-sm font-medium ${evidenceSubTab === 'signals' ? 'bg-slate-800 text-slate-200' : 'text-slate-500 hover:text-slate-300'}`}
            >
              Runtime Signals
            </button>
            <button
              type="button"
              onClick={() => setEvidenceSubTab('audit')}
              className={`px-4 py-2 rounded-t text-sm font-medium ${evidenceSubTab === 'audit' ? 'bg-slate-800 text-slate-200' : 'text-slate-500 hover:text-slate-300'}`}
            >
              Audit Trail
            </button>
          </div>
          {evidenceSubTab === 'signals' && (
            <div className="space-y-8">
              <div className="bg-slate-900 border border-slate-800 p-6 rounded-lg">
                <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-1 flex items-center gap-2">
                  <AlertTriangle size={16} className="text-pink-400" />
                  Runtime Evidence
                </h2>
                <p className="text-xs text-slate-500 mb-4">Pod runtime events used as supporting evidence for risk investigation.</p>
                <RuntimeSignalsTable podUid={podUidFromEvidenceUrl?.trim() || undefined} />
              </div>
              <div className="bg-slate-900 border border-slate-800 p-6 rounded-lg">
                <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-1 flex items-center gap-2">
                  <Shield size={16} className="text-pink-400" />
                  Capability Knowledge Reference
                </h2>
                <p className="text-xs text-slate-500 mb-4">Definitions and context: capability meaning, severity base, MITRE mapping, and mitigations.</p>
                <Link
                  to="/capabilities"
                  className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-pink-500/20 text-pink-400 border border-pink-500/50 hover:bg-pink-500/30 transition-colors text-sm font-medium"
                >
                  Open Capability Knowledge
                  <ArrowRight size={16} />
                </Link>
              </div>
            </div>
          )}
          {evidenceSubTab === 'audit' && (
            <div className="bg-slate-900 border border-slate-800 p-6 rounded-lg">
              <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-2">Audit Trail (insight actions)</h2>
              <p className="text-xs text-slate-500 mb-4">User actions on risk findings (view, acknowledge, resolve, dismiss).</p>
              {evidenceAuditLoading ? (
                <div className="py-8 text-slate-500 text-sm">Loading audit logs...</div>
              ) : evidenceAuditLogs.length === 0 ? (
                <p className="text-slate-500 text-sm py-4">No audit logs for insights yet.</p>
              ) : (
                <>
                  <div className="ui-table-scroll rounded-lg border border-border bg-surface">
                    <table className="w-full text-sm">
                      <thead className={`${TABLE_HEAD_ROW} sticky top-0 z-10`}>
                        <tr>
                          <th className="text-left px-3 py-2 font-medium">User</th>
                          <th className="text-left px-3 py-2 font-medium">Action</th>
                          <th className="text-left px-3 py-2 font-medium">Finding ID</th>
                          <th className="text-left px-3 py-2 font-medium">Timestamp</th>
                          <th className="text-left px-3 py-2 font-medium">IP</th>
                        </tr>
                      </thead>
                      <tbody>
                        {evidenceAuditLogs.map((log) => (
                          <tr key={log.id} className={TABLE_BODY_ROW}>
                            <td className="px-3 py-2 text-text">{log.actor ?? log.user ?? '—'}</td>
                            <td className="px-3 py-2 text-muted">{log.action}</td>
                            <td className="px-3 py-2 font-mono text-xs text-muted">{log.resourceId ?? '—'}</td>
                            <td className="px-3 py-2 text-muted text-xs whitespace-nowrap">
                              {log.timestamp
                                ? (() => {
                                    try {
                                      return new Date(log.timestamp).toLocaleString();
                                    } catch {
                                      return log.timestamp;
                                    }
                                  })()
                                : '—'}
                            </td>
                            <td className="px-3 py-2 text-muted text-xs">{log.ip ?? '—'}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                  {evidenceAuditTotal > 20 && (
                    <div className="mt-3 flex items-center gap-2 text-xs text-slate-500">
                      <span>Page {evidenceAuditPage} of {Math.ceil(evidenceAuditTotal / 20)}</span>
                      <Button variant="secondary" size="sm" disabled={evidenceAuditPage <= 1} onClick={() => setEvidenceAuditPage((p) => p - 1)}>Previous</Button>
                      <Button variant="secondary" size="sm" disabled={evidenceAuditPage >= Math.ceil(evidenceAuditTotal / 20)} onClick={() => setEvidenceAuditPage((p) => p + 1)}>Next</Button>
                    </div>
                  )}
                </>
              )}
            </div>
          )}
        </div>
      )}
      </div>

      {/* Risk Detail Drawer (spec §4): right-side contextual panel, no navigation away */}
      {selectedRisk && (
        <>
          <div className="fixed inset-0 bg-black/40 z-40" onClick={() => setSelectedRisk(null)} aria-hidden="true" />
          <div
            ref={drawerRef}
            className="fixed top-0 right-0 bottom-0 w-full max-w-lg bg-slate-900 border-l border-slate-800 shadow-xl z-50 overflow-y-auto flex flex-col"
            role="dialog"
            aria-modal="true"
            aria-labelledby="risk-drawer-title"
          >
            <div className="p-4 border-b border-slate-800 flex items-start justify-between shrink-0">
              <div className="min-w-0 pr-4">
                <div className="flex items-center gap-2 mb-1">
                  {getSeverityIcon(selectedRisk.severity)}
                  <h2 id="risk-drawer-title" className="text-lg font-bold text-white truncate">{selectedRisk.title}</h2>
                </div>
                <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-slate-500">
                  <span>
                    Score:{' '}
                    <span className="text-slate-300">
                      {selectedRisk.score ?? '—'}/100
                    </span>
                  </span>
                  <span>
                    Exploitability:{' '}
                    <span className="text-slate-300">
                      {selectedRisk.exploitabilityScore != null ? selectedRisk.exploitabilityScore.toFixed(1) : '—'}
                    </span>
                  </span>
                  <span>
                    Business impact:{' '}
                    <span className="text-slate-300">
                      {selectedRisk.businessImpactScore != null ? selectedRisk.businessImpactScore.toFixed(1) : '—'}
                    </span>
                  </span>
                  <span>
                    Time decay:{' '}
                    <span className="text-slate-300">
                      {selectedRisk.timeDecay != null ? selectedRisk.timeDecay.toFixed(2) : '—'}
                    </span>
                  </span>
                  <span>
                    Source:{' '}
                    {selectedRisk.insightType === 'vulnerability' || selectedRisk.insightType === 'supply_chain_malware'
                      ? 'Static scan'
                      : 'Runtime behavior'}
                  </span>
                  <span>
                    Workflow:{' '}
                    <span className="uppercase">
                      {statusLabelMap[selectedRisk.status || ''] ?? selectedRisk.status}
                    </span>
                  </span>
                </div>
              </div>
              <button onClick={() => setSelectedRisk(null)} className="p-1 text-slate-400 hover:text-white rounded" aria-label="Close"><X size={20} /></button>
            </div>
            <div className="p-4 space-y-5 flex-1">
              {selectedRiskLoading && (
                <div className="flex items-center gap-2 text-xs text-slate-500">
                  <Loader2 className="w-4 h-4 animate-spin" />
                  Loading linked risk, capability exposure, and evidence context...
                </div>
              )}
              {/* Drawer tabs */}
              <div className="border-b border-slate-800 mb-4">
                <nav className="flex gap-2 text-xs">
                  <button
                    type="button"
                    onClick={() => setDrawerTab('summary')}
                    className={`px-3 py-1.5 rounded-t-lg border-b-2 ${
                      drawerTab === 'summary'
                        ? 'border-pink-500 text-pink-400'
                        : 'border-transparent text-slate-400 hover:text-slate-200'
                    }`}
                  >
                    Summary
                  </button>
                  <button
                    type="button"
                    onClick={() => setDrawerTab('evidence')}
                    className={`px-3 py-1.5 rounded-t-lg border-b-2 ${
                      drawerTab === 'evidence'
                        ? 'border-pink-500 text-pink-400'
                        : 'border-transparent text-slate-400 hover:text-slate-200'
                    }`}
                  >
                    Evidence &amp; Audit
                  </button>
                  <button
                    type="button"
                    onClick={() => setDrawerTab('pce')}
                    className={`px-3 py-1.5 rounded-t-lg border-b-2 ${
                      drawerTab === 'pce'
                        ? 'border-pink-500 text-pink-400'
                        : 'border-transparent text-slate-400 hover:text-slate-200'
                    }`}
                  >
                    Capability Exposure &amp; Attack Path
                  </button>
                </nav>
              </div>

              {/* Summary tab */}
              {drawerTab === 'summary' && (
                <>
                  <section>
                    <div className="text-xs text-slate-400 bg-slate-950 border border-slate-800 rounded px-3 py-2">
                      Relationship: <span className="text-slate-200">Risk finding</span> → <span className="text-slate-200">affected Pod</span> → <span className="text-slate-200">capabilities</span> → <span className="text-slate-200">runtime evidence &amp; references</span>.
                    </div>
                  </section>
                  <section className="mt-4">
                    <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Finding Summary</h3>
                    <p className="text-slate-300 text-sm leading-relaxed">
                      {selectedRiskDetail?.riskExplanation ||
                        selectedRiskDetail?.description ||
                        selectedRisk.description ||
                        '—'}
                    </p>
                    <div className="mt-2 text-xs text-slate-500">
                      Severity: {selectedRisk.severity} · First detected:{' '}
                      {selectedRisk.timestamp ? new Date(selectedRisk.timestamp).toLocaleString() : '—'}
                    </div>
                    <div className="mt-3 grid grid-cols-2 gap-2 text-xs">
                      <div className="bg-slate-950 border border-slate-800 rounded px-2 py-1.5 text-slate-400">
                        Runtime evidence
                        <div className="text-slate-200 font-semibold">{selectedRiskSignals.length}</div>
                      </div>
                      <div className="bg-slate-950 border border-slate-800 rounded px-2 py-1.5 text-slate-400">
                        Linked capabilities
                        <div className="text-slate-200 font-semibold">{selectedRiskCapabilities.length}</div>
                      </div>
                    </div>
                  </section>
                  <section className="mt-4">
                    <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Recommended Actions</h3>
                    {selectedRiskDetail?.remediation ? (
                      <div className="space-y-1.5 text-xs text-slate-300">
                        {Array.isArray(selectedRiskDetail.remediation)
                          ? selectedRiskDetail.remediation.map((item, idx) => (
                              <div
                                key={idx}
                                className="flex items-start gap-2 bg-slate-950 border border-slate-800 rounded px-2 py-1.5"
                              >
                                <span className="mt-0.5 h-1.5 w-1.5 rounded-full bg-pink-400 shrink-0" />
                                <span className="whitespace-pre-line">
                                  {typeof item === 'string' ? item : JSON.stringify(item)}
                                </span>
                              </div>
                            ))
                          : (
                              <div className="bg-slate-950 border border-slate-800 rounded px-2 py-1.5 whitespace-pre-line">
                                {typeof selectedRiskDetail.remediation === 'string'
                                  ? selectedRiskDetail.remediation
                                  : JSON.stringify(selectedRiskDetail.remediation, null, 2)}
                              </div>
                            )}
                      </div>
                    ) : (
                      <p className="text-slate-500 text-xs">
                        No structured remediation steps available yet. Use recommendation field or linked rules for guidance.
                      </p>
                    )}
                  </section>
                  <section className="mt-4">
                    <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Impacted Resources</h3>
                    <div className="flex flex-wrap gap-2">
                      {selectedRisk.affectedResources?.length ? selectedRisk.affectedResources.map((res, idx) => (
                        <div key={idx} className="flex items-center bg-slate-950 border border-slate-700 rounded px-3 py-2 text-sm text-slate-300">
                          {res.kind === 'Pod' && <Box className="w-4 h-4 mr-2 text-blue-400" />}
                          {res.kind === 'ServiceAccount' && <User className="w-4 h-4 mr-2 text-green-400" />}
                          <span className="font-medium">{res.name}</span>
                          <span className="ml-2 text-xs text-slate-500">({res.kind})</span>
                          {res.namespace && <span className="ml-2 text-xs text-slate-500">{res.namespace}</span>}
                        </div>
                      )) : <span className="text-slate-500 text-sm">—</span>}
                    </div>
                  </section>
                  <section className="mt-4">
                    <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Context Links</h3>
                    <div className="flex flex-wrap gap-2 text-xs">
                      {selectedRiskContext?.pods?.length ? (
                        <button
                          type="button"
                          onClick={() => {
                            const podUid = selectedRiskContext.pods[0]?.uid || selectedRisk.affectedResources?.find(r => r.kind === 'Pod')?.id;
                            if (podUid) {
                              setSelectedRisk(null);
                              navigate(`/resources/pods/uid/${encodeURIComponent(String(podUid))}`);
                            }
                          }}
                          className="inline-flex items-center px-2 py-1 rounded bg-slate-950 border border-slate-700 text-slate-200 hover:border-pink-500"
                        >
                          <Box className="w-3 h-3 mr-1 text-blue-400" />
                          View pod in Resources
                        </button>
                      ) : null}
                      {selectedRiskContext?.cluster && selectedRiskContext.cluster.id && (
                        <button
                          type="button"
                          onClick={() => {
                            setSelectedRisk(null);
                            navigate(`/clusters/${encodeURIComponent(String(selectedRiskContext.cluster!.id))}`);
                          }}
                          className="inline-flex items-center px-2 py-1 rounded bg-slate-950 border border-slate-700 text-slate-200 hover:border-pink-500"
                        >
                          <span className="w-3 h-3 mr-1 rounded-full bg-slate-500" />
                          View cluster
                        </button>
                      )}
                      {selectedRiskContext?.rules?.length
                        ? selectedRiskContext.rules.slice(0, 3).map((rule: any) => (
                            <button
                              key={rule.ruleId || rule.id}
                              type="button"
                              onClick={() => {
                                if (!rule.ruleId) return;
                                setSelectedRisk(null);
                                navigate(`/rules/${encodeURIComponent(String(rule.ruleId))}`);
                              }}
                              className="inline-flex items-center px-2 py-1 rounded bg-slate-950 border border-slate-700 text-slate-200 hover:border-pink-500"
                            >
                              <span className="w-3 h-3 mr-1 rounded-full bg-amber-500" />
                              Rule {rule.ruleId}
                            </button>
                          ))
                        : (
                            <span className="text-slate-500">
                              No additional context links available yet.
                            </span>
                          )}
                    </div>
                  </section>
                </>
              )}

              {/* Evidence & Audit tab */}
              {drawerTab === 'evidence' && (
                <>
                  <section>
                    <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Runtime Evidence</h3>
                    {selectedRiskSignals.length === 0 ? (
                      <p className="text-slate-500 text-xs">No runtime evidence in current time window.</p>
                    ) : (
                      <div className="space-y-1.5">
                        {selectedRiskSignals.slice(0, 5).map((s) => (
                          <div key={s.id} className="text-xs bg-slate-950 border border-slate-800 rounded px-2 py-1.5 text-slate-300">
                            <span className={`px-2 py-0.5 rounded text-[11px] font-semibold border mr-2 ${runtimeSignalVisual(s.signalType).signalClass}`}>
                              {s.signalType}
                            </span>
                            <span className={`px-2 py-0.5 rounded text-[10px] font-medium border mr-2 ${runtimeSignalVisual(s.signalType).severityClass}`}>
                              {runtimeSignalVisual(s.signalType).severity}
                            </span>
                            <span className="text-slate-500"> · {s.category} · {new Date(s.createdAt).toLocaleString()}</span>
                          </div>
                        ))}
                        <p className="text-[10px] text-slate-500 mt-1">
                          Showing {Math.min(5, selectedRiskSignals.length)} of {selectedRiskSignals.length} signal(s) in this window.
                        </p>
                        <button
                          type="button"
                          onClick={() => {
                            const podUid = selectedRisk.affectedResources?.find((r) => r.kind === 'Pod' && r.id)?.id;
                            setSelectedRisk(null);
                            navigate(podUid ? `/risks/evidence?podUid=${encodeURIComponent(podUid)}` : '/risks/evidence');
                          }}
                          className="text-xs text-pink-400 hover:underline"
                        >
                          Open full runtime evidence (filtered by pod when available)
                        </button>
                      </div>
                    )}
                  </section>
                  <section className="mt-4">
                    <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Audit Trail</h3>
                    {drawerAuditLoading && (
                      <div className="flex items-center gap-2 text-xs text-slate-500 mb-2">
                        <Loader2 className="w-3 h-3 animate-spin" />
                        Loading audit logs...
                      </div>
                    )}
                    {drawerAuditLogs.length === 0 ? (
                      <p className="text-slate-500 text-xs">No audit logs for this finding yet.</p>
                    ) : (
                      <div className="max-h-40 overflow-y-auto border border-slate-800 rounded">
                        <table className="w-full text-xs">
                          <thead className="sticky top-0 z-[1]">
                            <tr className={TABLE_HEAD_ROW}>
                              <th className="text-left px-2 py-1.5 font-medium">Time</th>
                              <th className="text-left px-2 py-1.5 font-medium">User</th>
                              <th className="text-left px-2 py-1.5 font-medium">Action</th>
                              <th className="text-left px-2 py-1.5 font-medium">Details</th>
                            </tr>
                          </thead>
                          <tbody>
                            {drawerAuditLogs.map((log) => (
                              <tr key={log.id} className={TABLE_BODY_ROW}>
                                <td className="px-2 py-1.5 text-slate-400 whitespace-nowrap">
                                  {log.timestamp
                                    ? (() => {
                                        try {
                                          return new Date(log.timestamp).toLocaleString();
                                        } catch {
                                          return log.timestamp;
                                        }
                                      })()
                                    : '—'}
                                </td>
                                <td className="px-2 py-1.5 text-slate-300">{log.user || log.actor || 'system'}</td>
                                <td className="px-2 py-1.5 text-slate-300">{log.action}</td>
                                <td className="px-2 py-1.5 text-slate-400 truncate max-w-[160px]" title={log.details}>{log.details}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    )}
                  </section>
                </>
              )}

              {/* Capability exposure & attack path tab */}
              {drawerTab === 'pce' && (
                <>
                  <section>
                    <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Related Capability IDs</h3>
                    {selectedRiskCapabilities.length === 0 ? (
                      <p className="text-slate-500 text-xs">No linked capability found on the affected pod.</p>
                    ) : (
                      <div className="space-y-2">
                        {Array.from(new Set(selectedRiskCapabilities.map((c) => c.capabilityId))).slice(0, 5).map((capId) => (
                          <div key={capId} className="text-xs text-slate-300 bg-slate-950 border border-slate-800 rounded px-2 py-1.5">
                            Capability: <span className="font-mono text-amber-300">{capId}</span>
                          </div>
                        ))}
                      </div>
                    )}
                  </section>
                  <section className="mt-4">
                    <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Capability Details</h3>
                    {selectedRiskCapabilities.length === 0 ? (
                      <p className="text-slate-500 text-xs">No capability context available for the currently linked asset.</p>
                    ) : (
                      <div className="max-h-40 overflow-y-auto border border-slate-800 rounded">
                        <table className="w-full text-xs">
                          <thead className="bg-slate-950 text-slate-500">
                            <tr>
                              <th className="text-left px-2 py-1.5">Capability</th>
                              <th className="text-left px-2 py-1.5">Severity</th>
                              <th className="text-left px-2 py-1.5">State</th>
                            </tr>
                          </thead>
                          <tbody>
                            {selectedRiskCapabilities.slice(0, 8).map((c, idx) => (
                              <tr key={`${c.capabilityId}-${idx}`} className="border-t border-slate-800">
                                <td className="px-2 py-1.5 text-slate-300 font-mono">{c.capabilityId}</td>
                                <td className="px-2 py-1.5 text-slate-400 uppercase">{c.severity}</td>
                                <td className="px-2 py-1.5 text-slate-400">{c.state ?? 'detected'}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    )}
                  </section>
                  <section className="mt-4">
                    <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Capability Exposure Impact</h3>
                    <p className="text-slate-500 text-xs mb-2">
                      This risk is linked to capability exposure through the affected Pod UID. Open the Capability Exposure tab for detailed filtering.
                    </p>
                    <button
                      type="button"
                      onClick={() => {
                        setSelectedRisk(null);
                        setActiveTab('pce');
                      }}
                      className="text-xs text-pink-400 hover:underline"
                    >
                      Open capability exposure tab
                    </button>
                    <div className="mt-3">
                      <Button
                        size="sm"
                        variant="secondary"
                        onClick={() => {
                          const podUid = selectedRisk.affectedResources?.find((r) => r.kind === 'Pod' && r.id)?.id;
                          const q = new URLSearchParams();
                          q.set('insightId', selectedRisk.id);
                          if (podUid) q.set('podUid', podUid);
                          if (selectedRisk.clusterId) q.set('clusterId', selectedRisk.clusterId);
                          setSelectedRisk(null);
                          navigate(`/attack-paths?${q.toString()}`);
                        }}
                      >
                        Open attack path view
                      </Button>
                    </div>
                  </section>
                  <section className="mt-4">
                    <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">External References</h3>
                    {selectedRiskReferences.length === 0 ? (
                      <p className="text-slate-500 text-xs">No external references linked from capability catalog.</p>
                    ) : (
                      <div className="space-y-1.5">
                        {selectedRiskReferences.slice(0, 5).map((url) => (
                          <a
                            key={url}
                            href={url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="flex items-center gap-1 text-xs text-pink-400 hover:underline break-all"
                          >
                            <ExternalLink className="w-3 h-3 shrink-0" />
                            {url}
                          </a>
                        ))}
                      </div>
                    )}
                  </section>
                </>
              )}

              {drawerNotice && (
                <p className="text-xs text-slate-400 border border-slate-700 rounded-lg px-3 py-2 bg-slate-950/80">{drawerNotice}</p>
              )}
              {/* Actions */}
              <section className="pt-4 mt-4 border-t border-slate-800 flex flex-wrap gap-2">
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => setSelectedRisk(null)}
                >
                  Close
                </Button>
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => { setSelectedRisk(null); navigate(`/risks/${selectedRisk.id}`); }}
                >
                  Open full detail page
                </Button>
                <Button
                  size="sm"
                  variant="secondary"
                  disabled={drawerActionBusy !== null}
                  onClick={async () => {
                    if (drawerActionBusy) return;
                    if (!window.confirm('Acknowledge this finding? It will move to In review.')) return;
                    setDrawerActionBusy('ack');
                    try {
                      await api.bulkInsightsAction({ action: 'acknowledge', insightIds: [selectedRisk.id] });
                      setDrawerNotice(null);
                      setToast({ message: 'Finding acknowledged.', variant: 'success' });
                      fetchDataRef.current();
                    } catch (e) {
                      const msg = String(e instanceof Error ? e.message : e);
                      setDrawerNotice(msg);
                      setToast({ message: msg, variant: 'error' });
                    } finally {
                      setDrawerActionBusy(null);
                    }
                  }}
                >
                  {drawerActionBusy === 'ack' ? (
                    <span className="inline-flex items-center gap-1.5">
                      <Loader2 className="w-3.5 h-3.5 animate-spin" />
                      Acknowledge
                    </span>
                  ) : (
                    'Acknowledge'
                  )}
                </Button>
                <Button
                  size="sm"
                  disabled={drawerActionBusy !== null}
                  onClick={async () => {
                    if (drawerActionBusy) return;
                    if (!window.confirm('Resolve this finding?')) return;
                    setDrawerActionBusy('resolve');
                    try {
                      await api.bulkInsightsAction({ action: 'resolve', insightIds: [selectedRisk.id] });
                      setDrawerNotice(null);
                      setToast({ message: 'Finding resolved.', variant: 'success' });
                      fetchDataRef.current();
                    } catch (e) {
                      const msg = String(e instanceof Error ? e.message : e);
                      setDrawerNotice(msg);
                      setToast({ message: msg, variant: 'error' });
                    } finally {
                      setDrawerActionBusy(null);
                    }
                  }}
                >
                  {drawerActionBusy === 'resolve' ? (
                    <span className="inline-flex items-center gap-1.5">
                      <Loader2 className="w-3.5 h-3.5 animate-spin" />
                      Resolve
                    </span>
                  ) : (
                    'Resolve'
                  )}
                </Button>
                <Button
                  size="sm"
                  variant="secondary"
                  disabled={drawerActionBusy !== null}
                  onClick={async () => {
                    if (drawerActionBusy) return;
                    if (!window.confirm('Dismiss this finding?')) return;
                    setDrawerActionBusy('dismiss');
                    try {
                      await api.bulkInsightsAction({ action: 'dismiss', insightIds: [selectedRisk.id] });
                      setDrawerNotice(null);
                      setToast({ message: 'Finding dismissed.', variant: 'success' });
                      fetchDataRef.current();
                    } catch (e) {
                      const msg = String(e instanceof Error ? e.message : e);
                      setDrawerNotice(msg);
                      setToast({ message: msg, variant: 'error' });
                    } finally {
                      setDrawerActionBusy(null);
                    }
                  }}
                >
                  {drawerActionBusy === 'dismiss' ? (
                    <span className="inline-flex items-center gap-1.5">
                      <Loader2 className="w-3.5 h-3.5 animate-spin" />
                      Dismiss
                    </span>
                  ) : (
                    'Dismiss'
                  )}
                </Button>
              </section>
            </div>
          </div>
        </>
      )}
      {toast && (
        <div
          role="status"
          className={`fixed top-4 right-4 z-[60] max-w-sm px-4 py-3 rounded-lg border text-sm shadow-xl ${
            toast.variant === 'success'
              ? 'bg-emerald-950/95 border-emerald-600/50 text-emerald-100'
              : 'bg-red-950/95 border-red-600/50 text-red-100'
          }`}
        >
          {toast.message}
        </div>
      )}
    </PageLayout>
  );
};

