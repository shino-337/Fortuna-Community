import React, { useCallback, useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { PodWithRisk, PodSbom, Insight, Vulnerability } from '../types';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Tabs } from '../design-system/components/Tabs';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { PageLoading } from '../components/PageLoading';
import { PageEmpty } from '../components/PageEmpty';
import { ArrowLeft, Box, Package, ShieldAlert, Globe, Download, ChevronDown, ChevronRight, X, FileText, ExternalLink, CheckCircle2, Info, Cpu, Network, Activity, BarChart2, FileCode } from 'lucide-react';
import clsx from 'clsx';
import { getSeverityBadgeClass, getSeverityBarClass, getSeverityTextClass, getSeverityIcon, getPodStatusBadgeClass } from '../lib/severity';
import { formatDateTime, formatUptime } from '../lib/display';
import { exportSbomAsCsv, exportSbomAsJson } from '../lib/exportSbom';
import { useAuthStore } from '../store/authStore';
import type { SbomComponent as SbomComponentType, PodRuntimeMetric, PodProcessItem, PodNetworkConnectionItem, PodK8sEventItem } from '../types';

type TabId = 'overview' | 'sbom' | 'risks' | 'metrics' | 'processes' | 'network' | 'events' | 'spec';

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
  const [podEvents, setPodEvents] = useState<PodK8sEventItem[]>([]);
  const [specYaml, setSpecYaml] = useState<string>('');

  const idOrUid = uid ?? id;

  const podRiskLevel = (count: number): 'critical' | 'high' | 'medium' | 'low' => {
    if (count >= 10) return 'critical';
    if (count >= 4) return 'high';
    if (count >= 1) return 'medium';
    return 'low';
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
        if (tab === 'sbom') {
          const data = await api.getPodSbom(pod.uid);
          setSbom(data ?? null);
        } else if (tab === 'risks') {
          const { insights } = await api.getPodRiskReport(pod.uid);
          setRelatedRisks(insights);
        } else if (tab === 'processes') {
          const data = await api.getPodProcesses(pod.uid);
          setProcesses(data);
        } else if (tab === 'network') {
          const data = await api.getPodNetworkConnections(pod.uid);
          setNetworkConnections(data);
        } else if (tab === 'events') {
          const data = await api.getPodEvents(pod.uid);
          setPodEvents(data);
        } else if (tab === 'spec') {
          const yaml = await api.getPodSpecYaml(pod.uid);
          setSpecYaml(yaml);
        }
      } finally {
        setTabLoading(false);
      }
    },
    [pod]
  );

  useEffect(() => {
    fetchPod();
  }, [fetchPod]);

  // Load SBOM when pod is available (for Overview summary + SBOM tab)
  useEffect(() => {
    if (pod?.uid) {
      api.getPodSbom(pod.uid).then((data) => setSbom(data ?? null)).catch(() => setSbom(null));
    } else {
      setSbom(null);
    }
  }, [pod?.uid]);

  // Preload pod-detail (metrics, processes, network) so Overview shows counts and Network tab has data. All use pod UID.
  useEffect(() => {
    if (!pod?.uid) return;
    api.getPodRuntimeMetrics(pod.uid).then(setRuntimeMetrics).catch(() => []);
    api.getPodProcesses(pod.uid).then(setProcesses).catch(() => []);
    api.getPodNetworkConnections(pod.uid).then(setNetworkConnections).catch(() => []);
    api.getPodEvents(pod.uid).then(setPodEvents).catch(() => []);
  }, [pod?.uid]);

  useEffect(() => {
    if (pod && activeTab !== 'overview') fetchTabData(activeTab);
  }, [pod, activeTab, fetchTabData]);

  // Phase 5.1: WebSocket for live pod detail updates (metrics, processes, network, events). All APIs use pod UID.
  const wsUid = pod?.uid ?? uid ?? null;
  const hasToken = Boolean(useAuthStore((s) => s.token));
  const podUidRef = React.useRef(pod?.uid);
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
            api.getPodEvents(currentUid).then(setPodEvents).catch(() => {});
          } else {
            api.getPodRuntimeMetrics(currentUid).then(setRuntimeMetrics).catch(() => {});
            api.getPodProcesses(currentUid).then(setProcesses).catch(() => {});
            api.getPodNetworkConnections(currentUid).then(setNetworkConnections).catch(() => {});
            api.getPodEvents(currentUid).then(setPodEvents).catch(() => {});
          }
        } catch {
          api.getPodRuntimeMetrics(currentUid).then(setRuntimeMetrics).catch(() => {});
          api.getPodProcesses(currentUid).then(setProcesses).catch(() => {});
          api.getPodNetworkConnections(currentUid).then(setNetworkConnections).catch(() => {});
          api.getPodEvents(currentUid).then(setPodEvents).catch(() => {});
        }
      };
    } catch {
      // ignore WS connect errors
    }
    return () => {
      if (ws != null) ws.close();
    };
  }, [wsUid, pod?.uid, hasToken]);

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
    { id: 'spec', label: 'Spec', icon: <FileCode className="w-4 h-4" /> },
  ];

  return (
    <PageLayout
      title="Pod Detail"
      description=""
      actions={
        <Button variant="secondary" onClick={() => navigate('/resources')}>
          <ArrowLeft className="w-4 h-4 mr-2" /> Back to Resources
        </Button>
      }
    >
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-6 mb-6">
        <div className="flex items-center gap-4">
          <div className="p-4 bg-slate-900 border border-slate-800 rounded-xl">
            <Box className="w-8 h-8 text-pink-500" />
          </div>
          <div>
            <div className="flex items-center gap-3 flex-wrap">
              <h1 className="text-2xl font-bold text-white tracking-tight">{pod.name}</h1>
              <span className={clsx('px-2.5 py-0.5 rounded-full text-xs font-semibold uppercase border', getSeverityBadgeClass(podRiskLevel(pod.riskCount)))}>
                {podRiskLevel(pod.riskCount)} ({pod.riskCount})
              </span>
            </div>
            <div className="mt-1 text-slate-500 text-sm font-mono">
              <span>{pod.namespace}</span>
              <span className="mx-2 opacity-50">|</span>
              <span>{pod.nodeName ?? '—'}</span>
            </div>
          </div>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={() => sbom && exportSbomAsCsv(sbom)} disabled={!sbom?.components?.length}>
            <Download className="w-4 h-4 mr-2" /> Export SBOM
          </Button>
        </div>
      </div>

      {/* Hint when pod IP / start time are missing (filled by agent sync; wait for next sync or restart agent) */}
      {(!pod.podIP || !pod.startTime) && (
        <p className="text-slate-500 text-xs mb-2">
          Pod IP and Start time come from agent sync. If empty, wait for the next sync (~2 min) or restart the agent: <code className="bg-slate-800 px-1 rounded">kubectl rollout restart daemonset/fortuna-agent -n fortuna</code>
        </p>
      )}
      <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4 mb-6">
        <Card className="p-4 bg-slate-900/50">
          <p className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider mb-1">Status</p>
          <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border ${getPodStatusBadgeClass(pod.status ?? pod.phase)}`}>{pod.status ?? pod.phase ?? '—'}</span>
        </Card>
        <Card className="p-4 bg-slate-900/50">
          <p className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider mb-1">Pod IP</p>
          <p className="text-sm font-medium text-slate-300 font-mono">{pod.podIP ?? '—'}</p>
        </Card>
        <Card className="p-4 bg-slate-900/50">
          <p className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider mb-1">Start Time</p>
          <p className="text-sm font-medium text-slate-300">{pod.startTime ? formatDateTime(pod.startTime) : (runtimeMetrics.length > 0 && runtimeMetrics[0].lastObservedAt ? `Last reported: ${formatDateTime(runtimeMetrics[0].lastObservedAt)}` : '—')}</p>
        </Card>
        <Card className="p-4 bg-slate-900/50">
          <p className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider mb-1">Uptime</p>
          <p className="text-sm font-medium text-slate-300">{formatUptime(pod.startTime ?? undefined)}</p>
        </Card>
        <Card className="p-4 bg-slate-900/50">
          <p className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider mb-1">Restart Count</p>
          <p className="text-lg font-bold text-white">{pod.restartCount ?? 0}</p>
        </Card>
        <Card className="p-4 bg-slate-900/50">
          <p className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider mb-1">QoS Class</p>
          <p className="text-sm font-medium text-slate-300">{pod.qosClass ?? '—'}</p>
        </Card>
        <Card className="p-4 bg-slate-900/50">
          <p className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider mb-1">Risk Count</p>
          <p className="text-lg font-bold text-white">{pod.riskCount}</p>
        </Card>
        <Card className="p-4 bg-slate-900/50">
          <p className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider mb-1">Service Account</p>
          <p className="text-sm font-medium text-slate-300 truncate">{pod.serviceAccount ?? '—'}</p>
        </Card>
        <Card className="p-4 bg-slate-900/50">
          <p className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider mb-1">Created</p>
          <p className="text-sm font-medium text-slate-300">{pod.createdAt ? formatDateTime(pod.createdAt) : '—'}</p>
        </Card>
      </div>

      <Tabs items={tabs} value={activeTab} onChange={(id) => setActiveTab(id as TabId)} />

      {activeTab === 'overview' && (
        <div className="space-y-6">
        <Card className="p-5 md:p-6" variant="secondary">
            <h3 className="text-base md:text-lg font-semibold text-white mb-4 flex items-center gap-2">
              <Box className="w-5 h-5 text-pink-500" /> Overview
            </h3>
            <dl className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
              <div>
                <dt className="text-slate-500">Namespace</dt>
                <dd className="text-white font-mono">{pod.namespace}</dd>
              </div>
              <div>
                <dt className="text-slate-500">Node</dt>
                <dd className="text-slate-300 font-mono">
                  {pod.nodeName && pod.clusterId ? (
                    <button type="button" className="text-pink-400 hover:underline" onClick={() => navigate(`/clusters/${pod.clusterId}/nodes/${encodeURIComponent(pod.nodeName!)}`)}>
                      {pod.nodeName}
                    </button>
                  ) : (
                    pod.nodeName ?? '—'
                  )}
                </dd>
              </div>
              <div>
                <dt className="text-slate-500">Pod IP</dt>
                <dd className="text-slate-300 font-mono">{pod.podIP ?? '—'}</dd>
              </div>
              <div>
                <dt className="text-slate-500">Service Account</dt>
                <dd className="text-slate-300 font-mono">{pod.serviceAccount ?? '—'}</dd>
              </div>
              <div>
                <dt className="text-slate-500">UID</dt>
                <dd className="text-slate-400 font-mono text-xs break-all">{pod.uid}</dd>
              </div>
            </dl>
            {(pod.ownerKind ?? pod.ownerName ?? pod.replicaSetName ?? pod.qosClass) && (
              <div className="mt-4 pt-4 border-t border-slate-800">
                <h4 className="text-sm font-semibold text-slate-300 mb-3">Identity &amp; Ownership</h4>
                <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
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
                  {pod.qosClass && (
                    <div>
                      <dt className="text-slate-500">QoS Class</dt>
                      <dd className="text-slate-300 font-medium">{pod.qosClass}</dd>
                    </div>
                  )}
                </dl>
              </div>
            )}
            {sbom && (
              <div className="mt-4 pt-4 border-t border-slate-800">
                <h4 className="text-sm font-semibold text-slate-300 mb-3">Security</h4>
                <dl className="grid grid-cols-2 gap-x-4 gap-y-1 text-sm mb-4">
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
            {!sbom && (
              <div className="mt-4 pt-4 border-t border-slate-800">
                <p className="text-slate-500 text-sm">Security summary loading…</p>
              </div>
            )}
            <div className="mt-4 pt-4 border-t border-slate-800">
              <h4 className="text-sm font-semibold text-slate-300 mb-2 flex items-center gap-2">
                <BarChart2 className="w-4 h-4" /> Pod detail (agent)
              </h4>
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
                  View related risks in Risk Center
                </button>
              </div>
            </div>
            {pod.createdAt && (
              <dl className="grid grid-cols-1 gap-3 text-sm mt-4">
                <div>
                  <dt className="text-slate-500">Created</dt>
                  <dd className="text-slate-300">{formatDateTime(pod.createdAt)}</dd>
                </div>
              </dl>
            )}
          </Card>
        </div>
      )}

      {activeTab === 'sbom' && (
        <Card className="p-6">
          <div className="flex flex-wrap items-center justify-between gap-4 mb-4">
            <div className="flex items-center gap-2 flex-wrap">
              <h3 className="text-lg font-semibold text-white">SBOM</h3>
              {sbom?.sbomSource === 'distroless-heuristic' && (
                <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium bg-amber-500/20 text-amber-400 border border-amber-500/40" title="SBOM inferred from image ref/labels (no package DB). CVE match uses NVD fallback.">
                  <Info className="w-3.5 h-3.5" />
                  Distroless SBOM (heuristic)
                </span>
              )}
            </div>
            {sbom && (sbom.components?.length ?? 0) > 0 && (
              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => exportSbomAsCsv(sbom)}
                  title="Download SBOM as CSV"
                >
                  <Download className="w-4 h-4 mr-2" />
                  Export CSV
                </Button>
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => exportSbomAsJson(sbom)}
                  title="Download SBOM as JSON"
                >
                  <Download className="w-4 h-4 mr-2" />
                  Export JSON
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
                    if (sbomOnlyVulnerable && (c.cveCount ?? c.vulnerabilities?.length ?? 0) === 0) return false;
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
                    <div className="flex flex-wrap items-center gap-4 py-2 border-y border-slate-800">
                      <div className="flex items-center gap-2">
                        <span className="text-slate-500 text-sm">Severity:</span>
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
                      <div className="flex items-center gap-2">
                        <span className="text-slate-500 text-sm">Status:</span>
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
                      <label className="flex items-center gap-2 text-slate-400 text-sm cursor-pointer">
                        <input
                          type="checkbox"
                          checked={sbomOnlyVulnerable}
                          onChange={(e) => setSbomOnlyVulnerable(e.target.checked)}
                          className="rounded border-slate-600 bg-slate-800 text-pink-500"
                        />
                        Show only vulnerable packages
                      </label>
                      <div className="flex items-center gap-2">
                        <span className="text-slate-500 text-sm">Search:</span>
                        <input
                          type="text"
                          value={sbomSearch}
                          onChange={(e) => setSbomSearch(e.target.value)}
                          placeholder="Package or version..."
                          className="bg-slate-950 border border-slate-700 rounded px-2 py-1 text-xs text-slate-200 w-40"
                        />
                      </div>
                      <div className="flex items-center gap-2">
                        <span className="text-slate-500 text-sm">Sort:</span>
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
                          CVE count
                        </button>
                      </div>
                    </div>
                    <div className="overflow-x-auto max-h-[500px] overflow-y-auto">
                      <table className="w-full text-sm">
                        <thead className="text-slate-400 border-b border-slate-800 sticky top-0 bg-slate-900 z-10">
                          <tr>
                            <th className="w-8 py-2" />
                            <th className="text-left py-2">Package</th>
                            <th className="text-left py-2">Version</th>
                            <th className="text-left py-2">Type</th>
                            <th className="text-left py-2">CVE</th>
                            <th className="text-left py-2">Max Severity</th>
                            <th className="text-left py-2">CVSS</th>
                            <th className="text-left py-2">Fix</th>
                            <th className="text-left py-2">Status</th>
                            <th className="text-left py-2">Exploit</th>
                            <th className="text-left py-2">Allowed</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-slate-800">
                          {filtered.map((c, i) => {
                            const rowId = `${c.name}@${c.version ?? i}`;
                            const vulns = c.vulnerabilities ?? [];
                            const cveCount = c.cveCount ?? vulns.length;
                            const isExpanded = sbomExpandedId === rowId;
                            const hasCves = cveCount > 0;
                            return (
                              <React.Fragment key={rowId}>
                                <tr
                                  className={clsx(
                                    'border-b border-slate-800',
                                    hasCves ? 'cursor-pointer hover:bg-slate-800/50' : ''
                                  )}
                                  onClick={() => hasCves && setSbomExpandedId(isExpanded ? null : rowId)}
                                >
                                  <td className="py-2 w-8">
                                    {hasCves ? (
                                      isExpanded ? (
                                        <ChevronDown className="w-4 h-4 text-slate-400" />
                                      ) : (
                                        <ChevronRight className="w-4 h-4 text-slate-400" />
                                      )
                                    ) : (
                                      <span className="w-4 inline-block" />
                                    )}
                                  </td>
                                  <td className="py-2 font-mono text-white">{c.name}</td>
                                  <td className="py-2 text-slate-400 font-mono">{c.version ?? '—'}</td>
                                  <td className="py-2 text-slate-500 font-mono">{sbomTypeLabel(c.type)}</td>
                                  <td className="py-2">
                                    {cveCount > 0 ? (
                                      <span className="text-amber-400 font-medium">{cveCount}</span>
                                    ) : (
                                      <span className="text-slate-500">0</span>
                                    )}
                                  </td>
                                  <td className="py-2">
                                    {c.maxSeverity ? (
                                      <span className={clsx('inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium border', getSeverityBadgeClass(c.maxSeverity))}>
                                        <span>{getSeverityIcon(c.maxSeverity)}</span>
                                        <span className="capitalize">{c.maxSeverity}</span>
                                      </span>
                                    ) : (
                                      '—'
                                    )}
                                  </td>
                                  <td className="py-2 text-slate-400">{c.maxCvss != null ? c.maxCvss : '—'}</td>
                                  <td className="py-2 font-mono text-slate-400">{c.fixVersion ?? '—'}</td>
                                  <td className="py-2">
                                    {cveCount === 0 ? (
                                      <span className="px-1.5 py-0.5 rounded text-xs bg-slate-600/60 text-slate-400">Clean</span>
                                    ) : c.status ? (
                                      <span className={clsx('px-1.5 py-0.5 rounded text-xs capitalize', statusBadgeClass(c.status))}>
                                        {c.status.toLowerCase() === 'active' ? 'Not Fixed' : c.status.toLowerCase() === 'not_exploitable' ? 'Not exploitable' : c.status}
                                      </span>
                                    ) : (
                                      <span className={clsx('px-1.5 py-0.5 rounded text-xs', statusBadgeClass('active'))}>Not Fixed</span>
                                    )}
                                  </td>
                                  <td className="py-2">
                                    {vulns.some((v) => v.exploitKnown) ? (
                                      <span className="text-amber-400" title="Public exploit available">🔥</span>
                                    ) : vulns.some((v) => v.exploitMaturity && v.exploitMaturity.toLowerCase().includes('poc')) ? (
                                      <span className="text-amber-500" title="PoC available">⚠️</span>
                                    ) : (
                                      <span className="text-slate-500" title="No known exploit">—</span>
                                    )}
                                  </td>
                                  <td className="py-2">
                                    {vulns.length === 0 ? (
                                      '—'
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
                                    <td colSpan={11} className="py-3 px-4">
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
        <Card className="p-6 space-y-4">
          <div className="flex items-center justify-between gap-2">
            <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Related Risks</h3>
            {relatedRisks.length > 0 && (
              <span className="text-xs text-slate-400">
                {relatedRisks.length} finding{relatedRisks.length !== 1 ? 's' : ''} for this pod
              </span>
            )}
          </div>
          {tabLoading ? (
            <p className="text-slate-500 text-sm">Loading...</p>
          ) : relatedRisks.length > 0 ? (
            <div className="space-y-2">
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
                  {risk.description && <p className="text-slate-500 text-xs mt-0.5 line-clamp-2">{risk.description}</p>}
                </div>
              ))}
            </div>
          ) : (
            <p className="text-slate-500 text-sm">No related risks for this pod.</p>
          )}
        </Card>
      )}

      {activeTab === 'metrics' && (
        <Card className="p-6">
          <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
            <BarChart2 className="w-5 h-5 text-pink-500" /> Runtime metrics
          </h3>
          {tabLoading ? (
            <p className="text-slate-500 text-sm">Loading...</p>
          ) : runtimeMetrics.length > 0 ? (
            <div className="overflow-x-auto border border-slate-800 rounded-lg max-h-[60vh] overflow-y-auto">
              <table className="w-full text-sm">
                <thead className="bg-slate-800/80 text-slate-300 text-left">
                  <tr>
                    <th className="px-3 py-2 font-medium">Container</th>
                    <th className="px-3 py-2 font-medium">CPU (m)</th>
                    <th className="px-3 py-2 font-medium">Memory</th>
                    <th className="px-3 py-2 font-medium">Limit</th>
                    <th className="px-3 py-2 font-medium">Restarts</th>
                    <th className="px-3 py-2 font-medium">State</th>
                    <th className="px-3 py-2 font-medium">Last observed</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {runtimeMetrics.map((m, i) => (
                    <tr key={m.id ?? i} className="hover:bg-muted/30">
                      <td className="px-3 py-2 font-mono text-slate-300">{m.containerName ?? '—'}</td>
                      <td className="px-3 py-2 tabular-nums">{m.cpuUsageMillicore != null ? m.cpuUsageMillicore : '—'}</td>
                      <td className="px-3 py-2 tabular-nums font-mono text-slate-400">
                        {m.memoryUsageBytes != null && m.memoryUsageBytes > 0
                          ? (m.memoryUsageBytes >= 1024 * 1024 ? `${(m.memoryUsageBytes / 1024 / 1024).toFixed(1)} MB` : `${(m.memoryUsageBytes / 1024).toFixed(1)} KB`)
                          : '—'}
                      </td>
                      <td className="px-3 py-2 tabular-nums font-mono text-slate-400">
                        {m.memoryLimitBytes != null && m.memoryLimitBytes > 0
                          ? (m.memoryLimitBytes >= 1024 * 1024 ? `${(m.memoryLimitBytes / 1024 / 1024).toFixed(1)} MB` : `${(m.memoryLimitBytes / 1024).toFixed(1)} KB`)
                          : '—'}
                      </td>
                      <td className="px-3 py-2 tabular-nums">{m.restartCount ?? 0}</td>
                      <td className="px-3 py-2">
                        <span className={clsx('px-2 py-0.5 rounded text-xs', m.state === 'Running' ? 'bg-emerald-600/80 text-white' : 'bg-slate-600 text-slate-200')}>
                          {m.state ?? '—'}
                        </span>
                      </td>
                      <td className="px-3 py-2 text-slate-500 text-xs">{m.lastObservedAt ? formatDateTime(m.lastObservedAt) : '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <PageEmpty title="No runtime metrics" description="Per-container CPU/memory metrics are reported by the agent. Ensure the agent is running on the pod's node." className="py-6" />
          )}
        </Card>
      )}

      {activeTab === 'processes' && (
        <Card className="p-6">
          <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2 flex-wrap">
            <Cpu className="w-5 h-5 text-pink-500" /> Processes
            {processes.length > 0 && (
              <span className="text-xs font-normal text-slate-400 px-2 py-0.5 rounded bg-slate-800">
                Runtime: {processes[0]?.runtimeSource === 'host' ? 'Host Inspection' : 'Container Exec'}
              </span>
            )}
          </h3>
          {tabLoading ? (
            <p className="text-slate-500 text-sm">Loading...</p>
          ) : processes.length > 0 ? (
            <div className="overflow-x-auto border border-slate-800 rounded-lg max-h-[60vh] overflow-y-auto">
              <table className="w-full text-sm">
                <thead className="bg-slate-800/80 text-slate-300 text-left">
                  <tr>
                    <th className="px-3 py-2 font-medium">PID</th>
                    <th className="px-3 py-2 font-medium">User</th>
                    <th className="px-3 py-2 font-medium">CPU %</th>
                    <th className="px-3 py-2 font-medium">Mem %</th>
                    <th className="px-3 py-2 font-medium">Command</th>
                    <th className="px-3 py-2 font-medium">Start time</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {processes.map((proc, i) => (
                    <tr key={proc.id ?? i} className="hover:bg-muted/30">
                      <td className="px-3 py-2 tabular-nums font-mono">{proc.pid}</td>
                      <td className="px-3 py-2 text-slate-300">{proc.userName ?? '—'}</td>
                      <td className="px-3 py-2 tabular-nums">{proc.cpuPercent != null ? proc.cpuPercent.toFixed(1) : '—'}</td>
                      <td className="px-3 py-2 tabular-nums">{proc.memoryPercent != null ? proc.memoryPercent.toFixed(1) : '—'}</td>
                      <td className="px-3 py-2 font-mono text-slate-400 truncate max-w-[280px]" title={proc.command}>{proc.command ?? '—'}</td>
                      <td className="px-3 py-2 text-slate-500 text-xs">{proc.startedAt ? formatDateTime(proc.startedAt) : '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <PageEmpty title="No process data" description="Process list is collected by the agent. Ensure the agent is running on the pod's node and Pod Detail collection is enabled." className="py-6" />
          )}
        </Card>
      )}

      {activeTab === 'network' && (
        <Card className="p-6">
          <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2 flex-wrap">
            <Network className="w-5 h-5 text-pink-500" /> Network connections
            {networkConnections.length > 0 && (
              <span className="text-xs font-normal text-slate-400 px-2 py-0.5 rounded bg-slate-800">
                Runtime: {networkConnections[0]?.runtimeSource === 'host' ? 'Host Inspection' : 'Container Exec'}
              </span>
            )}
          </h3>
          {tabLoading ? (
            <p className="text-slate-500 text-sm">Loading...</p>
          ) : networkConnections.length > 0 ? (
            <div className="overflow-x-auto max-h-[60vh] overflow-y-auto rounded-lg border border-border">
              <table className="w-full text-sm">
                <thead className="text-xs text-muted uppercase bg-muted/50 border-b border-border sticky top-0 z-10">
                  <tr>
                    <th className="px-3 py-2 font-medium">Direction</th>
                    <th className="px-3 py-2 font-medium">Remote address</th>
                    <th className="px-3 py-2 font-medium">Local port</th>
                    <th className="px-3 py-2 font-medium">Protocol</th>
                    <th className="px-3 py-2 font-medium">Status</th>
                    <th className="px-3 py-2 font-medium">Timestamp</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {networkConnections.map((conn, i) => {
                    const podIP = (pod?.podIP ?? '').trim();
                    const isListen = (conn.state ?? '').toUpperCase() === 'LISTEN';
                    const srcIsPod = podIP && (conn.sourceIp === podIP || conn.sourceIp === '0.0.0.0' || conn.sourceIp === '::');
                    const dstIsPod = podIP && (conn.destIp === podIP || conn.destIp === '0.0.0.0' || conn.destIp === '::');
                    const isOutbound = isListen ? false : srcIsPod;
                    const direction = isOutbound ? 'Outbound' : 'Inbound';
                    const remoteAddr = isOutbound ? `${conn.destIp ?? '—'}:${conn.destPort ?? 0}` : `${conn.sourceIp ?? '—'}:${conn.sourcePort ?? 0}`;
                    const localPort = isOutbound ? (conn.sourcePort ?? 0) : (conn.destPort ?? 0);
                    return (
                      <tr key={conn.id ?? i} className="hover:bg-muted/30">
                        <td className="px-3 py-2">
                          <span className={clsx('px-2 py-0.5 rounded text-xs', direction === 'Outbound' ? 'bg-sky-600/80 text-white' : 'bg-slate-600 text-slate-200')}>
                            {direction}
                          </span>
                        </td>
                        <td className="px-3 py-2 font-mono">{remoteAddr}</td>
                        <td className="px-3 py-2 tabular-nums">{localPort || '—'}</td>
                        <td className="px-3 py-2">{conn.protocol ?? '—'}</td>
                        <td className="px-3 py-2 text-slate-400">{conn.state ?? '—'}</td>
                        <td className="px-3 py-2 text-slate-500 text-xs">{conn.observedAt ? formatDateTime(conn.observedAt) : '—'}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          ) : (
            <PageEmpty title="No network data" description="Network connections are collected by the agent. Enable network collection on the agent." className="py-6" />
          )}
        </Card>
      )}

      {activeTab === 'spec' && (
        <Card className="p-6">
          <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
            <FileCode className="w-5 h-5 text-pink-500" /> Pod Specification (YAML)
          </h3>
          {tabLoading ? (
            <p className="text-slate-500 text-sm">Loading...</p>
          ) : (
            <>
              <div className="flex justify-end mb-3">
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
              <pre className="overflow-auto max-h-[70vh] p-4 rounded-lg bg-slate-900 border border-slate-800 text-sm font-mono text-slate-300 whitespace-pre-wrap break-all">
                {specYaml || 'No spec data.'}
              </pre>
            </>
          )}
        </Card>
      )}

      {activeTab === 'events' && (
        <Card className="p-6">
          <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
            <Activity className="w-5 h-5 text-pink-500" /> Kubernetes events
          </h3>
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
            <Button className="w-full" size="sm" onClick={() => window.open(`https://nvd.nist.gov/vuln/detail/${selectedVulnerability.id}`, '_blank')}>
              <ExternalLink className="w-4 h-4 mr-2" /> View on NVD
            </Button>
          </div>
        </div>
      )}

      <Card className="mt-8" variant="secondary">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Related navigation</h3>
        </div>
        <div className="flex flex-wrap gap-2">
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
