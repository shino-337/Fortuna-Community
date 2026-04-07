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
import { PageLoading } from '../components/PageLoading';
import { RiskHistogram } from '../components/RiskHistogram';
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import type { RiskHistogramResponse } from '../types';
import { riskListSecondaryLabel } from '../lib/riskDisplay';

type TabId = 'risks' | 'pce' | 'reference';

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
  const [loading, setLoading] = useState(true);
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

  const runtimeSignalVisual = (signalType: string): { signalClass: string; severity: string; severityClass: string } => {
    const t = (signalType || '').trim().toUpperCase();
    if (t === 'NETWORK_QUEUE_ANOMALY') {
      return {
        signalClass: 'bg-cyan-500/20 text-cyan-300 border-cyan-500/40',
        severity: 'MEDIUM',
        severityClass: 'bg-amber-500/20 text-amber-300 border-amber-500/40',
      };
    }
    if (t === 'SUSPICIOUS_EXEC_FROM_SNAPSHOT') {
      return {
        signalClass: 'bg-orange-500/20 text-orange-300 border-orange-500/40',
        severity: 'HIGH',
        severityClass: 'bg-red-500/20 text-red-300 border-red-500/40',
      };
    }
    return {
      signalClass: 'bg-slate-500/20 text-slate-300 border-slate-500/40',
      severity: 'INFO',
      severityClass: 'bg-slate-500/20 text-slate-300 border-slate-500/40',
    };
  };
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

  /** Since minutes for API: if a chart date is selected, use from start of that day to now; else use URL/store. */
  const sinceMinutesForApi = useMemo(() => {
    if (selectedChartDate) {
      const start = new Date(selectedChartDate + 'T00:00:00Z').getTime();
      const now = Date.now();
      return Math.max(0, Math.floor((now - start) / 60000));
    }
    return effectiveSinceMinutesNum ?? (timeWindowMinutes > 0 ? timeWindowMinutes : undefined);
  }, [selectedChartDate, effectiveSinceMinutesNum, timeWindowMinutes]);

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

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    const sinceMinutes = sinceMinutesForApi;
    const clusterId = effectiveClusterId ?? selectedClusterId ?? undefined;
    const useScores =
      riskSort === 'score_desc' ||
      riskSort === 'score_asc' ||
      riskSort === 'exploitability_desc' ||
      riskSort === 'exploitability_asc' ||
      priorityLevel !== '';
    const risksPromise = api.getRisks({
      page: risksPage,
      pageSize: risksPageSize,
      severity: filter !== 'all' ? filter : undefined,
      status: statusFilter,
      search: searchTerm.trim() || undefined,
      clusterId: clusterId ?? undefined,
      namespace: namespaceFilter.trim() || undefined,
      type: typeFilter || undefined,
      sinceMinutes,
      withScores: useScores ? 1 : undefined,
      priorityLevel: priorityLevel || undefined,
      scoreBin: selectedScoreBin ?? undefined,
    });
    const summaryPromise = api.getInsightsSummary(clusterId ?? undefined, sinceMinutes);
    const threatPromise = api.getThreatVelocity(trendDays, clusterId ?? undefined).catch(() => []);
    const pceSummaryPromise = api.getPceSummaryBySeverity();
    const pceListPromise = api.getPceCapabilities({ limit: 50, clusterId: clusterId ?? undefined });
    const pceTrendPromise = api.getPceTrend(7, { clusterId: clusterId ?? undefined });
    const pceHeatmapPromise = api.getPceSummaryByNamespace({ clusterId: clusterId ?? undefined });
    const clustersPromise = api.getClusters();
    const statsPromise = api.getStats(clusterId ?? undefined, sinceMinutes, 'all');
    const byClusterPromise = !clusterId ? api.getInsightsSummaryByCluster(sinceMinutes) : Promise.resolve([] as InsightsSummaryByClusterItem[]);
    setHistogramLoading(true);
    const histogramPromise = api.getRiskHistogram({ clusterId: clusterId ?? undefined, sinceMinutes }).then((r) => { setHistogramData(r); return r; }).finally(() => setHistogramLoading(false));
    const results = await Promise.allSettled([
      risksPromise,
      summaryPromise,
      threatPromise,
      pceSummaryPromise,
      pceListPromise,
      pceTrendPromise,
      pceHeatmapPromise,
      clustersPromise,
      statsPromise,
      byClusterPromise,
      histogramPromise,
    ]);

    const [risksResult, summaryResult, threatResult, pceSummaryResult, pceListResult, pceTrendResult, pceHeatmapResult, clustersResult, statsResult, byClusterResult] = results;
    const errors: string[] = [];

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

    const num = (v: unknown) => (typeof v === 'number' ? v : Number(v) || 0);
    if (summaryResult.status === 'fulfilled') {
      const s = summaryResult.value;
      setInsightsSummary({
        total: num(s?.total),
        critical: num(s?.critical),
        high: num(s?.high),
        medium: num(s?.medium),
        low: num(s?.low),
      });
    } else if (risksResult.status === 'fulfilled') {
      // Fallback: derive from first page (same as Dashboard) so severity cards always show numbers
      const { insights, total } = risksResult.value;
      const bySev = { critical: 0, high: 0, medium: 0, low: 0 };
      insights.forEach((r: Insight) => {
        const sev = (r.severity || '').toLowerCase();
        if (sev in bySev) (bySev as Record<string, number>)[sev]++;
      });
      setInsightsSummary({
        total: num(total),
        critical: bySev.critical,
        high: bySev.high,
        medium: bySev.medium,
        low: bySev.low,
      });
    } else {
      setInsightsSummary(null);
    }

    if (pceSummaryResult.status === 'fulfilled') {
      setPceSummary(pceSummaryResult.value);
    } else {
      setPceSummary([]);
      errors.push('PCE summary: ' + (pceSummaryResult.reason?.message || String(pceSummaryResult.reason)));
    }

    if (pceListResult.status === 'fulfilled') {
      setPceDetails(pceListResult.value);
    } else {
      setPceDetails([]);
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

    if (errors.length > 0) {
      setError(errors.join('; '));
    }
    setLoading(false);
  }, [risksPage, risksPageSize, filter, statusFilter, searchTerm, effectiveClusterId, sinceMinutesForApi, selectedClusterId, riskSort, priorityLevel, selectedScoreBin, namespaceFilter, typeFilter, trendDays]);

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

  React.useEffect(() => { setRisksPage(1); }, [filter, statusFilter, searchTerm, timeWindowMinutes, priorityLevel, namespaceFilter, typeFilter]);
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
    }
  };

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

  if (loading) return <PageLoading message="Loading Risk Operations…" />;

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
      <div className="mt-3 p-3 bg-slate-900/60 border border-slate-800 rounded-lg text-xs text-slate-400">
        <strong className="text-slate-300">Risk Findings</strong> = active security findings. <strong className="text-slate-300">Capability Exposure</strong> = pod capability exposure counts (separate from findings). <strong className="text-slate-300">Evidence & References</strong> = runtime evidence and capability knowledge (no single total). <em>Total findings below applies only to Risk Findings.</em>
      </div>
      <div className="mt-2 p-3 bg-slate-900/40 border border-slate-800 rounded-lg text-xs text-slate-400 flex flex-wrap items-center gap-3">
        <span className="text-slate-300 font-medium">Page objective:</span>
        <span>Triage and prioritize active risks in operations.</span>
        <span className="text-slate-600">|</span>
        <span>Need rule tuning? <Link className="text-pink-400 hover:underline" to="/rules">Open Detection & Policy Catalog</Link></span>
        <span className="text-slate-600">|</span>
        <span>Need semantic explanation? <Link className="text-pink-400 hover:underline" to="/capabilities">Open Capability Knowledge</Link></span>
      </div>
      <div className="mt-3 p-3 bg-slate-900/40 border border-slate-800 rounded-lg text-xs text-slate-400 flex flex-wrap gap-x-4 gap-y-1 items-center">
        <span title="Count of risk findings in current scope. Does not include Capability Exposure or Evidence totals.">
          Total findings (Risk Findings only): <span className="text-slate-200 font-medium">{severityBar.total}</span>
        </span>
        <span>Scope: <span className="text-slate-200 font-medium">{effectiveClusterId ? `cluster ${effectiveClusterId}` : 'all clusters'}</span></span>
        <span>Time window: <span className="text-slate-200 font-medium">{sinceMinutesForApi ? `${sinceMinutesForApi} minutes` : 'all time'}</span></span>
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
                    search: searchTerm.trim() || undefined,
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
                    search: searchTerm.trim() || undefined,
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
          {/* Wireframe §1: KPI Cards (4) – Total Risks | Critical (P0) | Resolved 24h | Velocity */}
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            <button
              type="button"
              onClick={() => isOverviewPage ? navigate('/risks/findings') : undefined}
              className="bg-slate-900 border border-slate-800 rounded-lg p-4 text-left hover:border-slate-600 transition-colors"
              title="Total risk findings in scope"
            >
              <div className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Total Risks</div>
              <div className="mt-1 text-2xl font-bold text-slate-200">{severityBar.total}</div>
              {threatVelocity.length >= 2 && (() => {
                const sorted = [...threatVelocity].sort((a, b) => a.date.localeCompare(b.date));
                const first = sorted[0];
                const last = sorted[sorted.length - 1];
                const firstTotal = (first?.critical ?? 0) + (first?.high ?? 0) + (first?.medium ?? 0) + (first?.low ?? 0);
                const lastTotal = (last?.critical ?? 0) + (last?.high ?? 0) + (last?.medium ?? 0) + (last?.low ?? 0);
                const delta = lastTotal - firstTotal;
                if (delta !== 0) {
                  return <span className={`text-xs font-medium ${delta > 0 ? 'text-red-400' : 'text-emerald-400'}`}>({delta > 0 ? '+' : ''}{delta} vs 7d ago)</span>;
                }
                return null;
              })()}
            </button>
            <button
              type="button"
              onClick={() => isOverviewPage ? navigate('/risks/findings?severity=critical') : setFilter('critical')}
              className="bg-slate-900 border border-slate-800 rounded-lg p-4 text-left hover:border-slate-600 transition-colors"
              title="Critical findings (P0)"
            >
              <div className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Critical</div>
              <div className="mt-1 text-2xl font-bold text-red-500">{severityBar.critical}</div>
              {(histogramData?.p0Count ?? 0) > 0 && (
                <div className="text-xs text-slate-500">P0: {histogramData!.p0Count}</div>
              )}
            </button>
            <div className="bg-slate-900 border border-slate-800 rounded-lg p-4" title="Resolved in the last 24h">
              <div className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Resolved 24h</div>
              <div className="mt-1 text-2xl font-bold text-emerald-500">{Number(resolved24h ?? 0)}</div>
            </div>
            <div className="bg-slate-900 border border-slate-800 rounded-lg p-4" title="Trend over last 7 days">
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
                    <span className="text-xs text-slate-500">vs 7d ago</span>
                  </div>
                );
              })() : (
                <div className="mt-1 text-slate-500 text-sm">—</div>
              )}
            </div>
          </div>

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
              <p className="text-[11px] text-slate-500 mb-2 md:mb-3">Track whether open risks are increasing or decreasing. Same scope as Total findings (Risk Findings only). {effectiveClusterId ? `Scoped to cluster ${effectiveClusterId}.` : 'All clusters.'} Click a point to filter findings from that date.</p>
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
                      <defs>
                        <linearGradient id="colorRiskVelocity" x1="0" y1="0" x2="0" y2="1">
                          <stop offset="5%" stopColor="#ef4444" stopOpacity={0.3} />
                          <stop offset="95%" stopColor="#ef4444" stopOpacity={0} />
                        </linearGradient>
                      </defs>
                      <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#334155" />
                      <XAxis dataKey="name" axisLine={false} tickLine={false} tick={{ fill: '#94a3b8', fontSize: 11 }} />
                      <YAxis domain={[0, 'auto']} axisLine={false} tickLine={false} tick={{ fill: '#94a3b8', fontSize: 11 }} width={28} />
                      <Tooltip content={<RiskTrendTooltipContent />} cursor={{ stroke: '#64748b', strokeDasharray: '4 4' }} />
                      <Area type="monotone" dataKey="risk" name="Risks" stroke="#ef4444" strokeWidth={2} fillOpacity={1} fill="url(#colorRiskVelocity)" />
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
                <p className="text-slate-500 text-sm py-4">No trend data for the last 7 days.</p>
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

          {/* Layer 2 – Exposure Overview (spec §3.2): 2x2 severity cards, Resolved secondary; same scope as Total findings (Risk Findings only) */}
          <div>
            <h2 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Risk Level Overview (Active Findings)</h2>
            <p className="text-xs text-slate-500 mb-2">Same scope as &quot;Total findings&quot; above: Risk Findings (insights) only. From GET /insights/summary.</p>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              {(['critical', 'high', 'medium', 'low'] as const).map((sev) => (
                <button
                  key={sev}
                  type="button"
                  onClick={() => isOverviewPage ? navigate(`/risks/findings?severity=${sev}`) : setFilter(sev)}
                  className="bg-slate-900 border border-slate-800 p-4 rounded-lg flex items-center justify-between text-left hover:border-slate-600 transition-colors"
                  title={isOverviewPage ? `View ${sev} findings` : 'Filter by severity'}
                >
                  <span className="text-slate-400 text-sm capitalize">{sev}</span>
                  <span className={`font-bold text-xl ${sev === 'critical' ? 'text-red-500' : sev === 'high' ? 'text-orange-500' : sev === 'medium' ? 'text-yellow-500' : 'text-blue-500'}`}>
                    {severityBar[sev]}
                  </span>
                </button>
              ))}
            </div>
            <div className="mt-2 flex items-center gap-2 text-sm text-slate-500" title="Insights resolved in the last 24h. From GET /dashboard/stats.">
              <span>Resolved (24h):</span>
              <span className="font-medium text-emerald-500">{Number(resolved24h ?? 0)}</span>
            </div>
            {/* Risks by cluster (Phase 2.2) – when scope is all clusters */}
            {!effectiveClusterId && risksByCluster.length > 0 && (
              <div className="mt-4">
                <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Risks by cluster</h3>
                <div className="overflow-x-auto border border-slate-800 rounded-lg">
                  <table className="w-full text-sm">
                    <thead className="bg-slate-950 text-slate-400">
                      <tr>
                        <th className="text-left px-3 py-2">Cluster</th>
                        <th className="text-right px-3 py-2">Total</th>
                        <th className="text-right px-3 py-2 text-red-400">Critical</th>
                        <th className="text-right px-3 py-2 text-orange-400">High</th>
                        <th className="text-right px-3 py-2 text-yellow-400">Medium</th>
                        <th className="text-right px-3 py-2 text-blue-400">Low</th>
                      </tr>
                    </thead>
                    <tbody>
                      {risksByCluster.map((row) => (
                        <tr key={row.clusterId} className="border-t border-slate-800">
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
                <thead className="text-xs text-muted uppercase bg-muted/50 border-b border-border sticky top-0 z-10">
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
                    <th className="text-left px-4 py-3 w-24 hidden sm:table-cell">Type</th>
                    <th className="text-left px-4 py-3 w-20 hidden md:table-cell">Source</th>
                    <th className="text-left px-4 py-3 w-20">Impacted Resource</th>
                    <th className="text-left px-4 py-3 w-16">Risk Score</th>
                    {!effectiveClusterId && <th className="text-left px-4 py-3 hidden lg:table-cell w-28">Namespace/Cluster</th>}
                    <th className="text-left px-4 py-3 w-24">Workflow</th>
                    <th className="text-left px-4 py-3 w-24 hidden md:table-cell">Detected At</th>
                    <th className="text-left px-4 py-3 w-24 hidden lg:table-cell">Updated At</th>
                    <th className="text-right px-4 py-3 w-32">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {paginatedRisks.length === 0 ? (
                    <tr>
                      <td colSpan={effectiveClusterId ? 10 : 11} className="px-4 py-8 text-center text-slate-500">
                        No risks match the current filters.
                      </td>
                    </tr>
                  ) : (
                    paginatedRisks.map((risk) => (
                          <tr
                        key={risk.id}
                        className="border-b border-slate-800 hover:bg-slate-800/50 transition-colors cursor-pointer"
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
                        <td className="px-4 py-3 text-slate-400 hidden sm:table-cell capitalize text-xs">
                          {risk.insightType === 'vulnerability'
                            ? 'Vulnerability'
                            : risk.insightType === 'supply_chain_malware'
                              ? 'Supply-chain malware'
                              : risk.insightType === 'rbac'
                                ? 'Behavior'
                                : (risk.insightType ?? 'Finding')}
                        </td>
                        <td className="px-4 py-3 text-slate-400 hidden md:table-cell text-xs">
                          {risk.insightType === 'vulnerability' || risk.insightType === 'supply_chain_malware' ? 'Static' : 'Runtime'}
                        </td>
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
                        <td className="px-4 py-3 text-slate-300">
                          <div className="flex flex-col gap-1">
                            <div>
                              {risk.score != null ? `${risk.score}/100${risk.priorityLevel ? ` (${risk.priorityLevel})` : ''}` : '—'}
                            </div>
                            {(risk.exploitabilityScore != null || risk.businessImpactScore != null) && (
                              <div className="flex flex-wrap gap-1 text-[10px] text-slate-400">
                                {risk.exploitabilityScore != null && (
                                  <span className="px-1.5 py-0.5 rounded-full bg-slate-800/80 border border-slate-700" title="Exploitability score (0–30)">
                                    EXP {risk.exploitabilityScore.toFixed(1)}
                                  </span>
                                )}
                                {risk.businessImpactScore != null && (
                                  <span className="px-1.5 py-0.5 rounded-full bg-slate-800/80 border border-slate-700" title="Business impact score (0–30)">
                                    IMP {risk.businessImpactScore.toFixed(1)}
                                  </span>
                                )}
                              </div>
                            )}
                          </div>
                        </td>
                        {!effectiveClusterId && (
                          <td className="px-4 py-3 text-slate-400 hidden lg:table-cell truncate max-w-[140px]" title={risk.affectedResources?.[0]?.namespace ?? risk.clusterId}>
                            {risk.affectedResources?.[0]?.namespace ?? risk.clusterName ?? risk.clusterId ?? '—'}
                          </td>
                        )}
                        <td className="px-4 py-3">
                          <span className="uppercase px-2 py-0.5 bg-slate-800 rounded text-xs text-slate-300">
                            {statusLabelMap[risk.status || ''] ?? (risk.status || 'Unknown')}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-slate-500 hidden md:table-cell text-xs">
                          {risk.timestamp ? new Date(risk.timestamp).toLocaleDateString() : '—'}
                        </td>
                        <td className="px-4 py-3 text-slate-500 hidden lg:table-cell text-xs">
                          {risk.updatedAt ? new Date(risk.updatedAt).toLocaleDateString() : '—'}
                        </td>
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
                            onClick={() => navigate('/attack-paths')}
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

          {/* Capability exposure trend (7 days) */}
          {pceTrend.length > 0 && (
            <div className="bg-surface border border-border p-4 rounded-lg">
              <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-2">Capability Exposure Trend (7 Days)</h2>
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
              const data = await api.getPceCapabilities({
                clusterId: effectiveClusterId ?? undefined,
                namespace: ns,
                severity,
                limit: 100,
              });
              setPceDetails(data);
            };
            return (
              <div className="bg-surface border border-border p-4 rounded-lg">
                <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-2">Exposure by Namespace (heatmap)</h2>
                <p className="text-xs text-slate-500 mb-3">Click a cell to filter the table below by that namespace and severity.</p>
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b border-slate-700">
                        <th className="text-left py-2 px-3 text-slate-400">Namespace</th>
                        <th className="text-right py-2 px-2 text-red-400">Critical</th>
                        <th className="text-right py-2 px-2 text-orange-400">High</th>
                        <th className="text-right py-2 px-2 text-yellow-500">Medium</th>
                        <th className="text-right py-2 px-2 text-green-400">Low</th>
                      </tr>
                    </thead>
                    <tbody>
                      {namespaces.slice(0, 30).map((ns) => {
                        const row = byNs[ns];
                        return (
                          <tr key={ns} className="border-b border-slate-800">
                            <td className="py-1.5 px-3 text-slate-200 font-mono text-xs">{ns}</td>
                            <td className="text-right py-1.5 px-2 cursor-pointer hover:ring-2 hover:ring-inset hover:ring-white/50 rounded" style={{ backgroundColor: `rgba(220,38,38,${0.2 + (row.critical / maxCount) * 0.8})` }} onClick={() => row.critical > 0 && handleHeatmapCellClick(ns, 'critical')} title={row.critical > 0 ? `Filter table by ${ns}, critical` : undefined}>{row.critical}</td>
                            <td className="text-right py-1.5 px-2 cursor-pointer hover:ring-2 hover:ring-inset hover:ring-white/50 rounded" style={{ backgroundColor: `rgba(234,88,12,${0.2 + (row.high / maxCount) * 0.8})` }} onClick={() => row.high > 0 && handleHeatmapCellClick(ns, 'high')} title={row.high > 0 ? `Filter table by ${ns}, high` : undefined}>{row.high}</td>
                            <td className="text-right py-1.5 px-2 cursor-pointer hover:ring-2 hover:ring-inset hover:ring-white/50 rounded" style={{ backgroundColor: `rgba(202,138,4,${0.2 + (row.medium / maxCount) * 0.8})` }} onClick={() => row.medium > 0 && handleHeatmapCellClick(ns, 'medium')} title={row.medium > 0 ? `Filter table by ${ns}, medium` : undefined}>{row.medium}</td>
                            <td className="text-right py-1.5 px-2 cursor-pointer hover:ring-2 hover:ring-inset hover:ring-white/50 rounded" style={{ backgroundColor: `rgba(22,163,74,${0.2 + (row.low / maxCount) * 0.8})` }} onClick={() => row.low > 0 && handleHeatmapCellClick(ns, 'low')} title={row.low > 0 ? `Filter table by ${ns}, low` : undefined}>{row.low}</td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                  {namespaces.length > 30 && <p className="text-xs text-slate-500 mt-2">Showing first 30 namespaces. Use drill-down filters for more.</p>}
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
                    const data = await api.getPceCapabilities({
                      clusterId: pceClusterId || undefined,
                      limit: 100,
                    });
                    setPceDetails(data);
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
                    const data = await api.getPceCapabilities({
                      clusterId: pceClusterId || undefined,
                      namespace: pceNamespace || undefined,
                      severity: pceSeverityFilter || undefined,
                      podName: pcePodName || undefined,
                      capabilityId: pceCapabilityId || undefined,
                      limit: 100,
                    });
                    setPceDetails(data);
                  }}
                >
                  Run filters
                </Button>
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={async () => {
                    const data = await api.getPceCapabilities({
                      clusterId: pceClusterId || undefined,
                      namespace: pceNamespace || undefined,
                      severity: pceSeverityFilter || undefined,
                      podName: pcePodName || undefined,
                      capabilityId: pceCapabilityId || undefined,
                      limit: 100,
                    });
                    setPceDetails(data);
                  }}
                >
                  Refresh data
                </Button>
              </div>
            </div>
            <div className="mb-3 flex items-center gap-2">
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
            </div>
            <div className="ui-table-scroll rounded-lg border border-border bg-surface">
              <table className="w-full text-sm">
                <thead className="text-xs text-muted uppercase bg-muted/50 border-b border-border sticky top-0 z-10">
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
                      const evidenceStr = row.evidence && Object.keys(row.evidence).length > 0 ? JSON.stringify(row.evidence) : '';
                      const evidenceTruncate = evidenceStr.length > 100 ? evidenceStr.slice(0, 97) + '...' : evidenceStr;
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
                        <tr key={`${row.podUid}-${row.capabilityId}`} className="border-t border-slate-800">
                          <td className="px-3 py-2 text-slate-200 font-medium" title={row.podUid}>{row.podName ?? '—'}</td>
                          <td className="px-3 py-2 text-slate-500 font-mono text-xs truncate max-w-[120px] hidden lg:table-cell" title={row.podUid}>{row.podUid}</td>
                          <td className="px-3 py-2 text-slate-300">{row.namespace}</td>
                          <td className="px-3 py-2 text-slate-200">{row.capabilityId}</td>
                          <td className="px-3 py-2">
                            <span className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase border ${getSeverityBadgeClass(row.severity)}`}>
                              {row.severity}
                            </span>
                          </td>
                          <td className="px-3 py-2 text-slate-500 font-mono text-xs max-w-[140px] truncate break-all" title={evidenceStr || undefined}>{evidenceStr ? evidenceTruncate : '—'}</td>
                          <td className="px-3 py-2 text-slate-500 text-xs whitespace-nowrap" title={row.lastSeenAt || row.updatedAt || undefined}>{lastSeen}</td>
                        </tr>
                      );
                    })
                  )}
                </tbody>
              </table>
            </div>
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
                <RuntimeSignalsTable />
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
                      <thead className="text-xs text-muted uppercase bg-muted/50 border-b border-border sticky top-0 z-10">
                        <tr>
                          <th className="text-left px-3 py-2">User</th>
                          <th className="text-left px-3 py-2">Action</th>
                          <th className="text-left px-3 py-2">Finding ID</th>
                          <th className="text-left px-3 py-2">Timestamp</th>
                          <th className="text-left px-3 py-2">IP</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-border">
                        {evidenceAuditLogs.map((log) => (
                          <tr key={log.id} className="hover:bg-muted/30">
                            <td className="px-3 py-2 text-text">{log.actor ?? log.user ?? '—'}</td>
                            <td className="px-3 py-2 text-muted">{log.action}</td>
                            <td className="px-3 py-2 font-mono text-xs text-muted">{log.resourceId ?? '—'}</td>
                            <td className="px-3 py-2 text-muted text-xs">{log.timestamp}</td>
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

      {/* Risk Detail Drawer (spec §4): right-side contextual panel, no navigation away */}
      {selectedRisk && (
        <>
          <div className="fixed inset-0 bg-black/40 z-40" onClick={() => setSelectedRisk(null)} aria-hidden />
          <div className="fixed top-0 right-0 bottom-0 w-full max-w-lg bg-slate-900 border-l border-slate-800 shadow-xl z-50 overflow-y-auto flex flex-col">
            <div className="p-4 border-b border-slate-800 flex items-start justify-between shrink-0">
              <div className="min-w-0 pr-4">
                <div className="flex items-center gap-2 mb-1">
                  {getSeverityIcon(selectedRisk.severity)}
                  <h2 className="text-lg font-bold text-white truncate">{selectedRisk.title}</h2>
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
                        <button
                          type="button"
                          onClick={() => {
                            setSelectedRisk(null);
                            setActiveTab('reference');
                          }}
                          className="text-xs text-pink-400 hover:underline"
                        >
                          Open full runtime evidence tab
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
                          <thead className="bg-slate-950 text-slate-500">
                            <tr>
                              <th className="text-left px-2 py-1.5">Time</th>
                              <th className="text-left px-2 py-1.5">User</th>
                              <th className="text-left px-2 py-1.5">Action</th>
                              <th className="text-left px-2 py-1.5">Details</th>
                            </tr>
                          </thead>
                          <tbody>
                            {drawerAuditLogs.map((log) => (
                              <tr key={log.id} className="border-t border-slate-800">
                                <td className="px-2 py-1.5 text-slate-400 whitespace-nowrap">{log.timestamp || ''}</td>
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
                          setSelectedRisk(null);
                          navigate('/attack-paths');
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
                  onClick={async () => {
                    await api.bulkInsightsAction({ action: 'acknowledge', insightIds: [selectedRisk.id] });
                    fetchDataRef.current();
                  }}
                >
                  Acknowledge
                </Button>
                <Button
                  size="sm"
                  onClick={async () => {
                    await api.bulkInsightsAction({ action: 'resolve', insightIds: [selectedRisk.id] });
                    fetchDataRef.current();
                  }}
                >
                  Resolve
                </Button>
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={async () => {
                    await api.bulkInsightsAction({ action: 'dismiss', insightIds: [selectedRisk.id] });
                    fetchDataRef.current();
                  }}
                >
                  Dismiss
                </Button>
              </section>
            </div>
          </div>
        </>
      )}
    </PageLayout>
  );
};

