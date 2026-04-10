import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { PodWithRisk, PodSbom, Insight, Vulnerability, RuntimeSignal, RuntimeSignalSuppressionStats, PodRiskReportSummary, PodRuntimeSecurityEvent, PodRuntimeBehaviorFact, PodRuntimeIncident, PodCapabilityDetail } from '../types';
import { RUNTIME_SIGNALS_LOOKBACK_MINUTES } from '../lib/runtimeLookback';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Tabs } from '../design-system/components/Tabs';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { PageLoading } from '../components/PageLoading';
import { PageEmpty } from '../components/PageEmpty';
import { PodNetworkSummary } from '../components/PodNetworkSummary';
import { ArrowLeft, Box, Package, ShieldAlert, Globe, Download, ChevronDown, ChevronRight, X, FileText, ExternalLink, CheckCircle2, Info, Cpu, Network, Activity, BarChart2, FileCode, Shield, AlertTriangle, RefreshCw } from 'lucide-react';
import clsx from 'clsx';
import { getSeverityBadgeClass, getSeverityBarClass, getSeverityTextClass, getSeverityIcon, getPodStatusBadgeClass } from '../lib/severity';
import { formatDateTime, formatUptime } from '../lib/display';
import { exportSbomAsCsv, exportSbomAsCycloneDxJson, exportSbomAsJson, exportSbomAsSpdxJson } from '../lib/exportSbom';
import { SbomMetaBadges } from '../components/SbomMetaBadges';
import { useAuthStore } from '../store/authStore';
import type { SbomComponent as SbomComponentType, PodRuntimeMetric, PodProcessItem, PodNetworkConnectionItem, PodK8sEventItem, PodNetworkTopDestinationItem } from '../types';
import { formatRiskFindingReference, insightTypeUiLabel } from '../lib/riskDisplay';

type TabId = 'overview' | 'sbom' | 'risks' | 'metrics' | 'processes' | 'network' | 'events' | 'timeline' | 'coverage' | 'spec';

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
  if (s === 'fixed') return 'bg-slate-500/80 text-slate-200';
  if (s === 'not exploitable' || s === 'not_exploitable') return 'bg-emerald-500/80 text-white';
  return 'bg-slate-600/80 text-slate-300';
}

export const PodDetail: React.FC = () => {
  const { id, uid } = useParams<{ id?: string; uid?: string }>();
  const navigate = useNavigate();
  const [pod, setPod] = useState<PodWithRisk | null>(null);
  const [sbom, setSbom] = useState<PodSbom | null>(null);
  const [relatedRisks, setRelatedRisks] = useState<Insight[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<TabId>('overview');
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

  const idOrUid = uid ?? id;

  const podRiskLevel = (count: number): 'critical' | 'high' | 'medium' | 'low' => {
    if (count >= 10) return 'critical';
    if (count >= 4) return 'high';
    if (count >= 1) return 'medium';
    return 'low';
  };

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
      signalClass: 'bg-slate-500/20 text-slate-300 border-slate-500/40',
      severity: 'INFO',
      severityClass: 'bg-slate-500/20 text-slate-300 border-slate-500/40',
    };
  };

  const runtimeSourceBadge = (src: string | undefined): { label: string; className: string } => {
    const s = (src || '').toLowerCase().trim();
    if (s === 'falco') return { label: 'Falco', className: 'bg-violet-600/25 text-violet-200 border-violet-500/40' };
    if (s === 'ebpf') return { label: 'eBPF', className: 'bg-sky-600/25 text-sky-200 border-sky-500/40' };
    if (s === 'agent') return { label: 'Agent', className: 'bg-emerald-600/25 text-emerald-200 border-emerald-500/40' };
    return { label: s ? src! : '—', className: 'bg-slate-600/25 text-slate-300 border-slate-500/40' };
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
    if (!idOrUid) return;
    setLoading(true);
    // API only supports lookup by uid (Kubernetes UID). Legacy numeric id in URL causes 404.
    const data = await api.getPodByUid(idOrUid);
    setPod(data);
    setLoading(false);
  }, [idOrUid]);

  const fetchTabData = useCallback(
    async (tab: TabId) => {
      if (!pod?.uid) return;
      setTabLoading(true);
      try {
        const uid = pod.uid;
        if (tab === 'sbom') {
          if (!sbomLoaded || dataErrorsRef.current.includes('sbom')) {
            try {
              const data = await api.getPodSbom(uid);
              setSbom(data ?? null);
              setSbomLoaded(true);
              setDataErrors((p) => p.filter((e) => e !== 'sbom'));
            } catch (e) {
              console.error('sbom tab fetch', e);
              setSbom(null);
              setSbomLoaded(true);
              setDataErrors((p) => (p.includes('sbom') ? p : [...p, 'sbom']));
            }
          }
        } else if (tab === 'risks') {
          if (dataErrorsRef.current.includes('risk-report') || (relatedRisks.length === 0 && !podRiskReportSummary)) {
            try {
              const { insights, summary } = await api.getPodRiskReport(uid);
              setRelatedRisks(insights ?? []);
              setPodRiskReportSummary(summary ?? null);
              setDataErrors((p) => p.filter((e) => e !== 'risk-report'));
            } catch (e) {
              console.error('risks tab fetch', e);
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
            tasks.push(api.getPodEvents(uid).then(setPodEvents).catch((e) => console.error('pod events', e)));
          }
          if (runtimeSecurityEvents.length === 0) {
            tasks.push(
              api.getPodRuntimeSecurityEvents(uid, 150).then(setRuntimeSecurityEvents).catch((e) => console.error('runtime security events', e)),
            );
          }
          if (runtimeFacts.length === 0) {
            tasks.push(api.getPodRuntimeBehaviorFactsV2(uid, 120).then(setRuntimeFacts).catch((e) => console.error('runtime facts', e)));
          }
          if (runtimeIncidents.length === 0) {
            tasks.push(api.getPodRuntimeIncidentsV2(uid, 80).then(setRuntimeIncidents).catch((e) => console.error('runtime incidents', e)));
          }
          if (podCapabilities.length === 0) {
            tasks.push(api.getPodCapabilities(uid).then(setPodCapabilities).catch((e) => console.error('capabilities', e)));
          }
          if (runtimeSignals.length === 0) {
            tasks.push(
              api
                .getRuntimeSignalsByPod(uid, { sinceMinutes: RUNTIME_SIGNALS_LOOKBACK_MINUTES, limit: 200 })
                .then(setRuntimeSignals)
                .catch((e) => console.error('runtime signals', e)),
            );
          }
          if (signalStats === null) {
            tasks.push(
              api
                .getRuntimeSignalSuppressionStats({ podUid: uid, sinceMinutes: 60 })
                .then(setSignalStats)
                .catch((e) => console.error('signal stats', e)),
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
    ]
  );

  useEffect(() => {
    fetchPod();
  }, [fetchPod]);

  // Load SBOM when pod is available (for Overview summary + SBOM tab)
  useEffect(() => {
    if (pod?.uid) {
      api
        .getPodSbom(pod.uid)
        .then((data) => {
          setSbom(data ?? null);
          setSbomLoaded(true);
          setDataErrors((p) => p.filter((e) => e !== 'sbom'));
        })
        .catch((err) => {
          console.error('sbom preload', err);
          setSbom(null);
          setSbomLoaded(true);
          setDataErrors((p) => (p.includes('sbom') ? p : [...p, 'sbom']));
        });
    } else {
      setSbom(null);
      setSbomLoaded(false);
    }
  }, [pod?.uid]);

  useEffect(() => {
    if (!pod?.uid) {
      setPodRiskReportSummary(null);
      setRelatedRisks([]);
      return;
    }
    api
      .getPodRiskReport(pod.uid)
      .then(({ insights, summary }) => {
        setRelatedRisks(insights ?? []);
        setPodRiskReportSummary(summary ?? null);
        setDataErrors((p) => p.filter((e) => e !== 'risk-report'));
      })
      .catch((err) => {
        console.error('risk report preload', err);
        setRelatedRisks([]);
        setPodRiskReportSummary(null);
        setDataErrors((p) => (p.includes('risk-report') ? p : [...p, 'risk-report']));
      });
  }, [pod?.uid]);

  // Helper to refresh all pod-detail data (used by preload + WS + manual refresh)
  const refreshAllData = useCallback((podUid: string) => {
    const errors: string[] = [];
    const track = (label: string) => (err: unknown) => {
      errors.push(label);
      console.error(label, err);
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
  const wsUid = pod?.uid ?? uid ?? null;
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

  if (loading || !idOrUid) {
    return <PageLoading message="Loading pod detail..." className="min-h-[40vh]" />;
  }

  if (!pod) {
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
    { id: 'sbom', label: 'SBOM', icon: <Package className="w-4 h-4" /> },
    { id: 'risks', label: 'Related Risks', icon: <ShieldAlert className="w-4 h-4" /> },
    { id: 'metrics', label: 'Runtime metrics', icon: <BarChart2 className="w-4 h-4" /> },
    { id: 'processes', label: 'Processes', icon: <Cpu className="w-4 h-4" /> },
    { id: 'network', label: 'Network', icon: <Network className="w-4 h-4" /> },
    { id: 'events', label: 'Events', icon: <Activity className="w-4 h-4" /> },
    { id: 'timeline', label: 'Runtime Timeline', icon: <Activity className="w-4 h-4" /> },
    { id: 'coverage', label: 'Coverage', icon: <Shield className="w-4 h-4" /> },
    { id: 'spec', label: 'Spec', icon: <FileCode className="w-4 h-4" /> },
  ];

  return (
    <PageLayout
      title="Pod Detail"
      description=""
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
                await api
                  .getPodSbom(pod.uid)
                  .then((d) => {
                    setSbom(d ?? null);
                    setSbomLoaded(true);
                    setDataErrors((p) => p.filter((e) => e !== 'sbom'));
                  })
                  .catch((err) => {
                    console.error('sbom refresh', err);
                    setSbom(null);
                    setSbomLoaded(true);
                    setDataErrors((p) => (p.includes('sbom') ? p : [...p, 'sbom']));
                  });
                await api
                  .getPodRiskReport(pod.uid)
                  .then(({ insights, summary }) => {
                    setRelatedRisks(insights ?? []);
                    setPodRiskReportSummary(summary ?? null);
                    setDataErrors((p) => p.filter((e) => e !== 'risk-report'));
                  })
                  .catch((err) => {
                    console.error('risk report refresh', err);
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
        </div>
      }
    >
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4 md:gap-6 mb-6 min-w-0">
        <div className="flex items-center gap-3 sm:gap-4 min-w-0 flex-1">
          <div className="p-3 sm:p-4 bg-slate-900 border border-slate-800 rounded-xl shrink-0">
            <Box className="w-7 h-7 sm:w-8 sm:h-8 text-pink-500" />
          </div>
          <div className="min-w-0">
            <div className="flex items-center gap-2 sm:gap-3 flex-wrap">
              <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight truncate max-w-full">{pod.name}</h1>
              <span className={clsx('px-2.5 py-0.5 rounded-full text-xs font-semibold uppercase border', getSeverityBadgeClass(podRiskLevel(pod.riskCount)))}>
                {podRiskLevel(pod.riskCount)} ({pod.riskCount})
              </span>
            </div>
            <div className="mt-1 text-slate-500 text-xs sm:text-sm font-mono break-all sm:break-normal">
              <span className="inline-block">{pod.namespace}</span>
              <span className="mx-2 opacity-50">|</span>
              <span className="inline-block">
                {pod.nodeName && pod.clusterId ? (
                  <button
                    type="button"
                    className="text-pink-400 hover:underline text-left"
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
        <div />
      </div>

      {/* Hint when pod IP / start time are missing (filled by agent sync; wait for next sync or restart agent) */}
      {(!pod.podIP || !pod.startTime) && (
        <p className="text-slate-500 text-xs mb-2">
          Pod IP and Start time come from agent sync. If empty, wait for the next sync (~2 min) or restart the agent: <code className="bg-slate-800 px-1 rounded">kubectl rollout restart daemonset/fortuna-agent -n fortuna</code>
        </p>
      )}
      {dataErrors.length > 0 && (
        <div className="mb-3 p-2.5 rounded-lg border border-amber-700/50 bg-amber-950/30 flex items-center gap-2 text-xs text-amber-300">
          <AlertTriangle className="w-4 h-4 shrink-0" />
          <span>
            Failed to load: {dataErrors.join(', ')}.{' '}
            <button
              type="button"
              className="underline hover:text-white"
              onClick={() => {
                if (!pod?.uid) return;
                refreshAllData(pod.uid);
                void api
                  .getPodSbom(pod.uid)
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
                  .then(({ insights, summary }) => {
                    setRelatedRisks(insights ?? []);
                    setPodRiskReportSummary(summary ?? null);
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
      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-3 mb-6 min-w-0">
        <Card variant="panel" className="bg-slate-900/50 min-w-0">
          <p className="ui-micro-label mb-1">Status</p>
          <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border ${getPodStatusBadgeClass(pod.status ?? pod.phase)}`}>{pod.status ?? pod.phase ?? '—'}</span>
        </Card>
        <Card variant="panel" className="bg-slate-900/50 min-w-0">
          <p className="ui-micro-label mb-1">Pod IP</p>
          <p className="text-sm font-medium text-slate-300 font-mono">{pod.podIP ?? '—'}</p>
        </Card>
        <Card variant="panel" className="bg-slate-900/50 min-w-0">
          <p className="ui-micro-label mb-1">Start Time</p>
          <p className="text-sm font-medium text-slate-300">{pod.startTime ? formatDateTime(pod.startTime) : (runtimeMetrics.length > 0 && runtimeMetrics[0].lastObservedAt ? `Last reported: ${formatDateTime(runtimeMetrics[0].lastObservedAt)}` : '—')}</p>
        </Card>
        <Card variant="panel" className="bg-slate-900/50 min-w-0">
          <p className="ui-micro-label mb-1">Uptime</p>
          <p className="text-sm font-medium text-slate-300">{formatUptime(pod.startTime ?? undefined)}</p>
        </Card>
        <Card variant="panel" className="bg-slate-900/50 min-w-0">
          <p className="ui-micro-label mb-1">Restart Count</p>
          <p className="text-lg font-bold text-white">{pod.restartCount ?? 0}</p>
        </Card>
        <Card variant="panel" className="bg-slate-900/50 min-w-0">
          <p className="ui-micro-label mb-1">QoS Class</p>
          <p className="text-sm font-medium text-slate-300">{pod.qosClass ?? '—'}</p>
        </Card>
        <Card variant="panel" className="bg-slate-900/50 min-w-0">
          <p className="ui-micro-label mb-1">Risk Count</p>
          <p className="text-lg font-bold text-white">{pod.riskCount}</p>
        </Card>
        <Card variant="panel" className="bg-slate-900/50 min-w-0">
          <p className="ui-micro-label mb-1">Service Account</p>
          <p className="text-sm font-medium text-slate-300 truncate">{pod.serviceAccount ?? '—'}</p>
        </Card>
        <Card variant="panel" className="bg-slate-900/50 min-w-0">
          <p className="ui-micro-label mb-1">Created</p>
          <p className="text-sm font-medium text-slate-300">{pod.createdAt ? formatDateTime(pod.createdAt) : '—'}</p>
        </Card>
      </div>

      <Tabs items={tabs} value={activeTab} onChange={(id) => setActiveTab(id as TabId)} />

      {activeTab === 'overview' && (
        <div className="space-y-6">
        <Card variant="secondary">
            <h3 className="text-base md:text-lg font-semibold text-white mb-4 flex items-center gap-2 flex-wrap">
              <Box className="w-5 h-5 text-pink-500" /> Overview
            </h3>
            {/* Container image info — GAP 10: missing from original */}
            {(sbom?.image || sbom?.container) && (
              <div className="mb-4 pb-4 border-b border-slate-800">
                <h4 className="text-sm font-semibold text-slate-300 mb-3">Container Image</h4>
                <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-2 text-sm">
                  {sbom?.image && (
                    <div className="col-span-2">
                      <dt className="text-slate-500">Image</dt>
                      <dd className="text-slate-300 font-mono text-xs break-all">{sbom.image}</dd>
                    </div>
                  )}
                  {sbom?.container && (
                    <div>
                      <dt className="text-slate-500">Container</dt>
                      <dd className="text-slate-300 font-mono">{sbom.container}</dd>
                    </div>
                  )}
                </dl>
              </div>
            )}
            <dl className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
              <div className="sm:col-span-2">
                <dt className="text-slate-500">UID</dt>
                <dd className="text-slate-400 font-mono text-xs break-all">{pod.uid}</dd>
              </div>
            </dl>
            {(pod.ownerKind ?? pod.ownerName ?? pod.replicaSetName) && (
              <div className="mt-4 pt-4 border-t border-slate-800">
                <h4 className="text-sm font-semibold text-slate-300 mb-3">Identity &amp; Ownership</h4>
                <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-2 text-sm">
                  {pod.ownerKind && (
                    <div>
                      <dt className="text-slate-500">Owner Type</dt>
                      <dd className="text-slate-300 font-medium">{pod.ownerKind}</dd>
                    </div>
                  )}
                  {pod.ownerName && (
                    <div>
                      <dt className="text-slate-500">Owner Name</dt>
                      <dd className="text-slate-300 font-mono">{pod.ownerName}</dd>
                    </div>
                  )}
                  {pod.replicaSetName && (
                    <div>
                      <dt className="text-slate-500">ReplicaSet</dt>
                      <dd className="text-slate-300 font-mono">{pod.replicaSetName}</dd>
                    </div>
                  )}
                </dl>
              </div>
            )}
            {sbom && (
              <div className="mt-4 pt-4 border-t border-slate-800">
                <h4 className="text-sm font-semibold text-slate-300 mb-3">Security</h4>
                <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-1 text-sm mb-4">
                  <div>
                    <dt className="text-slate-500">Total Packages</dt>
                    <dd className="text-white font-medium">{sbom.components?.length ?? 0}</dd>
                  </div>
                  <div>
                    <dt className="text-slate-500">Vulnerable Packages</dt>
                    <dd className={(sbom.vulnerablePackageCount ?? 0) > 0 ? 'text-amber-400 font-medium' : 'text-slate-400'}>
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
                          <span className={clsx('text-xs font-medium w-20', getSeverityTextClass(level))}>
                            {label}
                          </span>
                          <div className="flex-1 h-5 bg-slate-800 rounded overflow-hidden min-w-[80px]">
                            <div
                              className={clsx('h-full rounded transition-all', getSeverityBarClass(level))}
                              style={{ width: `${Math.max(pct, pct > 0 ? 4 : 0)}%` }}
                            />
                          </div>
                          <span className="text-slate-300 text-xs tabular-nums w-8 font-medium">{count}</span>
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            )}
            {!sbom && sbomLoaded && (
              <div className="mt-4 pt-4 border-t border-slate-800">
                <p className="text-slate-500 text-sm">No SBOM data available for this pod.</p>
              </div>
            )}
            {!sbom && !sbomLoaded && (
              <div className="mt-4 pt-4 border-t border-slate-800">
                <p className="text-slate-500 text-sm">Security summary loading…</p>
              </div>
            )}
            <div className="mt-4 pt-4 border-t border-slate-800">
              <h4 className="text-sm font-semibold text-slate-300 mb-2 flex items-center gap-2">
                <BarChart2 className="w-4 h-4" /> Pod detail (agent)
              </h4>
              {podRiskReportSummary && (
                <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-1 text-sm mb-3 text-slate-400">
                  <div>
                    <dt className="text-slate-500">Runtime signals (24h, DB)</dt>
                    <dd className="text-slate-200 tabular-nums">{podRiskReportSummary.runtimeSignals24h ?? 0}</dd>
                  </div>
                  <div>
                    <dt className="text-slate-500">Insights on this pod (report)</dt>
                    <dd className="text-slate-200 tabular-nums">{podRiskReportSummary.podDirectInsightCount ?? 0}</dd>
                  </div>
                  {(podRiskReportSummary.runtimePolicyInsightCount ?? 0) > 0 && (
                    <div className="col-span-2">
                      <dt className="text-slate-500">Runtime / pod-security policy insights</dt>
                      <dd className="text-amber-300/90 tabular-nums">{podRiskReportSummary.runtimePolicyInsightCount}</dd>
                    </div>
                  )}
                </dl>
              )}
              {(runtimeMetrics.length > 0 || processes.length > 0 || networkConnections.length > 0) ? (
                <div className="flex flex-wrap gap-3 text-sm">
                  {runtimeMetrics.length > 0 && (
                    <button type="button" onClick={() => setActiveTab('metrics')} className="text-pink-400 hover:underline">
                      {runtimeMetrics.length} runtime metric(s)
                    </button>
                  )}
                  {processes.length > 0 && (
                    <button type="button" onClick={() => setActiveTab('processes')} className="text-pink-400 hover:underline">
                      {processes.length} process(es)
                    </button>
                  )}
                  {networkConnections.length > 0 && (
                    <button type="button" onClick={() => setActiveTab('network')} className="text-pink-400 hover:underline">
                      {networkConnections.length} connection(s)
                    </button>
                  )}
                </div>
              ) : (
                <p className="text-slate-500 text-sm">Open the Runtime metrics, Processes, or Network tab to load data from the agent (or wait for live updates).</p>
              )}
              <div className="mt-3 flex flex-wrap gap-3 text-sm">
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
                  className="inline-flex items-center px-2.5 py-1.5 rounded bg-slate-950 border border-slate-700 text-slate-200 hover:border-pink-500"
                >
                  View related risks in Risk Operations
                </button>
              </div>
            </div>
          </Card>
        </div>
      )}

      {activeTab === 'sbom' && (
        <Card>
          <div className="flex flex-col gap-3 lg:flex-row lg:flex-wrap lg:items-start lg:justify-between mb-4">
            <div className="flex flex-col gap-2 min-w-0 flex-1 basis-[min(100%,18rem)]">
              <div className="flex items-center gap-2 flex-wrap">
                <h3 className="text-lg font-semibold text-white">SBOM</h3>
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
            <p className="text-slate-500 text-sm">Loading...</p>
          ) : sbom ? (
            <div className="space-y-4">
              <p className="text-slate-400 text-sm">
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
                    <div className="flex flex-col gap-3 py-3 border-y border-slate-800">
                      <div className="flex flex-wrap items-center gap-x-2 gap-y-2">
                        <span className="text-slate-500 text-sm shrink-0 w-16 sm:w-auto">Severity</span>
                        <div className="flex flex-wrap items-center gap-1">
                          {severityOpts.map((s) => (
                            <button
                              key={s}
                              type="button"
                              onClick={() => setSbomSeverityFilter(s)}
                              className={clsx(
                                'px-2 py-1 rounded text-xs font-medium capitalize',
                                sbomSeverityFilter === s ? 'bg-pink-600 text-white' : 'bg-slate-800 text-slate-400 hover:text-white'
                              )}
                            >
                              {s}
                            </button>
                          ))}
                        </div>
                      </div>
                      <div className="flex flex-wrap items-center gap-x-2 gap-y-2">
                        <span className="text-slate-500 text-sm shrink-0 w-16 sm:w-auto">Status</span>
                        <div className="flex flex-wrap items-center gap-1">
                          {statusOpts.map((s) => (
                            <button
                              key={s}
                              type="button"
                              onClick={() => setSbomStatusFilter(s)}
                              className={clsx(
                                'px-2 py-1 rounded text-xs font-medium capitalize',
                                sbomStatusFilter === s ? 'bg-pink-600 text-white' : 'bg-slate-800 text-slate-400 hover:text-white'
                              )}
                            >
                              {s}
                            </button>
                          ))}
                        </div>
                      </div>
                      <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
                        <label className="flex items-center gap-2 text-slate-400 text-sm cursor-pointer min-w-0">
                          <input
                            type="checkbox"
                            checked={sbomOnlyVulnerable}
                            onChange={(e) => setSbomOnlyVulnerable(e.target.checked)}
                            className="rounded border-slate-600 bg-slate-800 text-pink-500 shrink-0"
                          />
                          <span className="leading-snug">Vulnerable / malware only</span>
                        </label>
                        <div className="flex flex-wrap items-center gap-2 min-w-0 flex-1 sm:flex-initial">
                          <span className="text-slate-500 text-sm shrink-0">Search</span>
                          <input
                            type="text"
                            value={sbomSearch}
                            onChange={(e) => setSbomSearch(e.target.value)}
                            placeholder="Package or version…"
                            className="bg-slate-950 border border-slate-700 rounded px-2 py-1.5 text-xs text-slate-200 min-w-0 w-full sm:w-48 max-w-full"
                          />
                        </div>
                        <div className="flex flex-wrap items-center gap-1">
                          <span className="text-slate-500 text-sm shrink-0 mr-1">Sort</span>
                          <button
                            type="button"
                            onClick={() => setSbomSort(sbomSort === 'name' ? 'none' : 'name')}
                            className={clsx(
                              'px-2 py-1 rounded text-xs font-medium',
                              sbomSort === 'name' ? 'bg-pink-600 text-white' : 'bg-slate-800 text-slate-400 hover:text-white'
                            )}
                          >
                            Name
                          </button>
                          <button
                            type="button"
                            onClick={() => setSbomSort(sbomSort === 'severity' ? 'none' : 'severity')}
                            className={clsx(
                              'px-2 py-1 rounded text-xs font-medium',
                              sbomSort === 'severity' ? 'bg-pink-600 text-white' : 'bg-slate-800 text-slate-400 hover:text-white'
                            )}
                          >
                            Severity
                          </button>
                          <button
                            type="button"
                            onClick={() => setSbomSort(sbomSort === 'cve' ? 'none' : 'cve')}
                            className={clsx(
                              'px-2 py-1 rounded text-xs font-medium',
                              sbomSort === 'cve' ? 'bg-pink-600 text-white' : 'bg-slate-800 text-slate-400 hover:text-white'
                            )}
                          >
                            CVEs
                          </button>
                        </div>
                      </div>
                    </div>
                    <div className="rounded-lg border border-slate-800 bg-slate-950/30 overflow-hidden -mx-1 sm:mx-0">
                      <div className="ui-table-scroll-compact max-h-[min(70vh,36rem)]">
                        <table className="w-full min-w-[1100px] text-sm border-collapse">
                          <thead>
                            <tr className="border-b border-slate-800 bg-slate-900/95 backdrop-blur-sm shadow-[inset_0_-1px_0_0_rgb(30_41_59)] sticky top-0 z-10">
                              <th className="py-2.5 px-1 text-center text-xs font-semibold uppercase tracking-wide text-slate-500" aria-hidden />
                              <th className="py-2.5 px-2 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Package</th>
                              <th className="py-2.5 px-2 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Malware</th>
                              <th className="py-2.5 px-2 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Version</th>
                              <th className="py-2.5 px-2 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Type</th>
                              <th className="py-2.5 px-2 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">License</th>
                              <th className="py-2.5 px-2 text-right text-xs font-semibold uppercase tracking-wide text-slate-500">CVE</th>
                              <th className="py-2.5 px-2 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Severity</th>
                              <th className="py-2.5 px-2 text-right text-xs font-semibold uppercase tracking-wide text-slate-500">CVSS</th>
                              <th className="py-2.5 px-2 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Fix</th>
                              <th className="py-2.5 px-2 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Status</th>
                              <th className="py-2.5 px-2 text-center text-xs font-semibold uppercase tracking-wide text-slate-500">Exploit</th>
                              <th className="py-2.5 px-2 text-center text-xs font-semibold uppercase tracking-wide text-slate-500">Allow</th>
                            </tr>
                          </thead>
                        <tbody className="divide-y divide-slate-800">
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
                                    'border-b border-slate-800',
                                    hasMalware && 'bg-red-950/15',
                                    rowExpandable ? 'cursor-pointer hover:bg-slate-800/50' : ''
                                  )}
                                  onClick={() => rowExpandable && setSbomExpandedId(isExpanded ? null : rowId)}
                                >
                                  <td className="py-2 px-1 w-9 text-center align-middle">
                                    {hasCves ? (
                                      isExpanded ? (
                                        <ChevronDown className="w-4 h-4 text-slate-400 mx-auto" />
                                      ) : (
                                        <ChevronRight className="w-4 h-4 text-slate-400 mx-auto" />
                                      )
                                    ) : (
                                      <span className="w-4 inline-block" />
                                    )}
                                  </td>
                                  <td
                                    className="py-2 px-2 align-middle font-mono text-white max-w-[14rem] truncate"
                                    title={c.name ?? undefined}
                                  >
                                    {c.name}
                                  </td>
                                  <td className="py-2 px-2 align-middle max-w-[10rem]">
                                    {mm ? (
                                      <span
                                        className="inline-flex items-center gap-1 max-w-full px-1.5 py-0.5 rounded text-xs font-semibold border bg-red-950/50 text-red-200 border-red-700/60"
                                        title={mm.malwareFamily ? `Family: ${mm.malwareFamily}` : mm.reason}
                                      >
                                        <AlertTriangle className="w-3 h-3 shrink-0" />
                                        <span className="truncate">{mm.reason}</span>
                                      </span>
                                    ) : (
                                      <span className="text-slate-600">—</span>
                                    )}
                                  </td>
                                  <td
                                    className="py-2 px-2 align-middle text-slate-400 font-mono max-w-[7rem] truncate whitespace-nowrap"
                                    title={c.version ?? undefined}
                                  >
                                    {c.version ?? '—'}
                                  </td>
                                  <td className="py-2 px-2 align-middle text-slate-500 font-mono whitespace-nowrap">
                                    {sbomTypeLabel(c.type)}
                                  </td>
                                  <td className="py-2 px-2 align-middle text-slate-500 text-xs max-w-[6rem] truncate" title={c.license ?? undefined}>
                                    {c.license ?? '—'}
                                  </td>
                                  <td className="py-2 px-2 align-middle text-right tabular-nums whitespace-nowrap">
                                    {cveCount > 0 ? (
                                      <span className="text-amber-400 font-medium">{cveCount}</span>
                                    ) : (
                                      <span className="text-slate-500">0</span>
                                    )}
                                  </td>
                                  <td className="py-2 px-2 align-middle whitespace-nowrap">
                                    {c.maxSeverity ? (
                                      <span className={clsx('inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium border', getSeverityBadgeClass(c.maxSeverity))}>
                                        <span>{getSeverityIcon(c.maxSeverity)}</span>
                                        <span className="capitalize">{c.maxSeverity}</span>
                                      </span>
                                    ) : (
                                      <span className="text-slate-500">—</span>
                                    )}
                                  </td>
                                  <td className="py-2 px-2 align-middle text-right tabular-nums text-slate-400 whitespace-nowrap">
                                    {c.maxCvss != null ? c.maxCvss : '—'}
                                  </td>
                                  <td
                                    className="py-2 px-2 align-middle font-mono text-slate-400 max-w-[6rem] truncate"
                                    title={c.fixVersion ?? undefined}
                                  >
                                    {c.fixVersion ?? '—'}
                                  </td>
                                  <td className="py-2 px-2 align-middle whitespace-nowrap">
                                    {mm ? (
                                      <span className="px-1.5 py-0.5 rounded text-xs bg-red-900/40 text-red-200 border border-red-800/50">Threat</span>
                                    ) : cveCount === 0 ? (
                                      <span className="px-1.5 py-0.5 rounded text-xs bg-slate-600/60 text-slate-400">Clean</span>
                                    ) : c.status ? (
                                      <span className={clsx('px-1.5 py-0.5 rounded text-xs capitalize', statusBadgeClass(c.status))}>
                                        {c.status.toLowerCase() === 'active' ? 'Not Fixed' : c.status.toLowerCase() === 'not_exploitable' ? 'Not exploitable' : c.status}
                                      </span>
                                    ) : (
                                      <span className={clsx('px-1.5 py-0.5 rounded text-xs', statusBadgeClass('active'))}>Not Fixed</span>
                                    )}
                                  </td>
                                  <td className="py-2 px-2 align-middle text-center whitespace-nowrap">
                                    {vulns.some((v) => v.exploitKnown) ? (
                                      <span className="text-amber-400" title="Public exploit available">🔥</span>
                                    ) : vulns.some((v) => v.exploitMaturity && v.exploitMaturity.toLowerCase().includes('poc')) ? (
                                      <span className="text-amber-500" title="PoC available">⚠️</span>
                                    ) : (
                                      <span className="text-slate-500" title="No known exploit">—</span>
                                    )}
                                  </td>
                                  <td className="py-2 px-2 align-middle text-center whitespace-nowrap">
                                    {vulns.length === 0 ? (
                                      <span className="text-slate-500">—</span>
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
                                  <tr className="bg-slate-800/40">
                                    <td colSpan={13} className="py-3 px-4">
                                      <div className="pl-6 space-y-2 text-sm">
                                        {vulns.map((v) => {
                                          const vStatus = (v.status ?? 'active').toLowerCase();
                                          const statusLabel = vStatus === 'active' ? 'Active' : vStatus === 'not_exploitable' ? 'Not exploitable' : (v.status ?? 'Active');
                                          return (
                                            <div
                                              key={v.id}
                                              className="flex flex-wrap items-center gap-3 py-1.5 border-b border-slate-700/50 last:border-0"
                                            >
                                              <button
                                                type="button"
                                                onClick={() => setSelectedVulnerability(v)}
                                                className="font-mono text-slate-300 hover:text-pink-400 underline cursor-pointer text-left"
                                              >
                                                {v.id}
                                              </button>
                                              <span className={clsx('inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium border', getSeverityBadgeClass(v.severity))}>
                                                {getSeverityIcon(v.severity)} <span className="capitalize">{v.severity}</span>
                                              </span>
                                              <span className="text-slate-400 tabular-nums">{v.cvssScore ?? '—'}</span>
                                              <span className="text-slate-400">
                                                Fix: {v.fixedVersion ?? '—'}
                                              </span>
                                              <span className={clsx('px-1.5 py-0.5 rounded text-xs capitalize', statusBadgeClass(v.status))}>
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
                      <p className="text-slate-500 text-sm py-4">No packages match the current filters.</p>
                    )}
                  </>
                );
              })()}
            </div>
          ) : (
            <p className="text-slate-500 text-sm">No SBOM data for this pod.</p>
          )}
        </Card>
      )}

      {activeTab === 'risks' && (
        <Card>
          <div className="space-y-4 min-w-0">
          <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between min-w-0">
            <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Related Risks</h3>
            {relatedRisks.length > 0 && (
              <span className="text-xs text-slate-400 shrink-0">
                {relatedRisks.length} finding{relatedRisks.length !== 1 ? 's' : ''} for this pod
              </span>
            )}
          </div>
          {tabLoading ? (
            <p className="text-slate-500 text-sm">Loading...</p>
          ) : relatedRisks.length > 0 ? (
            <div className="space-y-2 min-w-0">
              {relatedRisks.map((risk) => (
                <div
                  key={risk.id}
                  className="p-3 rounded-lg border border-slate-800 bg-slate-900/50 hover:border-pink-500/30 cursor-pointer flex flex-col gap-1"
                  onClick={() => navigate(`/risks/${risk.id}`)}
                >
                  <div className="flex items-center justify-between gap-3">
                    <span className="font-medium text-white text-sm line-clamp-1">{risk.title}</span>
                    <span className={clsx('px-2 py-0.5 rounded text-[11px] font-medium uppercase', getSeverityBadgeClass(risk.severity))}>
                      {risk.severity}
                    </span>
                  </div>
                  {(formatRiskFindingReference(risk) || risk.insightType) && (
                    <p className="text-slate-500 text-[11px] font-mono mt-0.5">
                      {insightTypeUiLabel(risk.insightType)}
                      {formatRiskFindingReference(risk) ? ` · ${formatRiskFindingReference(risk)}` : ''}
                    </p>
                  )}
                  {risk.description && <p className="text-slate-500 text-xs mt-0.5 line-clamp-2">{risk.description}</p>}
                </div>
              ))}
            </div>
          ) : (
            <p className="text-slate-500 text-sm">No related risks for this pod.</p>
          )}
          </div>
        </Card>
      )}

      {activeTab === 'metrics' && (
        <Card>
          <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2 flex-wrap">
            <BarChart2 className="w-5 h-5 text-pink-500 shrink-0" /> Runtime metrics
          </h3>
          {tabLoading ? (
            <p className="text-slate-500 text-sm">Loading...</p>
          ) : runtimeMetrics.length > 0 ? (
            <div className="rounded-lg border border-slate-800 bg-slate-950/30 overflow-hidden -mx-1 sm:mx-0">
              <div className="ui-table-scroll max-h-[min(70vh,36rem)]">
              <table className="w-full min-w-[720px] text-sm border-collapse">
                <thead>
                  <tr className="border-b border-slate-800 bg-slate-900/95 backdrop-blur-sm sticky top-0 z-10">
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Container</th>
                    <th className="px-3 py-2.5 text-right text-xs font-semibold uppercase tracking-wide text-slate-500">CPU (m)</th>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Memory</th>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Limit</th>
                    <th className="px-3 py-2.5 text-right text-xs font-semibold uppercase tracking-wide text-slate-500">Restarts</th>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">State</th>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Last observed</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {runtimeMetrics.map((m, i) => (
                    <tr key={m.id ?? i} className="hover:bg-muted/30">
                      <td className="px-3 py-2 font-mono text-slate-300 max-w-[10rem] truncate align-middle" title={m.containerName ?? undefined}>{m.containerName ?? '—'}</td>
                      <td className="px-3 py-2 tabular-nums text-right align-middle whitespace-nowrap">{m.cpuUsageMillicore != null ? m.cpuUsageMillicore : '—'}</td>
                      <td className="px-3 py-2 tabular-nums font-mono text-slate-400 align-middle whitespace-nowrap">
                        {m.memoryUsageBytes != null && m.memoryUsageBytes > 0
                          ? (m.memoryUsageBytes >= 1024 * 1024 ? `${(m.memoryUsageBytes / 1024 / 1024).toFixed(1)} MB` : `${(m.memoryUsageBytes / 1024).toFixed(1)} KB`)
                          : '—'}
                      </td>
                      <td className="px-3 py-2 tabular-nums font-mono text-slate-400 align-middle whitespace-nowrap">
                        {m.memoryLimitBytes != null && m.memoryLimitBytes > 0
                          ? (m.memoryLimitBytes >= 1024 * 1024 ? `${(m.memoryLimitBytes / 1024 / 1024).toFixed(1)} MB` : `${(m.memoryLimitBytes / 1024).toFixed(1)} KB`)
                          : '—'}
                      </td>
                      <td className="px-3 py-2 tabular-nums text-right align-middle">{m.restartCount ?? 0}</td>
                      <td className="px-3 py-2 align-middle whitespace-nowrap">
                        <span className={clsx('px-2 py-0.5 rounded text-xs', m.state === 'Running' ? 'bg-emerald-600/80 text-white' : 'bg-slate-600 text-slate-200')}>
                          {m.state ?? '—'}
                        </span>
                      </td>
                      <td className="px-3 py-2 text-slate-500 text-xs whitespace-nowrap align-middle">{m.lastObservedAt ? formatDateTime(m.lastObservedAt) : '—'}</td>
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

      {activeTab === 'processes' && (
        <Card>
          <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2 flex-wrap">
            <Cpu className="w-5 h-5 text-pink-500 shrink-0" /> Processes
            {processes.length > 0 && (
              <span className="text-xs font-normal text-slate-400 px-2 py-0.5 rounded bg-slate-800 max-w-full truncate">
                Runtime: {processes[0]?.runtimeSource === 'host' ? 'Host Inspection' : 'Container Exec'}
              </span>
            )}
          </h3>
          {tabLoading ? (
            <p className="text-slate-500 text-sm">Loading...</p>
          ) : processes.length > 0 ? (
            <div className="rounded-lg border border-slate-800 bg-slate-950/30 overflow-hidden -mx-1 sm:mx-0">
              <div className="ui-table-scroll max-h-[min(70vh,36rem)]">
              <table className="w-full min-w-[960px] text-sm border-collapse">
                <thead>
                  <tr className="border-b border-slate-800 bg-slate-900/95 backdrop-blur-sm sticky top-0 z-10">
                    <th className="px-3 py-2.5 text-right text-xs font-semibold uppercase tracking-wide text-slate-500">PID</th>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">User</th>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">UID:GID</th>
                    <th className="px-3 py-2.5 text-right text-xs font-semibold uppercase tracking-wide text-slate-500">CPU %</th>
                    <th className="px-3 py-2.5 text-right text-xs font-semibold uppercase tracking-wide text-slate-500">Mem %</th>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Command</th>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">CWD</th>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">CapEff</th>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Start</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {processes.map((proc, i) => (
                    <tr key={proc.id ?? i} className="hover:bg-muted/30">
                      <td className="px-3 py-2 tabular-nums font-mono text-right align-middle whitespace-nowrap">{proc.pid}</td>
                      <td className="px-3 py-2 text-slate-300 align-middle max-w-[6rem] truncate" title={proc.userName ?? undefined}>{proc.userName ?? '—'}</td>
                      <td className="px-3 py-2 font-mono text-slate-300 align-middle whitespace-nowrap">
                        {(proc.userId != null || proc.groupId != null)
                          ? `${proc.userId ?? '—'}:${proc.groupId ?? '—'}`
                          : '—'}
                      </td>
                      <td className="px-3 py-2 tabular-nums text-right align-middle">{proc.cpuPercent != null ? proc.cpuPercent.toFixed(1) : '—'}</td>
                      <td className="px-3 py-2 tabular-nums text-right align-middle">{proc.memoryPercent != null ? proc.memoryPercent.toFixed(1) : '—'}</td>
                      <td className="px-3 py-2 font-mono text-slate-400 max-w-[14rem] truncate align-middle" title={proc.command}>{proc.command ?? '—'}</td>
                      <td className="px-3 py-2 font-mono text-slate-500 max-w-[10rem] truncate align-middle" title={proc.workingDir}>{proc.workingDir ?? '—'}</td>
                      <td className="px-3 py-2 font-mono text-slate-500 max-w-[8rem] truncate align-middle" title={proc.capEff ?? undefined}>{proc.capEff ?? '—'}</td>
                      <td className="px-3 py-2 text-slate-500 text-xs whitespace-nowrap align-middle">{proc.startedAt ? formatDateTime(proc.startedAt) : '—'}</td>
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
          <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2 flex-wrap">
            <Network className="w-5 h-5 text-pink-500 shrink-0" /> Network connections
            {networkConnections.length > 0 && (
              <span className="text-xs font-normal text-slate-400 px-2 py-0.5 rounded bg-slate-800 max-w-full truncate">
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
                className={clsx('px-3 py-1 rounded-lg text-xs font-medium transition-colors', networkSubView === 'summary' ? 'bg-pink-600 text-white' : 'bg-slate-800 text-slate-300 hover:bg-slate-700')}
              >
                Summary & Top Destinations
              </button>
              <button
                type="button"
                onClick={() => setNetworkSubView('raw')}
                className={clsx('px-3 py-1 rounded-lg text-xs font-medium transition-colors', networkSubView === 'raw' ? 'bg-pink-600 text-white' : 'bg-slate-800 text-slate-300 hover:bg-slate-700')}
              >
                Raw connections ({networkConnections.length})
              </button>
            </div>
          )}

          {tabLoading ? (
            <p className="text-slate-500 text-sm">Loading...</p>
          ) : networkConnections.length > 0 ? (
            networkSubView === 'summary' ? (
              <PodNetworkSummary
                connections={networkConnections}
                topDestinations={networkTopDestinations}
                podIP={pod?.podIP}
                loading={tabLoading}
              />
            ) : (
            <div className="rounded-lg border border-slate-800 bg-slate-950/30 overflow-hidden -mx-1 sm:mx-0">
              <div className="ui-table-scroll max-h-[min(70vh,36rem)]">
              <table className="w-full min-w-[900px] text-sm border-collapse">
                <thead>
                  <tr className="border-b border-slate-800 bg-slate-900/95 backdrop-blur-sm sticky top-0 z-10">
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Dir</th>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Remote</th>
                    <th className="px-3 py-2.5 text-right text-xs font-semibold uppercase tracking-wide text-slate-500">L.port</th>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Proto</th>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Status</th>
                    <th className="px-3 py-2.5 text-right text-xs font-semibold uppercase tracking-wide text-slate-500" title="Kernel transmit queue snapshot from /proc networking data">
                      Tx Q
                    </th>
                    <th className="px-3 py-2.5 text-right text-xs font-semibold uppercase tracking-wide text-slate-500" title="Kernel receive queue snapshot from /proc networking data">
                      Rx Q
                    </th>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Time</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {networkConnections.map((conn, i) => {
                    const podIP = (pod?.podIP ?? '').trim();
                    const isListen = (conn.state ?? '').toUpperCase() === 'LISTEN';
                    const srcIsPod = podIP && (conn.sourceIp === podIP || conn.sourceIp === '0.0.0.0' || conn.sourceIp === '::');
                    const isOutbound = isListen ? false : srcIsPod;
                    const direction = isOutbound ? 'Outbound' : 'Inbound';
                    const remoteAddr = isOutbound ? `${conn.destIp ?? '—'}:${conn.destPort ?? 0}` : `${conn.sourceIp ?? '—'}:${conn.sourcePort ?? 0}`;
                    const localPort = isOutbound ? (conn.sourcePort ?? 0) : (conn.destPort ?? 0);
                    return (
                      <tr key={conn.id ?? i} className="hover:bg-muted/30">
                        <td className="px-3 py-2 align-middle whitespace-nowrap">
                          <span className={clsx('px-2 py-0.5 rounded text-xs', direction === 'Outbound' ? 'bg-sky-600/80 text-white' : 'bg-slate-600 text-slate-200')}>
                            {direction === 'Outbound' ? 'Out' : 'In'}
                          </span>
                        </td>
                        <td className="px-3 py-2 font-mono text-xs max-w-[14rem] truncate align-middle" title={remoteAddr}>{remoteAddr}</td>
                        <td className="px-3 py-2 tabular-nums text-right align-middle">{localPort || '—'}</td>
                        <td className="px-3 py-2 align-middle whitespace-nowrap">{conn.protocol ?? '—'}</td>
                        <td className="px-3 py-2 text-slate-400 align-middle max-w-[6rem] truncate" title={conn.state ?? undefined}>{conn.state ?? '—'}</td>
                        <td className="px-3 py-2 tabular-nums font-mono text-slate-400 text-right align-middle">{conn.bytesSent ?? 0}</td>
                        <td className="px-3 py-2 tabular-nums font-mono text-slate-400 text-right align-middle">{conn.bytesRecv ?? 0}</td>
                        <td className="px-3 py-2 text-slate-500 text-xs whitespace-nowrap align-middle">{conn.observedAt ? formatDateTime(conn.observedAt) : '—'}</td>
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
              description="Network connections are collected by the agent. Enable network collection on the agent. For cluster-wide topology, use Dashboard → Network activity → Topology graph."
              className="py-6"
            />
          )}
        </Card>
      )}

      {activeTab === 'spec' && (
        <Card>
          <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2 flex-wrap">
            <FileCode className="w-5 h-5 text-pink-500 shrink-0" /> Pod Specification (YAML)
          </h3>
          {tabLoading ? (
            <p className="text-slate-500 text-sm">Loading...</p>
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
                    } catch (e) {
                      console.error(e);
                    }
                  }}
                >
                  <Download className="w-4 h-4 mr-2" /> Download YAML
                </Button>
              </div>
              <pre className="ui-code-scroll p-3 sm:p-4 rounded-lg bg-slate-900 border border-slate-800 text-xs sm:text-sm font-mono text-slate-300 whitespace-pre-wrap break-words max-w-full overflow-x-auto">
                {specYaml || 'No spec data.'}
              </pre>
            </>
          )}
        </Card>
      )}

      {activeTab === 'events' && (
        <Card>
          <div className="mb-2 min-w-0">
            <h3 className="text-lg font-semibold text-white flex items-center gap-2 flex-wrap">
              <Activity className="w-5 h-5 text-pink-500 shrink-0" /> Events
            </h3>
            <p className="text-xs text-slate-500 mt-1">
              Kubernetes API events, deduplicated runtime signals, and raw security runtime events (e.g. Falco → Core ingest).
              For coverage breakdown, see the <button type="button" className="text-pink-400 hover:underline" onClick={() => setActiveTab('coverage')}>Coverage</button> tab.
            </p>
          </div>

          <div className="mb-8 pb-6 border-b border-slate-800">
            <h4 className="text-sm font-semibold text-white mb-2 flex items-center gap-2">
              <Shield className="w-4 h-4 text-violet-400" /> Security runtime events
            </h4>
            <p className="text-xs text-slate-500 mb-3">
              Stored in Core as <code className="text-slate-400">runtime_events</code> (source: Falco JSONL, eBPF, agent). Requires pod UID in the payload to persist.
            </p>
            <div className="flex flex-wrap items-center gap-2 mb-3">
              <Button variant={secRuntimeFilter === 'all' ? 'default' : 'secondary'} size="sm" onClick={() => setSecRuntimeFilter('all')}>
                All sources
              </Button>
              <Button variant={secRuntimeFilter === 'falco' ? 'default' : 'secondary'} size="sm" onClick={() => setSecRuntimeFilter('falco')}>
                Falco
              </Button>
              <Button variant={secRuntimeFilter === 'other' ? 'default' : 'secondary'} size="sm" onClick={() => setSecRuntimeFilter('other')}>
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
                            : 'bg-slate-600/80 text-slate-200 border-slate-500/50';
                    return (
                      <div key={ev.id} className="p-3 rounded-lg border border-slate-800 bg-slate-900/50 flex flex-col gap-1.5">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className={`px-2 py-0.5 rounded text-[10px] font-semibold border ${src.className}`}>{src.label}</span>
                          {ev.mitreTechnique && (
                            <span className="px-2 py-0.5 rounded text-[10px] font-mono bg-slate-800 border border-slate-600 text-amber-200/90">{ev.mitreTechnique}</span>
                          )}
                          {ev.severity && <span className={`px-2 py-0.5 rounded text-[10px] font-medium border ${sevClass}`}>{ev.severity}</span>}
                          <span className="text-[10px] text-slate-500 ml-auto tabular-nums">{ev.createdAt ? formatDateTime(ev.createdAt) : '—'}</span>
                        </div>
                        <div className="flex flex-wrap gap-x-3 gap-y-0.5 text-xs">
                          {ev.signal && <span className="text-violet-200/95 font-medium truncate max-w-full">{ev.signal}</span>}
                          {ev.eventType && <span className="text-slate-500 truncate">{ev.eventType}</span>}
                        </div>
                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-1 text-[11px] text-slate-400 font-mono">
                          <span>
                            <span className="text-slate-600">syscall </span>
                            {ev.syscall || '—'}
                          </span>
                          <span className="truncate" title={ev.targetPath}>
                            <span className="text-slate-600">target </span>
                            {ev.targetPath || '—'}
                          </span>
                        </div>
                        {ev.capability ? (
                          <p className="text-[10px] text-slate-500">
                            capability <span className="text-slate-300">{ev.capability}</span>
                          </p>
                        ) : null}
                      </div>
                    );
                  })}
                </div>
                {filtered.length > 80 && (
                  <p className="text-xs text-slate-500 mt-2">Showing 80 of {filtered.length} events. Use the Coverage tab for full breakdown.</p>
                )}
                </>
              );
            })()}
          </div>

          <h3 className="text-base font-semibold text-white mb-4 flex items-center gap-2 flex-wrap min-w-0">
            <Activity className="w-5 h-5 text-cyan-400 shrink-0" /> Runtime signals &amp; Kubernetes events
          </h3>
          <div className="mb-6 min-w-0">
            {signalStats && (
              <div className="mb-3 flex flex-wrap items-center gap-2 text-xs text-slate-400">
                <span className="px-2 py-0.5 rounded bg-slate-800 border border-slate-700 max-w-full break-words">Network anomaly events (60m): {signalStats.emittedEvents}</span>
                <span className="px-2 py-0.5 rounded bg-slate-800 border border-slate-700">keys: {signalStats.uniqueKeys}</span>
                <span className="px-2 py-0.5 rounded bg-slate-800 border border-slate-700">max ratio: {Number(signalStats.maxRatio ?? 0).toFixed(2)}</span>
              </div>
            )}
            <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between mb-3 min-w-0">
              <h4 className="text-sm font-semibold text-white min-w-0">Runtime signals (normalized, last 24h)</h4>
              <div className="flex flex-wrap items-center gap-2 shrink-0">
                <Button
                  variant={runtimeSignalFilter === 'all' ? 'default' : 'secondary'}
                  size="sm"
                  onClick={() => setRuntimeSignalFilter('all')}
                >
                  All
                </Button>
                <Button
                  variant={runtimeSignalFilter === 'NETWORK_QUEUE_ANOMALY' ? 'default' : 'secondary'}
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
                return <p className="text-slate-500 text-sm">No runtime signals.</p>;
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
                        className="p-3 rounded-lg border border-slate-800 bg-slate-900/50 flex items-center justify-between gap-3"
                      >
                        <div className="min-w-0">
                          <div className="flex items-center gap-2">
                            <span className={`px-2 py-0.5 rounded text-[11px] font-semibold border ${runtimeSignalVisual(s.signalType).signalClass}`}>
                              {s.signalType}
                            </span>
                            <span className={`px-2 py-0.5 rounded text-[10px] font-medium border ${runtimeSignalVisual(s.signalType).severityClass}`}>
                              {runtimeSignalVisual(s.signalType).severity}
                            </span>
                          </div>
                          <p className="text-xs text-slate-400">
                            {s.category} · confidence {Number(s.confidence ?? 0).toFixed(2)}
                          </p>
                          <p className="text-[11px] text-slate-500 mt-1">
                            evidence: syscall <span className="text-slate-300">{evSyscall || '—'}</span> · target{' '}
                            <span className="text-slate-300 break-all">{evTarget || '—'}</span>
                          </p>
                        </div>
                        <span className="text-xs text-slate-500 whitespace-nowrap">{s.createdAt ? formatDateTime(s.createdAt) : '—'}</span>
                      </div>
                    );
                  })}
                </div>
                {filteredSignals.length > 20 && (
                  <p className="text-xs text-slate-500 mt-2">Showing 20 of {filteredSignals.length} signals.</p>
                )}
                </>
              );
            })()}
          </div>
          <Card variant="panel" className="mb-6 bg-slate-900/30 border-slate-800 min-w-0">
            <p className="text-xs text-slate-400">
              Facts, incidents, capabilities and insights are split into dedicated views to reduce noise:
              <span className="text-slate-200"> Runtime Timeline</span>, <span className="text-slate-200">Coverage</span>, and
              <span className="text-slate-200"> Related Risks</span>.
            </p>
          </Card>
          {tabLoading ? (
            <p className="text-slate-500 text-sm">Loading...</p>
          ) : podEvents.length > 0 ? (
            <div className="space-y-2">
              {podEvents.map((ev, i) => (
                <div key={ev.id ?? i} className="p-3 rounded-lg border border-slate-800 bg-slate-900/50 flex flex-col gap-1">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className={clsx('px-2 py-0.5 rounded text-xs font-medium', ev.eventType === 'Warning' ? 'bg-amber-600/80 text-white' : 'bg-slate-600 text-slate-200')}>
                      {ev.eventType ?? 'Normal'}
                    </span>
                    <span className="font-medium text-white">{ev.reason ?? '—'}</span>
                    {ev.lastTimestamp && <span className="text-slate-500 text-xs">{formatDateTime(ev.lastTimestamp)}</span>}
                  </div>
                  {ev.message && <p className="text-slate-400 text-sm">{ev.message}</p>}
                  {ev.involvedName && <p className="text-slate-500 text-xs">Object: {ev.involvedName}</p>}
                </div>
              ))}
            </div>
          ) : (
            <PageEmpty title="No events" description="Kubernetes events for this pod are collected by the agent." className="py-6" />
          )}
        </Card>
      )}

      {activeTab === 'timeline' && (
        <Card>
          <h3 className="text-lg font-semibold text-white mb-2 flex items-center gap-2 flex-wrap">
            <Activity className="w-5 h-5 text-pink-500 shrink-0" /> Runtime Timeline
          </h3>
          <p className="text-xs text-slate-500 mb-4">
            Incident-first timeline with correlated facts, capabilities, and insights for this pod.
          </p>
          {tabLoading ? (
            <p className="text-slate-500 text-sm">Loading...</p>
          ) : runtimeIncidents.length === 0 ? (
            <>
              <PageEmpty title="No runtime incidents" description="No stateful incidents found in the selected lookback window." className="py-6" />
              {runtimeDataHints().length > 0 && (
                <Card variant="panel" className="mt-3 bg-slate-900/40 border-slate-800 min-w-0">
                  <p className="text-xs text-slate-400 mb-2">Diagnostics</p>
                  <ul className="space-y-1">
                    {runtimeDataHints().map((h) => (
                      <li key={h} className="text-xs text-slate-500">- {h}</li>
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
                  <div key={inc.id} className="p-3 rounded-lg border border-slate-800 bg-slate-900/50">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="px-2 py-0.5 rounded text-[11px] font-semibold border border-slate-700 bg-slate-800 text-slate-200">
                        {inc.incidentType}
                      </span>
                      <span className="px-2 py-0.5 rounded text-[10px] border border-slate-700 bg-slate-900 text-slate-300">
                        {inc.severityHint ?? '—'}
                      </span>
                      <span className="text-[11px] text-slate-500 ml-auto">
                        {inc.lastSeenAt ? formatDateTime(inc.lastSeenAt) : (inc.createdAt ? formatDateTime(inc.createdAt) : '—')}
                      </span>
                    </div>
                    <p className="text-xs text-slate-400 mt-1">
                      confidence {Number(inc.confidence ?? 0).toFixed(2)} · window {inc.window ?? '—'}
                    </p>
                    <p className="text-[11px] text-slate-500 mt-1">
                      first {inc.firstSeenAt ? formatDateTime(inc.firstSeenAt) : '—'} → last {inc.lastSeenAt ? formatDateTime(inc.lastSeenAt) : '—'}
                    </p>
                  </div>
                ))}
            </div>
          )}
          <div className="mt-6 grid grid-cols-1 lg:grid-cols-3 gap-3">
            <Card variant="panel" className="bg-slate-900/40 border-slate-800 min-w-0">
              <p className="text-[11px] text-slate-500 mb-1">Facts in scope</p>
              <p className="text-sm text-slate-200">{runtimeFacts.length}</p>
            </Card>
            <Card variant="panel" className="bg-slate-900/40 border-slate-800 min-w-0">
              <p className="text-[11px] text-slate-500 mb-1">Capabilities in scope</p>
              <p className="text-sm text-slate-200">{podCapabilities.length}</p>
            </Card>
            <Card variant="panel" className="bg-slate-900/40 border-slate-800 min-w-0">
              <p className="text-[11px] text-slate-500 mb-1">Insights in report</p>
              <p className="text-sm text-slate-200">{relatedRisks.length}</p>
            </Card>
          </div>
        </Card>
      )}

      {activeTab === 'coverage' && (
        <Card>
          <h3 className="text-lg font-semibold text-white mb-2 flex items-center gap-2 flex-wrap">
            <Shield className="w-5 h-5 text-pink-500 shrink-0" /> Runtime Coverage
          </h3>
          <p className="text-xs text-slate-500 mb-4">
            Coverage lens by source, layer, domain, signal type, and MITRE tags.
          </p>
          {tabLoading ? (
            <p className="text-slate-500 text-sm">Loading...</p>
          ) : (
            <>
              {runtimeDataHints().length > 0 && (
                <Card variant="panel" className="mb-4 bg-slate-900/40 border-slate-800 min-w-0">
                  <p className="text-xs text-slate-400 mb-2">Data availability diagnostics</p>
                  <ul className="space-y-1">
                    {runtimeDataHints().map((h) => (
                      <li key={h} className="text-xs text-slate-500">- {h}</li>
                    ))}
                  </ul>
                </Card>
              )}
              <div className="mb-4 grid grid-cols-1 md:grid-cols-3 gap-3">
                <Card variant="panel" className="bg-slate-900/40 border-slate-800 min-w-0">
                  <p className="text-[11px] text-slate-500 mb-1">Coverage by source</p>
                  <p className="text-xs text-slate-300">
                    Falco {runtimeSecurityEvents.filter((e) => (e.runtime || '').toLowerCase() === 'falco').length} · Other{' '}
                    {runtimeSecurityEvents.filter((e) => (e.runtime || '').toLowerCase() !== 'falco').length}
                  </p>
                </Card>
                <Card variant="panel" className="bg-slate-900/40 border-slate-800 min-w-0">
                  <p className="text-[11px] text-slate-500 mb-1">Coverage by layer</p>
                  <p className="text-xs text-slate-300">
                    Events {runtimeSecurityEvents.length} · Facts {runtimeFacts.length} · Signals {runtimeSignals.length} · Incidents {runtimeIncidents.length}
                  </p>
                </Card>
                <Card variant="panel" className="bg-slate-900/40 border-slate-800 min-w-0">
                  <p className="text-[11px] text-slate-500 mb-1">Coverage by MITRE tags</p>
                  <p className="text-xs text-slate-300">
                    {new Set(runtimeSecurityEvents.map((e) => (e.mitreTechnique || '').trim()).filter(Boolean)).size} distinct techniques
                  </p>
                </Card>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                <Card variant="panel" className="bg-slate-900/40 border-slate-800 min-w-0">
                  <p className="text-[11px] text-slate-500 mb-2">Fact domain distribution</p>
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
                        <span key={domain} className="px-2 py-0.5 rounded text-[10px] border border-slate-700 bg-slate-900 text-slate-300">
                          {domain}: {count}
                        </span>
                      ))}
                    {runtimeFacts.length === 0 ? <span className="text-xs text-slate-500">No fact coverage yet.</span> : null}
                  </div>
                </Card>
                <Card variant="panel" className="bg-slate-900/40 border-slate-800 min-w-0">
                  <p className="text-[11px] text-slate-500 mb-2">Signal type distribution</p>
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
                        <span key={signalType} className="px-2 py-0.5 rounded text-[10px] border border-slate-700 bg-slate-900 text-slate-300">
                          {signalType}: {count}
                        </span>
                      ))}
                    {runtimeSignals.length === 0 ? <span className="text-xs text-slate-500">No signal coverage yet.</span> : null}
                  </div>
                </Card>
              </div>
            </>
          )}
        </Card>
      )}

      {selectedVulnerability && (
        <div className="fixed inset-y-0 right-0 w-full max-w-md bg-slate-900 border-l border-slate-800 shadow-2xl z-50 flex flex-col animate-in slide-in-from-right-4 duration-200">
          <div className="flex justify-between items-center p-4 border-b border-slate-800 bg-slate-950/80">
            <div className="flex items-center gap-3">
              <div className={clsx('p-2 rounded-lg', getSeverityBadgeClass(selectedVulnerability.severity))}>
                <ShieldAlert className="w-5 h-5" />
              </div>
              <div>
                <h3 className="text-lg font-bold text-white font-mono">{selectedVulnerability.id}</h3>
                <span className={clsx('px-2 py-0.5 rounded text-xs font-medium border capitalize', getSeverityBadgeClass(selectedVulnerability.severity))}>
                  {selectedVulnerability.severity}
                </span>
              </div>
            </div>
            <button type="button" onClick={() => setSelectedVulnerability(null)} className="p-2 rounded-lg text-slate-400 hover:bg-slate-800 hover:text-white">
              <X className="w-5 h-5" />
            </button>
          </div>
          <div className="p-4 space-y-4 overflow-y-auto flex-1">
            <div className="flex items-center justify-between p-3 bg-slate-800/50 rounded-lg border border-slate-700">
              <div>
                <p className="text-[10px] text-slate-500 uppercase font-semibold mb-0.5">CVSS</p>
                <p className={clsx('text-2xl font-bold', (selectedVulnerability.cvssScore ?? 0) >= 7 ? 'text-red-400' : (selectedVulnerability.cvssScore ?? 0) >= 4 ? 'text-amber-400' : 'text-slate-300')}>
                  {selectedVulnerability.cvssScore?.toFixed(1) ?? '—'}
                </p>
              </div>
              <div>
                <p className="text-[10px] text-slate-500 uppercase font-semibold mb-0.5">Status</p>
                <p className="text-sm font-medium text-slate-300 capitalize">{selectedVulnerability.status ?? 'active'}</p>
              </div>
            </div>
            {selectedVulnerability.description && (
              <div>
                <p className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider mb-2 flex items-center gap-1">
                  <FileText className="w-3.5 h-3.5" /> Description
                </p>
                <p className="text-sm text-slate-300 leading-relaxed bg-slate-800/30 p-3 rounded-lg border border-slate-700">
                  {selectedVulnerability.description}
                </p>
              </div>
            )}
            {selectedVulnerability.fixedVersion ? (
              <div className="p-3 bg-emerald-500/10 border border-emerald-500/20 rounded-lg">
                <div className="flex items-center gap-2 text-emerald-400 mb-1">
                  <CheckCircle2 className="w-4 h-4" />
                  <span className="text-xs font-semibold uppercase">Remediation</span>
                </div>
                <p className="text-sm text-slate-300">
                  Fix available in version <span className="font-mono text-emerald-300">{selectedVulnerability.fixedVersion}</span>
                </p>
              </div>
            ) : (
              <div className="p-3 bg-amber-500/10 border border-amber-500/20 rounded-lg">
                <div className="flex items-center gap-2 text-amber-400 mb-1">
                  <Info className="w-4 h-4" />
                  <span className="text-xs font-semibold uppercase">No fix version yet</span>
                </div>
                <p className="text-sm text-slate-300">Monitor advisories for updates.</p>
              </div>
            )}
            {/* GAP 12: Exploit maturity & allowed status */}
            {(selectedVulnerability.exploitKnown || selectedVulnerability.exploitMaturity || selectedVulnerability.allowed != null) && (
              <div className="space-y-2">
                {(selectedVulnerability.exploitKnown || selectedVulnerability.exploitMaturity) && (
                  <div className="p-3 bg-slate-800/50 rounded-lg border border-slate-700">
                    <p className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider mb-2">Exploit Intelligence</p>
                    <div className="flex flex-wrap gap-3 text-sm text-slate-300">
                      {selectedVulnerability.exploitKnown && (
                        <span className="flex items-center gap-1.5">
                          <span className="text-amber-400">🔥</span> Public exploit known
                        </span>
                      )}
                      {selectedVulnerability.exploitMaturity && (
                        <span>Maturity: <span className="font-medium text-slate-200">{selectedVulnerability.exploitMaturity}</span></span>
                      )}
                    </div>
                  </div>
                )}
                {selectedVulnerability.allowed != null && (
                  <div className={clsx('p-3 rounded-lg border', selectedVulnerability.allowed ? 'bg-emerald-500/10 border-emerald-500/20' : 'bg-red-500/10 border-red-500/20')}>
                    <p className="text-sm text-slate-300">
                      Policy: <span className={clsx('font-medium', selectedVulnerability.allowed ? 'text-emerald-300' : 'text-red-300')}>
                        {selectedVulnerability.allowed ? '✅ Allowed by policy' : '❌ Not allowed'}
                      </span>
                    </p>
                  </div>
                )}
              </div>
            )}
            <Button className="w-full" size="sm" onClick={() => window.open(`https://nvd.nist.gov/vuln/detail/${selectedVulnerability.id}`, '_blank')}>
              <ExternalLink className="w-4 h-4 mr-2" /> View on NVD
            </Button>
          </div>
        </div>
      )}

      <Card className="mt-8 min-w-0" variant="secondary">
        <div className="flex items-center justify-between mb-3 min-w-0">
          <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Related navigation</h3>
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
