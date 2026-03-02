import React, { useCallback, useMemo, useState } from 'react';
import { api } from '../lib/api';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { Cluster, Insight, PodCapabilityDetail, PodCapabilitySummarySeverity, RuntimeSignal } from '../types';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../components/PageLayout';
import { Pagination } from '../components/Pagination';
import { Shield, AlertTriangle, Info, CheckCircle, Search, Box, User, ArrowRight, X, ExternalLink, Loader2 } from 'lucide-react';
import { useNavigate, useSearchParams, Link } from 'react-router-dom';
import { useClusterStore } from '../store/clusterStore';
import { useTimeWindowStore } from '../store/timeWindowStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';
import { getSeverityBadgeClass, getSeverityTextClass } from '../lib/severity';
import { RISK_CENTER_DESCRIPTION } from '../constants/labels';
import { RuntimeSignalsTable } from '../components/RuntimeSignalsTable';
import { PageLoading } from '../components/PageLoading';
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';

type TabId = 'risks' | 'pce' | 'reference';

export const RiskCenter: React.FC = () => {
  const navigate = useNavigate();
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
  const [filter, setFilter] = useState<'all' | string>(severityFromUrl && ['critical', 'high', 'medium', 'low'].includes(severityFromUrl) ? severityFromUrl : 'all');
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'resolved' | 'acknowledged'>('active');
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedRisk, setSelectedRisk] = useState<Insight | null>(null);
  const [selectedRiskDetail, setSelectedRiskDetail] = useState<Insight | null>(null);
  const [selectedRiskSignals, setSelectedRiskSignals] = useState<RuntimeSignal[]>([]);
  const [selectedRiskCapabilities, setSelectedRiskCapabilities] = useState<PodCapabilityDetail[]>([]);
  const [selectedRiskReferences, setSelectedRiskReferences] = useState<string[]>([]);
  const [selectedRiskLoading, setSelectedRiskLoading] = useState(false);
  const [resolved24h, setResolved24h] = useState<number>(0);
  const [insightsSummary, setInsightsSummary] = useState<{ total: number; critical: number; high: number; medium: number; low: number } | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [risksPage, setRisksPage] = useState(1);
  const [risksPageSize, setRisksPageSize] = useState(20);
  const [threatVelocity, setThreatVelocity] = useState<{ date: string; critical: number; high: number; medium: number; low: number }[]>([]);
  const [riskSort, setRiskSort] = useState<'newest' | 'oldest' | 'score_desc' | 'score_asc' | 'severity_desc' | 'title_asc'>('newest');
  const [pceSort, setPceSort] = useState<'severity_desc' | 'severity_asc' | 'capability_asc' | 'pod_asc' | 'namespace_asc'>('severity_desc');
  /** When user clicks a date on Threat Velocity chart, filter risk table to that day (spec: drill-down). */
  const [selectedChartDate, setSelectedChartDate] = useState<string | null>(null);
  // Resolve cluster + time: URL from Dashboard link overrides store so Risk Center shows same scope
  const effectiveClusterId = clusterIdFromUrl ?? selectedClusterId ?? undefined;
  const effectiveSinceMinutes = sinceMinutesFromUrl != null ? parseInt(sinceMinutesFromUrl, 10) : timeWindowMinutes;
  const effectiveSinceMinutesNum = Number.isFinite(effectiveSinceMinutes) && effectiveSinceMinutes > 0 ? effectiveSinceMinutes : undefined;

  /** Since minutes for API: if a chart date is selected, use from start of that day to now; else use URL/store. */
  const sinceMinutesForApi = useMemo(() => {
    if (selectedChartDate) {
      const start = new Date(selectedChartDate + 'T00:00:00Z').getTime();
      const now = Date.now();
      return Math.max(0, Math.floor((now - start) / 60000));
    }
    return effectiveSinceMinutesNum ?? (timeWindowMinutes > 0 ? timeWindowMinutes : undefined);
  }, [selectedChartDate, effectiveSinceMinutesNum, timeWindowMinutes]);

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
    const risksPromise = api.getRisks({
      page: risksPage,
      pageSize: risksPageSize,
      severity: filter !== 'all' ? filter : undefined,
      status: statusFilter,
      search: searchTerm.trim() || undefined,
      clusterId: clusterId ?? undefined,
      sinceMinutes,
    });
    const summaryPromise = api.getInsightsSummary(clusterId ?? undefined, sinceMinutes);
    const threatPromise = api.getThreatVelocity(7, clusterId ?? undefined).catch(() => []);
    const pceSummaryPromise = api.getPceSummaryBySeverity();
    const pceListPromise = api.getPceCapabilities({ limit: 50, clusterId: clusterId ?? undefined });
    const clustersPromise = api.getClusters();
    const statsPromise = api.getStats(clusterId ?? undefined, sinceMinutes);
    const results = await Promise.allSettled([
      risksPromise,
      summaryPromise,
      threatPromise,
      pceSummaryPromise,
      pceListPromise,
      clustersPromise,
      statsPromise,
    ]);

    const [risksResult, summaryResult, threatResult, pceSummaryResult, pceListResult, clustersResult, statsResult] = results;
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

    if (errors.length > 0) {
      setError(errors.join('; '));
    }
    setLoading(false);
  }, [risksPage, risksPageSize, filter, statusFilter, searchTerm, effectiveClusterId, sinceMinutesForApi, selectedClusterId]);

  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.SBOM_RISK_LIST));
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);
  usePolling(fetchData, intervalMs, { refreshTrigger });
  const isFirstFetch = React.useRef(true);
  React.useEffect(() => {
    if (isFirstFetch.current) {
      isFirstFetch.current = false;
      return;
    }
    fetchData();
  }, [fetchData]);

  React.useEffect(() => { setRisksPage(1); }, [filter, statusFilter, searchTerm, timeWindowMinutes]);

  React.useEffect(() => {
    if (!selectedRisk) {
      setSelectedRiskDetail(null);
      setSelectedRiskSignals([]);
      setSelectedRiskCapabilities([]);
      setSelectedRiskReferences([]);
      return;
    }

    let cancelled = false;
    const loadRiskContext = async () => {
      setSelectedRiskLoading(true);
      try {
        const podUid = selectedRisk.affectedResources?.find((r) => r.kind === 'Pod' && r.id)?.id;
        const [detailRes, signalsRes, capsRes] = await Promise.allSettled([
          api.getInsight(selectedRisk.id),
          podUid ? api.getRuntimeSignalsByPod(podUid, { limit: 10, sinceMinutes: sinceMinutesForApi }) : Promise.resolve([]),
          podUid ? api.getPodCapabilities(podUid) : Promise.resolve([]),
        ]);

        const detail = detailRes.status === 'fulfilled' ? detailRes.value : null;
        const signals = signalsRes.status === 'fulfilled' ? signalsRes.value : [];
        const capabilities = capsRes.status === 'fulfilled' ? capsRes.value : [];

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
        }
      } finally {
        if (!cancelled) setSelectedRiskLoading(false);
      }
    };

    loadRiskContext();
    return () => {
      cancelled = true;
    };
  }, [selectedRisk, sinceMinutesForApi]);

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

  if (loading) return <PageLoading message="Loading Risk Center…" />;

  const tabs: { id: TabId; label: string }[] = [
    { id: 'risks', label: 'Risk Findings' },
    { id: 'pce', label: 'Capability Exposure (PCE)' },
    { id: 'reference', label: 'Evidence & References' },
  ];

  return (
    <PageLayout
      title="Risk Center"
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
      <div className="flex gap-1 p-1 bg-slate-900/80 rounded-lg border border-slate-800 w-fit">
          {tabs.map(({ id, label }) => (
            <button
              key={id}
              onClick={() => setActiveTab(id)}
              className={`px-4 py-2 rounded-md text-sm font-medium transition-colors ${
                activeTab === id
                  ? 'bg-pink-600 text-white shadow'
                  : 'text-slate-400 hover:text-white hover:bg-slate-800'
              }`}
            >
              {label}
            </button>
          ))}
      </div>
      <div className="mt-3 p-3 bg-slate-900/60 border border-slate-800 rounded-lg text-xs text-slate-400">
        <strong className="text-slate-300">Risk Findings</strong> = security findings (insights). <strong className="text-slate-300">PCE</strong> = pod capability exposure (separate counts). <strong className="text-slate-300">Evidence & References</strong> = runtime signals and capability catalog (no single total). <em>Total findings below applies only to Risk Findings.</em>
      </div>
      <div className="mt-3 p-3 bg-slate-900/40 border border-slate-800 rounded-lg text-xs text-slate-400 flex flex-wrap gap-x-4 gap-y-1">
        <span title="Count of risk findings (insights) in current scope. Does not include PCE or Evidence totals.">
          Total findings (Risk Findings only): <span className="text-slate-200 font-medium">{severityBar.total}</span>
        </span>
        <span>Scope: <span className="text-slate-200 font-medium">{effectiveClusterId ? `cluster ${effectiveClusterId}` : 'all clusters'}</span></span>
        <span>Time window: <span className="text-slate-200 font-medium">{sinceMinutesForApi ? `${sinceMinutesForApi} minutes` : 'all time'}</span></span>
      </div>

      {/* ----- Risks tab ----- */}
      {activeTab === 'risks' && (
        <div className="space-y-6">
          {/* Layer 1 – Threat Velocity (spec §3.1): posture trend, click date → filter table */}
          <div className="bg-slate-900 border border-slate-800 rounded-lg p-4">
            <h2 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Risk Trend (Last 7 Days)</h2>
            <p className="text-xs text-slate-500 mb-3">Track whether open risks are increasing or decreasing. Same scope as Total findings (Risk Findings only). {effectiveClusterId ? `Scoped to cluster ${effectiveClusterId}.` : 'All clusters.'} Click a point to filter findings from that date.</p>
            {threatVelocity.length > 0 ? (
              <div className="w-full h-[260px]">
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
                    <Tooltip
                      contentStyle={{ backgroundColor: '#0f172a', borderRadius: '12px', border: '1px solid #1e293b', padding: '12px' }}
                      labelFormatter={(label) => `Date: ${label}`}
                      formatter={(value: number) => [value, 'Risks']}
                    />
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
              <p className="text-sm text-slate-500 py-4">No trend data available for the selected scope.</p>
            )}
          </div>

          {/* Layer 2 – Exposure Overview (spec §3.2): 2x2 severity cards, Resolved secondary; same scope as Total findings (Risk Findings only) */}
          <div>
            <h2 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Risk Level Overview (Active Findings)</h2>
            <p className="text-xs text-slate-500 mb-2">Same scope as &quot;Total findings&quot; above: Risk Findings (insights) only. From GET /insights/summary.</p>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              {(['critical', 'high', 'medium', 'low'] as const).map((sev) => (
                <div
                  key={sev}
                  className="bg-slate-900 border border-slate-800 p-4 rounded-lg flex items-center justify-between"
                  title="From GET /insights/summary. Risk Findings only; same scope as Total findings."
                >
                  <span className="text-slate-400 text-sm capitalize">{sev}</span>
                  <span className={`font-bold text-xl ${sev === 'critical' ? 'text-red-500' : sev === 'high' ? 'text-orange-500' : sev === 'medium' ? 'text-yellow-500' : 'text-blue-500'}`}>
                    {severityBar[sev]}
                  </span>
                </div>
              ))}
            </div>
            <div className="mt-2 flex items-center gap-2 text-sm text-slate-500" title="Insights resolved in the last 24h. From GET /dashboard/stats.">
              <span>Resolved (24h):</span>
              <span className="font-medium text-emerald-500">{Number(resolved24h ?? 0)}</span>
            </div>
          </div>

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
            </div>
            <div className="relative flex-1 md:max-w-xs">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 w-4 h-4" />
              <input
                type="text"
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                placeholder="Search title, CVE, pod, namespace..."
                className="pl-9 pr-4 py-2 bg-slate-900 border border-slate-700 rounded-lg text-sm text-white focus:ring-2 focus:ring-pink-500 outline-none w-full placeholder:text-slate-600"
              />
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
          {/* Risk list – compact table with pagination */}
          <div className="bg-slate-900 border border-slate-800 rounded-t-lg overflow-hidden">
            <div className="overflow-x-auto max-h-[calc(100vh-22rem)] overflow-y-auto">
              <table className="w-full text-sm">
                <thead className="bg-slate-950 text-slate-400 border-b border-slate-800">
                  <tr>
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
                        <td className="px-4 py-3">{getSeverityIcon(risk.severity)}</td>
                        <td className="px-4 py-3">
                          <span className="text-white font-medium">{risk.title}</span>
                          <span className="text-slate-500 ml-1 text-xs">({risk.cveId ?? risk.id})</span>
                        </td>
                        <td className="px-4 py-3 text-slate-400 hidden sm:table-cell capitalize text-xs">
                          {risk.insightType === 'vulnerability' ? 'Vulnerability' : risk.insightType === 'rbac' ? 'Behavior' : (risk.insightType ?? 'Vulnerability')}
                        </td>
                        <td className="px-4 py-3 text-slate-400 hidden md:table-cell text-xs">
                          {risk.insightType === 'vulnerability' ? 'Static' : 'Runtime'}
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
                        <td className="px-4 py-3 text-slate-300">{risk.score ?? '—'}/100</td>
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
        </div>
      )}

      {/* ----- PCE tab ----- */}
      {activeTab === 'pce' && (
        <div className="space-y-6">
          {/* PCE Summary – capability counts, not risk counts; not included in Total findings */}
          <div className="bg-slate-900 border border-slate-800 p-4 rounded-lg">
            <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-1">
              Capability Exposure Summary (PCE)
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

          {/* PCE Drill-down */}
          <div className="bg-slate-900 border border-slate-800 p-4 rounded-lg">
            <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-2">
              Capability Drill-down by Pod
            </h2>
            <p className="text-xs text-slate-500 mb-4">
              Explore pod-level capabilities that may explain risk findings. Filter by cluster, namespace, pod name, or capability ID.
            </p>
            <div className="grid grid-cols-1 md:grid-cols-5 gap-3 mb-4">
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
            <div className="overflow-auto border border-slate-800 rounded">
              <table className="w-full text-sm">
                <thead className="bg-slate-950 text-slate-400">
                  <tr>
                    <th className="text-left px-3 py-2">Pod Name</th>
                    <th className="text-left px-3 py-2 hidden lg:table-cell">Pod UID</th>
                    <th className="text-left px-3 py-2">Namespace</th>
                    <th className="text-left px-3 py-2">Capability</th>
                    <th className="text-left px-3 py-2">Severity</th>
                  </tr>
                </thead>
                <tbody>
                  {pceDetails.length === 0 ? (
                    <tr>
                      <td colSpan={5} className="px-3 py-4 text-center text-slate-500">No matching capability records. Adjust filters and run again.</td>
                    </tr>
                  ) : (
                    sortedPceDetails.map((row) => (
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
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* ----- Reference tab ----- */}
      {activeTab === 'reference' && (
        <div className="space-y-8">
          <div className="bg-slate-900 border border-slate-800 p-6 rounded-lg">
            <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-1 flex items-center gap-2">
              <AlertTriangle size={16} className="text-pink-400" />
              Runtime Evidence
            </h2>
            <p className="text-xs text-slate-500 mb-4">Pod runtime events used as supporting evidence for risk investigation. This tab has no single &quot;total&quot; count; it is separate from Total findings (Risk Findings).</p>
            <RuntimeSignalsTable />
          </div>
          <div className="bg-slate-900 border border-slate-800 p-6 rounded-lg">
            <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-1 flex items-center gap-2">
              <Shield size={16} className="text-pink-400" />
              Capability Reference Catalog
            </h2>
            <p className="text-xs text-slate-500 mb-4">Definitions and context: capability meaning, severity base, MITRE mapping, and mitigations.</p>
            <Link
              to="/capabilities"
              className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-pink-500/20 text-pink-400 border border-pink-500/50 hover:bg-pink-500/30 transition-colors text-sm font-medium"
            >
              Open capability catalog
              <ArrowRight size={16} />
            </Link>
          </div>
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
                  <span>Score: <span className="text-slate-300">{selectedRisk.score ?? '—'}/100</span></span>
                  <span>Source: {selectedRisk.insightType === 'vulnerability' ? 'Static scan' : 'Runtime behavior'}</span>
                  <span>Workflow: <span className="uppercase">{statusLabelMap[selectedRisk.status || ''] ?? selectedRisk.status}</span></span>
                </div>
              </div>
              <button onClick={() => setSelectedRisk(null)} className="p-1 text-slate-400 hover:text-white rounded" aria-label="Close"><X size={20} /></button>
            </div>
            <div className="p-4 space-y-5 flex-1">
              {selectedRiskLoading && (
                <div className="flex items-center gap-2 text-xs text-slate-500">
                  <Loader2 className="w-4 h-4 animate-spin" />
                  Loading linked Risk, PCE, and evidence context...
                </div>
              )}
              <section>
                <div className="text-xs text-slate-400 bg-slate-950 border border-slate-800 rounded px-3 py-2">
                  Relationship: <span className="text-slate-200">Risk finding</span> → <span className="text-slate-200">affected Pod</span> → <span className="text-slate-200">PCE capabilities</span> → <span className="text-slate-200">runtime evidence & references</span>.
                </div>
              </section>
              {/* 1. Risk Summary */}
              <section>
                <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Finding Summary</h3>
                <p className="text-slate-300 text-sm leading-relaxed">{selectedRiskDetail?.description || selectedRisk.description || '—'}</p>
                <div className="mt-2 text-xs text-slate-500">
                  Severity: {selectedRisk.severity} · First detected: {selectedRisk.timestamp ? new Date(selectedRisk.timestamp).toLocaleString() : '—'}
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
              {/* 2. Related Capabilities */}
              <section>
                <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Related Capability IDs (PCE)</h3>
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
              {/* 3. Capability Details */}
              <section>
                <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Capability Details</h3>
                {selectedRiskCapabilities.length === 0 ? (
                  <p className="text-slate-500 text-xs">No PCE capability context available for the currently linked asset.</p>
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
              {/* 4. Runtime Evidence */}
              <section>
                <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Runtime Evidence</h3>
                {selectedRiskSignals.length === 0 ? (
                  <p className="text-slate-500 text-xs">No runtime evidence in current time window.</p>
                ) : (
                  <div className="space-y-1.5">
                    {selectedRiskSignals.slice(0, 5).map((s) => (
                      <div key={s.id} className="text-xs bg-slate-950 border border-slate-800 rounded px-2 py-1.5 text-slate-300">
                        <span className="text-amber-300 font-medium">{s.signalType}</span>
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
              {/* 5. PCE Impact */}
              <section>
                <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">Capability Exposure Impact</h3>
                <p className="text-slate-500 text-xs mb-2">
                  Risk is linked to PCE through affected Pod UID. Open PCE tab for full drill-down with capability filters.
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
              </section>
              {/* 6. References */}
              <section>
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
              {/* 7. Impacted Resources */}
              <section>
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
              {/* 8. Actions */}
              <section className="pt-4 border-t border-slate-800 flex flex-wrap gap-2">
                <Button variant="secondary" size="sm" onClick={() => setSelectedRisk(null)}>Close</Button>
                <Button size="sm" onClick={() => { setSelectedRisk(null); navigate(`/risks/${selectedRisk.id}`); }}>Open full detail page</Button>
                <Button size="sm" variant="secondary" onClick={() => { setSelectedRisk(null); navigate('/attack-paths'); }}>Open attack path view</Button>
              </section>
            </div>
          </div>
        </>
      )}
    </PageLayout>
  );
};

