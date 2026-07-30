import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { api } from '../lib/api';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';
import { Card } from '../design-system/components/Card';
import {
  Certificate,
  Agent,
  ErrorLog,
  SyncStatus,
  WorkerStatus,
  PipelineHealth,
  AuditLog,
  DashboardDataIntegrity,
} from '../types';
import {
  Lock,
  Radio,
  RefreshCw,
  Download,
  History,
  AlertCircle,
  FileText,
  AlertTriangle,
  Activity,
  Layers,
  Shield,
  Target,
  Zap,
  CheckCircle2,
  CircleAlert,
  Ban,
  HelpCircle,
} from 'lucide-react';
import { Button } from '../components/ui/Button';
import { FilterBar } from '../design-system/components/FilterBar';
import { UI_FILTER_SELECT } from '../lib/formChrome';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PageEmpty, PageLoading } from '../design-system/components/PageStatus';
import { formatDateTime } from '../lib/display';
import {
  UI_TABLE,
  UI_THEAD_STICKY,
  UI_TH_COMPACT,
  UI_TH,
  UI_TR,
  UI_TD_COMPACT_TIGHT,
  UI_TD,
} from '../lib/tableChrome';
import { PAGE_TITLES } from '../lib/pageTitles';
import { Pagination } from '../components/Pagination';
import { can, canAny, P } from '../lib/permissions';
import { usePermUser } from '../hooks/usePermUser';
import { usePersona } from '../hooks/usePersona';
import { MonitoringPersonaStrip } from '../components/MonitoringPersonaStrip';

type LayerVerdict = 'OK' | 'DEGRADED' | 'BLOCKED' | 'UNKNOWN';
type SystemVerdict = 'HEALTHY' | 'DEGRADED' | 'BROKEN';

const HEARTBEAT_WARN_SEC = 120;
const HEARTBEAT_BAD_SEC = 300;
const FRESHNESS_DEG_MIN = 15;
const FRESHNESS_BLOCK_MIN = 45;
const ERR_LEVEL_OPTS = ['', 'ERROR', 'WARN', 'INFO'] as const;
const ERR_SOURCE_OPTS = ['', 'core', 'agent', 'worker'] as const;

function errorLogLevelClass(l: string): string {
  switch (l?.toUpperCase()) {
    case 'ERROR':
      return 'text-red-400 bg-red-500/10';
    case 'WARN':
      return 'text-amber-400 bg-amber-500/10';
    case 'INFO':
      return 'text-sky-400 bg-sky-500/10';
    default:
      return 'text-muted bg-muted/10';
  }
}

function formatAgo(iso?: string | null): string {
  if (!iso) return '—';
  const t = new Date(iso).getTime();
  if (!Number.isFinite(t)) return '—';
  const sec = Math.floor((Date.now() - t) / 1000);
  if (sec < 0) return '—';
  if (sec < 60) return `${sec}s ago`;
  const m = Math.floor(sec / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  return `${h}h ${m % 60}m ago`;
}

function minutesSince(iso?: string | null): number | null {
  if (!iso) return null;
  const t = new Date(iso).getTime();
  if (!Number.isFinite(t)) return null;
  return (Date.now() - t) / 60000;
}

function stalenessLabel(minutes: number | null): 'LOW' | 'MED' | 'HIGH' | '—' {
  if (minutes == null || !Number.isFinite(minutes)) return '—';
  if (minutes < 10) return 'LOW';
  if (minutes < 45) return 'MED';
  return 'HIGH';
}

function parseTs(s: string | null): number | null {
  if (!s) return null;
  const t = new Date(s).getTime();
  return Number.isFinite(t) ? t : null;
}

function pipelineLastActivity(ph: PipelineHealth): number | null {
  const ts = [
    parseTs(ph.layer1.lastPceEval),
    parseTs(ph.layer1.lastRiskEngineEval),
    parseTs(ph.layer2.lastStateChange),
    parseTs(ph.layer3.lastPathComputation),
    parseTs(ph.layer4.lastScoreCalc),
  ].filter((x): x is number => x != null);
  if (ts.length === 0) return null;
  return Math.max(...ts);
}

function layerVerdict(
  layer: { freshnessMinutes?: number; status?: string },
  hasPipelineTimestamp: boolean,
): LayerVerdict {
  const st = (layer.status || '').toLowerCase();
  if (st === 'stale') return 'BLOCKED';
  if (st === 'degraded') return 'DEGRADED';
  if (st === 'healthy') return 'OK';
  if (st === 'unknown') return 'UNKNOWN';
  const fm = layer.freshnessMinutes;
  if (fm != null && Number.isFinite(fm)) {
    if (fm < 0) return 'UNKNOWN';
    if (fm >= FRESHNESS_BLOCK_MIN) return 'BLOCKED';
    if (fm >= FRESHNESS_DEG_MIN) return 'DEGRADED';
    return 'OK';
  }
  if (!hasPipelineTimestamp) return 'UNKNOWN';
  return 'OK';
}

function pipelineFreshnessLabel(minutes?: number): string {
  if (minutes == null || !Number.isFinite(minutes) || minutes < 0) {
    return 'No timestamp';
  }
  if (minutes < 1) return '<1m';
  return `${Math.round(minutes)}m`;
}

function shortDigest(value?: string): string {
  if (!value) return '—';
  return value.length > 22 ? `${value.slice(0, 18)}...` : value;
}

function verdictStyle(v: LayerVerdict): {
  icon: React.ReactNode;
  cls: string;
  label: string;
} {
  switch (v) {
    case 'OK':
      return {
        icon: <CheckCircle2 className="w-3.5 h-3.5" />,
        cls: 'text-emerald-400 bg-emerald-950/40 border-emerald-800/50',
        label: 'OK',
      };
    case 'DEGRADED':
      return {
        icon: <CircleAlert className="w-3.5 h-3.5" />,
        cls: 'text-amber-400 bg-amber-950/35 border-amber-800/50',
        label: 'DEGRADED',
      };
    case 'BLOCKED':
      return {
        icon: <Ban className="w-3.5 h-3.5" />,
        cls: 'text-red-400 bg-red-950/40 border-red-800/50',
        label: 'BLOCKED',
      };
    default:
      return {
        icon: <HelpCircle className="w-3.5 h-3.5" />,
        cls: 'text-slate-400 bg-slate-900/40 border-border',
        label: 'UNKNOWN',
      };
  }
}

function systemVerdictStyle(v: SystemVerdict): {
  dot: string;
  ring: string;
  text: string;
} {
  switch (v) {
    case 'HEALTHY':
      return {
        dot: 'bg-emerald-500',
        ring: 'ring-emerald-500/30',
        text: 'text-emerald-400',
      };
    case 'DEGRADED':
      return {
        dot: 'bg-amber-500',
        ring: 'ring-amber-500/30',
        text: 'text-amber-400',
      };
    default:
      return {
        dot: 'bg-red-500',
        ring: 'ring-red-500/30',
        text: 'text-red-400',
      };
  }
}

function catalogStatusStyle(status?: string): {
  icon: React.ReactNode;
  className: string;
  label: string;
} {
  switch ((status || '').toLowerCase()) {
    case 'healthy':
      return {
        icon: <CheckCircle2 className="h-3.5 w-3.5" />,
        className: 'border-emerald-500/35 bg-emerald-500/10 text-emerald-300',
        label: 'healthy',
      };
    case 'degraded':
      return {
        icon: <CircleAlert className="h-3.5 w-3.5" />,
        className: 'border-amber-500/35 bg-amber-500/10 text-amber-300',
        label: 'degraded',
      };
    case 'stale':
      return {
        icon: <AlertTriangle className="h-3.5 w-3.5" />,
        className: 'border-orange-500/35 bg-orange-500/10 text-orange-300',
        label: 'stale',
      };
    default:
      return {
        icon: <Ban className="h-3.5 w-3.5" />,
        className: 'border-red-500/35 bg-red-500/10 text-red-300',
        label: status || 'unavailable',
      };
  }
}

function coveragePct(done: number, total: number): string {
  if (!Number.isFinite(total) || total <= 0) return '—';
  return `${Math.round((100 * done) / total)}%`;
}

function failureRatePct(w: WorkerStatus): number {
  const denom = w.processed + w.failed;
  if (denom <= 0) return 0;
  return (100 * w.failed) / denom;
}

function workerStageStatusStyle(status: WorkerStatus['status']): {
  className: string;
  label: string;
} {
  switch (status) {
    case 'running':
      return {
        className: 'bg-emerald-900/40 text-emerald-400',
        label: 'running',
      };
    case 'degraded':
      return {
        className: 'bg-yellow-900/40 text-yellow-400',
        label: 'degraded',
      };
    case 'blocked':
      return {
        className: 'bg-red-950/45 text-red-300',
        label: 'blocked',
      };
    case 'catalog-unavailable':
      return {
        className: 'bg-red-950/45 text-red-300',
        label: 'catalog unavailable',
      };
    case 'idle':
      return {
        className: 'bg-sky-950/40 text-sky-300',
        label: 'idle',
      };
    case 'not-configured':
      return {
        className: 'bg-surface-2 text-muted',
        label: 'not configured',
      };
    default:
      return {
        className: 'bg-surface-2 text-muted',
        label: status,
      };
  }
}

type LogFilter = 'ALL' | 'ERROR' | 'WARN' | 'INFO';

export const Monitoring: React.FC = () => {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const permUser = usePermUser();
  const canPlatformAudit = can(permUser, P.systemAuditRead);
  const canObservabilityMetrics = can(permUser, P.observabilityMetricsRead);
  const canObservabilityAgents = can(permUser, P.observabilityAgentsRead);
  const canObservabilityLogs = can(permUser, P.observabilityLogsRead);
  const canInventoryRead = can(permUser, P.inventoryRead);
  const canObservabilityShell = canAny(permUser, [
    P.observabilityMetricsRead,
    P.observabilityAgentsRead,
    P.observabilityLogsRead,
  ]);
  const { id: personaId } = usePersona();
  const [certs, setCerts] = useState<Certificate[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [errorLogs, setErrorLogs] = useState<ErrorLog[]>([]);
  const [syncStatus, setSyncStatus] = useState<SyncStatus | null>(null);
  const [workerStatus, setWorkerStatus] = useState<WorkerStatus[]>([]);
  const [pipelineHealth, setPipelineHealth] = useState<PipelineHealth | null>(
    null,
  );
  const [dataIntegrity, setDataIntegrity] =
    useState<DashboardDataIntegrity | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [partialErrors, setPartialErrors] = useState<string[]>([]);
  const [logFilter, setLogFilter] = useState<LogFilter>('ALL');

  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [auditTotal, setAuditTotal] = useState(0);
  const [auditPage, setAuditPage] = useState(1);
  const [auditPageSize, setAuditPageSize] = useState(20);
  const [auditLoading, setAuditLoading] = useState(false);

  const [errLogs, setErrLogs] = useState<ErrorLog[]>([]);
  const [errTotal, setErrTotal] = useState(0);
  const [errPage, setErrPage] = useState(1);
  const [errPageSize, setErrPageSize] = useState(20);
  const [errLevel, setErrLevel] = useState('');
  const [errSource, setErrSource] = useState('');
  const [errLoading, setErrLoading] = useState(false);

  const fetchData = useCallback(async () => {
    setError(null);
    setPartialErrors([]);
    try {
      const tasks: Array<Promise<string | null>> = [];
      const run = (label: string, task: Promise<void>) =>
        task
          .then(() => null)
          .catch((e) => {
            const message = e instanceof Error ? e.message : 'request failed';
            return `${label}: ${message}`;
          });

      if (canInventoryRead) {
        tasks.push(run('Certificates', api.getCertificates().then(setCerts)));
      } else {
        setCerts([]);
      }
      if (canObservabilityAgents) {
        tasks.push(run('Agents', api.getAgents().then(setAgents)));
      } else {
        setAgents([]);
      }
      if (canObservabilityMetrics) {
        tasks.push(run('Sync status', api.getSyncStatus().then(setSyncStatus)));
        tasks.push(
          run('Worker status', api.getWorkerStatus().then(setWorkerStatus)),
        );
        tasks.push(
          run(
            'Catalog health',
            api.getDashboardDataIntegrity().then(setDataIntegrity),
          ),
        );
        tasks.push(
          run(
            'Pipeline health',
            api.getPipelineHealth().then(setPipelineHealth),
          ),
        );
      } else {
        setSyncStatus(null);
        setWorkerStatus([]);
        setPipelineHealth(null);
        setDataIntegrity(null);
      }
      if (canObservabilityLogs) {
        tasks.push(
          run(
            'Recent error logs',
            api
              .getErrorLogs({ page: 1, pageSize: 10 })
              .then((logData) => setErrorLogs(logData.logs)),
          ),
        );
      } else {
        setErrorLogs([]);
      }

      const failures = (await Promise.all(tasks)).filter((x): x is string =>
        Boolean(x),
      );
      setPartialErrors(failures);
    } catch (e) {
      setError(
        e instanceof Error ? e.message : 'Failed to load monitoring data',
      );
    } finally {
      setLoading(false);
    }
  }, [
    canInventoryRead,
    canObservabilityAgents,
    canObservabilityLogs,
    canObservabilityMetrics,
  ]);

  const intervalMs = useRefreshIntervalStore((s) =>
    s.getIntervalMs(REFRESH_INTERVALS.METRICS),
  );
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);
  usePolling(fetchData, intervalMs, { refreshTrigger });
  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const fetchAuditPage = useCallback(async () => {
    if (!canPlatformAudit) {
      setAuditLogs([]);
      setAuditTotal(0);
      setAuditLoading(false);
      return;
    }
    setAuditLoading(true);
    try {
      const data = await api.getAuditLogs({
        page: auditPage,
        pageSize: auditPageSize,
      });
      setAuditLogs(data.logs);
      setAuditTotal(Number.isFinite(data.total) ? data.total : 0);
    } catch {
      setAuditLogs([]);
      setAuditTotal(0);
    } finally {
      setAuditLoading(false);
    }
  }, [canPlatformAudit, auditPage, auditPageSize]);

  useEffect(() => {
    void fetchAuditPage();
  }, [fetchAuditPage]);

  const fetchErrorLogsPage = useCallback(async () => {
    if (!canObservabilityLogs) {
      setErrLogs([]);
      setErrTotal(0);
      setErrLoading(false);
      return;
    }
    setErrLoading(true);
    try {
      const data = await api.getErrorLogs({
        page: errPage,
        pageSize: errPageSize,
        ...(errLevel ? { level: errLevel } : {}),
        ...(errSource ? { source: errSource } : {}),
      });
      setErrLogs(data.logs);
      setErrTotal(Number.isFinite(data.total) ? data.total : 0);
    } catch {
      setErrLogs([]);
      setErrTotal(0);
    } finally {
      setErrLoading(false);
    }
  }, [canObservabilityLogs, errPage, errPageSize, errLevel, errSource]);

  useEffect(() => {
    void fetchErrorLogsPage();
  }, [fetchErrorLogsPage]);

  const refreshMonitoringAll = useCallback(async () => {
    await fetchData();
    await Promise.allSettled([fetchAuditPage(), fetchErrorLogsPage()]);
  }, [fetchData, fetchAuditPage, fetchErrorLogsPage]);

  const sectionParam = searchParams.get('section');
  useEffect(() => {
    if (sectionParam !== 'audit' && sectionParam !== 'error-logs') return;
    if (sectionParam === 'audit' && !canPlatformAudit) {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          next.delete('section');
          return next;
        },
        { replace: true },
      );
      return;
    }
    if (sectionParam === 'error-logs' && !canObservabilityLogs) {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          next.delete('section');
          return next;
        },
        { replace: true },
      );
      return;
    }
    const id =
      sectionParam === 'audit'
        ? 'platform-audit-logs'
        : 'operational-error-logs';
    const raf = requestAnimationFrame(() => {
      document
        .getElementById(id)
        ?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    });
    const t = window.setTimeout(() => {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          next.delete('section');
          return next;
        },
        { replace: true },
      );
    }, 150);
    return () => {
      cancelAnimationFrame(raf);
      window.clearTimeout(t);
    };
  }, [sectionParam, canPlatformAudit, canObservabilityLogs, setSearchParams]);

  const lastHeartbeatIso = useMemo(() => {
    if (agents.length === 0) return null;
    const timestamps = agents
      .map((a) => new Date(a.lastHeartbeat).getTime())
      .filter((v) => Number.isFinite(v));
    if (timestamps.length === 0) return null;
    return new Date(Math.max(...timestamps)).toISOString();
  }, [agents]);

  const pipelineActivityTs = useMemo(
    () => (pipelineHealth ? pipelineLastActivity(pipelineHealth) : null),
    [pipelineHealth],
  );

  const hasPipelineTs = pipelineActivityTs != null;

  const layerVerdicts = useMemo(() => {
    if (!pipelineHealth) {
      return {
        l1: 'UNKNOWN' as LayerVerdict,
        l2: 'UNKNOWN' as LayerVerdict,
        l3: 'UNKNOWN' as LayerVerdict,
        l4: 'UNKNOWN' as LayerVerdict,
      };
    }
    const ph = pipelineHealth;
    return {
      l1: layerVerdict(ph.layer1, hasPipelineTs),
      l2: layerVerdict(ph.layer2, hasPipelineTs),
      l3: layerVerdict(ph.layer3, hasPipelineTs),
      l4: layerVerdict(ph.layer4, hasPipelineTs),
    };
  }, [pipelineHealth, hasPipelineTs]);

  const totalQueueDepth = useMemo(
    () => workerStatus.reduce((s, w) => s + w.queueDepth, 0),
    [workerStatus],
  );
  const totalActiveWorkers = useMemo(
    () => workerStatus.reduce((s, w) => s + w.activeWorkers, 0),
    [workerStatus],
  );
  const hasLiveWorkerTelemetry = useMemo(
    () =>
      workerStatus.some(
        (w) =>
          w.live === true ||
          w.source === 'prometheus' ||
          w.queueDepth > 0 ||
          w.activeWorkers > 0,
      ),
    [workerStatus],
  );

  const workerRows = useMemo(() => {
    return workerStatus.map((w) => ({ w, failPct: failureRatePct(w) }));
  }, [workerStatus]);
  const workerAttentionCount = useMemo(
    () =>
      workerStatus.filter(
        (w) =>
          w.status === 'degraded' ||
          w.status === 'blocked' ||
          w.status === 'catalog-unavailable' ||
          (w.failed > 0 && failureRatePct(w) >= 5),
      ).length,
    [workerStatus],
  );

  const alerts = useMemo(() => {
    const lines: string[] = [];
    for (const a of agents) {
      const sec = (Date.now() - new Date(a.lastHeartbeat).getTime()) / 1000;
      if (Number.isFinite(sec) && sec >= HEARTBEAT_BAD_SEC) {
        lines.push(
          `Agent ${a.node} heartbeat delayed (${formatAgo(a.lastHeartbeat)})`,
        );
      }
    }
    const badWorkers = workerStatus.filter(
      (w) =>
        w.status === 'degraded' ||
        w.status === 'blocked' ||
        w.status === 'catalog-unavailable' ||
        (w.failed > 0 && failureRatePct(w) >= 5),
    );
    for (const w of badWorkers.slice(0, 3)) {
      if (w.status === 'catalog-unavailable')
        lines.push(`Pipeline stage ${w.name} is blocked by unavailable catalog data`);
      else if (w.status === 'blocked')
        lines.push(`Pipeline stage ${w.name} is blocked`);
      else if (w.status === 'degraded')
        lines.push(`Pipeline stage ${w.name} is degraded`);
      else
        lines.push(
          `Pipeline stage ${w.name} failure rate ${failureRatePct(w).toFixed(1)}%`,
        );
    }
    return lines;
  }, [agents, workerStatus]);

  const systemVerdict: SystemVerdict = useMemo(() => {
    if (error) return 'BROKEN';
    if (!loading && agents.length === 0) return 'BROKEN';
    if (!pipelineHealth) return 'DEGRADED';
    const lv = layerVerdicts;
    if (
      lv.l1 === 'BLOCKED' ||
      lv.l2 === 'BLOCKED' ||
      lv.l3 === 'BLOCKED' ||
      lv.l4 === 'BLOCKED'
    )
      return 'BROKEN';
    if (!hasPipelineTs && Object.values(lv).every((x) => x === 'UNKNOWN'))
      return 'BROKEN';

    if (
      lv.l1 === 'DEGRADED' ||
      lv.l2 === 'DEGRADED' ||
      lv.l3 === 'DEGRADED' ||
      lv.l4 === 'DEGRADED' ||
      lv.l1 === 'UNKNOWN' ||
      lv.l2 === 'UNKNOWN' ||
      lv.l3 === 'UNKNOWN' ||
      lv.l4 === 'UNKNOWN'
    ) {
      return 'DEGRADED';
    }

    if (lastHeartbeatIso) {
      const hbSec = (Date.now() - new Date(lastHeartbeatIso).getTime()) / 1000;
      if (hbSec >= HEARTBEAT_BAD_SEC) return 'DEGRADED';
    }

    if (alerts.length > 0) return 'DEGRADED';

    if (lastHeartbeatIso) {
      const hbSec = (Date.now() - new Date(lastHeartbeatIso).getTime()) / 1000;
      if (hbSec >= HEARTBEAT_WARN_SEC) return 'DEGRADED';
    }

    return 'HEALTHY';
  }, [
    error,
    loading,
    agents.length,
    pipelineHealth,
    layerVerdicts,
    hasPipelineTs,
    lastHeartbeatIso,
    alerts.length,
  ]);

  const activityLagMs =
    pipelineActivityTs != null ? Date.now() - pipelineActivityTs : null;

  const syncMinutes = minutesSince(syncStatus?.lastScan ?? null);
  const syncStaleness = stalenessLabel(syncMinutes);
  const catalogHealth = dataIntegrity?.catalogHealth ?? null;
  const runtimeHealth = dataIntegrity?.runtimeHealth ?? null;
  const catalogStatus = catalogStatusStyle(catalogHealth?.status);
  const catalogUsesGeneration = Boolean(
    catalogHealth?.activeCatalogGenerationId,
  );
  const catalogMatchedActive = catalogHealth
    ? catalogUsesGeneration
      ? catalogHealth.activeSbomsMatchedGeneration
      : catalogHealth.activeSbomsMatchedMirror
    : 0;
  const catalogMissingActive = catalogHealth
    ? catalogUsesGeneration
      ? catalogHealth.activeSbomsMissingGenerationMatch
      : catalogHealth.activeSbomsMissingMirrorMatch
    : 0;
  const catalogCoverage = catalogHealth
    ? coveragePct(catalogMatchedActive, catalogHealth.activeSboms)
    : '—';
  const catalogReferenceBlocked = catalogHealth
    ? catalogHealth.cvesCount === 0 ||
      catalogHealth.packageVulnerabilitiesCount === 0
    : false;
  const catalogOsvMissing = catalogHealth
    ? catalogHealth.osvPackagesCount === 0
    : false;
  const catalogGenerationMissing = catalogHealth
    ? !catalogHealth.activeCatalogGenerationId
    : false;
  const catalogModeLabel = catalogUsesGeneration ? 'generation' : 'mirror';
  const catalogMatcherLabel = catalogReferenceBlocked
    ? 'Blocked'
    : catalogCoverage;
  const catalogMatcherClass = catalogReferenceBlocked
    ? 'text-red-300'
    : catalogMissingActive > 0
      ? 'text-amber-300'
      : 'text-text';
  const catalogPrimaryIssue = catalogReferenceBlocked
    ? 'CVE matching blocked: reference tables are empty.'
    : catalogMissingActive > 0
      ? 'Some inventory pod SBOMs need to be matched again.'
      : 'SBOM matching is current for inventory pods.';
  const catalogActionAlerts =
    dataIntegrity?.alerts?.filter((alert) =>
      /^(cve_reference_empty|osv_mirror_empty|catalog_unavailable|catalog_generation_missing|catalog_generation_match_missing|malware_catalog_empty|malware_generation_missing|malware_feed_sync_failed):/.test(
        alert,
      ),
    ) ?? [];

  const filteredLogs = useMemo(() => {
    if (logFilter === 'ALL') return errorLogs;
    return errorLogs.filter((l) => (l.level || '').toUpperCase() === logFilter);
  }, [errorLogs, logFilter]);

  const exportPayload = () => {
    const data = {
      exportedAt: new Date().toISOString(),
      agents,
      certificates: certs,
      syncStatus,
      workerStatus,
      pipelineHealth,
      dataIntegrity,
      recentErrorLogs: errorLogs,
    };
    const blob = new Blob([JSON.stringify(data, null, 2)], {
      type: 'application/json',
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `monitoring-export-${new Date().toISOString().replace(/[:.]/g, '-')}.json`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  };

  const sv = systemVerdictStyle(systemVerdict);

  if (loading)
    return (
      <PageLoading
        message="Loading operations monitoring..."
        className="min-h-[40dvh]"
      />
    );

  const pipelineFlow = pipelineHealth ? (
    <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
      {(
        [
          {
            key: 'L1',
            title: 'Layer 1',
            sub: 'Discovery',
            layer: pipelineHealth.layer1,
            v: layerVerdicts.l1,
            icon: Shield,
            timestampLabel: 'Last PCE',
            timestamp: pipelineHealth.layer1.lastPceEval,
            metricLabel: 'Active insights',
            metricValue: pipelineHealth.layer1.insightCount.toLocaleString(),
          },
          {
            key: 'L2',
            title: 'Layer 2',
            sub: 'Runtime',
            layer: pipelineHealth.layer2,
            v: layerVerdicts.l2,
            icon: Zap,
            timestampLabel: 'Last promotion',
            timestamp: pipelineHealth.layer2.lastStateChange,
            metricLabel: 'Rules / exploited',
            metricValue: `${pipelineHealth.layer2.activePromotionRules}/${pipelineHealth.layer2.exploitedCapCount}`,
          },
          {
            key: 'L3',
            title: 'Layer 3',
            sub: 'Attack paths',
            layer: pipelineHealth.layer3,
            v: layerVerdicts.l3,
            icon: Target,
            timestampLabel: 'Last build',
            timestamp: pipelineHealth.layer3.lastPathComputation,
            metricLabel: 'Paths / critical',
            metricValue: `${pipelineHealth.layer3.totalPaths.toLocaleString()}/${pipelineHealth.layer3.criticalPaths.toLocaleString()}`,
          },
          {
            key: 'L4',
            title: 'Layer 4',
            sub: 'Unified score',
            layer: pipelineHealth.layer4,
            v: layerVerdicts.l4,
            icon: Activity,
            timestampLabel: 'Last score',
            timestamp: pipelineHealth.layer4.lastScoreCalc,
            metricLabel: 'Scored / V3',
            metricValue: `${pipelineHealth.layer4.resourcesScored.toLocaleString()}/${pipelineHealth.layer4.v3Resources.toLocaleString()}`,
          },
        ] as const
      ).map((item) => {
        const vs = verdictStyle(item.v);
        const Icon = item.icon;
        return (
          <div
            key={item.key}
            className="rounded-lg border border-border/75 bg-base/35 p-3"
          >
            <div className="flex items-start justify-between gap-3">
              <div className="flex min-w-0 items-center gap-2">
                <Icon className="h-4 w-4 shrink-0 text-muted-2" />
                <div className="min-w-0">
                  <div className="text-caption font-semibold text-text">
                    {item.title}
                  </div>
                  <div className="text-micro text-muted">{item.sub}</div>
                </div>
              </div>
              <span
                className={`inline-flex shrink-0 items-center gap-1 rounded border px-2 py-0.5 text-micro font-semibold ${vs.cls}`}
              >
                {vs.icon}
                {vs.label}
              </span>
            </div>
            <div className="mt-3 grid grid-cols-2 gap-2 text-caption">
              <div className="min-w-0">
                <div className="text-micro text-muted">{item.timestampLabel}</div>
                <div className="truncate font-medium text-text">
                  {item.timestamp ? formatDateTime(item.timestamp) : 'No event'}
                </div>
              </div>
              <div className="min-w-0 text-right">
                <div className="text-micro text-muted">{item.metricLabel}</div>
                <div className="font-mono font-semibold text-text">
                  {item.metricValue}
                </div>
              </div>
            </div>
            <div className="mt-2 border-t border-border/60 pt-2 text-micro text-muted">
              Freshness: {pipelineFreshnessLabel(item.layer.freshnessMinutes)}
            </div>
          </div>
        );
      })}
    </div>
  ) : (
    <p className="text-caption text-muted">Pipeline telemetry unavailable.</p>
  );

  return (
    <PageLayout
      title={PAGE_TITLES.monitoring}
      description="Agent health, synchronization status, certificates, operational errors, and platform audit trail."
      actions={
        <div className="flex items-center gap-2 flex-wrap">
          <Button
            variant="secondary"
            onClick={() => void refreshMonitoringAll()}
          >
            <RefreshCw className="w-4 h-4 mr-2" /> Refresh
          </Button>
          <Button variant="secondary" onClick={exportPayload}>
            <Download className="w-4 h-4 mr-2" /> Export JSON
          </Button>
        </div>
      }
      toolbar={
        <FilterBar
          embedded
          trailing={
            <>
              {canPlatformAudit ? (
                <Button
                  variant="secondary"
                  size="sm"
                  className="text-body px-3 py-2"
                  type="button"
                  onClick={() => {
                    document
                      .getElementById('platform-audit-logs')
                      ?.scrollIntoView({ behavior: 'smooth', block: 'start' });
                  }}
                >
                  <History className="mr-1.5 h-4 w-4 shrink-0" /> Audit logs
                </Button>
              ) : null}
              {canObservabilityShell ? (
                <Button
                  variant="secondary"
                  size="sm"
                  className="text-body px-3 py-2"
                  type="button"
                  onClick={() => {
                    document
                      .getElementById('operational-error-logs')
                      ?.scrollIntoView({ behavior: 'smooth', block: 'start' });
                  }}
                >
                  <AlertCircle className="mr-1.5 h-4 w-4 shrink-0" /> Error logs
                </Button>
              ) : null}
              <Button
                variant="secondary"
                size="sm"
                className="text-body px-3 py-2"
                onClick={() => navigate('/certificates')}
              >
                <Lock className="mr-1.5 h-4 w-4 shrink-0" /> Certificates
              </Button>
              {canPlatformAudit ? (
                <Button
                  variant="secondary"
                  size="sm"
                  className="text-body px-3 py-2"
                  onClick={() => navigate('/reports')}
                >
                  <FileText className="mr-1.5 h-4 w-4 shrink-0" /> Reports
                </Button>
              ) : null}
            </>
          }
        />
      }
    >
      {personaId === 'admin' ? <MonitoringPersonaStrip /> : null}
      {error && (
        <div className="mb-4 flex items-center gap-3 p-3 rounded-lg border border-red-800 bg-red-950/40 text-red-300">
          <AlertTriangle className="w-4 h-4 shrink-0 text-red-400" />
          <span className="text-body flex-1">{error}</span>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => void refreshMonitoringAll()}
          >
            <RefreshCw className="w-3 h-3 mr-1.5" /> Retry
          </Button>
        </div>
      )}
      {!error && partialErrors.length > 0 ? (
        <div
          className="mb-4 rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-amber-200"
          role="status"
        >
          <div className="flex items-start gap-3">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-300" />
            <div className="min-w-0 flex-1">
              <p className="text-body font-medium text-amber-100">
                Some monitoring sections did not load
              </p>
              <p className="mt-1 text-caption text-amber-200/90">
                {partialErrors.join('; ')}
              </p>
            </div>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => void refreshMonitoringAll()}
            >
              <RefreshCw className="mr-1.5 h-3 w-3" /> Retry
            </Button>
          </div>
        </div>
      ) : null}

      {/* System health verdict */}
      <Card variant="panel" className="mb-4 border-border/80">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-start gap-3">
            <span
              className={`mt-1 inline-flex h-3 w-3 rounded-full ${sv.dot} ring-2 ${sv.ring}`}
              aria-hidden
            />
            <div>
              <div
                className={`text-body font-bold uppercase tracking-wide ${sv.text}`}
              >
                System health: {systemVerdict}
              </div>
              <div className="text-caption text-muted mt-1 space-y-0.5">
                <div>
                  <span className="text-text font-medium">Agents:</span>{' '}
                  {agents.length}
                  <span className="mx-2 text-border">|</span>
                  <span className="text-text font-medium">
                    Last heartbeat:
                  </span>{' '}
                  {lastHeartbeatIso ? formatAgo(lastHeartbeatIso) : '—'}
                  <span className="mx-2 text-border">|</span>
                  <span className="text-text font-medium">
                    Pipeline activity lag:
                  </span>{' '}
                  {activityLagMs != null && activityLagMs < 86_400_000
                    ? activityLagMs >= 60_000
                      ? `${Math.round(activityLagMs / 60000)}m`
                      : `${Math.round(activityLagMs / 1000)}s`
                    : '—'}
                </div>
                <div>
                  <span className="text-text font-medium">Data freshness:</span>{' '}
                  {syncStatus?.lastScan
                    ? `OK (last sync ${formatAgo(syncStatus.lastScan)}) · Staleness ${syncStaleness}`
                    : '—'}
                  <span className="mx-2 text-border">|</span>
                  <span className="text-text font-medium">
                    Cluster drift flag:
                  </span>{' '}
                  {syncStatus ? (syncStatus.drift ? 'yes' : 'none') : '—'}
                </div>
                {layerVerdicts.l4 === 'DEGRADED' ||
                layerVerdicts.l4 === 'BLOCKED' ? (
                  <div className="text-amber-400/90">
                    Risk Engine correlation: unified scores may be stale — check
                    Layer 4 timestamps.
                  </div>
                ) : null}
              </div>
            </div>
          </div>
        </div>
      </Card>

      {/* Alerts */}
      <Card title="Alerts / anomalies" variant="panel" className="mb-4">
        {alerts.length === 0 ? (
          <p className="text-caption text-muted">
            No active anomalies from heartbeat or pipeline failure heuristics.
          </p>
        ) : (
          <ul className="text-body text-amber-200/90 space-y-1.5 list-disc pl-5">
            {alerts.map((a) => (
              <li key={a}>{a}</li>
            ))}
          </ul>
        )}
      </Card>

      {canObservabilityMetrics ? (
        <Card
          title="Catalog detail"
          description="CVE reference availability, current inventory SBOM matching, and malware feed state."
          actions={
            catalogHealth ? (
              <span
                className={`inline-flex items-center gap-1 rounded border px-2 py-0.5 text-caption font-medium ${catalogStatus.className}`}
              >
                {catalogStatus.icon}
                {catalogStatus.label}
              </span>
            ) : null
          }
          className="mb-4"
        >
          {!catalogHealth ? (
            <PageEmpty
              title="Catalog health unavailable"
              description="The dashboard data integrity endpoint did not return catalog status."
              className="py-5"
            />
          ) : (
            <div className="space-y-3">
              <div
                className={`rounded-lg border px-3 py-2 ${
                  catalogReferenceBlocked
                    ? 'border-red-500/35 bg-red-500/10'
                    : catalogMissingActive > 0 || catalogOsvMissing || catalogGenerationMissing
                      ? 'border-amber-500/35 bg-amber-500/10'
                      : 'border-emerald-500/30 bg-emerald-500/10'
                }`}
              >
                <div className="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
                  <div className="min-w-0">
                    <div
                      className={`text-body font-semibold ${
                        catalogReferenceBlocked
                          ? 'text-red-200'
                          : catalogMissingActive > 0 || catalogOsvMissing || catalogGenerationMissing
                            ? 'text-amber-100'
                            : 'text-emerald-100'
                      }`}
                    >
                      {catalogPrimaryIssue}
                    </div>
                    <p className="mt-1 text-caption text-muted">
                      {catalogReferenceBlocked
                        ? 'SBOM match runs can complete, but vulnerability output is not trustworthy until CVE and package vulnerability rows are loaded.'
                        : `Matcher coverage is measured against ${catalogModeLabel} evidence for SBOMs attached to current inventory pods.`}
                    </p>
                  </div>
                  <div className="grid shrink-0 grid-cols-3 gap-2 text-right text-caption md:min-w-[21rem]">
                    <div>
                      <div className="text-micro uppercase tracking-wide text-muted">
                        CVE rows
                      </div>
                      <div className="font-mono font-semibold text-text">
                        {catalogHealth.cvesCount.toLocaleString()}
                      </div>
                    </div>
                    <div>
                      <div className="text-micro uppercase tracking-wide text-muted">
                        SBOMs
                      </div>
                      <div className="font-mono font-semibold text-text">
                        {catalogHealth.activeSboms.toLocaleString()}
                      </div>
                    </div>
                    <div>
                      <div className="text-micro uppercase tracking-wide text-muted">
                        Match
                      </div>
                      <div className={`font-mono font-semibold ${catalogMatcherClass}`}>
                        {catalogMatcherLabel}
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-5">
                <div className="rounded-md border border-border/70 bg-base/30 px-3 py-2">
                  <div className="text-micro uppercase tracking-wide text-muted">
                    CVE reference
                  </div>
                  <div
                    className={`mt-1 text-body font-semibold tabular-nums ${
                      catalogReferenceBlocked ? 'text-red-300' : 'text-text'
                    }`}
                  >
                    {catalogReferenceBlocked ? 'blocked' : 'ready'}
                  </div>
                  <div className="mt-0.5 text-micro text-muted">
                    {catalogHealth.cvesCount.toLocaleString()} CVEs ·{' '}
                    {catalogHealth.packageVulnerabilitiesCount.toLocaleString()}{' '}
                    package vulns
                  </div>
                </div>
                <div className="rounded-md border border-border/70 bg-base/30 px-3 py-2">
                  <div className="text-micro uppercase tracking-wide text-muted">
                    OSV mirror
                  </div>
                  <div
                    className={`mt-1 text-body font-semibold tabular-nums ${
                      catalogOsvMissing ? 'text-amber-300' : 'text-text'
                    }`}
                  >
                    {catalogOsvMissing
                      ? 'empty'
                      : catalogHealth.osvPackagesCount.toLocaleString()}
                  </div>
                  <div className="mt-0.5 text-micro text-muted">
                    mirror v{catalogHealth.mirrorVersion || '—'}
                  </div>
                </div>
                <div className="rounded-md border border-border/70 bg-base/30 px-3 py-2">
                  <div className="text-micro uppercase tracking-wide text-muted">
                    Generation
                  </div>
                  <div
                    className={`mt-1 text-body font-semibold ${
                      !catalogGenerationMissing
                        ? 'text-text'
                        : 'text-amber-300'
                    }`}
                  >
                    {!catalogGenerationMissing
                      ? `#${catalogHealth.activeCatalogGenerationId}`
                      : 'missing'}
                  </div>
                  <div className="mt-0.5 text-micro text-muted">
                    {catalogHealth.activeCatalogActivatedAt
                      ? formatAgo(catalogHealth.activeCatalogActivatedAt)
                      : catalogHealth.activeCatalogGenerationStatus || 'no active generation'}
                  </div>
                </div>
                <div className="rounded-md border border-border/70 bg-base/30 px-3 py-2">
                  <div className="text-micro uppercase tracking-wide text-muted">
                    SBOM match
                  </div>
                  <div
                    className={`mt-1 text-body font-semibold tabular-nums ${catalogMatcherClass}`}
                  >
                    {catalogMatcherLabel}
                  </div>
                  <div className="mt-0.5 text-micro text-muted">
                    {catalogMatchedActive.toLocaleString()}/
                    {catalogHealth.activeSboms.toLocaleString()} inventory pod SBOMs
                  </div>
                </div>
                <div className="rounded-md border border-border/70 bg-base/30 px-3 py-2">
                  <div className="text-micro uppercase tracking-wide text-muted">
                    Malware feed
                  </div>
                  <div
                    className={`mt-1 text-body font-semibold ${
                      catalogHealth.lastMalwareFeedSyncStatus === 'failed'
                        ? 'text-red-300'
                        : catalogHealth.lastMalwareFeedSyncStatus
                          ? 'text-text'
                          : 'text-amber-300'
                    }`}
                  >
                    {catalogHealth.lastMalwareFeedSyncStatus || 'unknown'}
                  </div>
                  <div className="mt-0.5 text-micro text-muted">
                    {catalogHealth.lastMalwareFeedSyncAt
                      ? formatAgo(catalogHealth.lastMalwareFeedSyncAt)
                      : `${catalogHealth.malwarePackagesCount.toLocaleString()} rows`}
                  </div>
                  <div className="mt-1 text-micro text-muted">
                    generation{' '}
                    {catalogHealth.activeMalwareGenerationId
                      ? `#${catalogHealth.activeMalwareGenerationId}`
                      : 'missing'}
                  </div>
                </div>
              </div>

              <div className="grid gap-2 text-caption text-muted lg:grid-cols-4">
                <div>
                  <span className="text-text font-medium">SBOM lifecycle:</span>{' '}
                  {catalogHealth.activeSboms.toLocaleString()} on current inventory pods,{' '}
                  {catalogHealth.staleSboms.toLocaleString()} stale.
                </div>
                <div>
                  <span className="text-text font-medium">
                    {catalogUsesGeneration ? 'Generation runs:' : 'Mirror runs:'}
                  </span>{' '}
                  {(catalogUsesGeneration
                    ? catalogHealth.currentGenerationSucceededRuns
                    : catalogHealth.currentMirrorSucceededRuns
                  ).toLocaleString()}{' '}
                  ok,{' '}
                  {(catalogUsesGeneration
                    ? catalogHealth.currentGenerationFailedRuns
                    : catalogHealth.currentMirrorFailedRuns
                  ).toLocaleString()}{' '}
                  failed.
                </div>
                <div>
                  <span className="text-text font-medium">CVE matches:</span>{' '}
                  {catalogHealth.activePodCveMatches.toLocaleString()} current,{' '}
                  {catalogHealth.stalePodCveMatches.toLocaleString()} stale.
                </div>
                <div>
                  <span className="text-text font-medium">Digests:</span>{' '}
                  CVE{' '}
                  <span className="font-mono">
                    {shortDigest(catalogHealth.activeCatalogSourceDigest)}
                  </span>{' '}
                  · malware{' '}
                  <span className="font-mono">
                    {shortDigest(catalogHealth.activeMalwareSourceDigest)}
                  </span>
                </div>
              </div>
              {catalogActionAlerts.length ? (
                <div className="rounded-lg border border-amber-500/35 bg-amber-500/10 px-3 py-2 text-caption text-amber-100">
                  <div className="mb-1 font-medium">Action required</div>
                  <ul className="list-disc space-y-1 pl-5">
                    {catalogActionAlerts.slice(0, 4).map((alert) => (
                      <li key={alert}>{alert}</li>
                    ))}
                  </ul>
                </div>
              ) : null}
            </div>
          )}
        </Card>
      ) : null}

      {canObservabilityMetrics ? (
        <Card
          title="Runtime monitor"
          actions={
            runtimeHealth ? (
              <span
                className={`inline-flex items-center gap-1 rounded border px-2 py-0.5 text-caption font-medium ${catalogStatusStyle(runtimeHealth.status).className}`}
              >
                {catalogStatusStyle(runtimeHealth.status).icon}
                {runtimeHealth.status}
              </span>
            ) : null
          }
          className="mb-4"
        >
          {!runtimeHealth ? (
            <PageEmpty
              title="Runtime monitor unavailable"
              description="The integrity endpoint did not return runtime monitor status."
              className="py-5"
            />
          ) : (
            <div className="space-y-3">
              <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
                <div className="rounded-md border border-border/70 bg-base/30 px-3 py-2">
                  <div className="text-micro uppercase tracking-wide text-muted">
                    Runtime events
                  </div>
                  <div className="mt-1 text-body font-semibold text-text tabular-nums">
                    {runtimeHealth.runtimeEventsCount.toLocaleString()}
                  </div>
                  <div className="mt-0.5 text-micro text-muted">
                    {runtimeHealth.lastRuntimeEventAt
                      ? formatAgo(runtimeHealth.lastRuntimeEventAt)
                      : 'no event ingest'}
                  </div>
                </div>
                <div className="rounded-md border border-border/70 bg-base/30 px-3 py-2">
                  <div className="text-micro uppercase tracking-wide text-muted">
                    Signals
                  </div>
                  <div className="mt-1 text-body font-semibold text-text tabular-nums">
                    {runtimeHealth.runtimeSignalsCount.toLocaleString()}
                  </div>
                  <div className="mt-0.5 text-micro text-muted">
                    {runtimeHealth.lastRuntimeSignalAt
                      ? formatAgo(runtimeHealth.lastRuntimeSignalAt)
                      : 'no semantic signal'}
                  </div>
                </div>
                <div className="rounded-md border border-border/70 bg-base/30 px-3 py-2">
                  <div className="text-micro uppercase tracking-wide text-muted">
                    Pod metrics
                  </div>
                  <div className="mt-1 text-body font-semibold text-text tabular-nums">
                    {runtimeHealth.runtimeMetricsCount.toLocaleString()}
                  </div>
                  <div className="mt-0.5 text-micro text-muted">
                    {runtimeHealth.lastRuntimeMetricAt
                      ? formatAgo(runtimeHealth.lastRuntimeMetricAt)
                      : 'no runtime metric'}
                  </div>
                </div>
                <div className="rounded-md border border-border/70 bg-base/30 px-3 py-2">
                  <div className="text-micro uppercase tracking-wide text-muted">
                    Falco
                  </div>
                  <div
                    className={`mt-1 text-body font-semibold ${
                      runtimeHealth.falcoStatus === 'active'
                        ? 'text-text'
                        : 'text-amber-300'
                    }`}
                  >
                    {runtimeHealth.falcoStatus}
                  </div>
                  <div className="mt-0.5 text-micro text-muted">
                    {runtimeHealth.falcoEventsCount.toLocaleString()} events
                    {runtimeHealth.lastFalcoEventAt
                      ? ` · ${formatAgo(runtimeHealth.lastFalcoEventAt)}`
                      : ''}
                  </div>
                </div>
              </div>
              <p className="text-caption text-muted">
                {runtimeHealth.message ||
                  'Runtime status is derived from persisted runtime events, signals, and pod metrics.'}
              </p>
            </div>
          )}
        </Card>
      ) : null}

      {/* Pipeline overview */}
      <Card
        title="Pipeline overview"
        description="Layer freshness and key persisted counters from the security pipeline."
        actions={<Layers className="w-4 h-4 text-muted" />}
        className="mb-4"
      >
        {pipelineFlow}
      </Card>

      <div className="grid lg:grid-cols-5 gap-6 mb-6">
        <div className="lg:col-span-3 space-y-4">
          <Card
            title="Pipeline processing activity"
            description="Stage evidence from persisted pipeline outputs. Live queue counters appear when worker telemetry is available."
            actions={<Activity className="w-4 h-4 text-muted" />}
          >
            <div className="space-y-3">
              <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                <div className="rounded-md border border-border/70 bg-base/30 px-3 py-2">
                  <div className="text-micro uppercase tracking-wide text-muted">
                    Stages
                  </div>
                  <div className="mt-1 text-body font-semibold text-text">
                    {workerStatus.length}
                  </div>
                </div>
                <div className="rounded-md border border-border/70 bg-base/30 px-3 py-2">
                  <div className="text-micro uppercase tracking-wide text-muted">
                    Processed
                  </div>
                  <div className="mt-1 text-body font-semibold text-text">
                    {workerStatus
                      .reduce((sum, w) => sum + w.processed, 0)
                      .toLocaleString()}
                  </div>
                </div>
                <div className="rounded-md border border-border/70 bg-base/30 px-3 py-2">
                  <div className="text-micro uppercase tracking-wide text-muted">
                    Failures
                  </div>
                  <div
                    className={`mt-1 text-body font-semibold ${workerStatus.some((w) => w.failed > 0) ? 'text-red-400' : 'text-text'}`}
                  >
                    {workerStatus
                      .reduce((sum, w) => sum + w.failed, 0)
                      .toLocaleString()}
                  </div>
                </div>
                <div className="rounded-md border border-border/70 bg-base/30 px-3 py-2">
                  <div className="text-micro uppercase tracking-wide text-muted">
                    Attention
                  </div>
                  <div
                    className={`mt-1 text-body font-semibold ${workerAttentionCount > 0 ? 'text-amber-300' : 'text-text'}`}
                  >
                    {workerAttentionCount.toLocaleString()}
                  </div>
                </div>
              </div>
              {hasLiveWorkerTelemetry ? (
                <p className="text-caption text-muted">
                  Queue depth {totalQueueDepth.toLocaleString()} · active
                  workers {totalActiveWorkers.toLocaleString()}.
                </p>
              ) : (
                <p className="text-caption text-muted">
                  DB-derived mode: processed and failed counts come from
                  persisted outputs; queue and active worker counts are not
                  exposed.
                </p>
              )}

              {workerStatus.length === 0 ? (
                <PageEmpty
                  title="No processing data"
                  description="Pipeline stage telemetry is unavailable."
                  className="py-6"
                />
              ) : (
                <div className="overflow-x-auto">
                  <table className={UI_TABLE}>
                    <thead className={UI_THEAD_STICKY}>
                      <tr>
                        <th className={UI_TH_COMPACT}>Stage</th>
                        <th className={UI_TH_COMPACT}>Status</th>
                        <th className={UI_TH_COMPACT}>Evidence</th>
                        <th className={`${UI_TH_COMPACT} text-right`}>
                          Processed
                        </th>
                        <th className={`${UI_TH_COMPACT} text-right`}>
                          Failed
                        </th>
                        <th className={`${UI_TH_COMPACT} text-right`}>
                          Fail %
                        </th>
                        {hasLiveWorkerTelemetry ? (
                          <th className={`${UI_TH_COMPACT} text-right`}>
                            Queue
                          </th>
                        ) : null}
                        {hasLiveWorkerTelemetry ? (
                          <th className={`${UI_TH_COMPACT} text-right`}>
                            Active
                          </th>
                        ) : null}
                      </tr>
                    </thead>
                    <tbody>
                      {workerRows.map(({ w, failPct }) => {
                        const stageStatus = workerStageStatusStyle(w.status);
                        return (
                          <tr key={w.name} className={UI_TR}>
                            <td
                              className={`${UI_TD_COMPACT_TIGHT} font-mono text-text`}
                            >
                              {w.name}
                            </td>
                            <td className={UI_TD_COMPACT_TIGHT}>
                              <span
                                className={`inline-flex items-center px-2 py-0.5 rounded text-micro font-semibold uppercase ${stageStatus.className}`}
                              >
                                {stageStatus.label}
                              </span>
                            </td>
                            <td
                              className={`${UI_TD_COMPACT_TIGHT} max-w-[18rem] text-muted`}
                            >
                              <span className="line-clamp-2">
                                {w.description ||
                                  (w.source === 'db-derived'
                                    ? 'Persisted database output.'
                                    : w.source || 'Runtime telemetry.')}
                              </span>
                            </td>
                            <td
                              className={`${UI_TD_COMPACT_TIGHT} text-right tabular-nums`}
                            >
                              {w.processed.toLocaleString()}
                            </td>
                            <td
                              className={`${UI_TD_COMPACT_TIGHT} text-right tabular-nums font-medium ${w.failed > 0 ? 'text-red-400' : 'text-muted'}`}
                            >
                              {w.failed}
                            </td>
                            <td
                              className={`${UI_TD_COMPACT_TIGHT} text-right tabular-nums`}
                            >
                              {failPct.toFixed(1)}%
                            </td>
                            {hasLiveWorkerTelemetry ? (
                              <td
                                className={`${UI_TD_COMPACT_TIGHT} text-right tabular-nums`}
                              >
                                {w.queueDepth}
                              </td>
                            ) : null}
                            {hasLiveWorkerTelemetry ? (
                              <td
                                className={`${UI_TD_COMPACT_TIGHT} text-right tabular-nums`}
                              >
                                {w.activeWorkers}
                              </td>
                            ) : null}
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          </Card>
        </div>

        <div className="lg:col-span-2 space-y-4">
          <Card
            title="Agents & certificates"
            actions={
              <Link
                to="/certificates"
                className="text-body font-medium text-brand hover:text-brand"
              >
                Certificates
              </Link>
            }
          >
            {agents.length === 0 ? (
              <PageEmpty
                title="No agents reported"
                description="Check daemonset and agent connectivity."
                className="py-5"
              />
            ) : (
              <div className="space-y-2">
                {agents.slice(0, 10).map((agent) => {
                  const lagMs =
                    Date.now() - new Date(agent.lastHeartbeat).getTime();
                  const lagOk =
                    Number.isFinite(lagMs) && lagMs < HEARTBEAT_WARN_SEC * 1000;
                  return (
                    <div
                      key={agent.id}
                      className="p-2.5 bg-base/50 rounded-lg border border-border space-y-1.5"
                    >
                      <div className="flex items-center justify-between gap-3">
                        <div className="flex min-w-0 items-center">
                          <Radio
                            className={`w-4 h-4 mr-3 shrink-0 ${
                              agent.status === 'up'
                                ? 'text-emerald-500'
                                : agent.status === 'down'
                                  ? 'text-red-500'
                                  : 'text-yellow-500'
                            }`}
                          />
                          <div className="min-w-0">
                            <div className="text-body font-medium text-text truncate">
                              {agent.node}
                            </div>
                          </div>
                        </div>
                        <span
                          className={`text-micro uppercase font-semibold shrink-0 ${
                            agent.status === 'up'
                              ? 'text-emerald-400'
                              : agent.status === 'down'
                                ? 'text-red-400'
                                : 'text-yellow-400'
                          }`}
                        >
                          {agent.status}
                        </span>
                      </div>
                      <div className="text-meta text-muted grid gap-0.5 pl-7">
                        <div>
                          <span className="text-muted-2">Last seen:</span>{' '}
                          {formatAgo(agent.lastHeartbeat)}
                        </div>
                        <div>
                          <span className="text-muted-2">Heartbeat lag:</span>{' '}
                          <span
                            className={lagOk ? 'text-text' : 'text-amber-400'}
                          >
                            {Number.isFinite(lagMs)
                              ? `${Math.round(lagMs / 1000)}s`
                              : '—'}
                          </span>
                        </div>
                        <div>
                          <span className="text-muted-2">Drift:</span>{' '}
                          {syncStatus
                            ? syncStatus.drift
                              ? 'cluster flag set'
                              : 'none'
                            : '—'}
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
            <div className="mt-5 border-t border-border pt-4">
              <div className="mb-2 flex items-center justify-between gap-2">
                <h4 className="text-caption font-semibold uppercase tracking-wide text-muted">
                  Certificates
                </h4>
                {certs.length > 0 && (
                  <Link
                    to="/certificates"
                    className="text-caption font-medium text-brand hover:text-brand"
                  >
                    View all
                  </Link>
                )}
              </div>
              {certs.length === 0 ? (
                <PageEmpty
                  title="No certificate rows"
                  description="TLS inventory may be empty in this environment."
                  className="py-4 border border-dashed border-border/80 rounded-lg"
                />
              ) : (
                <div className="space-y-2">
                  {certs.slice(0, 5).map((cert) => (
                    <div
                      key={cert.id}
                      className="p-2.5 rounded-lg border border-border bg-base/50"
                    >
                      <div className="flex items-center justify-between gap-2">
                        <div className="flex min-w-0 items-center gap-2 text-body text-text">
                          <Lock className="w-4 h-4 text-emerald-500 shrink-0" />
                          <span className="truncate">{cert.name}</span>
                        </div>
                        <div
                          className={`text-caption font-semibold shrink-0 ${(cert.daysRemaining ?? 0) <= 30 ? 'text-red-400' : 'text-emerald-400'}`}
                        >
                          {cert.daysRemaining ?? 0}d TTL
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </Card>
        </div>
      </div>

      {canObservabilityLogs ? (
        <section id="operational-error-logs" className="mb-4 scroll-mt-24">
          <Card
            variant="panel"
            contentClassName="p-0"
            title="Operational error logs"
            actions={
              <Button
                variant="secondary"
                size="sm"
                type="button"
                onClick={() => void fetchErrorLogsPage()}
                disabled={errLoading}
              >
                <RefreshCw className="w-4 h-4 mr-1.5" /> Refresh
              </Button>
            }
          >
            <div className="flex flex-wrap items-center gap-2 border-b border-border/60 px-3 py-2 bg-base/20">
              <span className="text-caption text-muted">Filter:</span>
              <select
                value={errLevel}
                onChange={(e) => {
                  setErrLevel(e.target.value);
                  setErrPage(1);
                }}
                className={UI_FILTER_SELECT}
                aria-label="Log level"
              >
                {ERR_LEVEL_OPTS.map((opt) => (
                  <option key={opt || 'all-level'} value={opt}>
                    {opt || 'All levels'}
                  </option>
                ))}
              </select>
              <select
                value={errSource}
                onChange={(e) => {
                  setErrSource(e.target.value);
                  setErrPage(1);
                }}
                className={UI_FILTER_SELECT}
                aria-label="Source"
              >
                {ERR_SOURCE_OPTS.map((opt) => (
                  <option key={opt || 'all-src'} value={opt}>
                    {opt || 'All sources'}
                  </option>
                ))}
              </select>
            </div>
            {errLoading ? (
              <PageLoading className="py-8 px-3" />
            ) : errLogs.length === 0 ? (
              <PageEmpty
                title="No error log rows"
                description="Nothing matched the current filters."
                className="py-8 px-3"
              />
            ) : (
              <>
                <div className="ui-table-scroll">
                  <table className={UI_TABLE}>
                    <thead className={UI_THEAD_STICKY}>
                      <tr>
                        <th className={UI_TH}>Time</th>
                        <th className={UI_TH}>Level</th>
                        <th className={UI_TH}>Source</th>
                        <th className={UI_TH}>Message</th>
                      </tr>
                    </thead>
                    <tbody>
                      {errLogs.map((log) => (
                        <tr key={log.id} className={UI_TR}>
                          <td
                            className={`${UI_TD} font-mono text-caption text-muted whitespace-nowrap`}
                          >
                            {formatDateTime(log.time)}
                          </td>
                          <td className={UI_TD}>
                            <span
                              className={`text-caption px-2 py-0.5 rounded ${errorLogLevelClass(log.level)}`}
                            >
                              {log.level}
                            </span>
                          </td>
                          <td className={`${UI_TD} text-muted text-caption`}>
                            {log.source || '—'}
                          </td>
                          <td className={`${UI_TD} text-text`}>
                            {log.message}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <Pagination
                  page={errPage}
                  pageSize={errPageSize}
                  total={errTotal}
                  onPageChange={setErrPage}
                  onPageSizeChange={(size) => {
                    setErrPageSize(size);
                    setErrPage(1);
                  }}
                  pageSizeOptions={[10, 20, 50, 100]}
                  itemLabel="rows"
                />
              </>
            )}
          </Card>
        </section>
      ) : null}

      {canPlatformAudit ? (
        <section id="platform-audit-logs" className="mb-4 scroll-mt-24">
          <Card
            variant="panel"
            contentClassName="p-0"
            title="Platform audit logs"
            actions={
              <Button
                variant="secondary"
                size="sm"
                type="button"
                onClick={() => void fetchAuditPage()}
                disabled={auditLoading}
              >
                <RefreshCw className="w-4 h-4 mr-1.5" /> Refresh
              </Button>
            }
          >
            {auditLoading ? (
              <PageLoading className="py-8 px-3" />
            ) : auditLogs.length === 0 ? (
              <PageEmpty
                title="No audit entries"
                description="No platform audit records for this page."
                className="py-8 px-3"
              />
            ) : (
              <>
                <div className="ui-table-scroll">
                  <table className={UI_TABLE}>
                    <thead className={UI_THEAD_STICKY}>
                      <tr>
                        <th className={UI_TH}>Time</th>
                        <th className={UI_TH}>Actor</th>
                        <th className={UI_TH}>Action</th>
                        <th className={UI_TH}>Resource</th>
                        <th className={UI_TH}>Status</th>
                      </tr>
                    </thead>
                    <tbody>
                      {auditLogs.map((log) => (
                        <tr
                          key={log.id || `${log.timestamp}-${log.action}`}
                          className={UI_TR}
                        >
                          <td
                            className={`${UI_TD} font-mono text-caption text-muted`}
                          >
                            {formatDateTime(log.timestamp)}
                          </td>
                          <td className={`${UI_TD} text-text font-medium`}>
                            {log.actor || log.user || 'system'}
                          </td>
                          <td className={`${UI_TD} text-muted`}>
                            {log.action}
                          </td>
                          <td
                            className={`${UI_TD} text-muted font-mono text-caption`}
                          >
                            {log.resource}
                          </td>
                          <td className={UI_TD}>{log.status}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <Pagination
                  page={auditPage}
                  pageSize={auditPageSize}
                  total={auditTotal}
                  onPageChange={setAuditPage}
                  onPageSizeChange={(size) => {
                    setAuditPageSize(size);
                    setAuditPage(1);
                  }}
                  pageSizeOptions={[10, 20, 50]}
                  itemLabel="entries"
                />
              </>
            )}
          </Card>
        </section>
      ) : null}

      {/* Logs — collapsed by default */}
      <details className="rounded-lg border border-border bg-base/20">
        <summary className="cursor-pointer select-none px-4 py-3 text-body font-semibold text-text list-none flex items-center justify-between gap-2 [&::-webkit-details-marker]:hidden">
          <span>Logs</span>
          <span className="text-caption font-normal text-muted">Show logs</span>
        </summary>
        <div className="px-4 pb-4 border-t border-border/60">
          <div className="flex flex-wrap items-center gap-2 py-3">
            <span className="text-caption text-muted">Filter:</span>
            {(['ALL', 'ERROR', 'WARN', 'INFO'] as const).map((f) => (
              <Button
                key={f}
                variant={logFilter === f ? 'primary' : 'secondary'}
                size="sm"
                className="!px-2.5 !py-0.5 !text-caption"
                onClick={() => setLogFilter(f)}
              >
                {f}
              </Button>
            ))}
            <Link
              to="/monitoring?section=error-logs"
              className="text-caption font-medium text-brand hover:text-brand ml-auto"
            >
              View all
            </Link>
          </div>
          {filteredLogs.length === 0 ? (
            <PageEmpty
              title="No matching logs"
              description="Try another level or open the full error log view."
              className="py-5 min-h-[6rem]"
            />
          ) : (
            <div className="space-y-2 max-h-[320px] overflow-y-auto pr-1">
              {filteredLogs.map((log) => (
                <div
                  key={log.id}
                  className="p-3 border border-border rounded bg-base/40"
                >
                  <div className="flex items-center justify-between text-caption mb-1">
                    <span className="text-muted font-mono">
                      {formatDateTime(log.time)}
                    </span>
                    <span
                      className={`${
                        log.level === 'ERROR'
                          ? 'text-red-400'
                          : log.level === 'WARN'
                            ? 'text-yellow-400'
                            : 'text-sky-400'
                      } font-semibold`}
                    >
                      {log.level}
                    </span>
                  </div>
                  <div className="text-body text-text">{log.message}</div>
                  {log.source && (
                    <div className="text-caption text-muted mt-1">
                      {log.source}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </details>
    </PageLayout>
  );
};
