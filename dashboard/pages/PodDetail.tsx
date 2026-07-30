import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { matchPath, useLocation, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { api } from '../lib/api';
import {
  PodWithRisk,
  PodSbom,
  Insight,
  Vulnerability,
  RuntimeSignal,
  RuntimeSignalSuppressionStats,
  PodRiskReportSummary,
  PodRuntimeSecurityEvent,
  PodRuntimeBehaviorFact,
  PodRuntimeIncident,
  PodCapabilityDetail,
  UnifiedRiskScore,
  K8sServiceAccount,
} from '../types';
import { useClusters } from '../hooks/useClusters';
import { RUNTIME_SIGNALS_LOOKBACK_MINUTES } from '../lib/runtimeLookback';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PinToInvestigationButton } from '../components/PinToInvestigationButton';
import { podInvestigationEntity } from '../lib/investigationEntities';
import { Tabs } from '../design-system/components/Tabs';
import { Card } from '../design-system/components/Card';
import { Button } from '../components/ui/Button';
import { PageEmpty, PageError, PageLoading } from '../design-system/components/PageStatus';
import { PodNetworkSummary } from '../components/PodNetworkSummary';
import { ArrowLeft, Box, Package, ShieldAlert, Globe, Download, ChevronDown, ChevronRight, X, FileText, ExternalLink, CheckCircle2, Info, Cpu, Network, Activity, BarChart2, FileCode, Shield, AlertTriangle, RefreshCw, Target, Zap } from 'lucide-react';
import clsx from 'clsx';
import { getSeverityBadgeClass, getSeverityBarClass, getSeverityTextClass, getSeverityIcon, getPodStatusBadgeClass, deriveUnifiedRiskLevelFromScore } from '../lib/severity';
import { formatDateTime, formatUptime } from '../lib/display';
import { exportSbomAsCsv, exportSbomAsCycloneDxJson, exportSbomAsJson, exportSbomAsSpdxJson } from '../lib/exportSbom';
import { SbomMetaBadges } from '../components/SbomMetaBadges';
import { useAuthStore } from '../store/authStore';
import type { SbomComponent as SbomComponentType, PodRuntimeMetric, PodProcessItem, PodNetworkConnectionItem, PodK8sEventItem, PodNetworkTopDestinationItem } from '../types';
import { formatRiskFindingReference, insightTypeUiLabel } from '../lib/riskDisplay';
import { getClusterDisplayName } from '../lib/clusterDisplay';
import { UI_TABLE, UI_THEAD_STICKY, UI_TH_COMPACT, UI_TR, UI_TD_COMPACT_TIGHT } from '../lib/tableChrome';
import {
  UI_PILL_ACTIVE,
  UI_PILL_ACTIVE_BORDERED,
  UI_PILL_IDLE_COMPACT,
  UI_PILL_IDLE_FILTER,
  UI_PILL_IDLE_SEGMENT,
} from '../lib/formChrome';

type TabId = 'overview' | 'sbom' | 'risks' | 'metrics' | 'processes' | 'network' | 'events' | 'timeline' | 'coverage' | 'spec'
  | 'risk_sbom' | 'runtime';  // consolidated tab aliases

/** Map legacy/granular tab IDs to consolidated tab for display */
const TAB_ALIAS: Partial<Record<TabId, TabId>> = {
  sbom: 'risk_sbom',
  risks: 'risk_sbom',
  metrics: 'runtime',
  processes: 'runtime',
  events: 'runtime',
  timeline: 'runtime',
  coverage: 'overview',
};

function normalizeTabId(value: string | null | undefined): TabId {
  const raw = String(value ?? '').trim();
  const known: TabId[] = [
    'overview',
    'sbom',
    'risks',
    'metrics',
    'processes',
    'network',
    'events',
    'timeline',
    'coverage',
    'spec',
    'risk_sbom',
    'runtime',
  ];
  return known.includes(raw as TabId) ? (raw as TabId) : 'overview';
}

/** Preload failures merged with refreshAllData errors; cleared independently on successful SBOM / risk fetch. */
const PRELOAD_DATA_ERROR_LABELS = new Set(['sbom', 'risk-report']);

/** Short type label for SBOM (os-package -> os, library -> lib, etc.) */
function sbomTypeLabel(type: string | undefined): string {
  if (!type) return '—';
  const t = type.toLowerCase();
  if (t === 'os-package' || t === 'os') return 'os';
  if (t === 'library' || t === 'lib') return 'lib';
  if (t === 'language-runtime' || t === 'runtime') return 'runtime';
  return type;
}

function statusBadgeClass(status: string | undefined): string {
  const s = (status ?? 'active').toLowerCase();
  if (s === 'active') return 'bg-red-600/80 text-white';
  if (s === 'allowed') return 'bg-sky-500/80 text-white';
  if (s === 'fixed') return 'bg-muted/80 text-text';
  if (s === 'not exploitable' || s === 'not_exploitable') return 'bg-emerald-500/80 text-white';
  return 'bg-muted-2/80 text-text';
}

export const PodDetail: React.FC = () => {
  const { id, uid } = useParams<{ id?: string; uid?: string }>();
  const location = useLocation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const requestedTab = searchParams.get('tab');
  const [pod, setPod] = useState<PodWithRisk | null>(null);
  const [sbom, setSbom] = useState<PodSbom | null>(null);
  const [relatedRisks, setRelatedRisks] = useState<Insight[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<TabId>(() => normalizeTabId(searchParams.get('tab')));
  const [tabLoading, setTabLoading] = useState(false);
  const [sbomLoaded, setSbomLoaded] = useState(false);
  const [sbomSeverityFilter, setSbomSeverityFilter] = useState<string>('all');
  const [sbomStatusFilter, setSbomStatusFilter] = useState<string>('all');
  const [sbomOnlyVulnerable, setSbomOnlyVulnerable] = useState(false);
  const [sbomSearch, setSbomSearch] = useState<string>('');
  const [sbomSort, setSbomSort] = useState<'name' | 'severity' | 'cve' | 'none'>('none');
  const [sbomExpandedId, setSbomExpandedId] = useState<string | null>(null);
  const [selectedVulnerability, setSelectedVulnerability] = useState<Vulnerability | null>(null);
  const [runtimeMetrics, setRuntimeMetrics] = useState<PodRuntimeMetric[]>([]);
  const [processes, setProcesses] = useState<PodProcessItem[]>([]);
  const [networkConnections, setNetworkConnections] = useState<PodNetworkConnectionItem[]>([]);
  const [networkTopDestinations, setNetworkTopDestinations] = useState<PodNetworkTopDestinationItem[]>([]);
  const [networkSubView, setNetworkSubView] = useState<'summary' | 'raw'>('summary');
  const [podEvents, setPodEvents] = useState<PodK8sEventItem[]>([]);
  const [runtimeSecurityEvents, setRuntimeSecurityEvents] = useState<PodRuntimeSecurityEvent[]>([]);
  const [runtimeSignals, setRuntimeSignals] = useState<RuntimeSignal[]>([]);
  const [runtimeFacts, setRuntimeFacts] = useState<PodRuntimeBehaviorFact[]>([]);
  const [runtimeIncidents, setRuntimeIncidents] = useState<PodRuntimeIncident[]>([]);
  const [podCapabilities, setPodCapabilities] = useState<PodCapabilityDetail[]>([]);
  const [signalStats, setSignalStats] = useState<RuntimeSignalSuppressionStats | null>(null);
  const [runtimeSignalFilter, setRuntimeSignalFilter] = useState<'all' | 'NETWORK_QUEUE_ANOMALY'>('all');
  /** Filter for GET /risk/.../runtime/events (Falco vs other collectors) */
  const [secRuntimeFilter, setSecRuntimeFilter] = useState<'all' | 'falco' | 'other'>('all');
  const [specYaml, setSpecYaml] = useState<string>('');
  const [refreshing, setRefreshing] = useState(false);
  const [dataErrors, setDataErrors] = useState<string[]>([]);
  const dataErrorsRef = useRef<string[]>([]);
  dataErrorsRef.current = dataErrors;
  /** From GET /risk/pods/:uid/report — same 24h window as summary.runtimeSignals24h */
  const [podRiskReportSummary, setPodRiskReportSummary] = useState<PodRiskReportSummary | null>(null);
  const [serviceAccountRef, setServiceAccountRef] = useState<K8sServiceAccount | null>(null);
  const [serviceAccountLookupComplete, setServiceAccountLookupComplete] = useState(false);
  /** Phase 3.1: Unified Risk Score V3 for this pod */
  const [unifiedScore, setUnifiedScore] = useState<UnifiedRiskScore | null>(null);
  const { clusters } = useClusters();

  const uidFromPath = matchPath({ path: '/resources/pods/uid/:uid', end: true }, location.pathname)?.params.uid;
  const idFromPath = matchPath({ path: '/resources/pods/:id', end: true }, location.pathname)?.params.id;
  const canonicalUid = uid ?? uidFromPath;
  const idOrUid = canonicalUid ?? id ?? idFromPath;

  useEffect(() => {
    const nextTab = normalizeTabId(requestedTab);
    setActiveTab((prev) => (prev === nextTab ? prev : nextTab));
  }, [requestedTab]);

  const loadSbomForPod = useCallback(
    async (podRef: Pick<PodWithRisk, 'uid' | 'name' | 'namespace'>): Promise<PodSbom | null> => {
      try {
        const detailed = await api.getPodSbom(podRef.uid);
        if (detailed) return detailed;
      } catch {
        // Fallback below
      }

      try {
        const summaries = await api.getSbomList({
          podName: podRef.name,
          namespace: podRef.namespace,
        });
        const exact =
          summaries.find((s) => s.podName === podRef.name && s.namespace === podRef.namespace) ??
          summaries[0];
        if (!exact) return null;
        const vulnSummary = exact.vulnerabilitySummary ?? { critical: 0, high: 0, medium: 0, low: 0 };
        const vulnerablePackageCount =
          Number(vulnSummary.critical ?? 0) +
          Number(vulnSummary.high ?? 0) +
          Number(vulnSummary.medium ?? 0) +
          Number(vulnSummary.low ?? 0);
        return {
          podId: exact.podId || podRef.uid,
          podName: exact.podName || podRef.name,
          namespace: exact.namespace || podRef.namespace,
          image: exact.image || '',
          imageDigest: exact.imageDigest,
          imageTrust: exact.imageTrust,
          packageCount: exact.packageCount,
          vulnerablePackageCount,
          vulnerabilitySummary: vulnSummary,
          components: [],
          generatedAt: exact.lastScan,
          activePod: exact.activePod,
          lifecycleState: exact.lifecycleState,
          sbomSource: exact.sbomSource,
          confidence: exact.confidence,
          goVersion: exact.goVersion,
        };
      } catch {
        return null;
      }
    },
    []
  );

  const serviceAccountName = String(pod?.serviceAccount ?? '').trim();
  const openServiceAccountIdentity = useCallback(() => {
    if (!serviceAccountRef?.uid) return;
    navigate(`/identities/uid/${encodeURIComponent(serviceAccountRef.uid)}`);
  }, [navigate, serviceAccountRef?.uid]);

  const resolveServiceAccountRef = useCallback(async (podRef: PodWithRisk): Promise<K8sServiceAccount | null> => {
    const saName = String(podRef.serviceAccount ?? '').trim();
    if (podRef.serviceAccountUid) {
      const sa = await api.getServiceAccountByUid(podRef.serviceAccountUid);
      if (sa) {
        return {
          id: Number(sa.id ?? 0),
          clusterId: String(sa.clusterId ?? podRef.clusterId ?? ''),
          name: String(sa.name ?? saName),
          namespace: String(sa.namespace ?? podRef.namespace ?? ''),
          uid: String(sa.uid ?? podRef.serviceAccountUid),
          labels: sa.labels != null ? String(sa.labels) : undefined,
          secrets: sa.secrets != null ? String(sa.secrets) : undefined,
          linkedPods: sa.linkedPods != null ? String(sa.linkedPods) : undefined,
          lastUsed: sa.lastUsed != null ? String(sa.lastUsed) : null,
          createdAt: sa.createdAt != null ? String(sa.createdAt) : undefined,
          updatedAt: sa.updatedAt != null ? String(sa.updatedAt) : undefined,
        };
      }
    }
    if (!podRef.clusterId || !podRef.namespace || !saName) return null;
    const list = await api.getServiceAccounts({
      clusterId: podRef.clusterId,
      namespace: podRef.namespace,
      pageSize: 1000,
    });
    return list.serviceAccounts.find(
      (sa) => sa.clusterId === podRef.clusterId && sa.namespace === podRef.namespace && sa.name === saName,
    ) ?? null;
  }, []);

  const applyPodRiskReport = useCallback((report: Awaited<ReturnType<typeof api.getPodRiskReport>>, podRef: PodWithRisk) => {
    setRelatedRisks(report.insights ?? []);
    setPodRiskReportSummary(report.summary ?? null);
    if (report.serviceAccountUid && podRef.serviceAccount) {
      setServiceAccountRef((prev) => prev ?? {
        id: 0,
        clusterId: report.clusterId || podRef.clusterId,
        name: report.serviceAccount || podRef.serviceAccount || '',
        namespace: report.namespace || podRef.namespace,
        uid: report.serviceAccountUid!,
      });
      setServiceAccountLookupComplete(true);
    }
  }, []);

  const runtimeSignalVisual = (signalType: string): { signalClass: string; severity: string; severityClass: string } => {
    const t = (signalType || '').trim().toUpperCase();
    const knownSignals: Record<string, { signalClass: string; severity: string; severityClass: string }> = {
      NETWORK_QUEUE_ANOMALY: {
        signalClass: 'bg-cyan-500/20 text-cyan-300 border-cyan-500/40',
        severity: 'MEDIUM',
        severityClass: 'bg-amber-500/20 text-amber-300 border-amber-500/40',
      },
      SUSPICIOUS_EXEC_FROM_SNAPSHOT: {
        signalClass: 'bg-orange-500/20 text-orange-300 border-orange-500/40',
        severity: 'HIGH',
        severityClass: 'bg-red-500/20 text-red-300 border-red-500/40',
      },
      PRIVILEGE_ESCALATION: {
        signalClass: 'bg-red-500/20 text-red-300 border-red-500/40',
        severity: 'CRITICAL',
        severityClass: 'bg-red-600/20 text-red-300 border-red-600/40',
      },
      UNEXPECTED_NETWORK_CONN: {
        signalClass: 'bg-amber-500/20 text-amber-300 border-amber-500/40',
        severity: 'MEDIUM',
        severityClass: 'bg-amber-500/20 text-amber-300 border-amber-500/40',
      },
      SENSITIVE_FILE_ACCESS: {
        signalClass: 'bg-yellow-500/20 text-yellow-300 border-yellow-500/40',
        severity: 'HIGH',
        severityClass: 'bg-red-500/20 text-red-300 border-red-500/40',
      },
      CRYPTOMINING_DETECTED: {
        signalClass: 'bg-red-500/20 text-red-300 border-red-500/40',
        severity: 'CRITICAL',
        severityClass: 'bg-red-600/20 text-red-300 border-red-600/40',
      },
    };
    if (knownSignals[t]) return knownSignals[t];
    // Dynamic fallback: treat any unknown signal with WARN-level styling instead of silent INFO
    if (t) {
      return {
        signalClass: 'bg-amber-500/15 text-amber-200 border-amber-500/30',
        severity: 'WARN',
        severityClass: 'bg-amber-500/15 text-amber-200 border-amber-500/30',
      };
    }
    return {
      signalClass: 'bg-muted/20 text-text border-border/40',
      severity: 'INFO',
      severityClass: 'bg-muted/20 text-text border-border/40',
    };
  };

  const capabilityImpactLabel = (severity?: string): string => {
    const s = String(severity ?? '').toLowerCase();
    if (s === 'critical') return 'High impact';
    if (s === 'high') return 'Elevated';
    if (s === 'medium') return 'Moderate';
    if (s === 'low') return 'Low impact';
    return s || 'Unknown';
  };

  const runtimeSourceBadge = (src: string | undefined): { label: string; className: string } => {
    const s = (src || '').toLowerCase().trim();
    if (s === 'falco') return { label: 'Falco', className: 'bg-violet-600/25 text-violet-200 border-violet-500/40' };
    if (s === 'ebpf') return { label: 'eBPF', className: 'bg-sky-600/25 text-sky-200 border-sky-500/40' };
    if (s === 'agent') return { label: 'Agent', className: 'bg-emerald-600/25 text-emerald-200 border-emerald-500/40' };
    return { label: s ? src! : '—', className: 'bg-muted-2/25 text-text border-border/40' };
  };

  const runtimeDataHints = (): string[] => {
    const out: string[] = [];
    if (runtimeSecurityEvents.length === 0) out.push('No runtime_events for this pod UID yet (sensor -> Core ingest).');
    if (runtimeFacts.length === 0) out.push('No behavior facts synthesized yet (REP-A output empty).');
    if (runtimeIncidents.length === 0) out.push('No correlated incidents yet (REP-C threshold/window not reached).');
    if (runtimeSignals.length === 0) out.push('No runtime signals in lookback window (check signal filters and lookback).');
    return out;
  };

  const fetchPod = useCallback(async () => {
    if (!idOrUid) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setLoadError(null);
    try {
      let data = await api.getPodByUid(idOrUid);
      if (!data && !canonicalUid && /^[0-9]+$/.test(String(idOrUid))) {
        data = await api.getPodByLegacyId(idOrUid);
        if (data?.uid) {
          navigate(
            `/resources/pods/uid/${encodeURIComponent(data.uid)}${requestedTab ? `?tab=${encodeURIComponent(requestedTab)}` : ''}`,
            { replace: true },
          );
        }
      }
      setPod(data);
      setServiceAccountRef(null);
      setServiceAccountLookupComplete(false);
      if (data?.serviceAccount) {
        resolveServiceAccountRef(data)
          .then((sa) => setServiceAccountRef(sa))
          .catch(() => setServiceAccountRef(null))
          .finally(() => setServiceAccountLookupComplete(true));
      } else {
        setServiceAccountLookupComplete(true);
      }
      // Seed unifiedScore from pod response immediately to avoid a visual flash
      // (badge shows riskCount fallback until async getUnifiedRiskScore resolves).
      if (data?.unifiedScore != null && data.finalLevel) {
        setUnifiedScore((prev) => prev ?? {
          totalScore: data.unifiedScore!,
          finalLevel: data.finalLevel as UnifiedRiskScore['finalLevel'],
          scorerVersion: data.scorerVersion ?? 'v3',
        } as UnifiedRiskScore);
      }
      // Then fetch full breakdown (dimensions, toxic combos) from the dedicated risk score endpoint.
      if (data?.uid) {
        api.getUnifiedRiskScore(data.uid, data.clusterId).then((score) => {
          if (score) setUnifiedScore(score);
        }).catch(() => {/* non-critical */});
      }
    } catch (e) {
      setPod(null);
      setLoadError(e instanceof Error ? e.message : 'Failed to load pod detail');
    } finally {
      setLoading(false);
    }
  }, [canonicalUid, idOrUid, navigate, requestedTab, resolveServiceAccountRef]);

  const fetchTabData = useCallback(
    async (tab: TabId) => {
      if (!pod?.uid) return;
      setTabLoading(true);
      try {
        const uid = pod.uid;
        if (tab === 'sbom') {
          if (!sbomLoaded || dataErrorsRef.current.includes('sbom')) {
            try {
              const data = await loadSbomForPod({
                uid,
                name: pod.name,
                namespace: pod.namespace,
              });
              setSbom(data ?? null);
              setSbomLoaded(true);
              setDataErrors((p) => p.filter((e) => e !== 'sbom'));
            } catch {
              setSbom(null);
              setSbomLoaded(true);
              setDataErrors((p) => (p.includes('sbom') ? p : [...p, 'sbom']));
            }
          }
        } else if (tab === 'risks') {
          if (dataErrorsRef.current.includes('risk-report') || (relatedRisks.length === 0 && !podRiskReportSummary)) {
            try {
              const report = await api.getPodRiskReport(uid);
              applyPodRiskReport(report, pod);
              setDataErrors((p) => p.filter((e) => e !== 'risk-report'));
            } catch {
              setRelatedRisks([]);
              setPodRiskReportSummary(null);
              setDataErrors((p) => (p.includes('risk-report') ? p : [...p, 'risk-report']));
            }
          }
        } else if (tab === 'processes') {
          if (processes.length === 0) {
            const data = await api.getPodProcesses(uid);
            setProcesses(data);
          }
        } else if (tab === 'network') {
          if (networkConnections.length === 0) {
            const [data, topDest] = await Promise.all([
              api.getPodNetworkConnections(uid),
              api.getPodNetworkTopDestinations(uid, { sinceMinutes: 1440 }),
            ]);
            setNetworkConnections(data);
            setNetworkTopDestinations(topDest);
          } else if (networkTopDestinations.length === 0) {
            const topDest = await api.getPodNetworkTopDestinations(uid, { sinceMinutes: 1440 });
            setNetworkTopDestinations(topDest);
          }
        } else if (tab === 'events' || tab === 'timeline' || tab === 'coverage') {
          // GAP 4: fetch each slice independently — avoids skipping when only one of preload/API calls failed
          const tasks: Promise<unknown>[] = [];
          if (podEvents.length === 0) {
            tasks.push(api.getPodEvents(uid).then(setPodEvents).catch(() => undefined));
          }
          if (runtimeSecurityEvents.length === 0) {
            tasks.push(
              api.getPodRuntimeSecurityEvents(uid, 150).then(setRuntimeSecurityEvents).catch(() => undefined),
            );
          }
          if (runtimeFacts.length === 0) {
            tasks.push(api.getPodRuntimeBehaviorFactsV2(uid, 120).then(setRuntimeFacts).catch(() => undefined));
          }
          if (runtimeIncidents.length === 0) {
            tasks.push(api.getPodRuntimeIncidentsV2(uid, 80).then(setRuntimeIncidents).catch(() => undefined));
          }
          if (podCapabilities.length === 0) {
            tasks.push(api.getPodCapabilities(uid).then(setPodCapabilities).catch(() => undefined));
          }
          if (runtimeSignals.length === 0) {
            tasks.push(
              api
                .getRuntimeSignalsByPod(uid, { sinceMinutes: RUNTIME_SIGNALS_LOOKBACK_MINUTES, limit: 200 })
                .then(setRuntimeSignals)
                .catch(() => undefined),
            );
          }
          if (signalStats === null) {
            tasks.push(
              api
                .getRuntimeSignalSuppressionStats({ podUid: uid, sinceMinutes: 60 })
                .then(setSignalStats)
                .catch(() => undefined),
            );
          }
          await Promise.all(tasks);
        } else if (tab === 'spec') {
          const yaml = await api.getPodSpecYaml(uid);
          setSpecYaml(yaml);
        }
      } finally {
        setTabLoading(false);
      }
    },
    [
      pod,
      sbomLoaded,
      relatedRisks.length,
      podRiskReportSummary,
      processes.length,
      networkConnections.length,
      networkTopDestinations.length,
      runtimeSecurityEvents.length,
      runtimeFacts.length,
      podEvents.length,
      runtimeIncidents.length,
      podCapabilities.length,
      runtimeSignals.length,
      signalStats,
      applyPodRiskReport,
    ]
  );

  useEffect(() => {
    fetchPod();
  }, [fetchPod]);

  // Load SBOM when pod is available (for Overview summary + SBOM tab)
  useEffect(() => {
    if (pod?.uid) {
      loadSbomForPod({
        uid: pod.uid,
        name: pod.name,
        namespace: pod.namespace,
      })
        .then((data) => {
          setSbom(data ?? null);
          setSbomLoaded(true);
          setDataErrors((p) => p.filter((e) => e !== 'sbom'));
        })
        .catch(() => {
          setSbom(null);
          setSbomLoaded(true);
          setDataErrors((p) => (p.includes('sbom') ? p : [...p, 'sbom']));
        });
    } else {
      setSbom(null);
      setSbomLoaded(false);
    }
  }, [pod?.uid, pod?.name, pod?.namespace, loadSbomForPod]);

  useEffect(() => {
    if (!pod?.uid) {
      setPodRiskReportSummary(null);
      setRelatedRisks([]);
      return;
    }
    api
      .getPodRiskReport(pod.uid)
      .then((report) => {
        applyPodRiskReport(report, pod);
        setDataErrors((p) => p.filter((e) => e !== 'risk-report'));
      })
      .catch(() => {
        setRelatedRisks([]);
        setPodRiskReportSummary(null);
        setDataErrors((p) => (p.includes('risk-report') ? p : [...p, 'risk-report']));
      });
  }, [pod?.uid]);

  // Helper to refresh all pod-detail data (used by preload + WS + manual refresh)
  const refreshAllData = useCallback((podUid: string) => {
    const errors: string[] = [];
    const track = (label: string) => () => {
      errors.push(label);
    };
    Promise.all([
      api.getPodRuntimeMetrics(podUid).then(setRuntimeMetrics).catch(track('metrics')),
      api.getPodProcesses(podUid).then(setProcesses).catch(track('processes')),
      api.getPodNetworkConnections(podUid).then(setNetworkConnections).catch(track('network')),
      api.getPodNetworkTopDestinations(podUid, { sinceMinutes: 1440 }).then(setNetworkTopDestinations).catch(track('top-dest')),
      api.getPodEvents(podUid).then(setPodEvents).catch(track('events')),
      api.getPodRuntimeSecurityEvents(podUid, 150).then(setRuntimeSecurityEvents).catch(track('security-events')),
      api.getRuntimeSignalsByPod(podUid, { sinceMinutes: RUNTIME_SIGNALS_LOOKBACK_MINUTES, limit: 200 }).then(setRuntimeSignals).catch(track('signals')),
      api.getPodRuntimeBehaviorFactsV2(podUid, 120).then(setRuntimeFacts).catch(track('facts')),
      api.getPodRuntimeIncidentsV2(podUid, 80).then(setRuntimeIncidents).catch(track('incidents')),
      api.getPodCapabilities(podUid).then(setPodCapabilities).catch(track('capabilities')),
      api.getRuntimeSignalSuppressionStats({ podUid, sinceMinutes: 60 }).then(setSignalStats).catch(track('signal-stats')),
    ]).then(() => {
      setDataErrors((prev) => {
        const kept = prev.filter((e) => PRELOAD_DATA_ERROR_LABELS.has(e));
        if (errors.length > 0) return [...new Set([...kept, ...errors])];
        return kept;
      });
    });
  }, []);

  // Preload pod-detail (metrics, processes, network) so Overview shows counts and Network tab has data. All use pod UID.
  useEffect(() => {
    if (!pod?.uid) return;
    refreshAllData(pod.uid);
  }, [pod?.uid, refreshAllData]);

  useEffect(() => {
    if (pod && activeTab !== 'overview') fetchTabData(activeTab);
  }, [pod, activeTab, fetchTabData]);

  // Phase 5.1: WebSocket for live pod detail updates (metrics, processes, network, events). All APIs use pod UID.
  const wsUid = pod?.uid ?? canonicalUid ?? null;
  const hasToken = Boolean(useAuthStore((s) => s.token));
  const podUidRef = useRef(pod?.uid);
  podUidRef.current = pod?.uid;
  useEffect(() => {
    if (!wsUid || !pod?.uid || !hasToken) return;
    const wsUrl = api.getPodDetailWsUrl(wsUid);
    let ws: WebSocket | null = null;
    try {
      ws = new WebSocket(wsUrl);
      ws.onmessage = (e) => {
        const currentUid = podUidRef.current;
        if (!currentUid) return;
        try {
          const d = JSON.parse(e.data as string) as { type?: string };
          const t = d?.type;
          if (t === 'metrics') {
            api.getPodRuntimeMetrics(currentUid).then(setRuntimeMetrics).catch(() => {});
          } else if (t === 'processes') {
            api.getPodProcesses(currentUid).then(setProcesses).catch(() => {});
          } else if (t === 'network') {
            api.getPodNetworkConnections(currentUid).then(setNetworkConnections).catch(() => {});
          } else if (t === 'events') {
            refreshAllData(currentUid);
          } else {
            refreshAllData(currentUid);
          }
        } catch {
          refreshAllData(currentUid);
        }
      };
    } catch {
      // ignore WS connect errors
    }
    return () => {
      if (ws != null) ws.close();
    };
  }, [wsUid, pod?.uid, hasToken, refreshAllData]);

  const pageSubtitle = useMemo(() => {
    if (!pod) return undefined;
    const parts: string[] = [];
    const ns = (pod.namespace ?? '').trim();
    if (ns) parts.push(ns);
    parts.push(getClusterDisplayName(clusters.find((c) => c.id === pod.clusterId) ?? { id: pod.clusterId }));
    return parts.join(' · ');
  }, [pod, clusters]);

  if (!idOrUid) {
    return (
      <PageLayout title="Pod detail route is incomplete" description="No pod UID or legacy pod ID was provided.">
        <PageError
          title="Pod identifier missing"
          description="Open pod detail from the Resources pod list so the route includes a Kubernetes pod UID."
          action={
            <Button variant="secondary" onClick={() => navigate('/resources')}>
              <ArrowLeft className="w-4 h-4 mr-2" /> Back to Resources
            </Button>
          }
        />
      </PageLayout>
    );
  }

  if (loading) {
    return <PageLoading message="Loading pod detail..." className="min-h-[40dvh]" />;
  }

  if (!pod) {
    if (loadError) {
      return (
        <PageLayout title="Could not load pod detail" description="The pod detail API returned an error for this request.">
          <PageError
            title="Could not load pod detail"
            description={loadError}
            action={
              <Button variant="secondary" onClick={() => navigate('/resources')}>
                <ArrowLeft className="w-4 h-4 mr-2" /> Back to Resources
              </Button>
            }
          />
        </PageLayout>
      );
    }
    const looksLikeLegacyId = idOrUid != null && /^[0-9]+$/.test(String(idOrUid));
    return (
      <PageLayout title="Pod not found" description="The pod may have been removed or you lack access.">
        <PageEmpty
          title="Pod not found"
          description={
            looksLikeLegacyId
              ? 'This link used an old pod ID. Pods are now identified by UID. Open the pod from Resources (Pods list).'
              : 'The pod may have been removed or is outside current data scope.'
          }
        />
        <div className="mt-4">
          <Button variant="secondary" onClick={() => navigate('/resources')}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Back to Resources
          </Button>
        </div>
      </PageLayout>
    );
  }

  const tabs: { id: TabId; label: string; icon: React.ReactNode }[] = [
    { id: 'overview', label: 'Overview', icon: <Box className="w-4 h-4" /> },
    { id: 'risk_sbom', label: 'Risk & SBOM', icon: <ShieldAlert className="w-4 h-4" /> },
    { id: 'runtime', label: 'Runtime', icon: <Activity className="w-4 h-4" /> },
    { id: 'network', label: 'Network', icon: <Network className="w-4 h-4" /> },
    { id: 'spec', label: 'Spec', icon: <FileCode className="w-4 h-4" /> },
  ];

  /** Resolve consolidated tab: e.g. if URL says ?tab=sbom, display risk_sbom */
  const resolvedTab = TAB_ALIAS[activeTab] ?? activeTab;
  const displayedRiskScore = unifiedScore?.totalScore ?? pod.unifiedScore ?? pod.totalScore;
  const displayedRiskLevel = (unifiedScore?.finalLevel ?? pod.finalLevel ?? deriveUnifiedRiskLevelFromScore(displayedRiskScore)) as
    | 'critical'
    | 'high'
    | 'medium'
    | 'low'
    | undefined;

  return (
    <PageLayout
      title={pod.name}
      description={pageSubtitle}
      actions={
        <div className="flex items-center gap-2">
          <Button
            variant="secondary"
            onClick={async () => {
              if (!pod?.uid) return;
              setRefreshing(true);
              try {
                await fetchPod();
                refreshAllData(pod.uid);
                await loadSbomForPod({
                  uid: pod.uid,
                  name: pod.name,
                  namespace: pod.namespace,
                })
                  .then((d) => {
                    setSbom(d ?? null);
                    setSbomLoaded(true);
                    setDataErrors((p) => p.filter((e) => e !== 'sbom'));
                  })
                  .catch(() => {
                    setSbom(null);
                    setSbomLoaded(true);
                    setDataErrors((p) => (p.includes('sbom') ? p : [...p, 'sbom']));
                  });
                await api
                  .getPodRiskReport(pod.uid)
                  .then((report) => {
                    applyPodRiskReport(report, pod);
                    setDataErrors((p) => p.filter((e) => e !== 'risk-report'));
                  })
                  .catch(() => {
                    setRelatedRisks([]);
                    setPodRiskReportSummary(null);
                    setDataErrors((p) => (p.includes('risk-report') ? p : [...p, 'risk-report']));
                  });
              } finally {
                setRefreshing(false);
              }
            }}
            disabled={refreshing}
          >
            <RefreshCw className={clsx('w-4 h-4 mr-2', refreshing && 'animate-spin')} /> {refreshing ? 'Refreshing…' : 'Refresh'}
          </Button>
          <Button variant="secondary" onClick={() => navigate('/resources')}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Back to Resources
          </Button>
          {pod?.uid ? (
            <PinToInvestigationButton
              entity={podInvestigationEntity({
                uid: pod.uid,
                name: pod.name,
                namespace: pod.namespace,
              })}
            />
          ) : null}
        </div>
      }
    >
      <div className="mb-6 flex min-w-0 flex-col gap-3 sm:flex-row sm:items-start sm:gap-4">
        <div className="flex min-w-0 flex-1 items-start gap-3 sm:gap-4">
          <div className="shrink-0 rounded-xl border border-border bg-surface p-3 sm:p-4" aria-hidden>
            <Box className="h-7 w-7 text-brand sm:h-8 sm:w-8" />
          </div>
          <div className="min-w-0 flex-1 space-y-2">
            {displayedRiskLevel ? (
              <div className="flex flex-wrap items-center gap-2">
                <span
                  title="Authoritative risk level from Unified Risk Score V3"
                  className={clsx(
                    'inline-flex rounded-full border px-2.5 py-0.5 text-caption font-semibold uppercase',
                    getSeverityBadgeClass(displayedRiskLevel)
                  )}
                >
                  Risk level · {displayedRiskLevel}
                  {typeof displayedRiskScore === 'number' && Number.isFinite(displayedRiskScore) ? (
                    <span className="ml-1.5 font-mono normal-case opacity-95">{Math.round(displayedRiskScore)}/100</span>
                  ) : null}
                </span>
              </div>
            ) : null}
            <div className="break-all font-mono text-caption text-muted sm:break-normal sm:text-body">
              <span className="inline-block">{pod.namespace}</span>
              <span className="mx-2 opacity-50" aria-hidden>
                |
              </span>
              <span className="inline-block">
                {pod.nodeName && pod.clusterId ? (
                  <button
                    type="button"
                    className="text-left text-brand hover:underline"
                    onClick={() => navigate(`/clusters/${pod.clusterId}/nodes/${encodeURIComponent(pod.nodeName!)}`)}
                  >
                    {pod.nodeName}
                  </button>
                ) : (
                  pod.nodeName ?? '—'
                )}
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Hint when pod IP / start time are missing (filled by agent sync; wait for next sync or restart agent) */}
      {(!pod.podIP || !pod.startTime) && (
        <p className="text-muted text-caption mb-2">
          Pod IP and Start time come from agent sync. If empty, wait for the next sync (~2 min) or restart the agent: <code className="bg-surface-2 px-1 rounded">kubectl rollout restart daemonset/fortuna-agent -n fortuna</code>
        </p>
      )}
      {dataErrors.length > 0 && (
        <div className="mb-3 p-2.5 rounded-lg border border-amber-700/50 bg-amber-950/30 flex items-center gap-2 text-caption text-amber-300">
          <AlertTriangle className="w-4 h-4 shrink-0" />
          <span>
            Failed to load: {dataErrors.join(', ')}.{' '}
            <button
              type="button"
              className="underline hover:text-text"
              onClick={() => {
                if (!pod?.uid) return;
                refreshAllData(pod.uid);
                void loadSbomForPod({
                  uid: pod.uid,
                  name: pod.name,
                  namespace: pod.namespace,
                })
                  .then((d) => {
                    setSbom(d ?? null);
                    setSbomLoaded(true);
                    setDataErrors((p) => p.filter((e) => e !== 'sbom'));
                  })
                  .catch(() => {
                    setSbom(null);
                    setSbomLoaded(true);
                    setDataErrors((p) => (p.includes('sbom') ? p : [...p, 'sbom']));
                  });
                void api
                  .getPodRiskReport(pod.uid)
                  .then((report) => {
                    applyPodRiskReport(report, pod);
                    setDataErrors((p) => p.filter((e) => e !== 'risk-report'));
                  })
                  .catch(() => {
                    setRelatedRisks([]);
                    setPodRiskReportSummary(null);
                    setDataErrors((p) => (p.includes('risk-report') ? p : [...p, 'risk-report']));
                  });
              }}
            >
              Retry
            </button>
          </span>
        </div>
      )}
      <div className="mb-6 grid min-w-0 grid-cols-1 overflow-hidden rounded-lg border border-border bg-surface/35 min-[420px]:grid-cols-2 md:grid-cols-3 xl:grid-cols-6">
        <div className="min-w-0 border-b border-border/70 p-3 min-[420px]:border-r md:border-r xl:border-r">
          <p className="ui-micro-label mb-1">Status</p>
          <span className={`inline-flex items-center px-2 py-0.5 rounded text-caption font-medium border ${getPodStatusBadgeClass(pod.status ?? pod.phase)}`}>{pod.status ?? pod.phase ?? '—'}</span>
        </div>
        <div className="min-w-0 border-b border-border/70 p-3 md:border-r xl:border-r">
          <p className="ui-micro-label mb-1">Pod IP</p>
          <p className="text-body font-medium text-text font-mono">{pod.podIP ?? '—'}</p>
        </div>
        <div className="min-w-0 border-b border-border/70 p-3 min-[420px]:border-r md:border-r xl:border-r">
          <p className="ui-micro-label mb-1">Start Time</p>
          <p className="text-body font-medium text-text">{pod.startTime ? formatDateTime(pod.startTime) : (runtimeMetrics.length > 0 && runtimeMetrics[0].lastObservedAt ? `Last reported: ${formatDateTime(runtimeMetrics[0].lastObservedAt)}` : '—')}</p>
        </div>
        <div className="min-w-0 border-b border-border/70 p-3 md:border-r xl:border-r">
          <p className="ui-micro-label mb-1">Uptime</p>
          <p className="text-body font-medium text-text">{formatUptime(pod.startTime ?? undefined)}</p>
        </div>
        <div className="min-w-0 border-b border-border/70 p-3 min-[420px]:border-r md:border-r xl:border-r">
          <p className="ui-micro-label mb-1">Restart Count</p>
          <p className="text-lg font-bold text-text">{pod.restartCount ?? 0}</p>
        </div>
        <div className="min-w-0 border-b border-border/70 p-3 xl:border-r">
          <p className="ui-micro-label mb-1">QoS Class</p>
          <p className="text-body font-medium text-text">{pod.qosClass ?? '—'}</p>
        </div>
        <div className="min-w-0 border-b border-border/70 p-3 min-[420px]:border-r md:border-r xl:border-r xl:border-b-0">
          <p className="ui-micro-label mb-1">Risk Count</p>
          <p className="text-lg font-bold text-text">{pod.riskCount}</p>
        </div>
        <div className="min-w-0 border-b border-border/70 p-3 md:border-r md:border-b-0 xl:border-r">
          <p className="ui-micro-label mb-1">Service Account</p>
          {serviceAccountName ? (
            <div className="min-w-0 space-y-1">
              <p className="text-body font-medium text-text truncate" title={`${pod.namespace}/${serviceAccountName}`}>
                {serviceAccountName}
              </p>
              {serviceAccountRef?.uid ? (
                <button
                  type="button"
                  onClick={openServiceAccountIdentity}
                  className="inline-flex max-w-full items-center gap-1 text-caption font-medium text-brand hover:underline"
                  title={`Open ServiceAccount identity ${serviceAccountRef.uid}`}
                >
                  Open identity
                  <ExternalLink className="h-3 w-3 shrink-0" aria-hidden />
                </button>
              ) : (
                <p className="text-caption text-muted-2">
                  {serviceAccountLookupComplete ? 'Identity not synced' : 'Resolving identity...'}
                </p>
              )}
            </div>
          ) : (
            <p className="text-body font-medium text-text">—</p>
          )}
        </div>
        <div className="min-w-0 p-3 min-[420px]:border-r md:border-r-0">
          <p className="ui-micro-label mb-1">Created</p>
          <p className="text-body font-medium text-text">{pod.createdAt ? formatDateTime(pod.createdAt) : '—'}</p>
        </div>
      </div>

      <Tabs items={tabs} value={resolvedTab} onChange={(id) => setActiveTab(id as TabId)} />

      {resolvedTab === 'overview' && (
        <div className="space-y-6">
        {/* Phase 3.1: Unified Risk Summary card */}
        {unifiedScore && (
          <Card variant="secondary" className="border border-border">
            <h3 className="text-body font-semibold text-text mb-3 flex items-center gap-2">
              <Shield className="w-4 h-4 text-brand" />
              Unified Risk Score
              <span className="text-caption text-muted font-normal ml-1">V3 · {unifiedScore.scorerVersion}</span>
              {unifiedScore.toxicCombos && unifiedScore.toxicCombos.length > 0 && (
                <span className="ml-auto flex items-center gap-1 text-caption text-red-400 bg-red-500/10 border border-red-500/20 rounded-full px-2 py-0.5">
                  <Zap className="w-2.5 h-2.5" /> {unifiedScore.toxicCombos.length} toxic combo{unifiedScore.toxicCombos.length > 1 ? 's' : ''}
                </span>
              )}
            </h3>
            <div className="flex flex-wrap items-end gap-4 mb-4">
              <div className="min-w-0">
                <div className={`text-3xl font-bold font-mono ${getSeverityTextClass(unifiedScore.finalLevel ?? deriveUnifiedRiskLevelFromScore(unifiedScore.totalScore))}`}>
                  {Math.round(unifiedScore.totalScore)}<span className="text-muted font-normal text-caption ml-1">/100</span>
                </div>
                <p className="text-caption text-muted mt-1">
                  ADR band:{' '}
                  <span className="text-text font-medium uppercase">
                    {unifiedScore.finalLevel ?? deriveUnifiedRiskLevelFromScore(unifiedScore.totalScore) ?? '—'}
                  </span>
                </p>
              </div>
              {pod.riskSignals ? (
                <div className="flex flex-col gap-1.5 text-caption sm:border-l sm:border-border sm:pl-4">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-muted shrink-0">Risk level</span>
                    <span className={clsx('inline-flex px-2 py-0.5 rounded text-caption font-medium border uppercase', getSeverityBadgeClass(unifiedScore.finalLevel ?? deriveUnifiedRiskLevelFromScore(unifiedScore.totalScore) ?? 'low'))}>
                      {unifiedScore.finalLevel ?? deriveUnifiedRiskLevelFromScore(unifiedScore.totalScore) ?? '—'}
                    </span>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-muted shrink-0">Attack-path context</span>
                    <span className="font-mono font-medium text-text uppercase">{pod.riskSignals.maxImpact}</span>
                    {pod.riskSignals.hasAttackPath && (pod.riskSignals.pathCount ?? 0) > 0 ? (
                      <span className="text-muted">({pod.riskSignals.pathCount} path{pod.riskSignals.pathCount !== 1 ? 's' : ''})</span>
                    ) : null}
                  </div>
                  {pod.riskSignals.summary ? <p className="text-muted max-w-xl leading-snug">{pod.riskSignals.summary}</p> : null}
                </div>
              ) : null}
              <div className="text-caption text-muted ml-auto w-full sm:w-auto sm:text-right">calc: {unifiedScore.calculatedAt ? formatDateTime(unifiedScore.calculatedAt) : '—'}</div>
            </div>
            {/* 7-dimension breakdown */}
            {unifiedScore.dimensions && (
              <div className="grid grid-cols-1 gap-2 min-[420px]:grid-cols-2 md:grid-cols-4 xl:grid-cols-7">
                {[
                  { label: 'Vuln', key: 'vulnerability', max: 15, color: 'bg-red-500' },
                  { label: 'Cap', key: 'capabilityExposure', max: 15, color: 'bg-orange-500' },
                  { label: 'Path', key: 'attackPath', max: 15, color: 'bg-yellow-500' },
                  { label: 'RBAC', key: 'rbacPolicy', max: 15, color: 'bg-purple-500' },
                  { label: 'Runtime', key: 'runtimeThreat', max: 15, color: 'bg-cyan-500' },
                  { label: 'Exposure', key: 'exposure', max: 15, color: 'bg-blue-500' },
                  { label: 'Blast', key: 'blastRadius', max: 10, color: 'bg-brand' },
                ].map((dim) => {
                  const val = (unifiedScore.dimensions as any)[dim.key] ?? 0;
                  const pct = Math.min(100, (val / dim.max) * 100);
                  return (
                    <div key={dim.key} className="flex flex-col items-center gap-1">
                      <div className="w-full h-2 bg-surface-2 rounded-full overflow-hidden">
                        <div className={`h-full rounded-full ${dim.color}`} style={{ width: `${pct}%` }} />
                      </div>
                      <span className="text-caption text-muted">{dim.label}</span>
                      <span className="text-caption text-text font-mono">{val.toFixed(1)}</span>
                    </div>
                  );
                })}
              </div>
            )}
            {/* Toxic combos */}
            {unifiedScore.toxicCombos && unifiedScore.toxicCombos.length > 0 && (
              <div className="mt-3 pt-3 border-t border-border">
                <div className="text-caption text-muted mb-1">Active toxic combos:</div>
                <div className="flex flex-wrap gap-1">
                  {unifiedScore.toxicCombos.map((combo, i) => (
                    <span key={i} className="text-caption text-red-300 bg-red-900/30 border border-red-700/30 rounded px-2 py-0.5">{combo}</span>
                  ))}
                </div>
              </div>
            )}
          </Card>
        )}
        <Card variant="secondary">
            <h3 className="text-section-title text-text mb-4 flex items-center gap-2 flex-wrap">
              <Box className="w-5 h-5 text-brand" /> Overview
            </h3>
            {/* Container image info — GAP 10: missing from original */}
            {(sbom?.image || sbom?.container) && (
              <div className="mb-4 pb-4 border-b border-border">
                <h4 className="text-body font-semibold text-text mb-3">Container Image</h4>
                <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-2 text-body">
                  {sbom?.image && (
                    <div className="col-span-2">
                      <dt className="text-muted">Image</dt>
                      <dd className="text-text font-mono text-caption break-all">{sbom.image}</dd>
                    </div>
                  )}
                  {sbom?.container && (
                    <div>
                      <dt className="text-muted">Container</dt>
                      <dd className="text-text font-mono">{sbom.container}</dd>
                    </div>
                  )}
                  {sbom?.imageDigest && (
                    <div className="col-span-2">
                      <dt className="text-muted">Image Digest</dt>
                      <dd className="break-all font-mono text-caption text-text">{sbom.imageDigest}</dd>
                    </div>
                  )}
                  {sbom?.imageTrust && (
                    <div className="col-span-2">
                      <dt className="text-muted">Image Trust</dt>
                      <dd className="mt-1 flex flex-wrap items-center gap-2 text-caption">
                        <span className={clsx(
                          'rounded border px-2 py-0.5 font-medium',
                          sbom.imageTrust.status === 'strong'
                            ? 'border-emerald-500/35 bg-emerald-500/10 text-emerald-300'
                            : sbom.imageTrust.status === 'blocked'
                              ? 'border-red-500/40 bg-red-500/10 text-red-300'
                            : sbom.imageTrust.status === 'weak'
                              ? 'border-amber-500/35 bg-amber-500/10 text-amber-300'
                              : 'border-border bg-surface-2 text-muted',
                        )}>
                          {sbom.imageTrust.status}
                        </span>
                        <span className="rounded border border-border bg-base/60 px-2 py-0.5 font-mono text-muted">
                          {sbom.imageTrust.registryClass}: {sbom.imageTrust.registry}
                        </span>
                        {sbom.imageTrust.mutableTag ? <span className="text-amber-300">Mutable tag</span> : null}
                        {!sbom.imageTrust.digestAvailable ? <span className="text-amber-300">Digest missing</span> : null}
                      </dd>
                    </div>
                  )}
                </dl>
              </div>
            )}
            <dl className="grid grid-cols-1 gap-4 text-body sm:grid-cols-2">
              <div className="sm:col-span-2">
                <dt className="text-muted">UID</dt>
                <dd className="break-all font-mono text-caption text-text">{pod.uid}</dd>
              </div>
            </dl>
            {(pod.ownerKind ?? pod.ownerName ?? pod.replicaSetName ?? serviceAccountName) && (
              <div className="mt-4 pt-4 border-t border-border">
                <h4 className="text-body font-semibold text-text mb-3">Identity &amp; Ownership</h4>
                <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-2 text-body">
                  {serviceAccountName && (
                    <div className="sm:col-span-2">
                      <dt className="text-muted">ServiceAccount identity</dt>
                      <dd className="min-w-0">
                        <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                          <span className="font-mono text-text">{pod.namespace}/{serviceAccountName}</span>
                          {serviceAccountRef?.uid ? (
                            <button
                              type="button"
                              onClick={openServiceAccountIdentity}
                              className="inline-flex items-center gap-1 text-caption font-medium text-brand hover:underline"
                            >
                              Open identity
                              <ExternalLink className="h-3 w-3" aria-hidden />
                            </button>
                          ) : (
                            <span className="text-caption text-muted-2">
                              {serviceAccountLookupComplete ? 'Identity row not synced yet' : 'Resolving identity...'}
                            </span>
                          )}
                        </div>
                        {serviceAccountRef?.uid ? (
                          <div className="mt-1 break-all font-mono text-caption text-muted-2">{serviceAccountRef.uid}</div>
                        ) : null}
                      </dd>
                    </div>
                  )}
                  {pod.ownerKind && (
                    <div>
                      <dt className="text-muted">Owner Type</dt>
                      <dd className="text-text font-medium">{pod.ownerKind}</dd>
                    </div>
                  )}
                  {pod.ownerName && (
                    <div>
                      <dt className="text-muted">Owner Name</dt>
                      <dd className="text-text font-mono">{pod.ownerName}</dd>
                    </div>
                  )}
                  {pod.replicaSetName && (
                    <div>
                      <dt className="text-muted">ReplicaSet</dt>
                      <dd className="text-text font-mono">{pod.replicaSetName}</dd>
                    </div>
                  )}
                </dl>
              </div>
            )}
            {sbom && (
              <div className="mt-4 pt-4 border-t border-border">
                <h4 className="text-body font-semibold text-text mb-3">Security</h4>
                <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-1 text-body mb-4">
                  <div>
                    <dt className="text-muted">Total Packages</dt>
                    <dd className="text-text font-medium">{sbom.components?.length ?? 0}</dd>
                  </div>
                  <div>
                    <dt className="text-muted">Vulnerable Packages</dt>
                    <dd className={(sbom.vulnerablePackageCount ?? 0) > 0 ? 'text-amber-400 font-medium' : 'text-muted'}>
                      {sbom.vulnerablePackageCount ?? 0}
                    </dd>
                  </div>
                </dl>
                {(sbom.vulnerabilitySummary && (() => {
                  const total = (sbom.vulnerabilitySummary!.critical ?? 0) + (sbom.vulnerabilitySummary!.high ?? 0) + (sbom.vulnerabilitySummary!.medium ?? 0) + (sbom.vulnerabilitySummary!.low ?? 0);
                  return total > 0;
                })()) && (
                  <div className="space-y-2">
                    {(['critical', 'high', 'medium', 'low'] as const).map((level) => {
                      const count = sbom.vulnerabilitySummary![level] ?? 0;
                      const total = (sbom.vulnerabilitySummary!.critical ?? 0) + (sbom.vulnerabilitySummary!.high ?? 0) + (sbom.vulnerabilitySummary!.medium ?? 0) + (sbom.vulnerabilitySummary!.low ?? 0);
                      const pct = total > 0 ? (count / total) * 100 : 0;
                      const label = level.charAt(0).toUpperCase() + level.slice(1);
                      return (
                        <div key={level} className="flex items-center gap-3">
                          <span className={clsx('text-caption font-medium w-20', getSeverityTextClass(level))}>
                            {label}
                          </span>
                          <div className="flex-1 h-5 bg-surface-2 rounded overflow-hidden min-w-[80px]">
                            <div
                              className={clsx('h-full rounded transition-all', getSeverityBarClass(level))}
                              style={{ width: `${Math.max(pct, pct > 0 ? 4 : 0)}%` }}
                            />
                          </div>
                          <span className="text-text text-caption tabular-nums w-8 font-medium">{count}</span>
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            )}
            {!sbom && sbomLoaded && (
              <div className="mt-4 pt-4 border-t border-border">
                <p className="text-muted text-body">No SBOM data available for this pod.</p>
              </div>
            )}
            {!sbom && !sbomLoaded && (
              <div className="mt-4 pt-4 border-t border-border">
                <p className="text-muted text-body">Security summary loading…</p>
              </div>
            )}
            {/* Phase 3.1: Capability list with state badge */}
            {podCapabilities.length > 0 && (
              <div className="mt-4 pt-4 border-t border-border">
                <h4 className="text-body font-semibold text-text mb-2 flex items-center gap-2 flex-wrap">
                  <Zap className="w-4 h-4 text-orange-400 shrink-0" /> Capabilities
                  <button
                    type="button"
                    onClick={() => navigate('/capabilities')}
                    className="ml-auto text-caption font-medium text-brand hover:text-brand inline-flex items-center gap-1"
                  >
                    View all <Target className="w-3.5 h-3.5" />
                  </button>
                </h4>
                <div className="space-y-1.5">
                  {podCapabilities.slice(0, 8).map((cap) => (
                    <div
                      key={cap.capabilityId}
                      className="flex flex-col gap-1.5 rounded-md border border-border/90 bg-surface/60 px-2.5 py-2 sm:flex-row sm:items-center sm:justify-between sm:gap-3"
                    >
                      <span className="font-mono text-caption text-text break-all min-w-0 leading-snug">{cap.capabilityId}</span>
                      <div className="flex shrink-0 items-center gap-2 flex-wrap sm:justify-end">
                        <span className={clsx('text-caption font-medium', getSeverityTextClass(cap.severity))}>
                          {capabilityImpactLabel(cap.severity)}
                        </span>
                        <span
                          className={clsx('text-caption px-2 py-0.5 rounded-full font-semibold uppercase tracking-wide', {
                            'text-muted bg-surface-2': cap.state === 'detected',
                            'text-yellow-400 bg-yellow-500/10': cap.state === 'confirmed',
                            'text-red-400 bg-red-500/10': cap.state === 'exploited' || cap.state === 'chained',
                          })}
                        >
                          {cap.state}
                        </span>
                      </div>
                    </div>
                  ))}
                  {podCapabilities.length > 8 && (
                    <p className="text-caption text-muted pl-2">+{podCapabilities.length - 8} more</p>
                  )}
                </div>
              </div>
            )}
            <div className="mt-4 pt-4 border-t border-border">
              <h4 className="text-body font-semibold text-text mb-2 flex items-center gap-2">
                <BarChart2 className="w-4 h-4" /> Pod detail (agent)
              </h4>
              {podRiskReportSummary && (
                <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 sm:gap-3 mb-3">
                  <div className="rounded-lg border border-border bg-base/60 p-3">
                    <div className="text-caption font-medium text-muted leading-snug">Runtime signals (24h, DB)</div>
                    <div className="mt-1 text-2xl font-semibold tabular-nums text-text">{podRiskReportSummary.runtimeSignals24h ?? 0}</div>
                  </div>
                  <div className="rounded-lg border border-border bg-base/60 p-3">
                    <div className="text-caption font-medium text-muted leading-snug">Insights on this pod (report)</div>
                    <div className="mt-1 text-2xl font-semibold tabular-nums text-text">{podRiskReportSummary.podDirectInsightCount ?? 0}</div>
                  </div>
                  {(podRiskReportSummary.runtimePolicyInsightCount ?? 0) > 0 && (
                    <div className="rounded-lg border border-amber-900/40 bg-amber-950/20 p-3 sm:col-span-2">
                      <div className="text-caption font-medium text-muted">Runtime / pod-security policy insights</div>
                      <div className="mt-1 text-xl font-semibold tabular-nums text-amber-200">{podRiskReportSummary.runtimePolicyInsightCount}</div>
                    </div>
                  )}
                </div>
              )}
              {(runtimeMetrics.length > 0 || processes.length > 0 || networkConnections.length > 0) ? (
                <div className="flex flex-wrap gap-3 text-body">
                  {runtimeMetrics.length > 0 && (
                    <button type="button" onClick={() => setActiveTab('metrics')} className="text-brand hover:underline">
                      {runtimeMetrics.length} runtime metric(s)
                    </button>
                  )}
                  {processes.length > 0 && (
                    <button type="button" onClick={() => setActiveTab('processes')} className="text-brand hover:underline">
                      {processes.length} process(es)
                    </button>
                  )}
                  {networkConnections.length > 0 && (
                    <button type="button" onClick={() => setActiveTab('network')} className="text-brand hover:underline">
                      {networkConnections.length} connection(s)
                    </button>
                  )}
                </div>
              ) : (
                <p className="text-muted text-body">Open the Runtime metrics, Processes, or Network tab to load data from the agent (or wait for live updates).</p>
              )}
              <div className="mt-3 flex flex-wrap gap-3 text-body">
                <button
                  type="button"
                  onClick={() => {
                    const ns = pod.namespace || '';
                    const name = pod.name || '';
                    const params = new URLSearchParams();
                    if (ns) params.set('resourceNamespace', ns);
                    if (name) params.set('search', name);
                    navigate(`/risks?${params.toString()}`);
                  }}
                  className="inline-flex items-center px-2.5 py-1.5 rounded bg-base border border-border text-text hover:border-brand"
                >
                  View risk insights in Risk Operations
                </button>
              </div>
            </div>
          </Card>
        </div>
      )}

      {resolvedTab === 'risk_sbom' && (
        <Card>
          <div className="flex flex-col gap-3 lg:flex-row lg:flex-wrap lg:items-start lg:justify-between mb-4">
            <div className="flex flex-col gap-2 min-w-0 flex-1 basis-[min(100%,18rem)]">
              <div className="flex items-center gap-2 flex-wrap">
                <h3 className="text-section-title text-text">Software Risk Evidence</h3>
              </div>
              <SbomMetaBadges
                sbomSource={sbom?.sbomSource}
                confidence={sbom?.confidence}
                goVersion={sbom?.goVersion}
              />
            </div>
            {sbom && (sbom.components?.length ?? 0) > 0 && (
              <div className="flex flex-wrap items-center justify-end gap-2 shrink-0 w-full lg:w-auto lg:max-w-full">
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => exportSbomAsCsv(sbom)}
                  title="Download SBOM as CSV"
                  className="shrink-0"
                >
                  <Download className="w-4 h-4 mr-2 shrink-0" />
                  <span className="hidden sm:inline">Export </span>CSV
                </Button>
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => exportSbomAsJson(sbom)}
                  title="Download SBOM as JSON"
                  className="shrink-0"
                >
                  <Download className="w-4 h-4 mr-2 shrink-0" />
                  <span className="hidden sm:inline">Export </span>JSON
                </Button>
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => exportSbomAsSpdxJson(sbom)}
                  title="Download SBOM as SPDX JSON"
                  className="shrink-0"
                >
                  <Download className="w-4 h-4 mr-2 shrink-0" />
                  SPDX
                </Button>
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => exportSbomAsCycloneDxJson(sbom)}
                  title="Download SBOM as CycloneDX JSON"
                  className="shrink-0"
                >
                  <Download className="w-4 h-4 mr-2 shrink-0" />
                  CycloneDX
                </Button>
              </div>
            )}
          </div>
          {tabLoading ? (
            <p className="text-muted text-body">Loading...</p>
          ) : sbom ? (
            <div className="space-y-4">
              <p className="text-muted text-body">
                {sbom.podName} · {sbom.namespace} · {sbom.components?.length ?? 0} packages
                {sbom.vulnerablePackageCount != null && sbom.vulnerablePackageCount > 0 && (
                  <> · {sbom.vulnerablePackageCount} vulnerable</>
                )}
              </p>
              {(() => {
                const comps = (sbom.components || []) as SbomComponentType[];
                const severityOpts = ['all', 'critical', 'high', 'medium', 'low'] as const;
                const statusOpts = ['all', 'active', 'allowed', 'fixed'] as const;
                const filtered = comps
                  .filter((c) => {
                    if (
                      sbomOnlyVulnerable &&
                      (c.cveCount ?? c.vulnerabilities?.length ?? 0) === 0 &&
                      !c.malwareMatch
                    )
                      return false;
                    const maxSev = (c.maxSeverity ?? '').toLowerCase();
                    if (sbomSeverityFilter !== 'all' && maxSev !== sbomSeverityFilter) return false;
                    const status = (c.status ?? 'active').toLowerCase();
                    if (sbomStatusFilter !== 'all' && status !== sbomStatusFilter) return false;
                    if (sbomSearch.trim()) {
                      const q = sbomSearch.trim().toLowerCase();
                      const name = (c.name ?? '').toLowerCase();
                      const version = (c.version ?? '').toLowerCase();
                      if (!name.includes(q) && !version.includes(q)) return false;
                    }
                    return true;
                  })
                  .sort((a, b) => {
                    if (sbomSort === 'none') return 0;
                    if (sbomSort === 'name') {
                      return (a.name ?? '').localeCompare(b.name ?? '');
                    }
                    if (sbomSort === 'severity') {
                      const order: Record<string, number> = { critical: 4, high: 3, medium: 2, low: 1, '': 0 };
                      const sa = order[(a.maxSeverity ?? '').toLowerCase()] ?? 0;
                      const sb = order[(b.maxSeverity ?? '').toLowerCase()] ?? 0;
                      return sb - sa;
                    }
                    if (sbomSort === 'cve') {
                      const ca = a.cveCount ?? a.vulnerabilities?.length ?? 0;
                      const cb = b.cveCount ?? b.vulnerabilities?.length ?? 0;
                      return cb - ca;
                    }
                    return 0;
                  });
                return (
                  <>
                    <div className="rounded-lg border border-border bg-base/35 p-3 space-y-3">
                      <div className="flex flex-col gap-3 lg:flex-row lg:flex-wrap lg:items-end lg:gap-x-6">
                        <div className="min-w-0 flex-1 space-y-1.5">
                          <span className="text-caption font-semibold uppercase tracking-wide text-muted">Severity</span>
                          <div className="flex flex-wrap gap-1">
                            {severityOpts.map((s) => (
                              <button
                                key={s}
                                type="button"
                                onClick={() => setSbomSeverityFilter(s)}
                                className={clsx(
                                  'px-2.5 py-1.5 rounded-md text-caption font-medium capitalize border border-transparent',
                                  sbomSeverityFilter === s ? UI_PILL_ACTIVE_BORDERED : UI_PILL_IDLE_FILTER
                                )}
                              >
                                {s}
                              </button>
                            ))}
                          </div>
                        </div>
                        <div className="min-w-0 flex-1 space-y-1.5">
                          <span className="text-caption font-semibold uppercase tracking-wide text-muted">Status</span>
                          <div className="flex flex-wrap gap-1">
                            {statusOpts.map((s) => (
                              <button
                                key={s}
                                type="button"
                                onClick={() => setSbomStatusFilter(s)}
                                className={clsx(
                                  'px-2.5 py-1.5 rounded-md text-caption font-medium capitalize border border-transparent',
                                  sbomStatusFilter === s ? UI_PILL_ACTIVE_BORDERED : UI_PILL_IDLE_FILTER
                                )}
                              >
                                {s}
                              </button>
                            ))}
                          </div>
                        </div>
                      </div>
                      <div className="flex flex-col gap-3 border-t border-border/80 pt-3 sm:flex-row sm:flex-wrap sm:items-center">
                        <label className="flex items-center gap-2 text-body text-text cursor-pointer min-w-0 shrink-0">
                          <input
                            type="checkbox"
                            checked={sbomOnlyVulnerable}
                            onChange={(e) => setSbomOnlyVulnerable(e.target.checked)}
                            className="rounded border-border bg-surface-2 text-brand shrink-0"
                          />
                          <span className="leading-snug">Vulnerable / malware only</span>
                        </label>
                        <div className="flex min-w-0 flex-1 flex-col gap-1 sm:max-w-xs">
                          <span className="text-caption font-semibold uppercase tracking-wide text-muted">Search</span>
                          <input
                            type="text"
                            value={sbomSearch}
                            onChange={(e) => setSbomSearch(e.target.value)}
                            placeholder="Package or version…"
                            className="bg-base border border-border rounded-md px-2.5 py-2 text-body text-text w-full placeholder:text-muted-2"
                          />
                        </div>
                        <div className="flex flex-wrap items-center gap-1.5">
                          <span className="text-caption font-semibold uppercase tracking-wide text-muted mr-1">Sort</span>
                          <button
                            type="button"
                            onClick={() => setSbomSort(sbomSort === 'name' ? 'none' : 'name')}
                            className={clsx(
                              'px-2 py-1 rounded text-caption font-medium',
                              sbomSort === 'name' ? UI_PILL_ACTIVE : UI_PILL_IDLE_COMPACT
                            )}
                          >
                            Name
                          </button>
                          <button
                            type="button"
                            onClick={() => setSbomSort(sbomSort === 'severity' ? 'none' : 'severity')}
                            className={clsx(
                              'px-2 py-1 rounded text-caption font-medium',
                              sbomSort === 'severity' ? UI_PILL_ACTIVE : UI_PILL_IDLE_COMPACT
                            )}
                          >
                            Severity
                          </button>
                          <button
                            type="button"
                            onClick={() => setSbomSort(sbomSort === 'cve' ? 'none' : 'cve')}
                            className={clsx(
                              'px-2 py-1 rounded text-caption font-medium',
                              sbomSort === 'cve' ? UI_PILL_ACTIVE : UI_PILL_IDLE_COMPACT
                            )}
                          >
                            CVEs
                          </button>
                        </div>
                      </div>
                    </div>
                    <div className="rounded-lg border border-border bg-base/30 overflow-hidden -mx-1 sm:mx-0">
                      <div className="ui-table-scroll-compact max-h-[min(70dvh,36rem)]">
                        <table className={`${UI_TABLE} min-w-[1100px]`}>
                          <thead className={UI_THEAD_STICKY}>
                            <tr>
                              <th className={`${UI_TH_COMPACT} text-center !px-1 w-9`} aria-hidden />
                              <th className={UI_TH_COMPACT}>Package</th>
                              <th className={UI_TH_COMPACT}>Malware</th>
                              <th className={UI_TH_COMPACT}>Version</th>
                              <th className={UI_TH_COMPACT}>Type</th>
                              <th className={UI_TH_COMPACT}>License</th>
                              <th className={`${UI_TH_COMPACT} text-right`}>CVE</th>
                              <th className={UI_TH_COMPACT}>Severity</th>
                              <th className={`${UI_TH_COMPACT} text-right`}>CVSS</th>
                              <th className={UI_TH_COMPACT}>Fix</th>
                              <th className={UI_TH_COMPACT}>Status</th>
                              <th className={`${UI_TH_COMPACT} text-center`}>Exploit</th>
                              <th className={`${UI_TH_COMPACT} text-center`}>Allow</th>
                            </tr>
                          </thead>
                        <tbody>
                          {filtered.map((c, i) => {
                            const rowId = `${c.name}@${c.version ?? i}`;
                            const vulns = c.vulnerabilities ?? [];
                            const cveCount = c.cveCount ?? vulns.length;
                            const isExpanded = sbomExpandedId === rowId;
                            const hasCves = cveCount > 0;
                            const mm = c.malwareMatch;
                            const hasMalware = !!mm;
                            const rowExpandable = hasCves;
                            return (
                              <React.Fragment key={rowId}>
                                <tr
                                  className={clsx(
                                    UI_TR,
                                    hasMalware && 'bg-red-950/15',
                                    rowExpandable ? 'cursor-pointer' : ''
                                  )}
                                  onClick={() => rowExpandable && setSbomExpandedId(isExpanded ? null : rowId)}
                                >
                                  <td className={`${UI_TD_COMPACT_TIGHT} w-9 text-center align-middle !px-1`}>
                                    {hasCves ? (
                                      isExpanded ? (
                                        <ChevronDown className="w-4 h-4 text-muted mx-auto" />
                                      ) : (
                                        <ChevronRight className="w-4 h-4 text-muted mx-auto" />
                                      )
                                    ) : (
                                      <span className="w-4 inline-block" />
                                    )}
                                  </td>
                                  <td
                                    className={`${UI_TD_COMPACT_TIGHT} align-middle font-mono text-text max-w-[14rem] truncate`}
                                    title={c.name ?? undefined}
                                  >
                                    {c.name}
                                  </td>
                                  <td className={`${UI_TD_COMPACT_TIGHT} align-middle max-w-[10rem]`}>
                                    {mm ? (
                                      <span
                                        className="inline-flex items-center gap-1 max-w-full px-1.5 py-0.5 rounded text-caption font-semibold border bg-red-950/50 text-red-200 border-red-700/60"
                                        title={mm.malwareFamily ? `Family: ${mm.malwareFamily}` : mm.reason}
                                      >
                                        <AlertTriangle className="w-3 h-3 shrink-0" />
                                        <span className="truncate">{mm.reason}</span>
                                      </span>
                                    ) : (
                                      <span className="text-muted-2">—</span>
                                    )}
                                  </td>
                                  <td
                                    className={`${UI_TD_COMPACT_TIGHT} align-middle text-muted font-mono max-w-[7rem] truncate whitespace-nowrap`}
                                    title={c.version ?? undefined}
                                  >
                                    {c.version ?? '—'}
                                  </td>
                                  <td className={`${UI_TD_COMPACT_TIGHT} align-middle text-muted font-mono whitespace-nowrap`}>
                                    {sbomTypeLabel(c.type)}
                                  </td>
                                  <td className={`${UI_TD_COMPACT_TIGHT} align-middle text-muted max-w-[6rem] truncate`} title={c.license ?? undefined}>
                                    {c.license ?? '—'}
                                  </td>
                                  <td className={`${UI_TD_COMPACT_TIGHT} align-middle text-right tabular-nums whitespace-nowrap`}>
                                    {cveCount > 0 ? (
                                      <span className="text-amber-400 font-medium">{cveCount}</span>
                                    ) : (
                                      <span className="text-muted">0</span>
                                    )}
                                  </td>
                                  <td className={`${UI_TD_COMPACT_TIGHT} align-middle whitespace-nowrap`}>
                                    {c.maxSeverity ? (
                                      <span className={clsx('inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-caption font-medium border', getSeverityBadgeClass(c.maxSeverity))}>
                                        <span>{getSeverityIcon(c.maxSeverity)}</span>
                                        <span className="capitalize">{c.maxSeverity}</span>
                                      </span>
                                    ) : (
                                      <span className="text-muted">—</span>
                                    )}
                                  </td>
                                  <td className={`${UI_TD_COMPACT_TIGHT} align-middle text-right tabular-nums text-muted whitespace-nowrap`}>
                                    {c.maxCvss != null ? c.maxCvss : '—'}
                                  </td>
                                  <td
                                    className={`${UI_TD_COMPACT_TIGHT} align-middle font-mono text-muted max-w-[6rem] truncate`}
                                    title={c.fixVersion ?? undefined}
                                  >
                                    {c.fixVersion ?? '—'}
                                  </td>
                                  <td className={`${UI_TD_COMPACT_TIGHT} align-middle whitespace-nowrap`}>
                                    {mm ? (
                                      <span className="px-1.5 py-0.5 rounded text-caption bg-red-900/40 text-red-200 border border-red-800/50">Threat</span>
                                    ) : cveCount === 0 ? (
                                      <span className="px-1.5 py-0.5 rounded text-caption bg-muted-2/60 text-muted">Clean</span>
                                    ) : c.status ? (
                                      <span className={clsx('px-1.5 py-0.5 rounded text-caption capitalize', statusBadgeClass(c.status))}>
                                        {c.status.toLowerCase() === 'active' ? 'Not Fixed' : c.status.toLowerCase() === 'not_exploitable' ? 'Not exploitable' : c.status}
                                      </span>
                                    ) : (
                                      <span className={clsx('px-1.5 py-0.5 rounded text-caption', statusBadgeClass('active'))}>Not Fixed</span>
                                    )}
                                  </td>
                                  <td className={`${UI_TD_COMPACT_TIGHT} align-middle text-center whitespace-nowrap`}>
                                    {vulns.some((v) => v.exploitKnown) ? (
                                      <span className="text-amber-400" title="Public exploit available">🔥</span>
                                    ) : vulns.some((v) => v.exploitMaturity && v.exploitMaturity.toLowerCase().includes('poc')) ? (
                                      <span className="text-amber-500" title="PoC available">⚠️</span>
                                    ) : (
                                      <span className="text-muted" title="No known exploit">—</span>
                                    )}
                                  </td>
                                  <td className={`${UI_TD_COMPACT_TIGHT} align-middle text-center whitespace-nowrap`}>
                                    {vulns.length === 0 ? (
                                      <span className="text-muted">—</span>
                                    ) : vulns.every((v) => v.allowed) ? (
                                      <span className="text-emerald-400" title="Allowed by policy">✅</span>
                                    ) : vulns.some((v) => v.allowed) ? (
                                      <span className="text-amber-400" title="Pending review">⏳</span>
                                    ) : (
                                      <span className="text-red-400" title="Not allowed">❌</span>
                                    )}
                                  </td>
                                </tr>
                                {isExpanded && vulns.length > 0 && (
                                  <tr className="bg-surface-2/40">
                                    <td colSpan={13} className="py-3 px-4">
                                      <div className="pl-6 space-y-2 text-body">
                                        {vulns.map((v) => {
                                          const vStatus = (v.status ?? 'active').toLowerCase();
                                          const statusLabel = vStatus === 'active' ? 'Active' : vStatus === 'not_exploitable' ? 'Not exploitable' : (v.status ?? 'Active');
                                          return (
                                            <div
                                              key={v.id}
                                              className="flex flex-wrap items-center gap-3 py-1.5 border-b border-border/50 last:border-0"
                                            >
                                              <button
                                                type="button"
                                                onClick={() => setSelectedVulnerability(v)}
                                                className="font-mono text-text hover:text-brand underline cursor-pointer text-left"
                                              >
                                                {v.id}
                                              </button>
                                              <span className={clsx('inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-caption font-medium border', getSeverityBadgeClass(v.severity))}>
                                                {getSeverityIcon(v.severity)} <span className="capitalize">{v.severity}</span>
                                              </span>
                                              <span className="text-muted tabular-nums">{v.cvssScore ?? '—'}</span>
                                              <span className="text-muted">
                                                Fix: {v.fixedVersion ?? '—'}
                                              </span>
                                              <span className={clsx('px-1.5 py-0.5 rounded text-caption capitalize', statusBadgeClass(v.status))}>
                                                {statusLabel}
                                              </span>
                                              {v.exploitKnown && <span className="text-amber-400" title="Exploit known">🔥</span>}
                                              {v.allowed && <span className="text-emerald-400">✅ Allowed</span>}
                                            </div>
                                          );
                                        })}
                                      </div>
                                    </td>
                                  </tr>
                                )}
                              </React.Fragment>
                            );
                          })}
                        </tbody>
                      </table>
                    </div>
                    </div>
                    {filtered.length === 0 && (
                      <p className="text-muted text-body py-4">No packages match the current filters.</p>
                    )}
                  </>
                );
              })()}
            </div>
          ) : (
            <p className="text-muted text-body">No software risk evidence for this pod.</p>
          )}
        </Card>
      )}

      {/* Risk Insights section — shown within the consolidated Risk & SBOM tab */}
      {resolvedTab === 'risk_sbom' && (
        <Card>
          <div className="space-y-4 min-w-0">
          <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between min-w-0">
            <h3 className="text-card-title text-text">Risk insights</h3>
            {relatedRisks.length > 0 && (
              <span className="text-caption text-muted shrink-0">
                {relatedRisks.length} finding{relatedRisks.length !== 1 ? 's' : ''} for this pod
              </span>
            )}
          </div>
          {tabLoading ? (
            <p className="text-muted text-body">Loading...</p>
          ) : relatedRisks.length > 0 ? (
            <div className="space-y-2 min-w-0">
              {relatedRisks.map((risk) => (
                <div
                  key={risk.id}
                  className="p-3 rounded-lg border border-border bg-surface/50 hover:border-brand/30 cursor-pointer flex flex-col gap-1"
                  onClick={() => navigate(`/risks/${risk.id}`)}
                >
                  <div className="flex items-center justify-between gap-3">
                    <span className="font-medium text-text text-body line-clamp-1">{risk.title}</span>
                    {risk.finalLevel ? (
                      <span className={clsx('px-2 py-0.5 rounded text-caption font-medium uppercase', getSeverityBadgeClass(risk.finalLevel))}>
                        {risk.finalLevel}
                      </span>
                    ) : (
                      <span className="px-2 py-0.5 rounded text-caption font-medium text-muted border border-border bg-surface-2/60">
                        No score
                      </span>
                    )}
                  </div>
                  {(formatRiskFindingReference(risk) || risk.insightType) && (
                    <p className="text-muted text-caption font-mono mt-0.5">
                      {insightTypeUiLabel(risk.insightType)}
                      {formatRiskFindingReference(risk) ? ` · ${formatRiskFindingReference(risk)}` : ''}
                      {!risk.finalLevel && (risk.severityHint ?? risk.severity) ? ` · severity hint: ${risk.severityHint ?? risk.severity}` : ''}
                    </p>
                  )}
                  {risk.description && <p className="text-muted text-caption mt-0.5 line-clamp-2">{risk.description}</p>}
                </div>
              ))}
            </div>
          ) : (
            <p className="text-muted text-body">No risk insights for this pod.</p>
          )}
          </div>
        </Card>
      )}

      {resolvedTab === 'runtime' && (
        <Card>
          <h3 className="text-section-title text-text mb-4 flex items-center gap-2 flex-wrap">
            <BarChart2 className="w-5 h-5 text-brand shrink-0" /> Runtime metrics
          </h3>
          {tabLoading ? (
            <p className="text-muted text-body">Loading...</p>
          ) : runtimeMetrics.length > 0 ? (
            <div className="rounded-lg border border-border bg-base/30 overflow-hidden -mx-1 sm:mx-0">
              <div className="ui-table-scroll max-h-[min(70dvh,36rem)]">
              <table className={`${UI_TABLE} min-w-[720px]`}>
                <thead className={UI_THEAD_STICKY}>
                  <tr>
                    <th className={UI_TH_COMPACT}>Container</th>
                    <th className={`${UI_TH_COMPACT} text-right`}>CPU (m)</th>
                    <th className={UI_TH_COMPACT}>Memory</th>
                    <th className={UI_TH_COMPACT}>Limit</th>
                    <th className={`${UI_TH_COMPACT} text-right`}>Restarts</th>
                    <th className={UI_TH_COMPACT}>State</th>
                    <th className={UI_TH_COMPACT}>Last observed</th>
                  </tr>
                </thead>
                <tbody>
                  {runtimeMetrics.map((m, i) => (
                    <tr key={m.id ?? i} className={UI_TR}>
                      <td className={`${UI_TD_COMPACT_TIGHT} font-mono text-text max-w-[10rem] truncate align-middle`} title={m.containerName ?? undefined}>{m.containerName ?? '—'}</td>
                      <td className={`${UI_TD_COMPACT_TIGHT} tabular-nums text-right align-middle whitespace-nowrap`}>{m.cpuUsageMillicore != null ? m.cpuUsageMillicore : '—'}</td>
                      <td className={`${UI_TD_COMPACT_TIGHT} tabular-nums font-mono text-muted align-middle whitespace-nowrap`}>
                        {m.memoryUsageBytes != null && m.memoryUsageBytes > 0
                          ? (m.memoryUsageBytes >= 1024 * 1024 ? `${(m.memoryUsageBytes / 1024 / 1024).toFixed(1)} MB` : `${(m.memoryUsageBytes / 1024).toFixed(1)} KB`)
                          : '—'}
                      </td>
                      <td className={`${UI_TD_COMPACT_TIGHT} tabular-nums font-mono text-muted align-middle whitespace-nowrap`}>
                        {m.memoryLimitBytes != null && m.memoryLimitBytes > 0
                          ? (m.memoryLimitBytes >= 1024 * 1024 ? `${(m.memoryLimitBytes / 1024 / 1024).toFixed(1)} MB` : `${(m.memoryLimitBytes / 1024).toFixed(1)} KB`)
                          : '—'}
                      </td>
                      <td className={`${UI_TD_COMPACT_TIGHT} tabular-nums text-right align-middle`}>{m.restartCount ?? 0}</td>
                      <td className={`${UI_TD_COMPACT_TIGHT} align-middle whitespace-nowrap`}>
                        <span className={clsx('px-2 py-0.5 rounded text-caption', m.state === 'Running' ? 'bg-emerald-600/80 text-white' : 'bg-muted-2 text-text')}>
                          {m.state ?? '—'}
                        </span>
                      </td>
                      <td className={`${UI_TD_COMPACT_TIGHT} text-muted whitespace-nowrap align-middle`}>{m.lastObservedAt ? formatDateTime(m.lastObservedAt) : '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              </div>
            </div>
          ) : (
            <PageEmpty title="No runtime metrics" description="Per-container CPU/memory metrics are reported by the agent. Ensure the agent is running on the pod's node." className="py-6" />
          )}
        </Card>
      )}

      {resolvedTab === 'runtime' && (
        <Card>
          <h3 className="text-section-title text-text mb-4 flex items-center gap-2 flex-wrap">
            <Cpu className="w-5 h-5 text-brand shrink-0" /> Processes
            {processes.length > 0 && (
              <span className="text-caption font-normal text-muted px-2 py-0.5 rounded bg-surface-2 max-w-full truncate">
                Runtime: {processes[0]?.runtimeSource === 'host' ? 'Host Inspection' : 'Container Exec'}
              </span>
            )}
          </h3>
          {tabLoading ? (
            <p className="text-muted text-body">Loading...</p>
          ) : processes.length > 0 ? (
            <div className="rounded-lg border border-border bg-base/30 overflow-hidden -mx-1 sm:mx-0">
              <div className="ui-table-scroll max-h-[min(70dvh,36rem)]">
              <table className={`${UI_TABLE} min-w-[960px]`}>
                <thead className={UI_THEAD_STICKY}>
                  <tr>
                    <th className={`${UI_TH_COMPACT} text-right`}>PID</th>
                    <th className={UI_TH_COMPACT}>User</th>
                    <th className={UI_TH_COMPACT}>UID:GID</th>
                    <th className={`${UI_TH_COMPACT} text-right`}>CPU %</th>
                    <th className={`${UI_TH_COMPACT} text-right`}>Mem %</th>
                    <th className={UI_TH_COMPACT}>Command</th>
                    <th className={UI_TH_COMPACT}>CWD</th>
                    <th className={UI_TH_COMPACT}>CapEff</th>
                    <th className={UI_TH_COMPACT}>Start</th>
                  </tr>
                </thead>
                <tbody>
                  {processes.map((proc, i) => (
                    <tr key={proc.id ?? i} className={UI_TR}>
                      <td className={`${UI_TD_COMPACT_TIGHT} tabular-nums font-mono text-right align-middle whitespace-nowrap`}>{proc.pid}</td>
                      <td className={`${UI_TD_COMPACT_TIGHT} text-text align-middle max-w-[6rem] truncate`} title={proc.userName ?? undefined}>{proc.userName ?? '—'}</td>
                      <td className={`${UI_TD_COMPACT_TIGHT} font-mono text-text align-middle whitespace-nowrap`}>
                        {(proc.userId != null || proc.groupId != null)
                          ? `${proc.userId ?? '—'}:${proc.groupId ?? '—'}`
                          : '—'}
                      </td>
                      <td className={`${UI_TD_COMPACT_TIGHT} tabular-nums text-right align-middle`}>{proc.cpuPercent != null ? proc.cpuPercent.toFixed(1) : '—'}</td>
                      <td className={`${UI_TD_COMPACT_TIGHT} tabular-nums text-right align-middle`}>{proc.memoryPercent != null ? proc.memoryPercent.toFixed(1) : '—'}</td>
                      <td className={`${UI_TD_COMPACT_TIGHT} font-mono text-muted max-w-[14rem] truncate align-middle`} title={proc.command}>{proc.command ?? '—'}</td>
                      <td className={`${UI_TD_COMPACT_TIGHT} font-mono text-muted max-w-[10rem] truncate align-middle`} title={proc.workingDir}>{proc.workingDir ?? '—'}</td>
                      <td className={`${UI_TD_COMPACT_TIGHT} font-mono text-muted max-w-[8rem] truncate align-middle`} title={proc.capEff ?? undefined}>{proc.capEff ?? '—'}</td>
                      <td className={`${UI_TD_COMPACT_TIGHT} text-muted whitespace-nowrap align-middle`}>{proc.startedAt ? formatDateTime(proc.startedAt) : '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              </div>
            </div>
          ) : (
            <PageEmpty title="No process data" description="Process list is collected by the agent. Ensure the agent is running on the pod's node and Pod Detail collection is enabled." className="py-6" />
          )}
        </Card>
      )}

      {activeTab === 'network' && (
        <Card>
          <h3 className="text-section-title text-text mb-4 flex items-center gap-2 flex-wrap">
            <Network className="w-5 h-5 text-brand shrink-0" /> Network connections
            {networkConnections.length > 0 && (
              <span className="text-caption font-normal text-muted px-2 py-0.5 rounded bg-surface-2 max-w-full truncate">
                Runtime: {networkConnections[0]?.runtimeSource === 'host' ? 'Host Inspection' : 'Container Exec'}
              </span>
            )}
          </h3>

          {/* Sub-view toggle: Summary vs Raw connections */}
          {networkConnections.length > 0 && (
            <div className="flex gap-2 mb-4">
              <button
                type="button"
                onClick={() => setNetworkSubView('summary')}
                className={clsx('px-3 py-1 rounded-lg text-caption font-medium transition-colors', networkSubView === 'summary' ? UI_PILL_ACTIVE : UI_PILL_IDLE_SEGMENT)}
              >
                Summary & Top Destinations
              </button>
              <button
                type="button"
                onClick={() => setNetworkSubView('raw')}
                className={clsx('px-3 py-1 rounded-lg text-caption font-medium transition-colors', networkSubView === 'raw' ? UI_PILL_ACTIVE : UI_PILL_IDLE_SEGMENT)}
              >
                Raw connections ({networkConnections.length})
              </button>
            </div>
          )}

          {tabLoading ? (
            <p className="text-muted text-body">Loading...</p>
          ) : networkConnections.length > 0 ? (
            networkSubView === 'summary' ? (
              <PodNetworkSummary
                connections={networkConnections}
                topDestinations={networkTopDestinations}
                podIP={pod?.podIP}
                loading={tabLoading}
              />
            ) : (
            <div className="rounded-lg border border-border bg-base/30 overflow-hidden -mx-1 sm:mx-0">
              <div className="ui-table-scroll max-h-[min(70dvh,36rem)]">
              <table className={`${UI_TABLE} min-w-[900px]`}>
                <thead className={UI_THEAD_STICKY}>
                  <tr>
                    <th className={UI_TH_COMPACT}>Dir</th>
                    <th className={UI_TH_COMPACT}>Remote</th>
                    <th className={`${UI_TH_COMPACT} text-right`}>L.port</th>
                    <th className={UI_TH_COMPACT}>Proto</th>
                    <th className={UI_TH_COMPACT}>Status</th>
                    <th className={`${UI_TH_COMPACT} text-right`} title="Kernel transmit queue snapshot from /proc networking data">
                      Tx Q
                    </th>
                    <th className={`${UI_TH_COMPACT} text-right`} title="Kernel receive queue snapshot from /proc networking data">
                      Rx Q
                    </th>
                    <th className={UI_TH_COMPACT}>Time</th>
                  </tr>
                </thead>
                <tbody>
                  {networkConnections.map((conn, i) => {
                    const podIP = (pod?.podIP ?? '').trim();
                    const isListen = (conn.state ?? '').toUpperCase() === 'LISTEN';
                    const srcIsPod = podIP && (conn.sourceIp === podIP || conn.sourceIp === '0.0.0.0' || conn.sourceIp === '::');
                    const isOutbound = isListen ? false : srcIsPod;
                    const direction = isOutbound ? 'Outbound' : 'Inbound';
                    const remoteAddr = isOutbound ? `${conn.destIp ?? '—'}:${conn.destPort ?? 0}` : `${conn.sourceIp ?? '—'}:${conn.sourcePort ?? 0}`;
                    const localPort = isOutbound ? (conn.sourcePort ?? 0) : (conn.destPort ?? 0);
                    return (
                      <tr key={conn.id ?? i} className={UI_TR}>
                        <td className={`${UI_TD_COMPACT_TIGHT} align-middle whitespace-nowrap`}>
                          <span className={clsx('px-2 py-0.5 rounded text-caption', direction === 'Outbound' ? 'bg-sky-600/80 text-white' : 'bg-muted-2 text-text')}>
                            {direction === 'Outbound' ? 'Out' : 'In'}
                          </span>
                        </td>
                        <td className={`${UI_TD_COMPACT_TIGHT} font-mono max-w-[14rem] truncate align-middle`} title={remoteAddr}>{remoteAddr}</td>
                        <td className={`${UI_TD_COMPACT_TIGHT} tabular-nums text-right align-middle`}>{localPort || '—'}</td>
                        <td className={`${UI_TD_COMPACT_TIGHT} align-middle whitespace-nowrap`}>{conn.protocol ?? '—'}</td>
                        <td className={`${UI_TD_COMPACT_TIGHT} text-muted align-middle max-w-[6rem] truncate`} title={conn.state ?? undefined}>{conn.state ?? '—'}</td>
                        <td className={`${UI_TD_COMPACT_TIGHT} tabular-nums font-mono text-muted text-right align-middle`}>{conn.bytesSent ?? 0}</td>
                        <td className={`${UI_TD_COMPACT_TIGHT} tabular-nums font-mono text-muted text-right align-middle`}>{conn.bytesRecv ?? 0}</td>
                        <td className={`${UI_TD_COMPACT_TIGHT} text-muted whitespace-nowrap align-middle`}>{conn.observedAt ? formatDateTime(conn.observedAt) : '—'}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
              </div>
            </div>
            )
          ) : (
            <PageEmpty
              title="No network data"
              description="Network connections are collected by the agent. Enable network collection on the agent. For cluster-wide topology, use Dashboard → Network Activity → Topology graph."
              className="py-6"
            />
          )}
        </Card>
      )}

      {resolvedTab === 'spec' && (
        <Card>
          <h3 className="text-section-title text-text mb-4 flex items-center gap-2 flex-wrap">
            <FileCode className="w-5 h-5 text-brand shrink-0" /> Pod Specification (YAML)
          </h3>
          {tabLoading ? (
            <p className="text-muted text-body">Loading...</p>
          ) : (
            <>
              <div className="flex flex-wrap justify-end gap-2 mb-3">
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={async () => {
                    if (!pod?.uid) return;
                    try {
                      const blob = await api.getPodSpecYamlBlob(pod.uid);
                      const a = document.createElement('a');
                      a.href = URL.createObjectURL(blob);
                      a.download = `pod-${pod?.name ?? 'spec'}.yaml`;
                      a.click();
                      URL.revokeObjectURL(a.href);
                    } catch {
                      setDataErrors((p) => (p.includes('spec') ? p : [...p, 'spec']));
                    }
                  }}
                >
                  <Download className="w-4 h-4 mr-2" /> Download YAML
                </Button>
              </div>
              <pre className="ui-code-scroll p-3 sm:p-4 rounded-lg bg-surface border border-border text-caption sm:text-body font-mono text-text whitespace-pre-wrap break-words max-w-full overflow-x-auto">
                {specYaml || 'No spec data.'}
              </pre>
            </>
          )}
        </Card>
      )}

      {resolvedTab === 'runtime' && (
        <Card>
          <div className="mb-2 min-w-0">
            <h3 className="text-section-title text-text flex items-center gap-2 flex-wrap">
              <Activity className="w-5 h-5 text-brand shrink-0" /> Events
            </h3>
            <p className="text-caption text-muted mt-1">
              Kubernetes API events, deduplicated runtime signals, and raw security runtime events (e.g. Falco → Core ingest).
              For coverage breakdown, see the <button type="button" className="text-brand hover:underline" onClick={() => setActiveTab('coverage')}>Coverage</button> tab.
            </p>
          </div>

          <div className="mb-8 pb-6 border-b border-border">
            <h4 className="text-body font-semibold text-text mb-2 flex items-center gap-2">
              <Shield className="w-4 h-4 text-violet-400" /> Security runtime events
            </h4>
            <p className="text-caption text-muted mb-3">
              Stored in Core as <code className="text-muted">runtime_events</code> (source: Falco JSONL, eBPF, agent). Requires pod UID in the payload to persist.
            </p>
            <div className="flex flex-wrap items-center gap-2 mb-3">
              <Button variant={secRuntimeFilter === 'all' ? 'primary' : 'secondary'} size="sm" onClick={() => setSecRuntimeFilter('all')}>
                All sources
              </Button>
              <Button variant={secRuntimeFilter === 'falco' ? 'primary' : 'secondary'} size="sm" onClick={() => setSecRuntimeFilter('falco')}>
                Falco
              </Button>
              <Button variant={secRuntimeFilter === 'other' ? 'primary' : 'secondary'} size="sm" onClick={() => setSecRuntimeFilter('other')}>
                Other
              </Button>
            </div>
            {(() => {
              const filtered = runtimeSecurityEvents.filter((ev) => {
                const r = (ev.runtime || '').toLowerCase();
                if (secRuntimeFilter === 'all') return true;
                if (secRuntimeFilter === 'falco') return r === 'falco';
                return r !== 'falco';
              });
              if (filtered.length === 0) {
                return (
                  <PageEmpty
                    title="No security runtime events"
                    description="When Falco or other sensors POST to Core, events appear here. Install Falco (scripts/deploy/install-falco-fortuna.sh) and set FALCO_EVENTS_ENABLED=true on the agent."
                    className="py-6"
                  />
                );
              }
              return (
                <>
                <div className="space-y-2 max-h-[420px] overflow-y-auto pr-1">
                  {filtered.slice(0, 80).map((ev) => {
                    const src = runtimeSourceBadge(ev.runtime);
                    const sev = (ev.severity || '').toLowerCase();
                    const sevClass =
                      sev === 'critical'
                        ? 'bg-red-600/80 text-white border-red-500/50'
                        : sev === 'high'
                          ? 'bg-orange-600/80 text-white border-orange-500/50'
                          : sev === 'medium'
                            ? 'bg-amber-600/70 text-white border-amber-500/50'
                            : 'bg-muted-2/80 text-text border-border/50';
                    return (
                      <div key={ev.id} className="p-3 rounded-lg border border-border bg-surface/50 flex flex-col gap-1.5">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className={`px-2 py-0.5 rounded text-caption font-semibold border ${src.className}`}>{src.label}</span>
                          {ev.mitreTechnique && (
                            <span className="px-2 py-0.5 rounded text-caption font-mono bg-surface-2 border border-border text-amber-200/90">{ev.mitreTechnique}</span>
                          )}
                          {ev.severity && <span className={`px-2 py-0.5 rounded text-caption font-medium border ${sevClass}`}>{ev.severity}</span>}
                          <span className="text-caption text-muted ml-auto tabular-nums">{ev.createdAt ? formatDateTime(ev.createdAt) : '—'}</span>
                        </div>
                        <div className="flex flex-wrap gap-x-3 gap-y-0.5 text-caption">
                          {ev.signal && <span className="text-violet-200/95 font-medium truncate max-w-full">{ev.signal}</span>}
                          {ev.eventType && <span className="text-muted truncate">{ev.eventType}</span>}
                        </div>
                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-1 text-caption text-muted font-mono">
                          <span>
                            <span className="text-muted-2">syscall </span>
                            {ev.syscall || '—'}
                          </span>
                          <span className="truncate" title={ev.targetPath}>
                            <span className="text-muted-2">target </span>
                            {ev.targetPath || '—'}
                          </span>
                        </div>
                        {ev.capability ? (
                          <p className="text-caption text-muted">
                            capability <span className="text-text">{ev.capability}</span>
                          </p>
                        ) : null}
                      </div>
                    );
                  })}
                </div>
                {filtered.length > 80 && (
                  <p className="text-caption text-muted mt-2">Showing 80 of {filtered.length} events. Use the Coverage tab for full breakdown.</p>
                )}
                </>
              );
            })()}
          </div>

          <h3 className="text-base font-semibold text-text mb-4 flex items-center gap-2 flex-wrap min-w-0">
            <Activity className="w-5 h-5 text-cyan-400 shrink-0" /> Runtime signals &amp; Kubernetes events
          </h3>
          <div className="mb-6 min-w-0">
            {signalStats && (
              <div className="mb-3 flex flex-wrap items-center gap-2 text-caption text-muted">
                <span className="px-2 py-0.5 rounded bg-surface-2 border border-border max-w-full break-words">Network anomaly events (60m): {signalStats.emittedEvents}</span>
                <span className="px-2 py-0.5 rounded bg-surface-2 border border-border">keys: {signalStats.uniqueKeys}</span>
                <span className="px-2 py-0.5 rounded bg-surface-2 border border-border">max ratio: {Number(signalStats.maxRatio ?? 0).toFixed(2)}</span>
              </div>
            )}
            <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between mb-3 min-w-0">
              <h4 className="text-body font-semibold text-text min-w-0">Runtime signals (normalized, last 24h)</h4>
              <div className="flex flex-wrap items-center gap-2 shrink-0">
                <Button
                  variant={runtimeSignalFilter === 'all' ? 'primary' : 'secondary'}
                  size="sm"
                  onClick={() => setRuntimeSignalFilter('all')}
                >
                  All
                </Button>
                <Button
                  variant={runtimeSignalFilter === 'NETWORK_QUEUE_ANOMALY' ? 'primary' : 'secondary'}
                  size="sm"
                  onClick={() => setRuntimeSignalFilter('NETWORK_QUEUE_ANOMALY')}
                >
                  Network queue anomaly
                </Button>
              </div>
            </div>
            {(() => {
              const filteredSignals = runtimeSignals.filter((s) =>
                runtimeSignalFilter === 'all' ? true : s.signalType === runtimeSignalFilter
              );
              if (filteredSignals.length === 0) {
                return <p className="text-muted text-body">No runtime signals.</p>;
              }
              return (
                <>
                <div className="space-y-2">
                  {filteredSignals.slice(0, 20).map((s) => {
                    const ev = (s.evidence ?? {}) as any;
                    const evSyscall = typeof ev?.syscall === 'string' ? ev.syscall : '';
                    const evTarget = typeof ev?.target === 'string' ? ev.target : '';
                    return (
                      <div
                        key={s.id}
                        className="p-3 rounded-lg border border-border bg-surface/50 flex items-center justify-between gap-3"
                      >
                        <div className="min-w-0">
                          <div className="flex items-center gap-2">
                            <span className={`px-2 py-0.5 rounded text-caption font-semibold border ${runtimeSignalVisual(s.signalType).signalClass}`}>
                              {s.signalType}
                            </span>
                            <span className={`px-2 py-0.5 rounded text-caption font-medium border ${runtimeSignalVisual(s.signalType).severityClass}`}>
                              {runtimeSignalVisual(s.signalType).severity}
                            </span>
                          </div>
                          <p className="text-caption text-muted">
                            {s.category} · confidence {Number(s.confidence ?? 0).toFixed(2)}
                          </p>
                          <p className="text-caption text-muted mt-1">
                            evidence: syscall <span className="text-text">{evSyscall || '—'}</span> · target{' '}
                            <span className="text-text break-all">{evTarget || '—'}</span>
                          </p>
                        </div>
                        <span className="text-caption text-muted whitespace-nowrap">{s.createdAt ? formatDateTime(s.createdAt) : '—'}</span>
                      </div>
                    );
                  })}
                </div>
                {filteredSignals.length > 20 && (
                  <p className="text-caption text-muted mt-2">Showing 20 of {filteredSignals.length} signals.</p>
                )}
                </>
              );
            })()}
          </div>
          <Card variant="panel" className="mb-6 bg-surface/30 border-border min-w-0">
            <p className="text-caption text-muted">
              Facts, incidents, capabilities and insights are split into dedicated views to reduce noise:
              <span className="text-text"> Runtime Timeline</span>, <span className="text-text">Coverage</span>, and
              <span className="text-text"> Risk Insights</span>.
            </p>
          </Card>
          {tabLoading ? (
            <p className="text-muted text-body">Loading...</p>
          ) : podEvents.length > 0 ? (
            <div className="space-y-2">
              {podEvents.map((ev, i) => (
                <div key={ev.id ?? i} className="p-3 rounded-lg border border-border bg-surface/50 flex flex-col gap-1">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className={clsx('px-2 py-0.5 rounded text-caption font-medium', ev.eventType === 'Warning' ? 'bg-amber-600/80 text-white' : 'bg-muted-2 text-text')}>
                      {ev.eventType ?? 'Normal'}
                    </span>
                    <span className="font-medium text-text">{ev.reason ?? '—'}</span>
                    {ev.lastTimestamp && <span className="text-muted text-caption">{formatDateTime(ev.lastTimestamp)}</span>}
                  </div>
                  {ev.message && <p className="text-muted text-body">{ev.message}</p>}
                  {ev.involvedName && <p className="text-muted text-caption">Object: {ev.involvedName}</p>}
                </div>
              ))}
            </div>
          ) : (
            <PageEmpty title="No events" description="Kubernetes events for this pod are collected by the agent." className="py-6" />
          )}
        </Card>
      )}

      {resolvedTab === 'runtime' && (
        <Card>
          <h3 className="text-section-title text-text mb-2 flex items-center gap-2 flex-wrap">
            <Activity className="w-5 h-5 text-brand shrink-0" /> Runtime Timeline
          </h3>
          <p className="text-caption text-muted mb-4">
            Incident-first timeline with correlated facts, capabilities, and insights for this pod.
          </p>
          {tabLoading ? (
            <p className="text-muted text-body">Loading...</p>
          ) : runtimeIncidents.length === 0 ? (
            <>
              <PageEmpty title="No runtime incidents" description="No stateful incidents found in the selected lookback window." className="py-6" />
              {runtimeDataHints().length > 0 && (
                <Card variant="panel" className="mt-3 bg-surface/40 border-border min-w-0">
                  <p className="text-caption text-muted mb-2">Diagnostics</p>
                  <ul className="space-y-1">
                    {runtimeDataHints().map((h) => (
                      <li key={h} className="text-caption text-muted">- {h}</li>
                    ))}
                  </ul>
                </Card>
              )}
            </>
          ) : (
            <div className="space-y-3">
              {[...runtimeIncidents]
                .sort((a, b) => new Date(b.lastSeenAt ?? b.createdAt ?? 0).getTime() - new Date(a.lastSeenAt ?? a.createdAt ?? 0).getTime())
                .map((inc) => (
                  <div key={inc.id} className="p-3 rounded-lg border border-border bg-surface/50">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="px-2 py-0.5 rounded text-caption font-semibold border border-border bg-surface-2 text-text">
                        {inc.incidentType}
                      </span>
                      <span className="px-2 py-0.5 rounded text-caption border border-border bg-surface text-text">
                        {inc.severityHint ?? '—'}
                      </span>
                      <span className="text-caption text-muted ml-auto">
                        {inc.lastSeenAt ? formatDateTime(inc.lastSeenAt) : (inc.createdAt ? formatDateTime(inc.createdAt) : '—')}
                      </span>
                    </div>
                    <p className="text-caption text-muted mt-1">
                      confidence {Number(inc.confidence ?? 0).toFixed(2)} · window {inc.window ?? '—'}
                    </p>
                    <p className="text-caption text-muted mt-1">
                      first {inc.firstSeenAt ? formatDateTime(inc.firstSeenAt) : '—'} → last {inc.lastSeenAt ? formatDateTime(inc.lastSeenAt) : '—'}
                    </p>
                  </div>
                ))}
            </div>
          )}
          <div className="mt-6 grid grid-cols-1 gap-3 md:grid-cols-3">
            <Card variant="panel" className="bg-surface/40 border-border min-w-0">
              <p className="text-caption text-muted mb-1">Facts in scope</p>
              <p className="text-body text-text">{runtimeFacts.length}</p>
            </Card>
            <Card variant="panel" className="bg-surface/40 border-border min-w-0">
              <p className="text-caption text-muted mb-1">Capabilities in scope</p>
              <p className="text-body text-text">{podCapabilities.length}</p>
            </Card>
            <Card variant="panel" className="bg-surface/40 border-border min-w-0">
              <p className="text-caption text-muted mb-1">Insights in report</p>
              <p className="text-body text-text">{relatedRisks.length}</p>
            </Card>
          </div>
        </Card>
      )}

      {/* Coverage section — shown within the consolidated Overview tab */}
      {resolvedTab === 'overview' && (
        <Card>
          <h3 className="text-section-title text-text mb-2 flex items-center gap-2 flex-wrap">
            <Shield className="w-5 h-5 text-brand shrink-0" /> Runtime Coverage
          </h3>
          <p className="text-caption text-muted mb-4">
            Coverage lens by source, layer, domain, signal type, and MITRE tags.
          </p>
          {tabLoading ? (
            <p className="text-muted text-body">Loading...</p>
          ) : (
            <>
              {runtimeDataHints().length > 0 && (
                <Card variant="panel" className="mb-4 bg-surface/40 border-border min-w-0">
                  <p className="text-caption text-muted mb-2">Data availability diagnostics</p>
                  <ul className="space-y-1">
                    {runtimeDataHints().map((h) => (
                      <li key={h} className="text-caption text-muted">- {h}</li>
                    ))}
                  </ul>
                </Card>
              )}
              <div className="mb-4 grid grid-cols-1 gap-3 md:grid-cols-3">
                <Card variant="panel" className="bg-surface/40 border-border min-w-0">
                  <p className="text-caption text-muted mb-1">Coverage by source</p>
                  <p className="text-caption text-text">
                    Falco {runtimeSecurityEvents.filter((e) => (e.runtime || '').toLowerCase() === 'falco').length} · Other{' '}
                    {runtimeSecurityEvents.filter((e) => (e.runtime || '').toLowerCase() !== 'falco').length}
                  </p>
                </Card>
                <Card variant="panel" className="bg-surface/40 border-border min-w-0">
                  <p className="text-caption text-muted mb-1">Coverage by layer</p>
                  <p className="text-caption text-text">
                    Events {runtimeSecurityEvents.length} · Facts {runtimeFacts.length} · Signals {runtimeSignals.length} · Incidents {runtimeIncidents.length}
                  </p>
                </Card>
                <Card variant="panel" className="bg-surface/40 border-border min-w-0">
                  <p className="text-caption text-muted mb-1">Coverage by MITRE tags</p>
                  <p className="text-caption text-text">
                    {new Set(runtimeSecurityEvents.map((e) => (e.mitreTechnique || '').trim()).filter(Boolean)).size} distinct techniques
                  </p>
                </Card>
              </div>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                <Card variant="panel" className="bg-surface/40 border-border min-w-0">
                  <p className="text-caption text-muted mb-2">Fact domain distribution</p>
                  <div className="flex flex-wrap gap-1.5">
                    {Array.from(
                      runtimeFacts.reduce((acc, f) => {
                        const d = (f.domain || 'unknown').trim() || 'unknown';
                        acc.set(d, (acc.get(d) || 0) + 1);
                        return acc;
                      }, new Map<string, number>())
                    )
                      .sort((a, b) => b[1] - a[1])
                      .map(([domain, count]) => (
                        <span key={domain} className="px-2 py-0.5 rounded text-caption border border-border bg-surface text-text">
                          {domain}: {count}
                        </span>
                      ))}
                    {runtimeFacts.length === 0 ? <span className="text-caption text-muted">No fact coverage yet.</span> : null}
                  </div>
                </Card>
                <Card variant="panel" className="bg-surface/40 border-border min-w-0">
                  <p className="text-caption text-muted mb-2">Signal type distribution</p>
                  <div className="flex flex-wrap gap-1.5">
                    {Array.from(
                      runtimeSignals.reduce((acc, s) => {
                        const t = (s.signalType || 'UNKNOWN').trim() || 'UNKNOWN';
                        acc.set(t, (acc.get(t) || 0) + 1);
                        return acc;
                      }, new Map<string, number>())
                    )
                      .sort((a, b) => b[1] - a[1])
                      .map(([signalType, count]) => (
                        <span key={signalType} className="px-2 py-0.5 rounded text-caption border border-border bg-surface text-text">
                          {signalType}: {count}
                        </span>
                      ))}
                    {runtimeSignals.length === 0 ? <span className="text-caption text-muted">No signal coverage yet.</span> : null}
                  </div>
                </Card>
              </div>
            </>
          )}
        </Card>
      )}

      {selectedVulnerability && (
        <div className="fixed inset-y-0 right-0 w-full max-w-md bg-surface border-l border-border shadow-2xl z-50 flex flex-col animate-in slide-in-from-right-4 duration-200">
          <div className="flex justify-between items-center p-4 border-b border-border bg-base/80">
            <div className="flex items-center gap-3">
              <div className={clsx('p-2 rounded-lg', getSeverityBadgeClass(selectedVulnerability.severity))}>
                <ShieldAlert className="w-5 h-5" />
              </div>
              <div>
                <h3 className="text-lg font-bold text-text font-mono">{selectedVulnerability.id}</h3>
                <span className={clsx('px-2 py-0.5 rounded text-caption font-medium border capitalize', getSeverityBadgeClass(selectedVulnerability.severity))}>
                  {selectedVulnerability.severity}
                </span>
              </div>
            </div>
            <button type="button" onClick={() => setSelectedVulnerability(null)} className="p-2 rounded-lg text-muted hover:bg-surface-2 hover:text-text">
              <X className="w-5 h-5" />
            </button>
          </div>
          <div className="p-4 space-y-4 overflow-y-auto flex-1">
            <div className="flex items-center justify-between p-3 bg-surface-2/50 rounded-lg border border-border">
              <div>
                <p className="text-caption text-muted uppercase font-semibold mb-0.5">CVSS</p>
                <p className={clsx('text-2xl font-bold', (selectedVulnerability.cvssScore ?? 0) >= 7 ? 'text-red-400' : (selectedVulnerability.cvssScore ?? 0) >= 4 ? 'text-amber-400' : 'text-text')}>
                  {selectedVulnerability.cvssScore?.toFixed(1) ?? '—'}
                </p>
              </div>
              <div>
                <p className="text-caption text-muted uppercase font-semibold mb-0.5">Status</p>
                <p className="text-body font-medium text-text capitalize">{selectedVulnerability.status ?? 'active'}</p>
              </div>
            </div>
            {selectedVulnerability.description && (
              <div>
                <p className="text-caption font-semibold text-muted uppercase tracking-wider mb-2 flex items-center gap-1">
                  <FileText className="w-3.5 h-3.5" /> Description
                </p>
                <p className="text-body text-text leading-relaxed bg-surface-2/30 p-3 rounded-lg border border-border">
                  {selectedVulnerability.description}
                </p>
              </div>
            )}
            {selectedVulnerability.fixedVersion ? (
              <div className="p-3 bg-emerald-500/10 border border-emerald-500/20 rounded-lg">
                <div className="flex items-center gap-2 text-emerald-400 mb-1">
                  <CheckCircle2 className="w-4 h-4" />
                  <span className="text-caption font-semibold uppercase">Remediation</span>
                </div>
                <p className="text-body text-text">
                  Fix available in version <span className="font-mono text-emerald-300">{selectedVulnerability.fixedVersion}</span>
                </p>
              </div>
            ) : (
              <div className="p-3 bg-amber-500/10 border border-amber-500/20 rounded-lg">
                <div className="flex items-center gap-2 text-amber-400 mb-1">
                  <Info className="w-4 h-4" />
                  <span className="text-caption font-semibold uppercase">No fix version yet</span>
                </div>
                <p className="text-body text-text">Monitor advisories for updates.</p>
              </div>
            )}
            {/* GAP 12: Exploit maturity & allowed status */}
            {(selectedVulnerability.exploitKnown || selectedVulnerability.exploitMaturity || selectedVulnerability.allowed != null) && (
              <div className="space-y-2">
                {(selectedVulnerability.exploitKnown || selectedVulnerability.exploitMaturity) && (
                  <div className="p-3 bg-surface-2/50 rounded-lg border border-border">
                    <p className="text-caption font-semibold text-muted uppercase tracking-wider mb-2">Exploit Intelligence</p>
                    <div className="flex flex-wrap gap-3 text-body text-text">
                      {selectedVulnerability.exploitKnown && (
                        <span className="flex items-center gap-1.5">
                          <span className="text-amber-400">🔥</span> Public exploit known
                        </span>
                      )}
                      {selectedVulnerability.exploitMaturity && (
                        <span>Maturity: <span className="font-medium text-text">{selectedVulnerability.exploitMaturity}</span></span>
                      )}
                    </div>
                  </div>
                )}
                {selectedVulnerability.allowed != null && (
                  <div className={clsx('p-3 rounded-lg border', selectedVulnerability.allowed ? 'bg-emerald-500/10 border-emerald-500/20' : 'bg-red-500/10 border-red-500/20')}>
                    <p className="text-body text-text">
                      Policy: <span className={clsx('font-medium', selectedVulnerability.allowed ? 'text-emerald-300' : 'text-red-300')}>
                        {selectedVulnerability.allowed ? '✅ Allowed by policy' : '❌ Not allowed'}
                      </span>
                    </p>
                  </div>
                )}
              </div>
            )}
            <Button className="w-full" size="sm" onClick={() => window.open(`https://www.cve.org/CVERecord?id=${encodeURIComponent(selectedVulnerability.id)}`, '_blank')}>
              <ExternalLink className="w-4 h-4 mr-2" /> View CVE record
            </Button>
          </div>
        </div>
      )}

      <Card className="mt-8 min-w-0" variant="secondary">
        <div className="flex items-center justify-between mb-3 min-w-0">
          <h3 className="text-caption font-semibold text-muted uppercase tracking-wider">Related navigation</h3>
        </div>
        <div className="flex flex-wrap gap-2 min-w-0">
          {pod.clusterId && (
            <Button variant="secondary" size="sm" onClick={() => navigate(`/clusters/${pod.clusterId}`)}>
              <Globe className="w-4 h-4 mr-1" /> View cluster
            </Button>
          )}
          {pod.nodeName && pod.clusterId && (
            <Button variant="secondary" size="sm" onClick={() => navigate(`/clusters/${pod.clusterId}/nodes/${encodeURIComponent(pod.nodeName)}`)}>
              View node
            </Button>
          )}
          <Button variant="secondary" size="sm" onClick={() => navigate('/risks')}>
            View all risks
          </Button>
          <Button variant="secondary" size="sm" onClick={() => navigate('/capabilities')}>
            Capabilities
          </Button>
          <Button variant="secondary" size="sm" onClick={() => navigate('/resources?tab=Pod')}>
            Back to Resources
          </Button>
        </div>
      </Card>
    </PageLayout>
  );
};
