import { summarizeBulkFindingResult } from '../lib/bulkFindingResult';
import { normalizeInsightStatus } from '../lib/api';
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { api, getAvailabilityIssue, type RiskInsightsSortKey } from '../lib/api';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useAbortSignal, isAbortError } from '../hooks/useAbortSignal';
import { useClusters } from '../hooks/useClusters';
import { useEntityStore } from '../store/entityStore';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { Insight } from '../types';
import { Button } from '../components/ui/Button';
import { ResetFiltersButton } from '../components/ResetFiltersButton';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Pagination } from '../components/Pagination';
import { AlertTriangle, Search, ArrowRight, X, Loader2, FileText, UserRound } from 'lucide-react';
import { useLocation, useNavigate, useSearchParams, Link } from 'react-router-dom';
import { useClusterStore } from '../store/clusterStore';
import { useTimeWindowStore } from '../store/timeWindowStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';
import { getSeverityBadgeClass } from '../lib/severity';
import { RISK_CENTER_DESCRIPTION } from '../constants/labels';
import { PAGE_TITLES } from '../lib/pageTitles';
import { RuntimeSignalsTable } from '../components/RuntimeSignalsTable';
import { RiskDrawer } from '../components/RiskDrawer';
import { insightTypeUiLabel, riskListSecondaryLabel } from '../lib/riskDisplay';
import { RiskFindingsSavedViews } from '../components/RiskFindingsSavedViews';
import { formatMinutesHuman } from '../lib/formatDuration';
import { formatDateTime } from '../lib/display';
import { podDetailPath } from '../lib/podRoute';
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
  UI_PILL_IDLE_SPLIT,
} from '../lib/formChrome';
import { can, P } from '../lib/permissions';
import { usePermUser } from '../hooks/usePermUser';
import { ACTION_IDS, canRunAction } from '../lib/actionAccess';
import { usePersona } from '../hooks/usePersona';
import { useOperationalMaterialization } from '../hooks/useOperationalMaterialization';
import { personaAllowsAction} from '../lib/persona';
import { getRiskWorkspaceConfig, type RiskTabId } from '../lib/personaRiskWorkspace';
import { ProvenanceBadge } from '../design-system/components/ProvenanceBadge';
import { When } from '../components/When';
import { insightProvenance, insightProvenanceTitle} from '../lib/provenance';
import { PageContract } from '../components/PageContract';
import { SemanticEmptyState } from '../design-system/components/SemanticEmptyState';
import { buildEvidenceLogEntries, summarizeEvidenceLog } from '../lib/evidenceLog';

type TabId = 'triage' | 'reference';
/** `open` (needs triage or in review) is only used by the Assigned to me view. */
type FindingStatusFilter = 'all' | 'active' | 'open' | 'resolved' | 'acknowledged' | 'dismissed';

/** Findings views, in workflow order. `view` is the URL value; the queue itself is the default. */
const QUEUE_VIEWS: { view: string; status: FindingStatusFilter; label: string }[] = [
  { view: 'triage', status: 'active', label: 'Needs triage' },
  { view: 'mine', status: 'open', label: 'Assigned to me' },
  { view: 'review', status: 'acknowledged', label: 'In review' },
  { view: 'resolved', status: 'resolved', label: 'Resolved' },
  { view: 'dismissed', status: 'dismissed', label: 'Dismissed' },
  { view: 'all', status: 'all', label: 'All' },
];

function statusForView(view: string | null): FindingStatusFilter {
  return QUEUE_VIEWS.find((v) => v.view === view)?.status ?? 'active';
}

/** The Assigned to me view is the open queue narrowed to the signed-in user. */
function assigneeForStatus(status: FindingStatusFilter): 'me' | undefined {
  return status === 'open' ? 'me' : undefined;
}
type BulkFindingAction = 'acknowledge' | 'resolve' | 'dismiss';

/** Shared table chrome for Risk Center data tables */
const RISK_FINDINGS_COLS_KEY = 'fortuna-risk-findings-table-cols-v2';
/** GET /risk/insights silently falls back to 20 rows when pageSize > 100, so never request more. */
const RISKS_API_MAX_PAGE_SIZE = 100;

type RiskFindingsTableCols = {
  type: boolean;
  resource: boolean;
  evidence: boolean;
  nsCluster: boolean;
  detected: boolean;
  updated: boolean;
};

const defaultRiskFindingsCols: RiskFindingsTableCols = {
  type: true,
  resource: true,
  evidence: false,
  nsCluster: true,
  detected: true,
  updated: false,
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

function findingResourceTarget(resource?: FindingResource, clusterId?: string): string | undefined {
  const id = String(resource?.id ?? '').trim();
  if (!resource || !id) return undefined;
  const kind = normalizeFindingResourceKind(resource.kind);
  if (kind === 'pod') return podDetailPath(id, clusterId);
  if (kind === 'serviceaccount') {
    const clusterQuery = clusterId ? `?clusterId=${encodeURIComponent(clusterId)}` : '';
    return `/identities/uid/${encodeURIComponent(id)}${clusterQuery}`;
  }
  return undefined;
}

/** Findings queue sort options → GET /risk/insights sort/order (sorted server-side, across pages). */
const RISK_SORT_OPTIONS = {
  newest: { label: 'Newest first', sort: 'detected', order: 'desc' },
  oldest: { label: 'Oldest first', sort: 'detected', order: 'asc' },
  updated_desc: { label: 'Recently updated', sort: 'updated', order: 'desc' },
  updated_asc: { label: 'Least recently updated', sort: 'updated', order: 'asc' },
  score_desc: { label: 'Priority score high to low', sort: 'score', order: 'desc' },
  score_asc: { label: 'Priority score low to high', sort: 'score', order: 'asc' },
  severity_desc: { label: 'Rule severity critical to info', sort: 'severity', order: 'desc' },
  severity_asc: { label: 'Rule severity info to critical', sort: 'severity', order: 'asc' },
  title_asc: { label: 'Title A-Z', sort: 'title', order: 'asc' },
  title_desc: { label: 'Title Z-A', sort: 'title', order: 'desc' },
} as const satisfies Record<string, { label: string; sort: RiskInsightsSortKey; order: 'asc' | 'desc' }>;
type RiskSortId = keyof typeof RISK_SORT_OPTIONS;

export const RiskCenter: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { selectedClusterId, setSelectedClusterId } = useClusterStore();
  const permUser = usePermUser();
  const { id: personaId, profile } = usePersona();
  const { allowedRoutes } = useOperationalMaterialization();
  const riskWorkspace = getRiskWorkspaceConfig(personaId);
  const canBulkFindings = personaAllowsAction(profile, 'bulk', permUser, P.findingsBulk);
  const canExportFindings = canRunAction(permUser, ACTION_IDS.findingExport);
  const canRiskEvaluate = canRunAction(permUser, ACTION_IDS.riskEvaluate);
  const canPlatformAudit = can(permUser, P.systemAuditRead);
  // Only people who can triage are ever assigned, so the view would always be empty for anyone else.
  const queueViews = QUEUE_VIEWS.filter((v) => v.view !== 'mine' || can(permUser, P.findingsAck));
  const { valueMinutes: timeWindowMinutes, setValueMinutes: setTimeWindowMinutes } = useTimeWindowStore();
  const [searchParams, setSearchParams] = useSearchParams();
  // `severity` is accepted only as a legacy deep-link alias for finalLevel.
  const finalLevelFromUrl = searchParams.get('finalLevel') ?? searchParams.get('severity');
  const searchFromUrl = searchParams.get('search') ?? '';
  const clusterIdFromUrl = searchParams.get('clusterId');
  const sinceMinutesFromUrl = searchParams.get('sinceMinutes');
  const [activeTab, setActiveTab] = useState<TabId>('triage');
  const [risks, setRisks] = useState<Insight[]>([]);
  const [risksTotal, setRisksTotal] = useState(0);
  const { clusters } = useClusters();
  const viewFromUrl = searchParams.get('view');
  const [statusFilter, setStatusFilter] = useState<FindingStatusFilter>(() => statusForView(viewFromUrl));
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedRisk, setSelectedRisk] = useState<Insight | null>(null);

  const [error, setError] = useState<string | null>(null);
  const [risksPage, setRisksPage] = useState(1);
  const [risksPageSize, setRisksPageSize] = useState(riskWorkspace.pageSize);
  const [riskSort, setRiskSort] = useState<RiskSortId>('score_desc');
  /** Risk Findings table: per-resource rows vs grouped by CVE/title key (backend view=group). */
  const [findingsListView, setFindingsListView] = useState<'instance' | 'group'>('instance');
  const [riskLevelFilter, setRiskLevelFilter] = useState<string>(
    finalLevelFromUrl && ['critical', 'high', 'medium', 'low'].includes(finalLevelFromUrl) ? finalLevelFromUrl : ''
  ); // ADR: low|medium|high|critical → API finalLevel
  const [namespaceFilter, setNamespaceFilter] = useState<string>('');
  const [debouncedNamespaceFilter, setDebouncedNamespaceFilter] = useState<string>('');
  const [typeFilter, setTypeFilter] = useState<string>('');
  const [debouncedSearchTerm, setDebouncedSearchTerm] = useState('');
  const [toast, setToast] = useState<{ message: string; variant: 'success' | 'error' } | null>(null);
  const [pageBlocking, setPageBlocking] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

  const hasLoadedOnceRef = React.useRef(false);

  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [pendingBulkAction, setPendingBulkAction] = useState<BulkFindingAction | null>(null);
  const [bulkActionReason, setBulkActionReason] = useState('');
  const [bulkActionBusy, setBulkActionBusy] = useState(false);
  const bulkActionDialogRef = useRef<HTMLDivElement>(null);

  /** Recalculate all scores (POST /risk/scores/sync). */
  const [syncScoresBusy, setSyncScoresBusy] = useState(false);
  const [riskFindingsCols, setRiskFindingsCols] = useState<RiskFindingsTableCols>(() => loadRiskFindingsCols());

  // Resolve cluster + time: URL from Dashboard link overrides store so Risk Center shows same scope
  const effectiveClusterId = clusterIdFromUrl ?? selectedClusterId ?? undefined;
  const effectiveSinceMinutes = sinceMinutesFromUrl != null ? parseInt(sinceMinutesFromUrl, 10) : timeWindowMinutes;
  const effectiveSinceMinutesNum = Number.isFinite(effectiveSinceMinutes) && effectiveSinceMinutes > 0 ? effectiveSinceMinutes : undefined;

  /** Export loading for UX feedback (tooltip 10k, filter warning). */
  const [exportLoading, setExportLoading] = useState(false);
  const insightIdFromUrl = searchParams.get('insightId');
  const podUidFromEvidenceUrl = searchParams.get('podUid');
  /** Set by links from pod detail, Network and Attack Paths: the queue narrowed to one workload. */
  const resourceUidFilter = searchParams.get('resourceUid')?.trim() ?? '';
  const suppressedInsightOpenRef = React.useRef<string | null>(null);

  const sinceMinutesForApi = effectiveSinceMinutesNum ?? (timeWindowMinutes > 0 ? timeWindowMinutes : undefined);

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
  // Free-text namespace is sent to the API; debounce it like search so each keystroke is not a request.
  React.useEffect(() => {
    const id = window.setTimeout(() => setDebouncedNamespaceFilter(namespaceFilter.trim()), 400);
    return () => window.clearTimeout(id);
  }, [namespaceFilter]);

  const clusterLabelById = useMemo(() => {
    const m = new Map<string, string>();
    clusters.forEach((c) => m.set(c.id, c.name || c.id));
    return m;
  }, [clusters]);
  const scopeClusterDisplay = effectiveClusterId
    ? (clusterLabelById.get(effectiveClusterId) ?? effectiveClusterId)
    : null;

  // Sync active tab with route: /risks and /risks/findings are the queue; /risks/evidence is the secondary view.
  React.useEffect(() => {
    const path = location.pathname || '';
    const tab: TabId = path.endsWith('/evidence') ? 'reference' : 'triage';
    // A deep link must not open a tab the persona's workspace hides.
    setActiveTab(riskWorkspace.visibleTabs.includes(tab as RiskTabId) ? tab : (riskWorkspace.defaultTab as TabId));
  }, [location.pathname, riskWorkspace]);

  // Sync URL -> store so Dashboard link scope is applied (and persisted for next visits)
  React.useEffect(() => {
    if (clusterIdFromUrl?.trim()) setSelectedClusterId(clusterIdFromUrl.trim());
  }, [clusterIdFromUrl, setSelectedClusterId]);
  React.useEffect(() => {
    const m = sinceMinutesFromUrl != null ? parseInt(sinceMinutesFromUrl, 10) : NaN;
    if (Number.isFinite(m) && m >= 0) setTimeWindowMinutes(m);
  }, [sinceMinutesFromUrl, setTimeWindowMinutes]);
  // URL scope wins over the store, so once the user changes the global cluster selector or time window,
  // drop the now-stale URL params; otherwise the header change would never reach this page.
  const scopeSyncMountedRef = React.useRef(false);
  React.useEffect(() => {
    if (!scopeSyncMountedRef.current) {
      scopeSyncMountedRef.current = true;
      return;
    }
    const urlCluster = clusterIdFromUrl?.trim() || null;
    const urlMinutes = sinceMinutesFromUrl != null ? parseInt(sinceMinutesFromUrl, 10) : NaN;
    const clusterDiverged = urlCluster != null && selectedClusterId !== urlCluster;
    const windowDiverged = Number.isFinite(urlMinutes) && urlMinutes >= 0 && timeWindowMinutes !== urlMinutes;
    if (!clusterDiverged && !windowDiverged) return;
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      if (clusterDiverged) next.delete('clusterId');
      if (windowDiverged) next.delete('sinceMinutes');
      return next;
    }, { replace: true });
    // Only react to store changes; the URL -> store effects above handle URL changes.
  }, [selectedClusterId, timeWindowMinutes]); // eslint-disable-line react-hooks/exhaustive-deps

  // Back/forward and links move between views through ?view=.
  React.useEffect(() => {
    setStatusFilter(statusForView(viewFromUrl));
  }, [viewFromUrl]);

  // Sync filter and search from URL (e.g. from global search or deep links)
  React.useEffect(() => {
    if (finalLevelFromUrl && ['critical', 'high', 'medium', 'low'].includes(finalLevelFromUrl)) {
      setRiskLevelFilter(finalLevelFromUrl);
    }
  }, [finalLevelFromUrl]);
  // Keep the input and the applied search in sync with the URL, including when ?search= is removed;
  // otherwise the box keeps stale text while the list is unfiltered.
  React.useEffect(() => {
    setSearchTerm(searchFromUrl);
    setDebouncedSearchTerm(searchFromUrl.trim());
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
    const sinceMinutes = sinceMinutesForApi;
    const clusterId = effectiveClusterId ?? selectedClusterId ?? undefined;
    const risksListParamsBase = {
      status: statusFilter,
      assignee: assigneeForStatus(statusFilter),
      search: debouncedSearchTerm || undefined,
      clusterId: clusterId ?? undefined,
      namespace: debouncedNamespaceFilter || undefined,
      resourceUid: resourceUidFilter || undefined,
      type: typeFilter || undefined,
      sinceMinutes,
      // Always ask for scores: the level badge is the risk level from the score, whatever the sort.
      withScores: 1,
      finalLevel: (riskLevelFilter || undefined) as '' | 'low' | 'medium' | 'high' | 'critical' | undefined,
      view: findingsListView === 'group' ? ('group' as const) : undefined,
      sort: RISK_SORT_OPTIONS[riskSort].sort,
      order: RISK_SORT_OPTIONS[riskSort].order,
    };
    try {
      const errors: string[] = [];
      {
        // The Runtime evidence view summarizes evidence across the first page of up to the API maximum.
        try {
          const page = await api.getRisks({
            page: activeTab === 'reference' ? 1 : risksPage,
            pageSize: activeTab === 'reference' ? RISKS_API_MAX_PAGE_SIZE : risksPageSize,
            ...risksListParamsBase,
          });
          if (signal.aborted) return;
          setRisks(page.insights);
          setRisksTotal(page.total);
        } catch (e) {
          if (signal.aborted || isAbortError(e)) return;
          setRisks([]);
          setRisksTotal(0);
          errors.push(getAvailabilityIssue(e, 'Findings').description);
        }
      }
      if (signal.aborted) return;
      if (errors.length > 0) setError(errors.join('; '));
    } catch (e) {
      if (isAbortError(e)) return; // cancelled by rapid filter change — discard silently
      throw e;
    } finally {
      // A superseded fetch leaves the loading state to the fetch that replaced it.
      if (!signal.aborted) {
        hasLoadedOnceRef.current = true;
        setPageBlocking(false);
        setRefreshing(false);
      }
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
    debouncedNamespaceFilter,
    resourceUidFilter,
    typeFilter,
    activeTab,
    findingsListView,
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
      const result = await api.bulkInsightsAction({
        action: pendingBulkAction,
        insightIds: ids,
        resolution: pendingBulkAction === 'resolve' ? reason : undefined,
        reason: pendingBulkAction === 'dismiss' ? reason : undefined,
      });
      const outcome = summarizeBulkFindingResult(result, ids);
      setSelectedIds(new Set(outcome.remainingIds));
      setPendingBulkAction(null);
      setBulkActionReason('');
      setToast({ message: outcome.message, variant: outcome.complete ? 'success' : 'error' });
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
      ws = new WebSocket(wsUrl, api.getWebSocketProtocols());
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

  React.useEffect(() => { setRisksPage(1); }, [statusFilter, debouncedSearchTerm, timeWindowMinutes, riskLevelFilter, debouncedNamespaceFilter, resourceUidFilter, typeFilter, effectiveClusterId, riskSort]);
  // Bulk selection must not carry hidden findings across a filter or scope change.
  React.useEffect(() => { setSelectedIds(new Set()); }, [statusFilter, debouncedSearchTerm, sinceMinutesForApi, riskLevelFilter, debouncedNamespaceFilter, typeFilter, effectiveClusterId, findingsListView]);

  const appliedInsightIdRef = React.useRef<string | null>(null);
  // Global drawer: open by URL ?insightId= (from Overview/PCE/Evidence deep link)
  React.useEffect(() => {
    if (!insightIdFromUrl) {
      suppressedInsightOpenRef.current = null;
      appliedInsightIdRef.current = null;
      return;
    }
    const id = insightIdFromUrl.trim();
    if (!id) return;
    if (suppressedInsightOpenRef.current === id) return;
    // Only a changed URL (deep link, back/forward) drives the selection. A URL that has not caught up
    // with a J/K step yet must not pull the selection back.
    if (appliedInsightIdRef.current === id && selectedRisk) return;
    appliedInsightIdRef.current = id;
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
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      next.set('insightId', risk.id);
      return next;
    }, { replace: true });
  }, [setSearchParams]);

  /** Opens the row `delta` away from the open finding (J/K, and after an action). Returns false at either end. */
  const stepRiskDrawer = React.useCallback((delta: 1 | -1): boolean => {
    if (risks.length === 0) return false;
    const idx = selectedRisk ? risks.findIndex((r) => r.id === selectedRisk.id) : -1;
    const next = idx === -1 ? (delta === 1 ? 0 : risks.length - 1) : idx + delta;
    if (next < 0 || next >= risks.length) return false;
    openRiskDrawer(risks[next]);
    document.querySelector(`[data-finding-row="${CSS.escape(risks[next].id)}"]`)?.scrollIntoView({ block: 'nearest' });
    return true;
  }, [risks, selectedRisk, openRiskDrawer]);

  // Queue shortcuts: J next, K previous, O open the full page. Ignored while typing or in a dialog.
  React.useEffect(() => {
    if (activeTab !== 'triage') return undefined;
    const onKey = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.altKey) return;
      const target = e.target as HTMLElement | null;
      if (target && (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName))) return;
      const otherDialogOpen = Array.from(document.querySelectorAll('[role="dialog"][aria-modal="true"]')).some(
        (el) => !el.hasAttribute('data-risk-drawer'),
      );
      if (otherDialogOpen) return;
      const key = e.key.toLowerCase();
      if (key === 'j' || key === 'k') {
        e.preventDefault();
        stepRiskDrawer(key === 'j' ? 1 : -1);
      } else if (key === 'o' && selectedRisk) {
        e.preventDefault();
        navigate(`/risks/${encodeURIComponent(selectedRisk.id)}`);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [activeTab, stepRiskDrawer, selectedRisk, navigate]);

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


  React.useEffect(() => {
    setRisksPage(1);
  }, [findingsListView]);



  const riskDataEmpty =
    risksTotal === 0 &&
    !debouncedSearchTerm &&
    !riskLevelFilter &&
    !debouncedNamespaceFilter &&
    !resourceUidFilter &&
    !typeFilter &&
    statusFilter === 'active';

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




  // Rows arrive sorted by the server (sort/order apply across all pages).
  const paginatedRisks = risks;

  /** True when any findings-queue filter, search or sort differs from its default (status=active, score sort, everything else empty). */
  const hasFindingsFilters =
    riskSort !== 'score_desc' ||
    riskLevelFilter !== '' ||
    searchTerm.trim() !== '' ||
    debouncedSearchTerm !== '' ||
    namespaceFilter.trim() !== '' ||
    resourceUidFilter !== '' ||
    typeFilter !== '';

  /** Clears every findings-queue filter, search and sort. Cluster scope and time window are global and stay as they are. */
  const resetFindingsFilters = useCallback(() => {
    setRiskSort('score_desc');
    setRiskLevelFilter('');
    setSearchTerm('');
    setDebouncedSearchTerm('');
    setNamespaceFilter('');
    setDebouncedNamespaceFilter('');
    setTypeFilter('');
    // search/finalLevel (and legacy severity) are re-synced from the URL, so drop them there too.
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      ['search', 'finalLevel', 'severity', 'resourceUid'].forEach((k) => next.delete(k));
      return next;
    }, { replace: true });
    setRisksPage(1);
  }, [setSearchParams]);


  const statusLabelMap: Record<string, string> = {
    dismissed: 'Dismissed',
    unknown: 'Unknown',
    new: 'Active',
    acknowledged: 'In review',
    resolved: 'Resolved',
  };

  const findingsTableColCount = useMemo(() => {
    let n = 4; // risk, finding, status, actions
    if (riskWorkspace.showRowSelection) n += 1;
    if (riskFindingsCols.evidence) n += 1;
    if (riskFindingsCols.type) n += 1;
    if (riskFindingsCols.resource) n += 1;
    if (!effectiveClusterId && riskFindingsCols.nsCluster) n += 1;
    if (riskFindingsCols.detected) n += 1;
    if (riskFindingsCols.updated) n += 1;
    return n;
  }, [riskFindingsCols, effectiveClusterId, riskWorkspace.showRowSelection]);

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
      <PageContract feature="risk_operations" loading loadingMessage="Loading findings…">
        <PageLayout title={PAGE_TITLES.riskOperations} description={RISK_CENTER_DESCRIPTION}>
        <div className="space-y-6 animate-pulse" aria-busy="true" aria-label="Loading findings">
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


  const secondaryViews = (
    [
      { id: 'reference' as const, label: 'Runtime evidence', path: '/risks/evidence' },
    ]
  ).filter((v) => riskWorkspace.visibleTabs.includes(v.id as RiskTabId));
  const openQueueView = (status: FindingStatusFilter) => {
    closeRiskDrawer();
    const sp = new URLSearchParams(searchParams);
    sp.delete('insightId');
    const view = QUEUE_VIEWS.find((v) => v.status === status)?.view;
    if (view && view !== 'triage') sp.set('view', view);
    else sp.delete('view');
    setStatusFilter(status);
    const qs = sp.toString();
    navigate(`/risks/findings${qs ? `?${qs}` : ''}`);
  };
  const openSecondaryView = (path: string) => {
    closeRiskDrawer();
    const sp = new URLSearchParams(searchParams);
    ['insightId', 'view'].forEach((k) => sp.delete(k));
    const qs = sp.toString();
    navigate(`${path}${qs ? `?${qs}` : ''}`);
  };
  const viewButtonClass = (active: boolean) =>
    `whitespace-nowrap rounded-md px-3 py-1.5 text-body font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 ${
      active ? 'bg-surface-2 text-text shadow-sm' : 'text-muted hover:text-text'
    }`;
  return (
    <PageContract
      feature="risk_operations"
      dataEmpty={riskDataEmpty}
    >
    <PageLayout
      title={PAGE_TITLES.riskOperations}
      description={RISK_CENTER_DESCRIPTION}
      actions={
        activeTab === 'triage' ? (
          <div className="flex flex-wrap gap-2">
            {canRiskEvaluate ? (
              <Button
                variant="secondary"
                size="sm"
                disabled={syncScoresBusy}
                title="Recalculate risk scores for every resource with active findings."
                onClick={async () => {
                  setSyncScoresBusy(true);
                  try {
                    const res = await api.syncRiskScores();
                    setToast({
                      message: res.resources === 0 ? 'No resources to rescore.' : `Rescoring ${res.resources} resources. The list refreshes in a few seconds.`,
                      variant: 'success',
                    });
                    if (res.resources > 0) window.setTimeout(() => fetchDataRef.current(), 4000);
                  } catch (e) {
                    setToast({ message: `Could not recalculate scores: ${e instanceof Error ? e.message : String(e)}`, variant: 'error' });
                  } finally {
                    setSyncScoresBusy(false);
                  }
                }}
              >
                {syncScoresBusy ? <Loader2 className="w-4 h-4 animate-spin inline mr-1" /> : null}
                Recalculate scores
              </Button>
            ) : null}
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
                assignee: assigneeForStatus(statusFilter),
                resourceUid: resourceUidFilter || undefined,
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
                assignee: assigneeForStatus(statusFilter),
                resourceUid: resourceUidFilter || undefined,
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
        ) : undefined
      }
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

      {/* Views replace tabs: one queue, filtered by where each finding is in its workflow. */}
      <nav aria-label="Finding views" className="flex flex-wrap items-center gap-1 rounded-lg border border-border bg-base/60 p-1">
        {queueViews.map((v) => {
          const active = activeTab === 'triage' && statusFilter === v.status;
          return (
            <button key={v.view} type="button" aria-current={active ? 'page' : undefined} onClick={() => openQueueView(v.status)} className={viewButtonClass(active)}>
              {v.label}
              {active && v.status === 'active' && risksTotal > 0 && findingsListView === 'instance' ? (
                <span className="ml-1.5 rounded-full bg-base px-1.5 text-meta font-semibold text-muted">{risksTotal}</span>
              ) : null}
            </button>
          );
        })}
        {secondaryViews.length > 0 ? <span className="mx-1 h-5 w-px bg-border" aria-hidden /> : null}
        {secondaryViews.map((v) => (
          <button key={v.id} type="button" aria-current={activeTab === v.id ? 'page' : undefined} onClick={() => openSecondaryView(v.path)} className={viewButtonClass(activeTab === v.id)}>
            {v.label}
          </button>
        ))}
      </nav>

      {/* ----- Risks tab ----- */}
      {activeTab === 'triage' && (
        <div className="flex flex-col gap-6">
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
              // A saved view fully defines the queue: drop filters it does not store and URL params
              // (which override the store) so the applied view is exactly what was saved.
              setNamespaceFilter('');
              setDebouncedNamespaceFilter('');
              setTypeFilter('');
              setSelectedClusterId(filters.clusterId ?? null);
              if (filters.sinceMinutes != null) setTimeWindowMinutes(filters.sinceMinutes);
              setSearchParams((prev) => {
                const next = new URLSearchParams(prev);
                ['severity', 'clusterId', 'sinceMinutes'].forEach((k) => next.delete(k));
                // search/finalLevel are re-synced from the URL, so mirror the view instead of deleting them.
                if (filters.searchTerm) next.set('search', filters.searchTerm);
                else next.delete('search');
                if (filters.riskLevelFilter) next.set('finalLevel', filters.riskLevelFilter);
                else next.delete('finalLevel');
                // The view (workflow status) is part of a saved view too.
                const view = QUEUE_VIEWS.find((v) => v.status === filters.statusFilter)?.view;
                if (view && view !== 'triage') next.set('view', view);
                else next.delete('view');
                return next;
              }, { replace: true });
              setRisksPage(1);
            }}
          />
            </div>
          </details>
          ) : null}
          {/* Filters: one row. The view above sets the workflow status. */}
          <div className="order-1 flex flex-wrap items-center gap-2 rounded-lg border border-border bg-surface p-3">
            {resourceUidFilter ? (
              <span className="inline-flex min-h-9 items-center gap-1.5 rounded-lg border border-brand/50 bg-brand/10 pl-3 pr-1 text-caption text-text" data-testid="resource-filter">
                Workload:{' '}
                <span className="font-mono">
                  {(() => {
                    const r = risks.find((x) => x.affectedResources?.[0]?.id === resourceUidFilter)?.affectedResources?.[0];
                    return r?.name ? `${r.namespace ? `${r.namespace}/` : ''}${r.name}` : resourceUidFilter;
                  })()}
                </span>
                <button
                  type="button"
                  aria-label="Show findings on every workload"
                  className="rounded p-1 text-muted hover:text-text"
                  onClick={() =>
                    setSearchParams((prev) => {
                      const next = new URLSearchParams(prev);
                      next.delete('resourceUid');
                      return next;
                    }, { replace: true })
                  }
                >
                  <X className="h-3.5 w-3.5" aria-hidden />
                </button>
              </span>
            ) : null}
            <div className="relative min-w-[16rem] flex-1">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" aria-hidden />
              <input
                type="search"
                aria-label="Search findings"
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                placeholder="Search title, CVE, package, pod, namespace"
                className="h-9 rounded-lg border border-border bg-base px-3 text-body text-text focus:outline-none focus:border-brand w-full pl-9 placeholder:text-muted-2"
              />
            </div>
            <select aria-label="Risk level" value={riskLevelFilter} onChange={(e) => setRiskLevelFilter(e.target.value)} className="h-9 rounded-lg border border-border bg-base px-3 text-body text-text focus:outline-none focus:border-brand">
              <option value="">All risk levels</option>
              <option value="critical">Critical</option>
              <option value="high">High</option>
              <option value="medium">Medium</option>
              <option value="low">Low</option>
            </select>
            <input
              aria-label="Namespace"
              value={namespaceFilter}
              onChange={(e) => setNamespaceFilter(e.target.value)}
              placeholder="All namespaces"
              className="h-9 rounded-lg border border-border bg-base px-3 text-body text-text focus:outline-none focus:border-brand w-40 placeholder:text-muted-2"
            />
            <select aria-label="Rule type" value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)} className="h-9 rounded-lg border border-border bg-base px-3 text-body text-text focus:outline-none focus:border-brand">
              <option value="">All types</option>
              <option value="vulnerability">Vulnerability</option>
              <option value="supply_chain_malware">Supply-chain malware</option>
              <option value="rbac_risk">RBAC risk</option>
              <option value="capability">Capability</option>
            </select>
            <select aria-label="Sort" value={riskSort} onChange={(e) => setRiskSort(e.target.value as RiskSortId)} className="h-9 rounded-lg border border-border bg-base px-3 text-body text-text focus:outline-none focus:border-brand">
              {(Object.keys(RISK_SORT_OPTIONS) as RiskSortId[]).map((id) => (
                <option key={id} value={id}>
                  {RISK_SORT_OPTIONS[id].label}
                </option>
              ))}
            </select>
            <div className="inline-flex h-9 overflow-hidden rounded-lg border border-border text-caption" role="group" aria-label="Layout">
              <button
                type="button"
                aria-pressed={findingsListView === 'instance'}
                onClick={() => setFindingsListView('instance')}
                className={`px-3 font-medium ${findingsListView === 'instance' ? UI_PILL_ACTIVE : UI_PILL_IDLE_SPLIT}`}
              >
                By resource
              </button>
              <button
                type="button"
                aria-pressed={findingsListView === 'group'}
                onClick={() => setFindingsListView('group')}
                className={`border-l border-border px-3 font-medium ${findingsListView === 'group' ? UI_PILL_ACTIVE : UI_PILL_IDLE_SPLIT}`}
                title="Group findings by rule type and CVE (or title when no CVE). A row opens a sample finding."
              >
                Grouped
              </button>
            </div>
            <span className="ml-auto text-caption text-muted">
              {risksTotal} {findingsListView === 'group' ? 'groups' : 'findings'}
            </span>
            <ResetFiltersButton
              onReset={resetFindingsFilters}
              active={hasFindingsFilters}
              title="Clear risk level, namespace, rule type, search and sort"
            />
          </div>
	          <details className="order-1 rounded-lg border border-border/70 bg-surface/60 px-4 py-2 text-caption text-muted">
	            <summary className="cursor-pointer select-none text-muted hover:text-text">Columns and table display</summary>
	            <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-2">
	            <span className="text-muted uppercase tracking-wider whitespace-nowrap">Visible columns:</span>
            {(
              [
                { key: 'type' as const, label: 'Type' },
                { key: 'resource' as const, label: 'Resource' },
                { key: 'evidence' as const, label: 'Evidence kind' },
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
                    <th className={`${tableHeadClass} w-28`}>Risk</th>
                    <th className={`${tableHeadClass} min-w-[16rem]`}>Finding</th>
                    {riskFindingsCols.evidence ? (
                      <th className={`${tableHeadClass} w-24 hidden md:table-cell`}>Evidence</th>
                    ) : null}
                    {riskFindingsCols.type && (
                      <th className={`${tableHeadClass} w-28 hidden sm:table-cell`}>Type</th>
                    )}
                    {riskFindingsCols.resource && (
                      <th className={`${tableHeadClass} w-48`}>Resource</th>
                    )}
                    {!effectiveClusterId && riskFindingsCols.nsCluster && (
                      <th className={`${tableHeadClass} hidden lg:table-cell w-32`}>Namespace / Cluster</th>
                    )}
                    <th className={`${tableHeadClass} w-28`}>Status</th>
                    {riskFindingsCols.detected && (
                      <th className={`${tableHeadClass} w-24 hidden md:table-cell`}>Detected</th>
                    )}
                    {riskFindingsCols.updated && (
                      <th className={`${tableHeadClass} w-24 hidden lg:table-cell`}>Updated</th>
                    )}
                    <th className={`${tableHeadClass} w-24 text-right`}><span className="sr-only">Actions</span></th>
                  </tr>
                </thead>
                <tbody>
                  {paginatedRisks.length === 0 ? (
                    <tr>
                      <td colSpan={findingsTableColCount} className="px-2 py-4">
                        {statusFilter === 'open' && !debouncedSearchTerm && !riskLevelFilter && !debouncedNamespaceFilter && !typeFilter ? (
                          <SemanticEmptyState
                            state="no_data"
                            compact
                            title="Nothing is assigned to you"
                            reason="Open a finding and use Take it, or ask a teammate to assign one to you. Resolved and dismissed findings leave this view."
                          />
                        ) : (
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
                        )}
                      </td>
                    </tr>
                  ) : (
                    paginatedRisks.map((risk) => (
                          <tr
                        key={risk.id}
                        data-finding-row={risk.id}
                        role="button"
                        tabIndex={0}
                        aria-current={selectedRisk?.id === risk.id ? 'true' : undefined}
                        className={`${UI_TR} ${selectedRisk?.id === risk.id ? 'bg-brand/10' : ''} cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 focus-visible:ring-inset`}
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
                        <td
                          className={tableCellClass}
                          title={
                            [
                              risk.score != null ? `Risk score ${risk.score}/100 (a priority signal, not exploit probability)` : 'No risk score yet',
                              risk.exploitabilityScore != null ? `Exploitability factor ${risk.exploitabilityScore.toFixed(1)}` : null,
                              risk.businessImpactScore != null ? `Business impact factor ${risk.businessImpactScore.toFixed(1)}` : null,
                            ]
                              .filter(Boolean)
                              .join(' · ')
                          }
                        >
                          {risk.finalLevel ? (
                            <div className="flex items-center gap-2 whitespace-nowrap">
                              <span className={`text-caption font-semibold px-1.5 py-0.5 rounded capitalize border ${getSeverityBadgeClass(risk.finalLevel)}`}>
                                {risk.finalLevel}
                              </span>
                              {risk.score != null ? <span className="text-caption text-muted tabular-nums">{risk.score}</span> : null}
                            </div>
                          ) : (
                            <span className="text-caption text-muted">No score</span>
                          )}
                        </td>
                        <td className={tableCellClass}>
                          <div className="font-medium text-text leading-snug">{risk.title}</div>
                          <div className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-meta text-muted">
                            <span className="font-mono" title={risk.cveId}>{riskListSecondaryLabel(risk)}</span>
                            {risk.assignee ? (
                              <span className="inline-flex items-center gap-1 rounded-full border border-border px-1.5" title="Owner">
                                <UserRound className="h-3 w-3" aria-hidden />
                                {String(risk.assigneeUserId ?? '') === String(permUser?.id ?? '') ? 'You' : risk.assignee}
                              </span>
                            ) : null}
                          </div>
                          {riskWorkspace.narrativeTable && risk.riskExplanation ? (
                            <p className="text-meta text-muted mt-1 line-clamp-2 max-w-prose">{risk.riskExplanation}</p>
                          ) : null}
                        </td>
                        {riskFindingsCols.evidence ? (
                          <td className={`${tableCellClass} hidden md:table-cell`}>
                            <ProvenanceBadge kind={insightProvenance(risk)} title={insightProvenanceTitle(risk)} />
                          </td>
                        ) : null}
                        {riskFindingsCols.type && (
                          <td className={`${UI_TD} text-muted hidden sm:table-cell text-caption leading-snug`}>
                            <span className="text-text">{insightTypeUiLabel(risk.insightType)}</span>
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
                                const target = findingResourceTarget(resource, risk.clusterId);
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
                            <When iso={risk.timestamp} />
                          </td>
                        )}
                        {riskFindingsCols.updated && (
                          <td className={`${UI_TD} text-muted hidden lg:table-cell text-caption`}>
                            <When iso={risk.updatedAt} />
                          </td>
                        )}
                        <td className={`${UI_TD} text-right`} onClick={(e) => e.stopPropagation()}>
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
              // Persona defaults (15, 25) are not in the stock list; keep the select in sync with the real page size.
              pageSizeOptions={Array.from(new Set([10, 20, 50, 100, risksPageSize])).sort((a, b) => a - b)}
	              itemLabel={findingsListView === 'group' ? 'groups' : 'findings'}
	              className="order-2"
	            />
          )}
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
                              {risk.finalLevel ? (
                                <span className={`rounded border px-1.5 py-0.5 text-micro font-semibold uppercase ${getSeverityBadgeClass(risk.finalLevel)}`}>
                                  {risk.finalLevel}
                                </span>
                              ) : (
                                <span className="rounded border border-border px-1.5 py-0.5 text-micro font-semibold text-muted" title={`Not scored yet; rule severity ${risk.severity ?? 'unknown'}`}>
                                  no score
                                </span>
                              )}
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
            <p className="text-caption text-muted mb-4">Runtime events used as supporting telemetry for risk investigation. Filtered to the Findings time window.</p>
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
          onClose={closeRiskDrawer}
          onActionComplete={(action) => {
            const done = { acknowledge: 'acknowledged', resolve: 'resolved', dismiss: 'dismissed', reopen: 'reopened' }[action];
            setToast({ message: `Finding ${done}.`, variant: 'success' });
            // Decide, then move on: the next finding opens, or the panel closes at the end of the page.
            if (!stepRiskDrawer(1) && !stepRiskDrawer(-1)) closeRiskDrawer();
            fetchDataRef.current();
          }}
          onAssigneeChange={(name) => {
            setToast({ message: name ? `Assigned to ${name}.` : 'Finding unassigned.', variant: 'success' });
            fetchDataRef.current();
          }}
          onOpenPceTab={allowedRoutes.includes('/rules/exposure') ? () => navigate('/rules/exposure') : undefined}
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
                  This updates {selectedIds.size} selected finding(s) in the current Findings queue.
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
