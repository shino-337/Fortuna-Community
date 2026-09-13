import { normalizeInsightStatus } from '../lib/api';
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { api } from '../lib/api';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useAbortSignal, isAbortError } from '../hooks/useAbortSignal';
import { useClusters } from '../hooks/useClusters';
import { useEntityStore } from '../store/entityStore';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import {
  AttackStepSummary,
  Cluster,
  Insight,
  InsightsSummary,
  InsightsSummaryByClusterItem,
  PipelineHealth,
  PodCapabilityDetail,
  PodCapabilitySummaryNamespace,
  PodCapabilitySummarySeverity,
  PodCapabilityTrendPoint,
  RuntimeSignal,
  AuditLog,
} from '../types';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Tabs } from '../design-system/components/Tabs';
import { Pagination } from '../components/Pagination';
import { Shield, AlertTriangle, Info, CheckCircle, Search, Box, User, ArrowRight, X, ExternalLink, Loader2, FileText } from 'lucide-react';
import { useLocation, useNavigate, useSearchParams, Link } from 'react-router-dom';
import { useClusterStore } from '../store/clusterStore';
import { useTimeWindowStore } from '../store/timeWindowStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';
import { getSeverityBadgeClass, getSeverityTextClass } from '../lib/severity';
import { RISK_CENTER_DESCRIPTION } from '../constants/labels';
import { PAGE_TITLES } from '../lib/pageTitles';
import { RuntimeSignalsTable } from '../components/RuntimeSignalsTable';
import { RiskHistogram } from '../components/RiskHistogram';
import { RiskDrawer } from '../components/RiskDrawer';
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, ReferenceDot } from 'recharts';
import type { RiskHistogramResponse } from '../types';
import { insightTypeUiLabel, riskListSecondaryLabel } from '../lib/riskDisplay';
import { DataQualityNotice } from '../components/DataQualityNotice';
import { RiskFindingsSavedViews } from '../components/RiskFindingsSavedViews';
import { runtimeSignalVisual } from '../lib/runtimeSignalVisual';
import { formatMinutesHuman } from '../lib/formatDuration';
import { formatDateTime } from '../lib/display';
import {
  UI_TABLE,
  UI_TD,
  UI_TD_COMPACT,
  UI_TD_COMPACT_TIGHT,
  UI_TH,
  UI_TH_COMPACT,
  UI_TR,
  UI_THEAD_STICKY,
} from '../lib/tableChrome';
import {
  UI_PILL_ACTIVE,
  UI_PILL_ACTIVE_ELEVATED,
  UI_PILL_IDLE_ROUNDED,
  UI_PILL_IDLE_SPLIT,
} from '../lib/formChrome';
import { can, P } from '../lib/permissions';
import { usePermUser } from '../hooks/usePermUser';
import { ACTION_IDS, canRunAction } from '../lib/actionAccess';
import { usePersona } from '../hooks/usePersona';
import { personaAllowsAction, tableDensityClass } from '../lib/persona';
import { getRiskWorkspaceConfig, type RiskTabId } from '../lib/personaRiskWorkspace';
import { RiskPersonaWorkspaceBanner } from '../components/RiskPersonaWorkspaceBanner';
import { ProvenanceBadge } from '../design-system/components/ProvenanceBadge';
import { insightProvenance, insightProvenanceTitle, severityCountTrustLabel } from '../lib/provenance';
import { PageContract } from '../components/PageContract';
import { SemanticEmptyState } from '../design-system/components/SemanticEmptyState';
import { getChartThemeColors } from '../lib/chartTheme';
import {
  attackNarrativeHint,
  blastRadiusBuckets,
  criticalAttackPathSplit,
  exploitabilityBuckets,
  histogramScoreTailInsight,
  newVsExistingFindings,
  privilegedDriftHint,
  trendSpikeAnnotation,
  velocityFromTrend,
} from '../lib/riskOperatorAnalytics';
import { buildEvidenceLogEntries, summarizeEvidenceLog } from '../lib/evidenceLog';

type TabId = 'overview' | 'triage' | 'pce' | 'reference';
type BulkFindingAction = 'acknowledge' | 'resolve' | 'dismiss';

/** Shared table chrome for Risk Center data tables */
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

type FindingResource = NonNullable<Insight['affectedResources']>[number];

function normalizeFindingResourceKind(kind?: string): string {
  return String(kind ?? '')
    .trim()
    .toLowerCase()
    .replace(/[\s_-]+/g, '');
}

function findingResourceKindLabel(kind?: string): string {
  const normalized = normalizeFindingResourceKind(kind);
  if (normalized === 'pod') return 'Pod';
  if (normalized === 'serviceaccount') return 'ServiceAccount';
  return String(kind ?? 'Resource').trim() || 'Resource';
}

function findingResourceDisplayName(resource?: FindingResource): string {
  if (!resource) return '';
  const name = String(resource.name ?? '').trim();
  const namespace = String(resource.namespace ?? '').trim();
  if (name && namespace) return `${namespace}/${name}`;
  if (name) return name;
  return String(resource.id ?? '').trim();
}

function findingPodUid(insight: Insight): string | undefined {
  const resource = insight.affectedResources?.find((r) => normalizeFindingResourceKind(r.kind) === 'pod' && String(r.id ?? '').trim());
  const uid = String(resource?.id ?? '').trim();
  return uid || undefined;
}

function findingResourceTarget(resource?: FindingResource): string | undefined {
  const id = String(resource?.id ?? '').trim();
  if (!resource || !id) return undefined;
  const kind = normalizeFindingResourceKind(resource.kind);
  if (kind === 'pod') return `/resources/pods/uid/${encodeURIComponent(id)}`;
  if (kind === 'serviceaccount') return `/identities/uid/${encodeURIComponent(id)}`;
  return undefined;
}

export const RiskCenter: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { selectedClusterId, setSelectedClusterId } = useClusterStore();
  const permUser = usePermUser();
  const { id: personaId, profile } = usePersona();
  const riskWorkspace = getRiskWorkspaceConfig(personaId);
  const canBulkFindings = personaAllowsAction(profile, 'bulk', permUser, P.findingsBulk);
  const canExportFindings = canRunAction(permUser, ACTION_IDS.findingExport);
  const canDrawerAck = canRunAction(permUser, ACTION_IDS.findingAcknowledge);
  const canDrawerResolve = canRunAction(permUser, ACTION_IDS.findingResolve);
  const canDrawerDismiss = canRunAction(permUser, ACTION_IDS.findingDismiss);
  const canRiskEvaluate = canRunAction(permUser, ACTION_IDS.riskEvaluate);
  const canPlatformAudit = can(permUser, P.systemAuditRead);
  const { valueMinutes: timeWindowMinutes, setValueMinutes: setTimeWindowMinutes } = useTimeWindowStore();
  const [searchParams, setSearchParams] = useSearchParams();
  // `severity` is accepted only as a legacy deep-link alias for finalLevel.
  const finalLevelFromUrl = searchParams.get('finalLevel') ?? searchParams.get('severity');
  const searchFromUrl = searchParams.get('search') ?? '';
  const clusterIdFromUrl = searchParams.get('clusterId');
  const sinceMinutesFromUrl = searchParams.get('sinceMinutes');
  const [activeTab, setActiveTab] = useState<TabId>('overview');
  const [risks, setRisks] = useState<Insight[]>([]);
  const [risksTotal, setRisksTotal] = useState(0);
  const [pceSummary, setPceSummary] = useState<PodCapabilitySummarySeverity[]>([]);
  const [pceDetails, setPceDetails] = useState<PodCapabilityDetail[]>([]);
  const { clusters } = useClusters();
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
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'resolved' | 'acknowledged'>('active');
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedRisk, setSelectedRisk] = useState<Insight | null>(null);

  const [resolved24h, setResolved24h] = useState<number>(0);
  const [insightsSummary, setInsightsSummary] = useState<{
    total: number;
    critical: number;
    high: number;
    medium: number;
    low: number;
    byType?: Record<string, number>;
    riskLevelCounts?: InsightsSummary['riskLevelCounts'];
  } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [risksPage, setRisksPage] = useState(1);
  const [risksPageSize, setRisksPageSize] = useState(riskWorkspace.pageSize);
  const [threatVelocity, setThreatVelocity] = useState<{ date: string; critical: number; high: number; medium: number; low: number }[]>([]);
  const [riskSort, setRiskSort] = useState<'newest' | 'oldest' | 'score_desc' | 'score_asc' | 'title_asc'>('score_desc');
  /** Risk Findings table: per-resource rows vs grouped by CVE/title key (backend view=group). */
  const [findingsListView, setFindingsListView] = useState<'instance' | 'group'>('instance');
  const [riskLevelFilter, setRiskLevelFilter] = useState<string>(
    finalLevelFromUrl && ['critical', 'high', 'medium', 'low'].includes(finalLevelFromUrl) ? finalLevelFromUrl : ''
  ); // ADR: low|medium|high|critical → API finalLevel
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

  const hasLoadedOnceRef = React.useRef(false);

  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [pendingBulkAction, setPendingBulkAction] = useState<BulkFindingAction | null>(null);
  const [bulkActionReason, setBulkActionReason] = useState('');
  const [bulkActionBusy, setBulkActionBusy] = useState(false);
  const bulkActionDialogRef = useRef<HTMLDivElement>(null);
  const chartTheme = getChartThemeColors();

  /** Risk Score Distribution histogram (GET /risk/histogram) */
  const [histogramData, setHistogramData] = useState<RiskHistogramResponse | null>(null);
  const [histogramLoading, setHistogramLoading] = useState(false);
  /** When user clicks a histogram bar, filter findings to this score bin (e.g. 10 = 10–20) */
  const [selectedScoreBin, setSelectedScoreBin] = useState<number | null>(null);
  const [trendDays, setTrendDays] = useState<number>(7);
  /** Recalculate all scores: loading and success message (POST /risk/scores/sync) */
  const [syncScoresLoading, setSyncScoresLoading] = useState(false);
  const [syncScoresMessage, setSyncScoresMessage] = useState<string | null>(null);
  const [attackStepsSummary, setAttackStepsSummary] = useState<AttackStepSummary[]>([]);
  const [riskPipelineHealth, setRiskPipelineHealth] = useState<PipelineHealth | null>(null);
  const [riskFindingsCols, setRiskFindingsCols] = useState<RiskFindingsTableCols>(() => loadRiskFindingsCols());

  // Resolve cluster + time: URL from Dashboard link overrides store so Risk Center shows same scope
  const effectiveClusterId = clusterIdFromUrl ?? selectedClusterId ?? undefined;
  const effectiveSinceMinutes = sinceMinutesFromUrl != null ? parseInt(sinceMinutesFromUrl, 10) : timeWindowMinutes;
  const effectiveSinceMinutesNum = Number.isFinite(effectiveSinceMinutes) && effectiveSinceMinutes > 0 ? effectiveSinceMinutes : undefined;

  /** Phase 2 design: /risks = Overview only (KPI, trend, histogram, Quick Links). /risks/findings = full table. */
  const isOverviewPage = location.pathname === '/risks' || location.pathname.replace(/\/$/, '') === '/risks';
  /** Export loading for UX feedback (tooltip 10k, filter warning). */
  const [exportLoading, setExportLoading] = useState(false);
  const insightIdFromUrl = searchParams.get('insightId');
  const podUidFromEvidenceUrl = searchParams.get('podUid');
  const riskTrendChartRef = React.useRef<HTMLDivElement | null>(null);
  const [riskTrendChartWidth, setRiskTrendChartWidth] = useState(0);
  const suppressedInsightOpenRef = React.useRef<string | null>(null);

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
    setRiskFindingsCols((prev) => ({ ...prev, ...riskWorkspace.defaultCols }));
    setRisksPageSize(riskWorkspace.pageSize);
  }, [personaId]); // eslint-disable-line react-hooks/exhaustive-deps

  const tableCellClass = profile.tableDensity === 'compact' ? UI_TD_COMPACT : UI_TD;
  const tableHeadClass = profile.tableDensity === 'compact' ? UI_TH_COMPACT : UI_TH;



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

  // Sync active tab with route: /risks, /risks/findings, /risks/pce, /risks/evidence
  React.useEffect(() => {
    const path = location.pathname || '';
    if (path.endsWith('/pce')) {
      setActiveTab('pce');
    } else if (path.endsWith('/evidence')) {
      setActiveTab('reference');
    } else if (path.endsWith('/findings')) {
      setActiveTab('triage');
    } else {
      setActiveTab('overview');
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
    if (finalLevelFromUrl && ['critical', 'high', 'medium', 'low'].includes(finalLevelFromUrl)) {
      setRiskLevelFilter(finalLevelFromUrl);
    }
  }, [finalLevelFromUrl]);
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

  const getAbortSignal = useAbortSignal();

  const fetchData = useCallback(async () => {
    const signal = getAbortSignal();
    const isFirst = !hasLoadedOnceRef.current;
    if (!isFirst) setRefreshing(true);
    setError(null);
    const num = (v: unknown) => (typeof v === 'number' ? v : Number(v) || 0);
    const sinceMinutes = sinceMinutesForApi;
    const clusterId = effectiveClusterId ?? selectedClusterId ?? undefined;
    const useScores = riskSort === 'score_desc' || riskSort === 'score_asc' || riskLevelFilter !== '';
    const risksListParamsBase = {
      status: statusFilter,
      search: debouncedSearchTerm || undefined,
      clusterId: clusterId ?? undefined,
      namespace: namespaceFilter.trim() || undefined,
      type: typeFilter || undefined,
      sinceMinutes,
      withScores: useScores ? 1 : undefined,
      finalLevel: (riskLevelFilter || undefined) as '' | 'low' | 'medium' | 'high' | 'critical' | undefined,
      scoreBin: selectedScoreBin ?? undefined,
      view: findingsListView === 'group' ? ('group' as const) : undefined,
    };
    const countSeverity = (insights: Insight[]) => {
      const bySev = { critical: 0, high: 0, medium: 0, low: 0 };
      insights.forEach((r: Insight) => {
        const sev = (r.severity || '').toLowerCase();
        if (sev in bySev) (bySev as Record<string, number>)[sev]++;
      });
      return bySev;
    };
    const countRiskLevel = (insights: Insight[]) => {
      const byLevel = { critical: 0, high: 0, medium: 0, low: 0 };
      insights.forEach((r: Insight) => {
        const level = (r.finalLevel || '').toLowerCase();
        if (level in byLevel) (byLevel as Record<string, number>)[level]++;
      });
      return byLevel;
    };
    const runPceBlock = async (errors: string[]) => {
      const pceDrillCluster = (pceClusterId || '').trim() || clusterId;
      const pceResults = await Promise.allSettled([
        api.getPceSummaryBySeverity({ clusterId: pceDrillCluster || undefined }),
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
      const loadRisksHeavy = activeTab === 'overview' || activeTab === 'triage' || activeTab === 'reference';
      if (!loadRisksHeavy) {
        const errors: string[] = [];
        const lightResults = await Promise.allSettled([
          api.getInsightsSummary(clusterId ?? undefined, sinceMinutes),
          api.getStats(clusterId ?? undefined, sinceMinutes, 'all'),
        ]);
        const [summaryResult, statsResult] = lightResults;

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
            byType: s?.byType && typeof s.byType === 'object' ? (s.byType as Record<string, number>) : undefined,
            riskLevelCounts: s?.riskLevelCounts,
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
        const overviewRisksPageSize = isOverviewPage || activeTab === 'reference' ? 250 : risksPageSize;
        const risksPromise = api.getRisks({
          page: activeTab === 'reference' ? 1 : risksPage,
          pageSize: overviewRisksPageSize,
          ...risksListParamsBase,
        });
        const summaryPromise = api.getInsightsSummary(clusterId ?? undefined, sinceMinutes);
        const threatPromise = api.getThreatVelocity(trendDays, clusterId ?? undefined).catch(() => []);

        const statsPromise = api.getStats(clusterId ?? undefined, sinceMinutes, 'all');
        const byClusterPromise = !clusterId ? api.getInsightsSummaryByCluster(sinceMinutes) : Promise.resolve([] as InsightsSummaryByClusterItem[]);
        const attackStepsPromise = api.getAttackStepsSummary();
        const pipelineHealthPromise = api.getPipelineHealth();
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
          statsPromise,
          byClusterPromise,
          histogramPromise,
          attackStepsPromise,
          pipelineHealthPromise,
        ]);
        const [
          risksResult,
          summaryResult,
          threatResult,
          statsResult,
          byClusterResult,
          _histogramSettled,
          attackStepsResult,
          pipelineHealthResult,
        ] = coreResults;
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
            byType: s?.byType && typeof s.byType === 'object' ? (s.byType as Record<string, number>) : undefined,
            riskLevelCounts: s?.riskLevelCounts,
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
              view: 'instance',
            });
            const bySev = countSeverity(wide.insights);
            const byLevel = countRiskLevel(wide.insights);
            const wideTotal = num(wide.total);
            setInsightsSummary({
              total: wideTotal,
              critical: bySev.critical,
              high: bySev.high,
              medium: bySev.medium,
              low: bySev.low,
              riskLevelCounts: byLevel,
            });
            setSeverityCountTrust(wide.insights.length >= wideTotal ? 'exact' : 'sample');
          } catch {
            const bySev = countSeverity(insights);
            const byLevel = countRiskLevel(insights);
            setInsightsSummary({
              total: totalN,
              critical: bySev.critical,
              high: bySev.high,
              medium: bySev.medium,
              low: bySev.low,
              riskLevelCounts: byLevel,
            });
            setSeverityCountTrust('page');
          }
        } else {
          setSeverityCountTrust('exact');
          setInsightsSummary(null);
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
        if (attackStepsResult.status === 'fulfilled') {
          setAttackStepsSummary(attackStepsResult.value);
        } else {
          setAttackStepsSummary([]);
        }
        if (pipelineHealthResult.status === 'fulfilled') {
          setRiskPipelineHealth(pipelineHealthResult.value);
        } else {
          setRiskPipelineHealth(null);
        }
        if (errors.length > 0) setError(errors.join('; '));
      }
    } catch (e) {
      if (isAbortError(e)) return; // cancelled by rapid filter change — discard silently
      throw e;
    } finally {
      hasLoadedOnceRef.current = true;
      setPageBlocking(false);
      setRefreshing(false);
    }
  }, [
    getAbortSignal,
    risksPage,
    risksPageSize,
    statusFilter,
    debouncedSearchTerm,
    effectiveClusterId,
    sinceMinutesForApi,
    selectedClusterId,
    riskSort,
    riskLevelFilter,
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
    findingsListView,
    isOverviewPage,
  ]);

  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.SBOM_RISK_LIST));
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);
  usePolling(fetchData, intervalMs, { refreshTrigger });
  const fetchDataRef = React.useRef(fetchData);
  fetchDataRef.current = fetchData;

  const openBulkActionReview = (action: BulkFindingAction) => {
    if (selectedIds.size === 0) return;
    setPendingBulkAction(action);
    setBulkActionReason('');
    setToast(null);
  };

  const runBulkAction = async () => {
    if (!pendingBulkAction || bulkActionBusy) return;
    const ids = Array.from(selectedIds);
    if (!ids.length) return;
    const reason = bulkActionReason.trim();
    if ((pendingBulkAction === 'resolve' || pendingBulkAction === 'dismiss') && reason.length < 8) {
      setToast({ message: 'Resolution or dismissal requires a reason of at least 8 characters.', variant: 'error' });
      return;
    }
    setBulkActionBusy(true);
    try {
      await api.bulkInsightsAction({
        action: pendingBulkAction,
        insightIds: ids,
        resolution: pendingBulkAction === 'resolve' ? reason : undefined,
        reason: pendingBulkAction === 'dismiss' ? reason : undefined,
      });
      setSelectedIds(new Set());
      setPendingBulkAction(null);
      setBulkActionReason('');
      setToast({ message: `${ids.length} finding(s) updated.`, variant: 'success' });
      fetchDataRef.current();
    } catch (err) {
      setToast({ message: String(err instanceof Error ? err.message : err), variant: 'error' });
    } finally {
      setBulkActionBusy(false);
    }
  };

  React.useEffect(() => {
    const wsUrl = api.getRisksWsUrl();
    let ws: WebSocket | null = null;
    try {
      ws = new WebSocket(wsUrl);
      ws.onmessage = (e) => {
        try {
          const d = JSON.parse(e.data as string) as { type?: string; insightId?: string };
          if (d?.type === 'insights_updated') {
            // Reconcile entity store: invalidate stale cached insights
            if (d.insightId) {
              useEntityStore.getState().invalidateInsight(d.insightId);
            }
            fetchDataRef.current();
          }
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

  React.useEffect(() => { setRisksPage(1); }, [statusFilter, debouncedSearchTerm, timeWindowMinutes, riskLevelFilter, namespaceFilter, typeFilter]);
  React.useEffect(() => { setSelectedScoreBin(null); }, [effectiveClusterId, effectiveSinceMinutesNum]);

  // Global drawer: open by URL ?insightId= (from Overview/PCE/Evidence deep link)
  React.useEffect(() => {
    if (!insightIdFromUrl) {
      suppressedInsightOpenRef.current = null;
      return;
    }
    const id = insightIdFromUrl.trim();
    if (!id) return;
    if (suppressedInsightOpenRef.current === id) return;
    if (selectedRisk?.id === id) return;
    const inList = risks.find((r) => r.id === id);
    if (inList) {
      setSelectedRisk(inList);
      return;
    }
    api.getInsight(id).then((detail) => {
      if (!detail) return;
      const d = detail as Insight & Record<string, unknown>;
      const insightTypeRaw = (d.insightType ?? d.insight_type ?? 'vulnerability') as string;
      const ts = (d.timestamp ?? d.detectedAt ?? d.createdAt) as string | undefined;
      const resUid = (d.affectedResources?.[0]?.id ?? d.resourceUid ?? d.resource_uid) as string | undefined;
      const resName = (d.affectedResources?.[0]?.name ?? d.resourceName ?? d.resource_name) as string | undefined;
      const resType = (d.affectedResources?.[0]?.kind ?? d.resourceType ?? d.resource_type ?? 'Pod') as string;
      const resNs = (d.affectedResources?.[0]?.namespace ?? d.resourceNamespace ?? d.resource_namespace) as string | undefined;
      const asInsight: Insight = {
        id: d.id,
        title: d.title ?? '',
        description: d.description,
        severity: String(d.severity ?? 'medium').toLowerCase() as Insight['severity'],
        score: d.totalScore != null ? Math.round(Number(d.totalScore)) : undefined,
        insightType: insightTypeRaw ? String(insightTypeRaw) : undefined,
        status: normalizeInsightStatus(d.status) as Insight['status'],
        timestamp: ts != null ? String(ts) : undefined,
        clusterId: '',
        clusterName: '',
        affectedResources: resUid ? [{ id: String(resUid), name: resName, kind: resType, namespace: resNs }] : [],
        totalScore: d.totalScore,
      };
      setSelectedRisk(asInsight);
    }).catch(() => {});
  }, [insightIdFromUrl, risks, selectedRisk?.id]);

  const closeRiskDrawer = React.useCallback(() => {
    const idToSuppress = selectedRisk?.id ?? searchParams.get('insightId');
    if (idToSuppress) suppressedInsightOpenRef.current = idToSuppress;
    setSelectedRisk(null);
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      next.delete('insightId');
      return next;
    }, { replace: true });
  }, [searchParams, selectedRisk?.id, setSearchParams]);

  const openRiskDrawer = React.useCallback((risk: Insight) => {
    suppressedInsightOpenRef.current = null;
    setSelectedRisk(risk);
  }, []);

  const toggleRiskDrawer = React.useCallback((risk: Insight) => {
    if (selectedRisk?.id === risk.id) {
      closeRiskDrawer();
      return;
    }
    openRiskDrawer(risk);
  }, [closeRiskDrawer, openRiskDrawer, selectedRisk?.id]);

  // Sync drawer open state to URL (so bookmark/share works)
  React.useEffect(() => {
    const currentInsightId = searchParams.get('insightId');
    if (selectedRisk) {
      if (currentInsightId === selectedRisk.id) return;
      setSearchParams((prev) => {
        const next = new URLSearchParams(prev);
        next.set('insightId', selectedRisk.id);
        return next;
      }, { replace: true });
    } else {
      if (!currentInsightId) return;
      setSearchParams((prev) => {
        const next = new URLSearchParams(prev);
        next.delete('insightId');
        return next;
      }, { replace: true });
    }
  }, [selectedRisk, searchParams, setSearchParams]);


  const getSeverityIcon = (severity: Insight['severity']) => {
    switch (severity) {
      case 'critical': return <Shield className="w-5 h-5 text-critical" />;
      case 'high': return <AlertTriangle className="w-5 h-5 text-high" />;
      case 'medium': return <Info className="w-5 h-5 text-medium" />;
      case 'low': return <CheckCircle className="w-5 h-5 text-low" />;
      default: return <Info className="w-5 h-5 text-muted" aria-label={severity ? String(severity) : 'Unknown severity'} />;
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
    setRisksPage(1);
  }, [findingsListView]);

  React.useEffect(() => {
    const el = riskTrendChartRef.current;
    if (!el || typeof ResizeObserver === 'undefined') return undefined;
    const update = () => {
      setRiskTrendChartWidth(Math.max(0, Math.floor(el.getBoundingClientRect().width)));
    };
    update();
    const observer = new ResizeObserver(update);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);


  // Primary Risk Operations KPI: unified risk level from risk_scores.final_level.
  // Raw finding severity remains available in the API but is not used for drill-down totals.
  const riskLevelBar = {
    total: Number(insightsSummary?.total ?? risksTotal ?? 0),
    critical: Number(insightsSummary?.riskLevelCounts?.critical ?? risks.filter(r => (r.finalLevel || '').toLowerCase() === 'critical').length),
    high: Number(insightsSummary?.riskLevelCounts?.high ?? risks.filter(r => (r.finalLevel || '').toLowerCase() === 'high').length),
    medium: Number(insightsSummary?.riskLevelCounts?.medium ?? risks.filter(r => (r.finalLevel || '').toLowerCase() === 'medium').length),
    low: Number(insightsSummary?.riskLevelCounts?.low ?? risks.filter(r => (r.finalLevel || '').toLowerCase() === 'low').length),
  };
  const riskDataEmpty =
    risksTotal === 0 &&
    !debouncedSearchTerm &&
    !riskLevelFilter &&
    !namespaceFilter &&
    !typeFilter &&
    statusFilter === 'active';

  const critPathSplit = useMemo(() => criticalAttackPathSplit(risks), [risks]);
  const velocityStats = useMemo(() => velocityFromTrend(threatVelocity), [threatVelocity]);
  const trendSpike = useMemo(() => trendSpikeAnnotation(threatVelocity), [threatVelocity]);
  const histogramTail = useMemo(() => histogramScoreTailInsight(histogramData), [histogramData]);
  const newExisting = useMemo(() => newVsExistingFindings(risks, 24), [risks]);
  const exploitBuckets = useMemo(() => exploitabilityBuckets(risks), [risks]);
  const blastBuckets = useMemo(() => blastRadiusBuckets(risks), [risks]);
  const privilegedDrift = useMemo(() => privilegedDriftHint(risks), [risks]);
  const attackNarrative = useMemo(() => attackNarrativeHint(attackStepsSummary), [attackStepsSummary]);
  const findingEvidenceRows = useMemo(
    () =>
      risks
        .map((risk) => ({
          risk,
          entries: buildEvidenceLogEntries(risk),
          summary: summarizeEvidenceLog(risk),
        }))
        .filter((row) => row.entries.length > 0),
    [risks],
  );
  const riskTrendChartData = useMemo(
    () =>
      threatVelocity
        .map((p) => ({
          ...p,
          name: p.date,
          risk: (p.critical ?? 0) + (p.high ?? 0) + (p.medium ?? 0) + (p.low ?? 0),
        }))
        .sort((a, b) => a.name.localeCompare(b.name)),
    [threatVelocity],
  );
  const spikeChartIdx = useMemo(() => {
    if (!trendSpike) return -1;
    return riskTrendChartData.findIndex((d) => d.name === trendSpike.date);
  }, [trendSpike, riskTrendChartData]);
  const riskComputationFreshness = useMemo(() => {
    const iso = riskPipelineHealth?.layer4?.lastScoreCalc;
    if (!iso) return { ago: '—', staleness: '—' as string };
    const min = (Date.now() - new Date(iso).getTime()) / 60000;
    const staleness = min < 10 ? 'LOW' : min < 45 ? 'MED' : 'HIGH';
    const ago =
      min < 1 ? '<1m ago' : min < 60 ? `${Math.round(min)}m ago` : `${Math.round(min / 60)}h ago`;
    return { ago, staleness };
  }, [riskPipelineHealth]);

  const severityRank: Record<string, number> = { critical: 4, high: 3, medium: 2, low: 1 };

  const RiskTrendTooltipContent = ({ active, payload, label }: { active?: boolean; payload?: any[]; label?: string }) => {
    if (!active || !payload || !payload.length) return null;
    const row = payload[0]?.payload as { date: string; critical?: number; high?: number; medium?: number; low?: number; risk?: number };
    if (!row) return null;
    const total = row.risk ?? ((row.critical ?? 0) + (row.high ?? 0) + (row.medium ?? 0) + (row.low ?? 0));
    return (
      <div className="bg-surface border border-border rounded-lg shadow-xl p-3 text-left min-w-[180px]">
        <div className="text-text font-medium">Date: {label}</div>
        <div className="text-muted text-body mt-1">
          Total risks: <span className="text-text font-semibold">{total}</span>
        </div>
        <div className="flex flex-wrap gap-x-3 gap-y-0.5 mt-2 text-caption">
          <span className="text-red-400">Critical: {row.critical ?? 0}</span>
          <span className="text-orange-400">High: {row.high ?? 0}</span>
          <span className="text-yellow-400">Medium: {row.medium ?? 0}</span>
          <span className="text-blue-400">Low: {row.low ?? 0}</span>
        </div>
      </div>
    );
  };

  const renderRiskHistogramCard = () => (
    <div className="bg-surface border border-border rounded-lg p-3 md:p-4 flex flex-col">
      <div className="flex flex-wrap items-center justify-between gap-2 mb-2">
        <h2 className="text-caption font-semibold text-muted uppercase tracking-wider">Priority score distribution</h2>
        <Button
          variant="secondary"
          size="sm"
          disabled={syncScoresLoading || !canRiskEvaluate}
          title={
            canRiskEvaluate
              ? 'Recalculate priority scores for all resources with active findings.'
              : 'Requires risk.evaluate permission (operator or admin).'
          }
          onClick={async () => {
            setSyncScoresLoading(true);
            setSyncScoresMessage(null);
            try {
              const res = await api.syncRiskScores();
              setSyncScoresMessage(
                res.resources === 0
                  ? 'No resources to sync.'
                  : `Sync started for ${res.resources} resources. Refreshing in a few seconds…`,
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
      {syncScoresMessage && <p className="text-caption text-muted mb-2">{syncScoresMessage}</p>}
      {histogramData?.totalFindings === 0 && !histogramLoading ? (
        <div className="py-3 text-center">
          <p className="text-caption text-muted-2">
            No score data yet. Click &quot;Recalculate all scores&quot; to compute priority scores for all resources with findings.
          </p>
          <p className="text-caption text-muted-2 mt-1">Bins 0–10 … 90–100. Stacked by severity. Click a bar to filter findings by score range.</p>
        </div>
      ) : (
        <>
          <p className="text-caption text-muted-2 mb-3">
            Bins 0–10 … 90–100. Stacked by severity. Click a bar to filter the table to that score range. Ref lines: P0 (90), P1 (70).
          </p>
          {selectedScoreBin != null && (
            <p className="text-caption text-muted mb-2">
              Filtered to score{' '}
              <span className="font-medium text-brand">
                {selectedScoreBin}–{selectedScoreBin + 10}
              </span>
              <button
                type="button"
                onClick={() => {
                  setSelectedScoreBin(null);
                  setRisksPage(1);
                  fetchDataRef.current();
                }}
                className="ml-2 text-brand hover:underline"
              >
                Clear filter
              </button>
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
        </>
      )}
      {isOverviewPage && histogramTail ? (
        <p className="text-caption text-sky-200/85 mt-3 leading-relaxed border-t border-border/50 pt-2">{histogramTail}</p>
      ) : null}
    </div>
  );

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
  const latestRiskUpdate = useMemo(() => {
    const latest = risks
      .map((r) => r.updatedAt ?? r.timestamp)
      .filter((t): t is string => Boolean(t))
      .map((t) => new Date(t).getTime())
      .filter((t) => Number.isFinite(t))
      .sort((a, b) => b - a)[0];
    return latest ? formatDateTime(new Date(latest).toISOString()) : '—';
  }, [risks]);

  const statusLabelMap: Record<string, string> = {
    dismissed: 'Dismissed',
    unknown: 'Unknown',
    new: 'Active',
    acknowledged: 'In review',
    resolved: 'Resolved',
  };

  const findingsTableColCount = useMemo(() => {
    let n = 4; // level, finding, workflow, actions
    if (riskWorkspace.showRowSelection) n += 1;
    if (riskWorkspace.showProvenanceColumn) n += 1;
    if (riskFindingsCols.type) n += 1;
    if (riskFindingsCols.resource) n += 1;
    if (riskFindingsCols.score) n += 1;
    if (!effectiveClusterId && riskFindingsCols.nsCluster) n += 1;
    if (riskFindingsCols.detected) n += 1;
    if (riskFindingsCols.updated) n += 1;
    return n;
  }, [riskFindingsCols, effectiveClusterId, riskWorkspace.showRowSelection, riskWorkspace.showProvenanceColumn]);

  const pendingBulkActionLabel = pendingBulkAction === 'acknowledge'
    ? 'Acknowledge'
    : pendingBulkAction === 'resolve'
      ? 'Resolve'
      : pendingBulkAction === 'dismiss'
        ? 'Dismiss'
        : '';
  const pendingBulkReasonRequired = pendingBulkAction === 'resolve' || pendingBulkAction === 'dismiss';
  const pendingBulkSubmitDisabled =
    bulkActionBusy || (pendingBulkReasonRequired && bulkActionReason.trim().length < 8);

  useEffect(() => {
    if (!pendingBulkAction) return;
    const root = bulkActionDialogRef.current;
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        setPendingBulkAction(null);
        setBulkActionReason('');
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
      root?.querySelector<HTMLElement>('button, [href], textarea, input')?.focus();
    }, 0);
    return () => {
      window.clearTimeout(t);
      window.removeEventListener('keydown', onKeyDown, true);
      previousFocus?.focus();
    };
  }, [pendingBulkAction]);

  if (pageBlocking) {
    return (
      <PageContract feature="risk_operations" loading loadingMessage="Loading Risk Operations…">
        <PageLayout title={PAGE_TITLES.riskOperations} description={RISK_CENTER_DESCRIPTION}>
        <div className="space-y-6 animate-pulse" aria-busy="true" aria-label="Loading Risk Operations">
          <div className="h-10 bg-surface-2/80 rounded-lg border border-border" />
          <div className="h-14 bg-surface-2/60 rounded-lg border border-border" />
          <div className="grid gap-3 sm:grid-cols-3">
            <div className="h-28 bg-surface-2/60 rounded-lg border border-border" />
            <div className="h-28 bg-surface-2/60 rounded-lg border border-border" />
            <div className="h-28 bg-surface-2/60 rounded-lg border border-border" />
          </div>
          <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
            <div className="h-[220px] bg-surface-2/40 rounded-lg border border-border" />
            <div className="h-[220px] bg-surface-2/40 rounded-lg border border-border" />
          </div>
          <div className="h-40 bg-surface-2/40 rounded-lg border border-border" />
        </div>
      </PageLayout>
      </PageContract>
    );
  }

  const allTabs: { id: TabId; label: string }[] = [
    { id: 'overview', label: 'Summary' },
    { id: 'triage', label: 'Findings queue' },
    { id: 'pce', label: 'Exposure' },
    { id: 'reference', label: 'Evidence' },
  ];
  const tabs = allTabs.filter((t) => riskWorkspace.visibleTabs.includes(t.id as RiskTabId));
  const routeObjective: Record<TabId, { title: string; body: string; api: string }> = {
    overview: {
      title: 'Risk Operations = one prioritization workflow',
      body: 'Start with unified risk levels, drill into the queue, use exposure to explain why a pod is risky, then verify with evidence.',
      api: 'risk summary, risk list, score histogram',
    },
    triage: {
      title: 'Work the active findings queue',
      body: 'Filter by unified risk level, inspect impacted resources, and update workflow status.',
      api: 'GET /risk/insights, export APIs',
    },
    pce: {
      title: 'Explain pod capability exposure',
      body: 'Inventory-derived exposure records. They explain why risk exists; they are not counted as findings.',
      api: 'inventory pod-capability APIs',
    },
    reference: {
      title: 'Inspect supporting evidence',
      body: 'Normalized evidence and runtime signals for findings loaded in the current scope.',
      api: 'finding evidence, runtime signals',
    },
  };
  return (
    <PageContract
      feature="risk_operations"
      dataEmpty={riskDataEmpty}
      telemetry={{ pipelineDegraded: severityCountTrust !== 'exact' ? true : undefined }}
    >
    <PageLayout
      title={PAGE_TITLES.riskOperations}
      description={RISK_CENTER_DESCRIPTION}
    >
      <div className="contents">
      {/* Error banner when some APIs failed */}
      {error && (
        <div className="flex items-center justify-between gap-4 p-4 bg-amber-500/10 border border-amber-500/30 rounded-lg text-amber-200">
          <div className="flex items-center gap-2 min-w-0">
            <AlertTriangle className="w-5 h-5 text-amber-500 shrink-0" />
            <span className="text-body truncate" title={error}>{error}</span>
          </div>
          <Button variant="secondary" size="sm" onClick={() => { setError(null); fetchData(); }}>
            Retry
          </Button>
        </div>
      )}
      {refreshing && (
        <div className="flex items-center gap-2 px-3 py-2 text-caption text-muted border border-border rounded-lg bg-surface/80">
          <Loader2 className="w-3.5 h-3.5 animate-spin shrink-0" />
          Updating data…
        </div>
      )}

      <RiskPersonaWorkspaceBanner personaId={personaId} />

      {/* Tabs */}
      <Tabs
        items={tabs}
        value={activeTab}
        onChange={(id) => {
          const tabId = id as TabId;
          closeRiskDrawer();
          const sp = new URLSearchParams(searchParams);
          sp.delete('insightId');
          const qs = sp.toString();
          const suffix = qs ? `?${qs}` : '';
          if (tabId === 'overview') {
            navigate(`/risks${suffix}`);
          } else if (tabId === 'triage') {
            navigate(`/risks/findings${suffix}`);
          } else if (tabId === 'pce') {
            navigate(`/risks/pce${suffix}`);
          } else {
            navigate(`/risks/evidence${suffix}`);
          }
        }}
      />
      <div className="rounded-lg border border-border bg-surface/70 px-4 py-3">
        <div className="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
          <div className="min-w-0">
            <h2 className="text-body font-semibold text-text">{routeObjective[activeTab].title}</h2>
            <p className="mt-1 max-w-3xl text-caption text-muted">{routeObjective[activeTab].body}</p>
          </div>
          <div className="shrink-0 rounded-md border border-border/70 bg-base/40 px-2.5 py-1 text-caption text-muted xl:text-right">
            Metric: <span className="text-text">Unified risk level</span>
            <span className="mx-1 text-muted-2">·</span>
            Data: <span className="text-text">{routeObjective[activeTab].api}</span>
          </div>
        </div>
      </div>
      <div className="mt-3 flex flex-col gap-3 rounded-lg border border-border bg-surface/70 px-4 py-3 text-caption text-muted lg:flex-row lg:items-center lg:justify-between">
        <div className="grid gap-2 sm:grid-cols-3 lg:flex lg:flex-wrap lg:items-center lg:gap-x-4 lg:gap-y-1">
          {activeTab !== 'overview' && activeTab !== 'triage' && (
            <span title="Count of active findings in current scope. On Findings queue, see Priority overview for the same total.">
              Active findings: <span className="text-text font-medium">{riskLevelBar.total}</span>
            </span>
          )}
          <span>
            Scope:{' '}
            <span className="text-text font-medium" title={effectiveClusterId ?? undefined}>
              {scopeClusterDisplay ? `cluster: ${scopeClusterDisplay}` : 'all clusters'}
            </span>
          </span>
          <span>
            Window: <span className="text-text font-medium">{formatMinutesHuman(sinceMinutesForApi)}</span>
          </span>
          <span>
            Updated: <span className="text-text font-medium">{latestRiskUpdate}</span>
          </span>
        </div>
        {activeTab === 'triage' && (
          <div className="flex flex-wrap gap-2 lg:justify-end">
            <Button
              variant="secondary"
              size="sm"
              disabled={exportLoading || !canExportFindings}
              title={
                canExportFindings
                  ? 'Max 10,000 rows. Current filters (cluster, time, risk level, status, search) apply.'
                  : 'Requires export.findings permission (server-enforced on download).'
              }
              onClick={async () => {
                setExportLoading(true);
                try {
                  await api.exportRisksCSV({
                    clusterId: effectiveClusterId ?? undefined,
                    sinceMinutes: sinceMinutesForApi,
                    status: statusFilter,
                    finalLevel: riskLevelFilter || undefined,
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
              disabled={exportLoading || !canExportFindings}
              title={
                canExportFindings
                  ? 'Max 10,000 rows. Print-optimized HTML; use browser Print → Save as PDF. Current filters apply.'
                  : 'Requires export.findings permission (server-enforced on download).'
              }
              onClick={async () => {
                setExportLoading(true);
                try {
                  await api.exportRisksPDF({
                    clusterId: effectiveClusterId ?? undefined,
                    sinceMinutes: sinceMinutesForApi,
                    status: statusFilter,
                    finalLevel: riskLevelFilter || undefined,
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
          </div>
        )}
      </div>

      {/* ----- Risks tab ----- */}
      {(activeTab === 'overview' || activeTab === 'triage') && (
        <div className="flex flex-col gap-6">
          {isOverviewPage && (
            <div className="rounded-lg border border-border bg-surface p-4 md:p-5">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <h2 className="text-body font-bold text-text flex items-center gap-2">
                    <AlertTriangle className="w-5 h-5 text-amber-400 shrink-0" />
                    Operations summary
                  </h2>
                  <p className="text-caption text-muted mt-1 max-w-2xl">
                    Unified risk counts use risk score bands. Hints below use the findings loaded for this screen.
                  </p>
                </div>
                <div className="text-caption text-muted text-right">
                  <div>
                    Last risk computation:{' '}
                    <span className="text-text font-medium">{riskComputationFreshness.ago}</span>
                  </div>
                  <div>
                    Staleness: <span className="text-text font-medium">{riskComputationFreshness.staleness}</span>
                  </div>
                  <div className="mt-1 text-muted-2">
                    Resolved (24h): <span className="text-emerald-400 font-semibold">{Number(resolved24h ?? 0)}</span>
                  </div>
                </div>
              </div>
              <div className="mt-4 grid grid-cols-1 gap-3 min-[420px]:grid-cols-2 lg:grid-cols-4">
                {(['critical', 'high', 'medium', 'low'] as const).map((sev) => (
                  <button
                    key={sev}
                    type="button"
                    onClick={() => navigate(`/risks/findings?finalLevel=${sev}`)}
                    className={`rounded-lg border px-4 py-3 text-left transition-colors hover:border-brand/50 ${
                      sev === 'critical'
                        ? 'border-red-900/50 bg-red-950/20'
                        : sev === 'high'
                          ? 'border-orange-900/40 bg-orange-950/15'
                          : sev === 'medium'
                            ? 'border-yellow-900/35 bg-yellow-950/10'
                            : 'border-sky-900/35 bg-sky-950/10'
                    }`}
                  >
                    <div className="text-micro font-semibold uppercase tracking-wide text-muted">{sev} risk</div>
                    <div
                      className={`text-2xl font-bold tabular-nums ${
                        sev === 'critical' ? 'text-red-400' : sev === 'high' ? 'text-orange-400' : sev === 'medium' ? 'text-yellow-400' : 'text-sky-300'
                      }`}
                    >
                      {riskLevelBar[sev]}
                    </div>
                  </button>
                ))}
              </div>
              <div className="mt-4 flex flex-wrap gap-2">
                <Button variant="secondary" size="sm" onClick={() => navigate('/risks/findings?finalLevel=critical')}>
                  Open critical queue
                </Button>
                <Button variant="secondary" size="sm" onClick={() => navigate('/attack-paths')}>
                  Review attack paths
                </Button>
                <Button variant="secondary" size="sm" onClick={() => navigate('/risks/pce')}>
                  Explain exposure
                </Button>
              </div>
              <details className="mt-4 rounded-lg border border-border/70 bg-base/20 px-3 py-2 text-caption text-muted">
                <summary className="cursor-pointer select-none font-medium text-muted hover:text-text">
                  Advanced context
                </summary>
                <div className="mt-3 grid gap-3 lg:grid-cols-3">
                  <div className="rounded border border-border/60 bg-surface/35 p-2">
                    <div className="text-micro uppercase tracking-wide text-muted-2">Loaded findings</div>
                    <div className="mt-1 text-text">
                      <span className="font-mono font-semibold text-amber-200">{newExisting.nu}</span> new ·{' '}
                      <span className="font-mono">{newExisting.existing}</span> existing
                    </div>
                  </div>
                  <div className="rounded border border-border/60 bg-surface/35 p-2">
                    <div className="text-micro uppercase tracking-wide text-muted-2">Attack-path hints</div>
                    <div className="mt-1 text-text">
                      Critical: <span className="font-mono">{riskLevelBar.critical}</span> · In path:{' '}
                      <span className="font-mono">{critPathSplit.inPath}</span>
                    </div>
                  </div>
                  <div className="rounded border border-border/60 bg-surface/35 p-2">
                    <div className="text-micro uppercase tracking-wide text-muted-2">Exposure shape</div>
                    <div className="mt-1 text-text">
                      External {exploitBuckets.external} · Auth {exploitBuckets.auth} · Cluster {blastBuckets.cluster}
                    </div>
                  </div>
                </div>
                {attackNarrative || privilegedDrift ? (
                  <p className="mt-2 text-muted">
                    {[attackNarrative, privilegedDrift ? `Risk drift: ${privilegedDrift}` : null].filter(Boolean).join(' · ')}
                  </p>
                ) : null}
              </details>
            </div>
          )}

          {/* Risk Trend + Risk Score Distribution — split layout on overview (trend full width, then histogram | velocity) */}
          {isOverviewPage && (
          <div className="space-y-4">
            {/* Risk Trend (AreaChart) */}
            <div className="bg-surface border border-border rounded-lg p-3 md:p-4 flex flex-col">
              <div className="flex items-center justify-between gap-2 mb-1">
                <h2 className="text-caption font-semibold text-muted uppercase tracking-wider">
                  Risk trend + events (last {trendDays} {trendDays === 1 ? 'day' : 'days'})
                </h2>
                <div className="flex items-center gap-1 text-caption text-muted">
                  <span>Range:</span>
                  {[1, 7, 30].map((d) => (
                    <button
                      key={d}
                      type="button"
                      onClick={() => setTrendDays(d)}
                      className={`px-2 py-0.5 rounded-full border ${
                        trendDays === d
                          ? 'border-brand text-brand/90 bg-brand/10'
                          : 'border-border text-muted hover:border-muted'
                      }`}
                    >
                      {d === 1 ? '24h' : `${d}d`}
                    </button>
                  ))}
                </div>
              </div>
              <p className="text-caption text-muted mb-2 md:mb-3">
	                Track whether open findings are increasing or decreasing. Same scope as active findings only.{' '}
                {scopeClusterDisplay ? `Scoped to cluster: ${scopeClusterDisplay}.` : 'All clusters.'} Click a point to filter findings from that date.
              </p>
              {riskTrendChartData.length >= 2 ? (
                <div ref={riskTrendChartRef} className="h-[240px] min-h-[240px] w-full min-w-0">
                  {riskTrendChartWidth > 0 ? (
                    <AreaChart
                      data={riskTrendChartData}
                      width={riskTrendChartWidth}
                      height={240}
                      margin={{ top: 10, right: 10, left: 5, bottom: 0 }}
                      onClick={(state) => {
                        const st = state as { activePayload?: Array<{ payload?: { name?: string } }> };
                        const name = st?.activePayload?.[0]?.payload?.name;
                        if (name) {
                          setSelectedChartDate(name);
                          setRisksPage(1);
                        }
                      }}
                      style={{ cursor: 'pointer' }}
                    >
                      <CartesianGrid strokeDasharray="3 3" vertical={false} stroke={chartTheme.grid} />
                      <XAxis dataKey="name" axisLine={false} tickLine={false} tick={{ fill: chartTheme.axis, fontSize: 11 }} />
                      <YAxis domain={[0, 'auto']} axisLine={false} tickLine={false} tick={{ fill: chartTheme.axis, fontSize: 11 }} width={28} />
                      <Tooltip content={<RiskTrendTooltipContent />} cursor={{ stroke: chartTheme.threshold, strokeDasharray: '4 4' }} />
                      <Area type="monotone" dataKey="low" stackId="risk" stroke={chartTheme.low} fill={chartTheme.low} fillOpacity={0.65} name="Low" />
                      <Area type="monotone" dataKey="medium" stackId="risk" stroke={chartTheme.medium} fill={chartTheme.medium} fillOpacity={0.65} name="Medium" />
                      <Area type="monotone" dataKey="high" stackId="risk" stroke={chartTheme.high} fill={chartTheme.high} fillOpacity={0.65} name="High" />
                      <Area type="monotone" dataKey="critical" stackId="risk" stroke={chartTheme.critical} fill={chartTheme.critical} fillOpacity={0.7} name="Critical" />
                      {spikeChartIdx >= 0 && riskTrendChartData[spikeChartIdx] ? (
                        <ReferenceDot
                          x={riskTrendChartData[spikeChartIdx].name}
                          y={riskTrendChartData[spikeChartIdx].risk}
                          r={6}
                          fill={chartTheme.spike}
                          stroke={chartTheme.tooltipBg}
                          strokeWidth={1}
                        />
                      ) : null}
                    </AreaChart>
                  ) : null}
                  {selectedChartDate && (
                    <p className="text-caption text-muted mt-2">
                      Showing findings since <span className="font-medium text-brand">{selectedChartDate}</span>
                      <button type="button" onClick={() => setSelectedChartDate(null)} className="ml-2 text-brand hover:underline">Clear</button>
                    </p>
                  )}
                  {trendSpike && trendSpike.delta > 0 ? (
                    <p className="text-caption text-pink-300/90 mt-2">
                      ↑ {trendSpike.date}: +{trendSpike.delta} findings (day-over-day); spike marker on chart. Cause not inferred — correlate with deploys or config changes.
                    </p>
                  ) : null}
                </div>
              ) : (
                <div className="flex h-[220px] items-center justify-center rounded-lg border border-border/60 bg-base/25 px-4 text-center text-body text-muted md:h-[240px]">
                  Trend needs at least two time buckets for this scope and range.
                </div>
              )}
            </div>

              <div className="grid gap-4 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)] items-stretch">
                {renderRiskHistogramCard()}
                <div className="bg-surface border border-border rounded-lg p-3 md:p-4 flex flex-col">
                  <h2 className="text-caption font-semibold text-muted uppercase tracking-wider mb-2">Velocity</h2>
                  <p className="text-meta text-muted-2 mb-3">
                    Baseline = mean of earlier day-over-day steps in the visible trend (excluding the latest step).
                  </p>
                  {velocityStats ? (
                    <div className="space-y-3 text-body">
                      <div>
                        <div className="text-caption text-muted uppercase tracking-wide">Latest step Δ</div>
                        <div className={`text-2xl font-bold tabular-nums ${velocityStats.lastDelta >= 0 ? 'text-red-400' : 'text-emerald-400'}`}>
                          {velocityStats.lastDelta >= 0 ? '+' : ''}
                          {velocityStats.lastDelta}
                        </div>
                      </div>
                      {velocityStats.pctVsBaseline != null ? (
                        <div className="text-caption">
                          <span className="text-muted">vs baseline: </span>
                          <span className="text-amber-200 font-semibold">
                            {velocityStats.pctVsBaseline >= 0 ? '+' : ''}
                            {velocityStats.pctVsBaseline}%
                          </span>
                        </div>
                      ) : null}
                      <div className="text-caption text-muted">
                        Baseline: ~{velocityStats.baselinePerDay.toFixed(1)} findings / trend step
                      </div>
                      <div className="text-caption text-muted border-t border-border/50 pt-2">
                        Range Δ (first → last bucket):{' '}
                        <span className="text-text font-mono">{velocityStats.rangeDelta >= 0 ? '+' : ''}{velocityStats.rangeDelta}</span>
                      </div>
                    </div>
                  ) : (
                    <p className="text-caption text-muted">Need at least two trend points.</p>
                  )}
                </div>
              </div>
          </div>
          )}

          {/* Layer 2 - Risk level overview: totals, velocity, resolved, and level breakdown */}
          {!isOverviewPage ? (
          <div className="rounded-lg border border-border bg-surface p-4">
	                  <h2 className="text-caption font-semibold text-muted uppercase tracking-wider mb-2">Priority overview (active findings)</h2>
            {severityCountTrust !== 'exact' && (
              <DataQualityNotice
                level={severityCountTrust === 'sample' ? 'sampled' : 'estimated'}
                className="mb-2"
              >
                {severityCountTrust === 'sample' ? (
                  <>
                    Summary API unavailable: risk-level counts use the <strong>first up to 1,000</strong> findings matching your filters. Total count is still the server total.
                  </>
                ) : (
                  <>
	                    Summary API unavailable: priority counts reflect the <strong>current table page</strong> only. Active findings still matches the server total.
                  </>
                )}
              </DataQualityNotice>
            )}
            <div className="grid grid-cols-2 gap-2 md:grid-cols-6">
              <button
                type="button"
                onClick={() => setRiskLevelFilter('')}
                className={`rounded-lg border px-3 py-2 text-left ${riskLevelFilter === '' ? 'border-brand/50 bg-brand/10' : 'border-border bg-base/25 hover:border-brand/40'}`}
                title="All active findings in current scope"
              >
                <div className="text-micro font-semibold uppercase tracking-wide text-muted">Total</div>
                <div className="text-xl font-bold tabular-nums text-text">{riskLevelBar.total}</div>
              </button>
              {(['critical', 'high', 'medium', 'low'] as const).map((sev) => (
                <button
                  key={sev}
                  type="button"
                  onClick={() => setRiskLevelFilter(sev)}
                  className={`rounded-lg border px-3 py-2 text-left ${riskLevelFilter === sev ? 'border-brand/50 bg-brand/10' : 'border-border bg-base/25 hover:border-brand/40'}`}
                  title={`Filter queue to ${sev} unified risk`}
                >
                  <div className={`text-micro font-semibold uppercase tracking-wide ${getSeverityTextClass(sev)}`}>
                    {sev}
                  </div>
                  <div className="text-xl font-bold tabular-nums text-text">{riskLevelBar[sev]}</div>
                </button>
              ))}
              <div className="rounded-lg border border-border bg-base/25 px-3 py-2" title="Insights resolved in the last 24h">
                <div className="text-micro font-semibold uppercase tracking-wide text-muted">Resolved 24h</div>
                <div className="text-xl font-bold tabular-nums text-emerald-400">{Number(resolved24h ?? 0)}</div>
              </div>
            </div>
            {/* Risks by cluster (Phase 2.2) – when scope is all clusters */}
            {!effectiveClusterId && risksByCluster.length > 0 && (
              <div className="mt-4">
                <h3 className="text-caption font-semibold text-muted uppercase tracking-wider mb-2">Risks by cluster</h3>
                <div className="overflow-x-auto border border-border rounded-lg">
                  <table className={UI_TABLE}>
                    <thead className={UI_THEAD_STICKY}>
                      <tr>
                        <th className={UI_TH_COMPACT}>Cluster</th>
                        <th className={`${UI_TH_COMPACT} text-right`}>Total</th>
                        <th className={`${UI_TH_COMPACT} text-right text-red-400`}>Critical</th>
                        <th className={`${UI_TH_COMPACT} text-right text-orange-400`}>High</th>
                        <th className={`${UI_TH_COMPACT} text-right text-yellow-400`}>Medium</th>
                        <th className={`${UI_TH_COMPACT} text-right text-blue-400`}>Low</th>
                      </tr>
                    </thead>
                    <tbody>
                      {risksByCluster.map((row) => (
                        <tr key={row.clusterId} className={UI_TR}>
                          <td className={UI_TD_COMPACT_TIGHT}>
                            <button
                              type="button"
                              onClick={() => setSelectedClusterId(row.clusterId)}
                              className="text-brand hover:text-brand/90 hover:underline font-medium text-left"
                            >
                              {row.clusterName || row.clusterId}
                            </button>
                          </td>
                          <td className={`${UI_TD_COMPACT_TIGHT} text-right text-text`}>{row.total}</td>
                          <td className={`${UI_TD_COMPACT_TIGHT} text-right text-red-400`}>{row.critical}</td>
                          <td className={`${UI_TD_COMPACT_TIGHT} text-right text-orange-400`}>{row.high}</td>
                          <td className={`${UI_TD_COMPACT_TIGHT} text-right text-yellow-400`}>{row.medium}</td>
                          <td className={`${UI_TD_COMPACT_TIGHT} text-right text-blue-400`}>{row.low}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
          </div>
          ) : null}

          {/* Overview: suggested actions + drill-down. Findings page: filters + table. */}
          {isOverviewPage ? (
            <details className="rounded-lg border border-border/70 bg-base/20 px-4 py-2 text-caption text-muted">
              <summary className="cursor-pointer select-none font-medium hover:text-text">
                More shortcuts
              </summary>
              <div className="mt-3 flex flex-wrap gap-2">
                  <Button variant="secondary" size="sm" onClick={() => navigate('/risks/findings?finalLevel=critical')}>
                    View critical only
                  </Button>
                  <Button variant="secondary" size="sm" onClick={() => navigate('/attack-paths')}>
                    Investigate attack paths
                  </Button>
                  <Button variant="secondary" size="sm" onClick={() => navigate('/risks/findings?sinceMinutes=1440')}>
                    New findings (24h)
                  </Button>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => navigate(`/risks/findings?search=${encodeURIComponent('privileged')}`)}
                  >
                    Filter: privileged pods
                  </Button>
              </div>
            </details>
          ) : (
            <>
          {riskWorkspace.showSavedViews ? (
          <details className="order-1 rounded-lg border border-border/70 bg-surface/60 px-4 py-2 text-caption text-muted">
            <summary className="cursor-pointer select-none font-medium hover:text-text">Saved views</summary>
            <div className="mt-3">
          <RiskFindingsSavedViews
            personaId={personaId}
            current={{
              statusFilter,
              riskLevelFilter,
              searchTerm: debouncedSearchTerm,
              clusterId: effectiveClusterId,
              sinceMinutes: sinceMinutesForApi,
            }}
            onApply={(filters) => {
              setStatusFilter(filters.statusFilter);
              setRiskLevelFilter(filters.riskLevelFilter);
              setSearchTerm(filters.searchTerm);
              setDebouncedSearchTerm(filters.searchTerm);
              if (filters.clusterId) setSelectedClusterId(filters.clusterId);
              if (filters.sinceMinutes != null) setTimeWindowMinutes(filters.sinceMinutes);
              setRisksPage(1);
            }}
          />
            </div>
          </details>
          ) : null}
          {/* Filters: Final risk level, workflow, search */}
          <div className="order-1 rounded-lg border border-border bg-surface p-4">
            <div className="mb-3 flex flex-col gap-1 sm:flex-row sm:items-start sm:justify-between">
              <div>
	                <h2 className="text-section-title text-text">Findings queue</h2>
	                <p className="text-caption text-muted">Prioritize, inspect, and update findings in the current operational scope.</p>
              </div>
              <div className="text-caption text-muted sm:text-right">
                {risksTotal} {findingsListView === 'group' ? 'groups' : 'findings'}
              </div>
            </div>
            <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(280px,360px)_220px_220px] xl:items-end">
              <div className="xl:col-span-4 flex flex-wrap gap-2 items-center">
	              <span className="text-caption text-muted uppercase tracking-wider mr-1">Priority level</span>
              {['all', 'critical', 'high', 'medium', 'low'].map((sev) => (
                <button
                  key={sev}
                  onClick={() => setRiskLevelFilter(sev === 'all' ? '' : sev)}
                  className={`px-3 py-1.5 rounded-full text-body font-medium capitalize transition-colors whitespace-nowrap ${
                    (riskLevelFilter || 'all') === sev ? UI_PILL_ACTIVE_ELEVATED : UI_PILL_IDLE_ROUNDED
                  }`}
                >
                  {sev}
                </button>
              ))}
              <span className="text-caption text-muted uppercase tracking-wider ml-2 mr-1">Workflow</span>
              {(['all', 'active', 'resolved', 'acknowledged'] as const).map((st) => (
                <button
                  key={st}
                  onClick={() => setStatusFilter(st)}
                  className={`px-3 py-1.5 rounded-full text-body font-medium capitalize transition-colors whitespace-nowrap ${
                    statusFilter === st ? UI_PILL_ACTIVE_ELEVATED : UI_PILL_IDLE_ROUNDED
                  }`}
                >
                  {st === 'all' ? 'All' : st === 'active' ? 'Active' : st === 'resolved' ? 'Resolved' : 'In review'}
                </button>
              ))}
              </div>
              <div>
              <label className="mb-1 block text-caption text-muted uppercase tracking-wider">Namespace</label>
              <input
                value={namespaceFilter}
                onChange={(e) => setNamespaceFilter(e.target.value)}
                placeholder="All"
                className="h-10 w-full rounded-lg border border-border bg-base px-3 text-body text-text placeholder:text-muted-2 focus:outline-none focus:border-brand"
              />
              </div>
              <div>
              <label className="mb-1 block text-caption text-muted uppercase tracking-wider">Rule type</label>
              <select
                value={typeFilter}
                onChange={(e) => setTypeFilter(e.target.value)}
                className="h-10 w-full rounded-lg border border-border bg-base px-3 text-body text-text focus:outline-none focus:border-brand"
              >
                <option value="">All</option>
                <option value="vulnerability">Vulnerability</option>
                <option value="supply_chain_malware">Supply-chain malware</option>
                <option value="rbac_risk">RBAC risk</option>
                <option value="capability">Capability</option>
              </select>
              </div>
            <div className="relative min-w-0">
              <label className="mb-1 block text-caption text-muted uppercase tracking-wider">Search</label>
              <Search className="absolute left-3 top-[2.15rem] text-muted w-4 h-4" />
              <input
                type="text"
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                placeholder="Search title, CVE, package, pod, namespace..."
                className="h-10 w-full rounded-lg border border-border bg-base pl-9 pr-4 text-body text-text placeholder:text-muted-2 focus:border-brand focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/60"
              />
            </div>
            <div>
              <label className="mb-1 block text-caption text-muted uppercase tracking-wider">Sort</label>
              <select
                value={riskSort}
                onChange={(e) => setRiskSort(e.target.value as typeof riskSort)}
                className="h-10 w-full rounded-lg border border-border bg-base px-3 text-body text-text focus:outline-none focus:border-brand"
              >
                <option value="newest">Newest first</option>
                <option value="oldest">Oldest first</option>
	                <option value="score_desc">Priority score high to low</option>
	                <option value="score_asc">Priority score low to high</option>
                <option value="title_asc">Title A-Z</option>
              </select>
            </div>
            <div>
              <label className="mb-1 block text-caption text-muted uppercase tracking-wider">Layout</label>
              <div className="inline-flex h-10 w-full rounded-lg border border-border overflow-hidden text-caption">
                <button
                  type="button"
                  onClick={() => setFindingsListView('instance')}
                  className={`flex-1 px-3 py-2 font-medium ${
                    findingsListView === 'instance' ? UI_PILL_ACTIVE : UI_PILL_IDLE_SPLIT
                  }`}
                >
                  By resource
                </button>
                <button
                  type="button"
                  onClick={() => setFindingsListView('group')}
                  className={`flex-1 px-3 py-2 font-medium border-l border-border ${
                    findingsListView === 'group' ? UI_PILL_ACTIVE : UI_PILL_IDLE_SPLIT
                  }`}
                  title="Group findings by rule type and CVE (or title when no CVE). Row opens a sample finding."
                >
                  Grouped
                </button>
              </div>
            </div>
          </div>
          </div>
	          <details className="order-1 rounded-lg border border-border/70 bg-surface/60 px-4 py-2 text-caption text-muted">
	            <summary className="cursor-pointer select-none text-muted hover:text-text">Columns and table display</summary>
	            <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-2">
	            <span className="text-muted uppercase tracking-wider whitespace-nowrap">Visible columns:</span>
            {(
              [
                { key: 'type' as const, label: 'Type' },
                { key: 'resource' as const, label: 'Impacted resource' },
	                { key: 'score' as const, label: 'Priority score' },
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
                    className="h-3.5 w-3.5 rounded border-border bg-surface"
                    checked={riskFindingsCols[c.key]}
                    onChange={() =>
                      setRiskFindingsCols((prev) => ({ ...prev, [c.key]: !prev[c.key] }))
                    }
                  />
                  <span>{c.label}</span>
                </label>
              ))}
	            </div>
	          </details>

	          {/* Layer 3 – Risk Exploration (spec §3.3): core table. Runtime signals only in Reference tab / detail drawer. */}
	          {riskWorkspace.showBulkToolbar && selectedIds.size > 0 ? (
	          <div className="order-1 flex flex-col gap-3 rounded-lg border border-border bg-surface/70 px-4 py-3 text-caption text-muted sm:flex-row sm:items-center sm:justify-between">
	            <div>
	                <span>
	                  {selectedIds.size} selected.{' '}
                  <button
                    type="button"
                    className="text-brand hover:underline"
                    onClick={() => setSelectedIds(new Set())}
                  >
                    Clear selection
                  </button>
	                </span>
	            </div>
            <div className="flex flex-wrap gap-2">
              <Button
                size="sm"
                variant="secondary"
                disabled={selectedIds.size === 0 || !canBulkFindings || bulkActionBusy}
                onClick={() => openBulkActionReview('acknowledge')}
              >
                Acknowledge selected
              </Button>
              <Button
                size="sm"
                variant="secondary"
                disabled={selectedIds.size === 0 || !canBulkFindings || bulkActionBusy}
                onClick={() => openBulkActionReview('resolve')}
              >
                Resolve selected
              </Button>
              <Button
                size="sm"
                variant="secondary"
                disabled={selectedIds.size === 0 || !canBulkFindings || bulkActionBusy}
                onClick={() => openBulkActionReview('dismiss')}
              >
                Dismiss selected
              </Button>
            </div>
          </div>
          ) : null}

          {/* Risk list – compact table with internal scroll to keep layout consistent */}
          <div className="order-2 bg-surface rounded-lg border border-border shadow-sm p-0 overflow-hidden">
            <div className="ui-table-scroll">
              <table className={UI_TABLE}>
                <thead className={UI_THEAD_STICKY}>
                  <tr>
                    {riskWorkspace.showRowSelection ? (
                    <th className={`${tableHeadClass} w-8`}>
                      <input
                        type="checkbox"
                        className="h-4 w-4 rounded border-border bg-surface"
                        disabled={!canBulkFindings}
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
                    ) : null}
	                    <th className={`${tableHeadClass} w-10`}>Priority</th>
                    {riskWorkspace.showProvenanceColumn ? (
                      <th className={`${tableHeadClass} w-24 hidden md:table-cell`}>Evidence</th>
                    ) : null}
                    <th className={tableHeadClass}>Finding</th>
                    {riskFindingsCols.type && (
                      <th className={`${tableHeadClass} w-36 hidden sm:table-cell`}>Type</th>
                    )}
                    {riskFindingsCols.resource && (
                      <th className={`${tableHeadClass} w-20`}>Impacted Resource</th>
                    )}
                    {riskFindingsCols.score && (
	                      <th className={`${tableHeadClass} w-20 min-w-[5rem]`}>Priority Score</th>
                    )}
                    {!effectiveClusterId && riskFindingsCols.nsCluster && (
                      <th className={`${tableHeadClass} hidden lg:table-cell w-32`}>Namespace / Cluster</th>
                    )}
                    <th className={`${tableHeadClass} w-24`}>Workflow</th>
                    {riskFindingsCols.detected && (
                      <th className={`${tableHeadClass} w-24 hidden md:table-cell`}>Detected At</th>
                    )}
                    {riskFindingsCols.updated && (
                      <th className={`${tableHeadClass} w-24 hidden lg:table-cell`}>Updated At</th>
                    )}
                    <th className={`${tableHeadClass} w-32 text-right`}>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {paginatedRisks.length === 0 ? (
                    <tr>
                      <td colSpan={findingsTableColCount} className="px-2 py-4">
                        <SemanticEmptyState
                          state={riskDataEmpty ? 'no_data' : 'no_scope'}
                          compact
	                          title={riskDataEmpty ? 'No active findings in scope' : 'No findings match filters'}
	                          reason={
	                            riskDataEmpty
	                              ? `No active findings exist in ${scopeClusterDisplay ?? 'all clusters'}. This is a legitimate empty state, not a visibility restriction.`
	                              : 'Adjust severity, status, search, or time window — results may also be limited by your operational scope.'
	                          }
                        />
                      </td>
                    </tr>
                  ) : (
                    paginatedRisks.map((risk) => (
                          <tr
                        key={risk.id}
                        role="button"
                        tabIndex={0}
                        className={`${UI_TR} cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 focus-visible:ring-inset`}
                        onClick={() => openRiskDrawer(risk)}
                        onKeyDown={(event) => {
                          if (event.key === 'Enter' || event.key === ' ') {
                            event.preventDefault();
                            openRiskDrawer(risk);
                          }
                        }}
                      >
                        {riskWorkspace.showRowSelection ? (
                        <td className={tableCellClass} onClick={(e) => e.stopPropagation()}>
                          <input
                            type="checkbox"
                            className="h-4 w-4 rounded border-border bg-surface"
                            disabled={!canBulkFindings}
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
                        ) : null}
                        <td className={tableCellClass}>
                          <div className="flex items-center gap-1">
                            {risk.finalLevel ? getSeverityIcon(risk.finalLevel) : <span className="text-muted">·</span>}
                            {risk.finalLevel ? (
                              <span className={`text-caption font-bold px-1.5 py-0.5 rounded capitalize border ${getSeverityBadgeClass(risk.finalLevel)}`} title="Unified risk level (from score)">
                                {risk.finalLevel}
                              </span>
                            ) : (
                              <span className="text-caption text-muted">No score</span>
                            )}
                          </div>
                        </td>
                        {riskWorkspace.showProvenanceColumn ? (
                          <td className={`${tableCellClass} hidden md:table-cell`}>
                            <ProvenanceBadge kind={insightProvenance(risk)} title={insightProvenanceTitle(risk)} />
                          </td>
                        ) : null}
                        <td className={tableCellClass}>
                          <span className="text-text font-medium">{risk.title}</span>
                          <span className="text-muted ml-1 text-caption font-mono" title={risk.cveId}>
                            ({riskListSecondaryLabel(risk)})
                          </span>
                          {riskWorkspace.narrativeTable && risk.riskExplanation ? (
                            <p className="text-meta text-muted mt-1 line-clamp-2 max-w-prose">{risk.riskExplanation}</p>
                          ) : null}
                        </td>
                        {riskFindingsCols.type && (
                          <td className={`${UI_TD} text-muted hidden sm:table-cell text-caption leading-snug`}>
                            {(() => {
                              const label = insightTypeUiLabel(risk.insightType);
                              const src =
                                risk.insightType === 'vulnerability' || risk.insightType === 'supply_chain_malware'
                                  ? 'Static'
                                  : 'Runtime';
                              return (
                                <>
                                  <span className="block text-text">{label}</span>
                                  <span className="text-caption text-muted">({src})</span>
                                </>
                              );
                            })()}
                          </td>
                        )}
                        {riskFindingsCols.resource && (
                          <td className={`${UI_TD} text-muted text-caption`} onClick={(e) => e.stopPropagation()}>
                            {risk.isGroupRow && risk.groupMemberCount != null ? (
                              <span className="text-text" title="Grouped view: count of findings in this group">
                                {risk.groupMemberCount} finding{risk.groupMemberCount === 1 ? '' : 's'}
                              </span>
                            ) : risk.affectedResources?.length ? (
                              (() => {
                                const resource = risk.affectedResources?.[0];
                                const label = findingResourceDisplayName(resource);
                                const kindLabel = findingResourceKindLabel(resource?.kind);
                                const target = findingResourceTarget(resource);
                                if (risk.affectedResources.length > 1) {
                                  return `${risk.affectedResources.length} resources`;
                                }
                                if (!label) return kindLabel;
                                if (!target) {
                                  return (
                                    <span className="text-text" title={`${kindLabel}: ${label}`}>
                                      {label}
                                      <span className="ml-1 text-muted">({kindLabel})</span>
                                    </span>
                                  );
                                }
                                return (
                                  <Link
                                    to={target}
                                    className="inline-flex max-w-[14rem] items-center gap-1 text-brand hover:text-brand/90 hover:underline font-medium"
                                    title={`Open ${kindLabel} detail: ${label}`}
                                  >
                                    <span className="truncate">{label}</span>
                                    <ArrowRight className="h-3 w-3 shrink-0" />
                                  </Link>
                                );
                              })()
                            ) : '—'}
                          </td>
                        )}
                        {riskFindingsCols.score && (
                          <td
                            className={`${UI_TD} text-text`}
                            title={
                              [
	                                'Priority score, not exploit probability',
                                risk.score != null ? `Score ${risk.score}/100` : null,
                                risk.finalLevel ? `Level ${risk.finalLevel}` : null,
                                risk.exploitabilityScore != null ? `Exploitability factor ${risk.exploitabilityScore.toFixed(1)}` : null,
                                risk.businessImpactScore != null ? `Business impact factor ${risk.businessImpactScore.toFixed(1)}` : null,
                              ]
                                .filter(Boolean)
                                .join(' · ') || undefined
                            }
                          >
                            {risk.score != null ? (
                              <div className="space-y-1">
                                <div className="font-medium tabular-nums">
                                  {risk.score}/100
                                  {risk.finalLevel ? <span className="text-muted font-normal capitalize"> {risk.finalLevel}</span> : null}
                                </div>
                                <div className="text-micro text-muted">prioritization</div>
                                <div
                                  className="h-1 rounded-full bg-surface-2 overflow-hidden max-w-[4.5rem]"
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
                          <td
                            className={`${UI_TD} text-muted hidden lg:table-cell max-w-[160px]`} title={`ns: ${risk.affectedResources?.[0]?.namespace ?? '—'} · cl: ${risk.clusterName ?? (risk.clusterId ? clusterLabelById.get(risk.clusterId) ?? risk.clusterId : '—')}`}>
                            <div className="text-caption text-text truncate">
                              ns: {risk.affectedResources?.[0]?.namespace ?? '—'}
                            </div>
                            <div className="text-caption text-muted truncate">
                              cl: {risk.clusterName ?? (risk.clusterId ? clusterLabelById.get(risk.clusterId) ?? risk.clusterId : '—')}
                            </div>
                          </td>
                        )}
                        <td className={UI_TD}>
                          <span className="uppercase px-2 py-0.5 bg-surface-2 rounded text-caption text-text">
                            {statusLabelMap[risk.status || ''] ?? (risk.status || 'Unknown')}
                          </span>
                        </td>
                        {riskFindingsCols.detected && (
                          <td className={`${UI_TD} text-muted hidden md:table-cell text-caption`}>
                            {risk.timestamp ? new Date(risk.timestamp).toLocaleDateString() : '—'}
                          </td>
                        )}
                        {riskFindingsCols.updated && (
                          <td className={`${UI_TD} text-muted hidden lg:table-cell text-caption`}>
                            {risk.updatedAt ? new Date(risk.updatedAt).toLocaleDateString() : '—'}
                          </td>
                        )}
                        <td className={`${UI_TD} text-right`} onClick={(e) => e.stopPropagation()}>
                          <button
                            onClick={() => toggleRiskDrawer(risk)}
                            className="text-brand hover:text-brand/90 text-body font-medium mr-2"
                          >
                            {selectedRisk?.id === risk.id ? 'Close view' : 'Quick view'}
                          </button>
                          <Button
                            size="sm"
                            variant="secondary"
                            onClick={() => {
                              const podUid = findingPodUid(risk);
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
	              itemLabel={findingsListView === 'group' ? 'groups' : 'findings'}
	              className="order-2"
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
            <h1 className="text-section-title text-text">Capability exposure</h1>
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
	                <h2 className="text-body font-semibold text-muted uppercase tracking-wider mb-1">
	              Capability exposure summary
            </h2>
            <p className="text-caption text-muted mb-3">
              Counts by severity from current pod capability inventory. These numbers are separate from active findings and do not use the Risk Operations time window.
              {scopeClusterDisplay ? ` Scoped to cluster: ${scopeClusterDisplay}.` : ' All clusters.'}
            </p>
            {pceSummary.length === 0 ? (
              <p className="text-body text-muted">No capability exposure data available for this scope.</p>
            ) : (
              <div className="grid grid-cols-1 gap-3 min-[420px]:grid-cols-2 lg:grid-cols-4">
                {pceSummary.map((row) => (
                  <div key={row.severity} className="bg-base border border-border rounded p-3 flex items-center justify-between">
                    <span className={`text-caption uppercase ${getSeverityTextClass(row.severity)}`}>{row.severity}</span>
                    <span className="text-body font-bold text-text">{row.count}</span>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Capability exposure trend */}
          {pceTrend.length > 0 && (
            <div className="bg-surface border border-border p-4 rounded-lg">
              <div className="flex flex-wrap items-center justify-between gap-2 mb-2">
                <h2 className="text-body font-semibold text-muted uppercase tracking-wider">Capability exposure trend (last {pceTrendDays === 1 ? '24h' : `${pceTrendDays}d`})</h2>
                <div className="flex items-center gap-1 text-caption text-muted">
                  <span>Range:</span>
                  {[1, 7, 30].map((d) => (
                    <button
                      key={d}
                      type="button"
                      onClick={() => setPceTrendDays(d)}
                      className={`px-2 py-0.5 rounded-full border ${
                        pceTrendDays === d
                          ? 'border-brand text-brand/90 bg-brand/10'
                          : 'border-border text-muted hover:border-muted'
                      }`}
                    >
                      {d === 1 ? '24h' : `${d}d`}
                    </button>
                  ))}
                </div>
              </div>
              <p className="text-caption text-muted mb-3">Capability exposure counts by day. Same cluster scope as this route.</p>
              {pceTrend.length >= 2 ? (
                <div className="h-[220px] min-h-[220px] w-full min-w-0">
                  <ResponsiveContainer width="100%" height="100%" minWidth={1} minHeight={220}>
                    <AreaChart data={pceTrend.map((p) => ({ ...p, total: (p.critical ?? 0) + (p.high ?? 0) + (p.medium ?? 0) + (p.low ?? 0) }))}>
                      <CartesianGrid strokeDasharray="3 3" stroke={chartTheme.grid} />
                      <XAxis dataKey="date" tick={{ fill: chartTheme.axis, fontSize: 11 }} />
                      <YAxis tick={{ fill: chartTheme.axis, fontSize: 11 }} />
                      <Tooltip contentStyle={{ backgroundColor: chartTheme.tooltipBg, border: `1px solid ${chartTheme.tooltipBorder}` }} labelStyle={{ color: chartTheme.axis }} />
                      <Area type="monotone" dataKey="critical" stackId="1" stroke={chartTheme.critical} fill={chartTheme.critical} fillOpacity={0.6} name="Critical" />
                      <Area type="monotone" dataKey="high" stackId="1" stroke={chartTheme.high} fill={chartTheme.high} fillOpacity={0.6} name="High" />
                      <Area type="monotone" dataKey="medium" stackId="1" stroke={chartTheme.medium} fill={chartTheme.medium} fillOpacity={0.6} name="Medium" />
                      <Area type="monotone" dataKey="low" stackId="1" stroke={chartTheme.low} fill={chartTheme.low} fillOpacity={0.6} name="Low" />
                    </AreaChart>
                  </ResponsiveContainer>
                </div>
              ) : (
                <div className="flex h-[220px] items-center justify-center rounded-lg border border-border/60 bg-base/25 px-4 text-center text-body text-muted">
                  Trend needs at least two time buckets for this scope and range.
                </div>
              )}
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
                  <h2 className="text-body font-semibold text-muted uppercase tracking-wider">Exposure by namespace</h2>
                  {namespaces.length > 30 && (
                    <Button variant="secondary" size="sm" type="button" onClick={() => setHeatmapShowAll((v) => !v)}>
                      {heatmapShowAll ? 'Show first 30 only' : `Show all (${namespaces.length})`}
                    </Button>
                  )}
                </div>
                <p className="text-caption text-muted mb-3">Click a cell with count &gt; 0 to filter the table below by that namespace and severity.</p>
                <div className="overflow-x-auto max-h-[min(70dvh,520px)] overflow-y-auto overscroll-contain rounded-lg border border-border">
                  <table className={UI_TABLE}>
                    <thead className={UI_THEAD_STICKY}>
                      <tr>
                        <th className={UI_TH_COMPACT}>Namespace</th>
                        <th className={`${UI_TH_COMPACT} text-right text-red-400`}>Critical</th>
                        <th className={`${UI_TH_COMPACT} text-right text-orange-400`}>High</th>
                        <th className={`${UI_TH_COMPACT} text-right text-yellow-500`}>Medium</th>
                        <th className={`${UI_TH_COMPACT} text-right text-green-400`}>Low</th>
                      </tr>
                    </thead>
                    <tbody>
                      {(heatmapShowAll ? namespaces : namespaces.slice(0, 30)).map((ns) => {
                        const row = byNs[ns];
                        const td = (n: number, sev: string, rgba: string) => (
                          <td
                            key={sev}
                            role={n > 0 ? 'button' : undefined}
                            tabIndex={n > 0 ? 0 : undefined}
                            className={`text-right py-1.5 px-2 rounded ${n > 0 ? 'cursor-pointer hover:ring-2 hover:ring-inset hover:ring-white/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 focus-visible:ring-inset' : 'cursor-default opacity-50'}`}
                            style={{ backgroundColor: `rgba(${rgba},${0.15 + (n / maxCount) * 0.75})` }}
                            onClick={() => n > 0 && handleHeatmapCellClick(ns, sev)}
                            onKeyDown={(event) => {
                              if (n > 0 && (event.key === 'Enter' || event.key === ' ')) {
                                event.preventDefault();
                                handleHeatmapCellClick(ns, sev);
                              }
                            }}
                            title={n > 0 ? `Filter table by ${ns}, ${sev}` : 'No exposures in this cell'}
                          >
                            {n}
                          </td>
                        );
                        return (
                          <tr key={ns} className={UI_TR}>
                            <td className={`${UI_TD_COMPACT_TIGHT} font-mono text-text`}>{ns}</td>
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
                    <p className="text-caption text-muted mt-2">Showing first 30 of {namespaces.length} namespaces. Use &quot;Show all&quot; to expand.</p>
                  )}
                </div>
              </div>
            );
          })()}

          {/* PCE Drill-down */}
          <div className="bg-surface border border-border p-4 rounded-lg">
            <h2 className="text-body font-semibold text-muted uppercase tracking-wider mb-2">
              Capability drill-down by pod
            </h2>
            <p className="text-caption text-muted mb-2">
              Explore pod-level capabilities that may explain exposure behind findings. Click a heatmap cell above to filter by namespace and severity, or use the filters below.
            </p>
            {pceHeatmapFilter && (
              <div className="mb-3 flex items-center gap-2 flex-wrap">
                <span className="text-caption text-muted">Filtered by namespace <strong className="text-text">{pceHeatmapFilter.namespace}</strong>, severity <strong className="text-text">{pceHeatmapFilter.severity}</strong></span>
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
                <label className="block text-caption text-muted mb-1">Cluster</label>
                <select
                  value={pceClusterId}
                  onChange={(e) => setPceClusterId(e.target.value)}
                  className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
                >
                  <option value="">All Clusters</option>
                  {clusters.map((c) => (
                    <option key={c.id} value={c.id}>{c.name || c.id}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-caption text-muted mb-1">Namespace</label>
                <input
                  value={pceNamespace}
                  onChange={(e) => setPceNamespace(e.target.value)}
                  placeholder="e.g. default"
                  className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
                />
              </div>
              <div>
                <label className="block text-caption text-muted mb-1">Severity</label>
                <select
                  value={pceSeverityFilter}
                  onChange={(e) => setPceSeverityFilter(e.target.value)}
                  className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
                >
                  <option value="">All</option>
                  <option value="critical">Critical</option>
                  <option value="high">High</option>
                  <option value="medium">Medium</option>
                  <option value="low">Low</option>
                </select>
              </div>
              <div>
                <label className="block text-caption text-muted mb-1">Pod name</label>
                <input
                  value={pcePodName}
                  onChange={(e) => setPcePodName(e.target.value)}
                  placeholder="e.g. my-pod"
                  className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
                />
              </div>
              <div>
                <label className="block text-caption text-muted mb-1">Capability ID</label>
                <input
                  value={pceCapabilityId}
                  onChange={(e) => setPceCapabilityId(e.target.value)}
                  placeholder="e.g. ESC_PRIV_POD"
                  className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
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
              <label className="text-caption text-muted uppercase tracking-wider">Sort:</label>
              <select
                value={pceSort}
                onChange={(e) => setPceSort(e.target.value as typeof pceSort)}
                className="bg-base border border-border rounded px-3 py-1.5 text-body text-text focus:outline-none focus:border-brand"
              >
                <option value="severity_desc">Severity high to low</option>
                <option value="severity_asc">Severity low to high</option>
                <option value="capability_asc">Capability A-Z</option>
                <option value="pod_asc">Pod name A-Z</option>
                <option value="namespace_asc">Namespace A-Z</option>
              </select>
              <span className="text-caption text-muted">Sort applies to the current page of results.</span>
            </div>
            <div className="ui-table-scroll rounded-lg border border-border bg-surface">
              <table className={UI_TABLE}>
                <thead className={UI_THEAD_STICKY}>
                  <tr>
                    <th className={UI_TH_COMPACT}>Pod Name</th>
                    <th className={`${UI_TH_COMPACT} hidden lg:table-cell`}>Pod UID</th>
                    <th className={UI_TH_COMPACT}>Namespace</th>
                    <th className={UI_TH_COMPACT}>Capability</th>
                    <th className={UI_TH_COMPACT}>Severity</th>
                    <th className={`${UI_TH_COMPACT} max-w-[140px]`}>Evidence</th>
                    <th className={`${UI_TH_COMPACT} whitespace-nowrap`}>Last Seen</th>
                  </tr>
                </thead>
                <tbody>
                  {pceDetails.length === 0 ? (
                    <tr>
                      <td colSpan={7} className={`${UI_TD_COMPACT_TIGHT} py-4 text-center text-muted`}>No matching capability records. Adjust filters and run again.</td>
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
                        <tr key={`${row.podUid}-${row.capabilityId}`} className={UI_TR}>
                          <td className={`${UI_TD_COMPACT_TIGHT} text-text font-medium`} title={row.podUid}>{row.podName ?? '—'}</td>
                          <td className={`${UI_TD_COMPACT_TIGHT} text-muted font-mono text-caption truncate max-w-[120px] hidden lg:table-cell`} title={row.podUid}>{row.podUid}</td>
                          <td className={`${UI_TD_COMPACT_TIGHT} text-text`}>{row.namespace}</td>
                          <td className={`${UI_TD_COMPACT_TIGHT} text-text`}>{row.capabilityId}</td>
                          <td className={UI_TD_COMPACT_TIGHT}>
                            <span className={`px-2 py-0.5 rounded text-caption font-bold uppercase border ${getSeverityBadgeClass(row.severity)}`}>
                              {row.severity}
                            </span>
                          </td>
                          <td className={`${UI_TD_COMPACT_TIGHT} text-muted text-caption max-w-[200px] leading-snug`} title={evSum.title || undefined}>
                            {evSum.label ? <span className="line-clamp-2">{evSum.label}</span> : '—'}
                          </td>
                          <td className={`${UI_TD_COMPACT_TIGHT} text-muted text-caption whitespace-nowrap`} title={row.lastSeenAt || row.updatedAt || undefined}>{lastSeen}</td>
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

      {/* ----- Evidence & References tab ----- */}
      {activeTab === 'reference' && (
        <div className="space-y-6">
          <div className="rounded-lg border border-border bg-surface p-4 md:p-6">
            <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
              <div className="min-w-0">
                <h2 className="text-section-title text-text flex items-center gap-2">
                  <FileText size={18} className="text-brand" />
                  Finding evidence log
                </h2>
                <p className="mt-1 max-w-[72ch] text-caption text-muted">
                  Evidence summarized from findings loaded in the current scope. Open a finding for raw payloads and workflow history.
                </p>
              </div>
              <div className="rounded-md border border-border/70 bg-base/40 px-2.5 py-1 text-caption text-muted md:text-right">
                {findingEvidenceRows.length} finding{findingEvidenceRows.length !== 1 ? 's' : ''} loaded
              </div>
            </div>
            {findingEvidenceRows.length === 0 ? (
              <SemanticEmptyState
                state="no_data"
	                title="No evidence in loaded findings"
	                reason="Change filters, widen the time window, or open a finding detail to inspect raw backend evidence for that finding."
              />
            ) : (
              <div className="overflow-x-auto rounded-lg border border-border">
                <table className={UI_TABLE}>
                  <thead className={UI_THEAD_STICKY}>
                    <tr>
                      <th className={UI_TH_COMPACT}>Finding</th>
                      <th className={UI_TH_COMPACT}>Summary</th>
                      <th className={`${UI_TH_COMPACT} w-20`}>Records</th>
                      <th className={`${UI_TH_COMPACT} w-36`}>Updated</th>
                      <th className={`${UI_TH_COMPACT} text-right`}>Action</th>
                    </tr>
                  </thead>
                  <tbody>
                    {findingEvidenceRows.map(({ risk, entries, summary }) => (
                      <tr key={risk.id} className={UI_TR}>
                        <td className={UI_TD_COMPACT_TIGHT}>
                          <div className="min-w-[14rem]">
                            <div className="text-body font-medium text-text leading-snug">{risk.title || `Finding ${risk.id}`}</div>
                            <div className="mt-1 flex flex-wrap items-center gap-1.5 text-caption text-muted">
                              <span className="font-mono">#{risk.id}</span>
                              <span>{insightTypeUiLabel(risk.insightType)}</span>
                              <span className={`rounded border px-1.5 py-0.5 text-micro font-semibold uppercase ${getSeverityBadgeClass(risk.finalLevel ?? risk.severity)}`}>
                                {risk.finalLevel ?? risk.severity}
                              </span>
                            </div>
                          </div>
                        </td>
                        <td className={`${UI_TD_COMPACT_TIGHT} max-w-[34rem] text-caption text-muted leading-snug`}>
                          <span className="line-clamp-2" title={summary}>{summary || '—'}</span>
                        </td>
                        <td className={`${UI_TD_COMPACT_TIGHT} tabular-nums text-text`}>{entries.length}</td>
                        <td className={`${UI_TD_COMPACT_TIGHT} text-caption text-muted whitespace-nowrap`}>
                          {risk.updatedAt ? formatDateTime(risk.updatedAt) : risk.timestamp ? formatDateTime(risk.timestamp) : '—'}
                        </td>
                        <td className={`${UI_TD_COMPACT_TIGHT} text-right`}>
                          <Button size="sm" variant="secondary" onClick={() => toggleRiskDrawer(risk)}>
                            Quick view
                          </Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          <div className="bg-surface border border-border p-4 md:p-6 rounded-lg">
            <h2 className="text-section-title text-text mb-1 flex items-center gap-2">
              <AlertTriangle size={18} className="text-brand" />
              Runtime evidence
            </h2>
            <p className="text-caption text-muted mb-4">Runtime events used as supporting telemetry for risk investigation. Filtered to the Risk Operations time scope.</p>
            <RuntimeSignalsTable
              clusterId={effectiveClusterId || undefined}
              podUid={podUidFromEvidenceUrl?.trim() || undefined}
              riskAlignedSinceMinutes={sinceMinutesForApi ?? null}
            />
          </div>
          <div className="rounded-lg border border-border bg-base/30 px-4 py-3 text-caption text-muted">
            {canPlatformAudit ? (
              <p>
                Platform-wide audit trail and operational error logs live under{' '}
                <Link to="/monitoring?section=audit" className="text-brand font-medium hover:underline">
                  Monitoring
                </Link>
                . Per-finding actions still appear in the drawer for each finding you open.
              </p>
            ) : (
              <p>
                Per-finding activity (view, acknowledge, resolve, dismiss) appears in the finding drawer under Evidence when you open a row from the table.
              </p>
            )}
          </div>
        </div>
      )}
      </div>

      {/* Risk Detail Drawer — extracted component (owns its own data loading + focus trap) */}
      {selectedRisk && (
        <RiskDrawer
          insight={selectedRisk}
          sinceMinutesForApi={sinceMinutesForApi}
          canAck={canDrawerAck}
          canResolve={canDrawerResolve}
          canDismiss={canDrawerDismiss}
          onClose={closeRiskDrawer}
          onActionComplete={(action) => {
            setToast({ message: `Finding ${action === 'acknowledge' ? 'acknowledged' : action === 'resolve' ? 'resolved' : 'dismissed'}.`, variant: 'success' });
            fetchDataRef.current();
          }}
          onOpenPceTab={() => setActiveTab('pce')}
        />
      )}
      {pendingBulkAction && (
        <div className="fixed inset-0 z-modal flex items-center justify-center bg-black/55 px-4">
          <div
            ref={bulkActionDialogRef}
            role="dialog"
            aria-modal="true"
            aria-labelledby="bulk-risk-action-title"
            className="w-full max-w-md rounded-xl border border-border bg-surface p-4 shadow-2xl"
          >
            <div className="flex items-start justify-between gap-3">
              <div>
                <h3 id="bulk-risk-action-title" className="text-body font-semibold text-text">
                  Review bulk {pendingBulkActionLabel.toLowerCase()} action
                </h3>
                <p className="mt-1 text-caption text-muted">
                  This updates {selectedIds.size} selected finding(s) in the current Risk Operations queue.
                </p>
              </div>
              <button
                type="button"
                className="inline-flex min-h-10 min-w-10 items-center justify-center rounded text-muted hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70"
                aria-label="Cancel bulk workflow action"
                onClick={() => {
                  setPendingBulkAction(null);
                  setBulkActionReason('');
                }}
              >
                <X size={18} />
              </button>
            </div>

            <div className="mt-4 rounded-lg border border-border bg-base/70 px-3 py-2 text-caption text-muted">
              <div>Target state: <span className="text-text">{pendingBulkActionLabel}</span></div>
              <div>Selected findings: <span className="text-text">{selectedIds.size}</span></div>
              <div>Scope: <span className="text-text">{effectiveClusterId || 'All clusters'}</span></div>
              <div>Time window: <span className="text-text">{effectiveSinceMinutesNum ? formatMinutesHuman(effectiveSinceMinutesNum) : 'All time'}</span></div>
            </div>

            <label className="mt-4 block text-caption font-semibold text-muted" htmlFor="bulk-risk-action-reason">
              {pendingBulkReasonRequired ? 'Reason required' : 'Analyst note'}
            </label>
            <textarea
              id="bulk-risk-action-reason"
              value={bulkActionReason}
              onChange={(event) => setBulkActionReason(event.target.value)}
              className="mt-1 min-h-24 w-full rounded-lg border border-border bg-base px-3 py-2 text-body text-text outline-none focus:border-brand focus-visible:ring-2 focus-visible:ring-brand/60"
              placeholder={
                pendingBulkAction === 'resolve'
                  ? 'Describe the remediation or compensating control.'
                  : pendingBulkAction === 'dismiss'
                    ? 'Explain why these findings are not actionable.'
                    : 'Optional local review note.'
              }
            />
            {pendingBulkAction === 'acknowledge' && (
              <p className="mt-1 text-micro text-muted">
                Current API persists acknowledgement state; notes are for review before submitting.
              </p>
            )}

            <div className="mt-4 flex flex-wrap justify-end gap-2">
              <Button
                size="sm"
                variant="secondary"
                onClick={() => {
                  setPendingBulkAction(null);
                  setBulkActionReason('');
                }}
              >
                Cancel
              </Button>
              <Button size="sm" disabled={pendingBulkSubmitDisabled} onClick={() => void runBulkAction()}>
                {bulkActionBusy ? (
                  <span className="inline-flex items-center gap-1.5">
                    <Loader2 className="w-3.5 h-3.5 animate-spin" />
                    Submitting
                  </span>
                ) : (
                  pendingBulkActionLabel
                )}
              </Button>
            </div>
          </div>
        </div>
      )}
      {toast && (
        <div
          role="status"
          className={`fixed top-4 right-4 z-toast max-w-sm px-4 py-3 rounded-lg border text-body shadow-xl ${
            toast.variant === 'success'
              ? 'bg-emerald-950/95 border-emerald-600/50 text-emerald-100'
              : 'bg-red-950/95 border-red-600/50 text-red-100'
          }`}
        >
          {toast.message}
        </div>
      )}
    </PageLayout>
    </PageContract>
  );
};
