import React, { useCallback, useMemo, useRef, useState } from 'react';
import { api } from '../lib/api';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { PodSbom, PodSbomSummary, ThreatSummary } from '../types';
import { Card } from '../design-system/components/Card';
import { Badge } from '../design-system/components/Badge';
import { FilterBar } from '../design-system/components/FilterBar';
import { Button } from '../components/ui/Button';
import { DataFreshness } from '../components/DataFreshness';
import { PageLoading } from '../design-system/components/PageStatus';
import { Package, ChevronRight, ChevronDown, Info, ExternalLink, Box, AlertTriangle, CheckCircle2, Download, RefreshCw, ShieldCheck, ShieldQuestion } from 'lucide-react';
import { exportSbomAsCsv, exportSbomAsCycloneDxJson, exportSbomAsJson, exportSbomAsSpdxJson } from '../lib/exportSbom';
import { getSeverityBorderClass } from '../lib/severity';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { useNavigate } from 'react-router-dom';
import { SbomMetaBadges } from '../components/SbomMetaBadges';
import { UI_FILTER_SELECT_SM, UI_SELECTABLE_ACTIVE, UI_SELECTABLE_IDLE } from '../lib/formChrome';
import { PAGE_TITLES } from '../lib/pageTitles';

function trustTone(status?: string): string {
  switch ((status || '').toLowerCase()) {
    case 'strong':
      return 'border-emerald-500/35 bg-emerald-500/10 text-emerald-300';
    case 'blocked':
      return 'border-red-500/40 bg-red-500/10 text-red-300';
    case 'weak':
      return 'border-amber-500/35 bg-amber-500/10 text-amber-300';
    default:
      return 'border-border bg-surface-2 text-muted';
  }
}

function lifecycleTone(state?: string): string {
  return (state || '').toLowerCase() === 'current'
    ? 'border-emerald-500/35 bg-emerald-500/10 text-emerald-300'
    : 'border-rose-500/35 bg-rose-500/10 text-rose-200';
}

function compactDigest(digest?: string): string {
  const d = (digest || '').trim();
  if (!d) return 'missing';
  if (d.length <= 22) return d;
  return `${d.slice(0, 18)}...`;
}

const ImageTrustStrip: React.FC<{ item?: Pick<PodSbomSummary, 'imageTrust' | 'imageDigest' | 'lifecycleState' | 'activePod'> | Pick<PodSbom, 'imageTrust' | 'imageDigest' | 'lifecycleState' | 'activePod'>; compact?: boolean }> = ({ item, compact }) => {
  if (!item) return null;
  const trust = item.imageTrust;
  const lifecycle = item.lifecycleState || (item.activePod ? 'current' : undefined);
  if (!trust && !lifecycle && !item.imageDigest) return null;
  const Icon = trust?.status === 'strong' ? ShieldCheck : ShieldQuestion;
  return (
    <div className={`flex flex-wrap items-center gap-2 ${compact ? 'mb-2' : 'mb-4'}`}>
      {lifecycle ? (
        <span className={`inline-flex items-center rounded border px-2 py-0.5 text-caption font-medium ${lifecycleTone(lifecycle)}`} title="SBOM lifecycle relative to active Kubernetes pods">
          {lifecycle === 'current' ? 'Current pod' : 'Historical SBOM'}
        </span>
      ) : null}
      {trust ? (
        <span className={`inline-flex items-center gap-1 rounded border px-2 py-0.5 text-caption font-medium ${trustTone(trust.status)}`} title={(trust.warnings || trust.evidence || []).join(' · ') || 'Image trust signal'}>
          <Icon className="h-3.5 w-3.5" aria-hidden />
          Image trust: {trust.status}
        </span>
      ) : null}
      {trust?.registry ? (
        <span className="rounded border border-border bg-base/60 px-2 py-0.5 font-mono text-caption text-muted" title="Image registry classification">
          {trust.registryClass}: {trust.registry}
        </span>
      ) : null}
      {!compact && (
        <span className="rounded border border-border bg-base/60 px-2 py-0.5 font-mono text-caption text-muted" title={item.imageDigest || 'Image digest missing'}>
          digest: {compactDigest(item.imageDigest)}
        </span>
      )}
    </div>
  );
};

export const Sbom: React.FC = () => {
  const navigate = useNavigate();
  const [sbomList, setSbomList] = useState<PodSbomSummary[]>([]);
  const [selectedPod, setSelectedPod] = useState<PodSbomSummary | null>(null);
  const [selectedDetail, setSelectedDetail] = useState<PodSbom | null>(null);
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');
  const [podNameFilter, setPodNameFilter] = useState('');
  const [namespaceFilter, setNamespaceFilter] = useState('');
  const [expandedComponents, setExpandedComponents] = useState<Set<string>>(new Set());
  const [loadError, setLoadError] = useState<string | null>(null);
  const [listUpdatedAt, setListUpdatedAt] = useState<Date | null>(null);
  const [threatSummary, setThreatSummary] = useState<ThreatSummary | null>(null);
  const initialSelectionDone = useRef(false);

  const fetchSbomList = useCallback(async (opts?: { showOverlay?: boolean }) => {
    if (opts?.showOverlay) setLoading(true);
    setLoadError(null);
    const params =
      podNameFilter.trim() || namespaceFilter.trim()
        ? { podName: podNameFilter.trim() || undefined, namespace: namespaceFilter.trim() || undefined }
        : undefined;
    try {
      const data = await api.getSbomList(params);
      setSbomList(data);
      setListUpdatedAt(new Date());
      if (data.length > 0 && !initialSelectionDone.current) {
        initialSelectionDone.current = true;
        const first = data[0];
        setSelectedPod(first);
        setDetailLoading(true);
        const detail = await api.getPodSbom(first.podId);
        setSelectedDetail(detail || null);
        setDetailLoading(false);
        const threats = await api.getPodThreatSummary(first.podId);
        setThreatSummary(threats);
      }
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : 'Failed to load SBOM list. Please log in or check connection.');
      setSbomList([]);
    }
    setLoading(false);
  }, [podNameFilter, namespaceFilter]);

  const namespaceOptions = useMemo(() => {
    const s = new Set<string>();
    for (const p of sbomList) {
      const ns = (p.namespace ?? '').trim();
      if (ns) s.add(ns);
    }
    if (namespaceFilter.trim()) s.add(namespaceFilter.trim());
    return [...s].sort((a, b) => a.localeCompare(b, undefined, { sensitivity: 'base' }));
  }, [sbomList, namespaceFilter]);

  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.SBOM_RISK_LIST));
  usePolling(fetchSbomList, intervalMs);

  // Refetch when pod/namespace filter changes (debounce 400ms)
  React.useEffect(() => {
    const t = window.setTimeout(() => fetchSbomList(), 400);
    return () => clearTimeout(t);
  }, [podNameFilter, namespaceFilter]);

  const toggleExpand = (compId: string) => {
    const newExpanded = new Set(expandedComponents);
    if (newExpanded.has(compId)) {
      newExpanded.delete(compId);
    } else {
      newExpanded.add(compId);
    }
    setExpandedComponents(newExpanded);
  };

  const handleSelectPod = async (pod: PodSbomSummary) => {
    setSelectedPod(pod);
    setDetailLoading(true);
    setThreatSummary(null);
    const detail = await api.getPodSbom(pod.podId);
    setSelectedDetail(detail || null);
    setDetailLoading(false);
    const threats = await api.getPodThreatSummary(pod.podId);
    setThreatSummary(threats);
  };

  const getSeverityBorder = (severity: string) => {
    return getSeverityBorderClass(severity);
  };

  const filteredComponents = selectedDetail?.components.filter((c) => {
    const q = searchTerm.toLowerCase();
    if (!q) return true;
    return (
      c.name.toLowerCase().includes(q) ||
      (c.version && c.version.toLowerCase().includes(q)) ||
      c.vulnerabilities.some((v) => v.id.toLowerCase().includes(q)) ||
      (c.malwareMatch && (c.malwareMatch.reason?.toLowerCase().includes(q) || (c.malwareMatch.malwareFamily && c.malwareMatch.malwareFamily.toLowerCase().includes(q))))
    );
  }) || [];

  if (loading && sbomList.length === 0 && !loadError) {
    return <PageLoading message="Scanning SBOM repositories…" className="min-h-[60dvh]" />;
  }

  return (
    <PageLayout
      title={PAGE_TITLES.sbom}
      description="Deep analysis of container image components and CVEs."
      actions={
        <div className="flex flex-wrap items-center gap-2">
          <DataFreshness
            updatedAt={listUpdatedAt}
            loading={loading}
            error={loadError}
            staleAfterMs={intervalMs * 2}
          />
          <Button variant="secondary" size="sm" onClick={() => void fetchSbomList({ showOverlay: true })} isLoading={loading}>
            <RefreshCw size={16} className="mr-2 shrink-0" />
            Refresh
          </Button>
          {selectedDetail && (selectedDetail.components?.length ?? 0) > 0 ? (
            <>
              <Button size="sm" variant="secondary" onClick={() => exportSbomAsCsv(selectedDetail)} title="Download SBOM as CSV">
                <Download size={14} className="mr-1.5 shrink-0" />
                CSV
              </Button>
              <Button size="sm" variant="secondary" onClick={() => exportSbomAsJson(selectedDetail)} title="Download SBOM as JSON">
                JSON
              </Button>
              <Button size="sm" variant="secondary" onClick={() => exportSbomAsSpdxJson(selectedDetail)} title="SPDX JSON">
                SPDX
              </Button>
              <Button size="sm" variant="secondary" onClick={() => exportSbomAsCycloneDxJson(selectedDetail)} title="CycloneDX JSON">
                CycloneDX
              </Button>
            </>
          ) : null}
        </div>
      }
      toolbar={
        <FilterBar
          embedded
          leading={
            <div className="flex min-w-[10rem] flex-col gap-1">
              <span className="ui-micro-label">Pod name</span>
              <input
                type="text"
                placeholder="Filter list by pod name…"
                value={podNameFilter}
                onChange={(e) => setPodNameFilter(e.target.value)}
                className={`${UI_FILTER_SELECT_SM} w-full sm:w-52`}
              />
            </div>
          }
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: 'Search packages or CVEs…',
            inputClassName: 'max-w-md min-w-[12rem]',
          }}
          namespace={{
            value: namespaceFilter,
            onChange: setNamespaceFilter,
            options: namespaceOptions,
            emptyLabel: 'All namespaces',
            title: 'Filter SBOM pod list by namespace',
          }}
        />
      }
    >
      {loadError && (
        <div className="flex items-center gap-2 rounded-lg border border-amber-500/50 bg-amber-500/10 px-4 py-3 text-amber-200 text-body">
          <AlertTriangle className="w-5 h-5 shrink-0" />
          <span>{loadError}</span>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 min-h-0">
        {/* Pod List: 1/3 width, scrollable */}
        <div className="flex flex-col min-h-0 lg:max-h-[calc(100dvh-14rem)]">
          <h2 className="mb-3 shrink-0 text-card-title text-text">Monitored pods</h2>
          <div className="shrink-0 space-y-3">
            {!loading && sbomList.length === 0 && !loadError && (
              <p className="py-4 text-body text-muted">No pods with SBOM data yet. Pods appear after the agent scans them (1–3 min after pod is Running).</p>
            )}
          </div>
          <div className="space-y-3 overflow-y-auto min-h-0 flex-1 pr-1 mt-2">
            {sbomList.map(pod => {
              const summary = pod.vulnerabilitySummary || { critical: 0, high: 0, medium: 0, low: 0 };
              const createdLabel = pod.podCreatedAt
                ? new Date(pod.podCreatedAt).toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' })
                : '—';
              const statusLabel = pod.podStatus?.trim() || '—';
              return (
              <div 
                key={pod.podId} 
                onClick={() => handleSelectPod(pod)}
                className={`p-4 rounded-xl border cursor-pointer transition-all duration-200 group ${
                  selectedPod?.podId === pod.podId ? UI_SELECTABLE_ACTIVE : UI_SELECTABLE_IDLE
                }`}
              >
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center">
                    <Box className={`w-4 h-4 mr-2 transition-colors ${selectedPod?.podId === pod.podId ? 'text-brand' : 'text-muted group-hover:text-text'}`} />
                    <span className="font-bold text-text text-body">{pod.podName}</span>
                  </div>
                  <ChevronRight size={14} className={`transition-transform duration-200 ${selectedPod?.podId === pod.podId ? 'text-brand rotate-90 lg:rotate-0' : 'text-muted-2'}`} />
                </div>
                <div className="text-caption text-muted truncate mb-1 font-mono">{pod.image}</div>
                <ImageTrustStrip item={pod} compact />
                <SbomMetaBadges
                  compact
                  className="mb-2"
                  sbomSource={pod.sbomSource}
                  confidence={pod.confidence}
                  goVersion={pod.goVersion}
                />
                <div className="text-caption text-muted mb-2">
                  <span title="Pod creation time">Created: {createdLabel}</span>
                  <span className="mx-1.5">·</span>
                  <span title="Pod phase (synced)">Phase: {statusLabel}</span>
                </div>
                <div className="flex gap-2">
                  {summary.critical > 0 && (
                    <span className="bg-red-500/10 text-red-400 px-2 py-0.5 rounded text-caption font-bold">
                      {summary.critical} Critical
                    </span>
                  )}
                  {summary.high > 0 && (
                    <span className="bg-orange-500/10 text-orange-400 px-2 py-0.5 rounded text-caption font-bold">
                      {summary.high} High
                    </span>
                  )}
                </div>
              </div>
              );
            })}
          </div>
        </div>

        {/* SBOM Detail: 2/3 width, scrollable */}
        <div className="lg:col-span-2 flex flex-col min-h-0 lg:max-h-[calc(100dvh-14rem)] overflow-y-auto space-y-6">
          {selectedPod ? (
            <div className="animate-in fade-in slide-in-from-right-4 duration-300">
              <Card 
                title={`SBOM: ${selectedPod.podName}`} 
                description={[
                  `Namespace: ${selectedPod.namespace}`,
                  `Last Scan: ${selectedPod.lastScan ? new Date(selectedPod.lastScan).toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' }) : '—'}`,
                  selectedPod.podCreatedAt ? `Created: ${new Date(selectedPod.podCreatedAt).toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' })}` : '',
                  selectedPod.podStatus?.trim() ? `Status: ${selectedPod.podStatus}` : '',
                ].filter(Boolean).join(' · ')}
                actions={
                  selectedPod ? (
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => {
                        const ns = selectedPod.namespace || '';
                        const name = selectedPod.podName || '';
                        const params = new URLSearchParams();
                        if (ns) params.set('resourceNamespace', ns);
                        if (name) params.set('search', name);
                        navigate(`/risks?${params.toString()}`);
                      }}
                      title="Open related risks in Risk Operations"
                    >
                      View related risks
                    </Button>
                  ) : null
                }
              >
                {detailLoading && (
                  <div className="flex items-center text-body text-muted mb-4">
                    <div className="w-4 h-4 border-2 border-brand border-t-transparent rounded-full animate-spin mr-2"></div>
                    Loading SBOM detail...
                  </div>
                )}
                {!detailLoading && (selectedDetail || selectedPod) && (
                  <>
                    <ImageTrustStrip item={selectedDetail ?? selectedPod} />
                    <SbomMetaBadges
                      className="mb-4"
                      sbomSource={selectedDetail?.sbomSource ?? selectedPod?.sbomSource}
                      confidence={selectedDetail?.confidence ?? selectedPod?.confidence}
                      goVersion={selectedDetail?.goVersion ?? selectedPod?.goVersion}
                    />
                  </>
                )}
                {threatSummary && threatSummary.totalThreats > 0 && (
                  <div className={`mb-4 p-4 rounded-lg border-2 ${
                    threatSummary.requiresAction
                      ? 'bg-red-950/20 border-red-700/50'
                      : 'bg-yellow-950/20 border-yellow-700/50'
                  }`}>
                    <div className="flex items-center justify-between mb-2">
                      <div className="flex items-center gap-2">
                        <AlertTriangle size={16} className={threatSummary.requiresAction ? 'text-red-500' : 'text-yellow-500'} />
                        <span className={`font-bold text-body ${threatSummary.requiresAction ? 'text-red-100' : 'text-yellow-100'}`}>
                          {threatSummary.requiresAction ? 'MALWARE DETECTED' : 'Supply Chain Risk'}
                        </span>
                      </div>
                      <span className="text-caption font-mono px-2 py-0.5 rounded bg-surface text-text">
                        {threatSummary.malwareCount} malware
                        {threatSummary.protestwareCount > 0 && ` / ${threatSummary.protestwareCount} protestware`}
                        {threatSummary.telemetryCount > 0 && ` / ${threatSummary.telemetryCount} telemetry`}
                      </span>
                    </div>
                    <div className="flex flex-wrap gap-1">
                      {threatSummary.affectedPackages.map((pkg) => (
                        <span key={`${pkg.name}@${pkg.version}`} className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-caption font-mono ${
                          pkg.reason === 'MALWARE' ? 'bg-red-900/40 text-red-200'
                          : pkg.reason === 'PROTESTWARE' ? 'bg-orange-900/40 text-orange-200'
                          : 'bg-yellow-900/40 text-yellow-200'
                        }`}>
                          {pkg.name}@{pkg.version}
                        </span>
                      ))}
                    </div>
                  </div>
                )}
                <div className="mb-6 flex flex-col gap-4 rounded-lg border border-border bg-base p-4 md:flex-row md:items-center md:justify-between">
                  <div className="flex items-center space-x-6">
                    <div className="text-center">
                      <div className="ui-micro-label">Components</div>
                      <div className="text-xl font-bold text-text">{selectedDetail?.components.length || 0}</div>
                    </div>
                    <div className="h-8 w-px bg-surface-2"></div>
                    <div className="text-center">
                      <div className="ui-micro-label">Vulnerabilities</div>
                      <div className="text-xl font-bold text-red-500">
                        {selectedDetail?.components.reduce((acc, c) => acc + c.vulnerabilities.length, 0) || 0}
                      </div>
                    </div>
                  </div>
                  <p className="text-caption text-muted-2">Use toolbar search to filter packages and CVEs in this pod.</p>
                </div>

                <div className="space-y-4">
                  {filteredComponents.length > 0 ? filteredComponents.map(comp => {
                    const compKey = String(comp.id);
                    const isExpanded = expandedComponents.has(compKey);
                    const vulnTotal = comp.vulnerabilities.length;
                    const canExpand = vulnTotal > 0 || !!comp.malwareMatch;
                    
                    return (
                      <div key={compKey} className={`bg-surface/50 border rounded-lg overflow-hidden transition-all duration-200 ${isExpanded ? 'border-border' : 'border-border'}`}>
                        {/* Summary Header */}
                        <div 
                          className={`p-4 flex justify-between items-center transition-colors group ${canExpand ? 'cursor-pointer hover:bg-surface/80' : ''}`}
                          onClick={() => canExpand && toggleExpand(compKey)}
                        >
                          <div className="flex items-center space-x-3">
                            <div className={`p-2 rounded-lg ${
                              comp.malwareMatch ? 'bg-red-500/20 text-red-300 ring-1 ring-red-700' :
                              vulnTotal > 0 ? 'bg-red-500/10 text-red-400' : 'bg-emerald-500/10 text-emerald-400'
                            }`}>
                              {comp.malwareMatch ? <AlertTriangle size={18} /> : <Package size={18} />}
                            </div>
                            <div>
                              <div className="flex items-center">
                                <span className="font-bold text-text">{comp.name}</span>
                                <span className="ml-2 text-caption text-muted bg-surface-2 px-1.5 py-0.5 rounded font-mono">v{comp.version}</span>
                              </div>
                              <div className="text-caption text-muted uppercase mt-1 tracking-wider">{comp.type} {comp.language ? `• ${comp.language}` : ''}</div>
                            </div>
                          </div>
                          <div className="flex items-center space-x-4">
                            {comp.malwareMatch && (
                              <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-caption font-bold uppercase tracking-wide ${
                                comp.malwareMatch.reason === 'MALWARE'
                                  ? 'bg-red-950 border border-red-700 text-red-200'
                                  : comp.malwareMatch.reason === 'PROTESTWARE'
                                  ? 'bg-orange-950 border border-orange-700 text-orange-200'
                                  : 'bg-yellow-950 border border-yellow-700 text-yellow-200'
                              }`}>
                                <AlertTriangle size={10} />
                                {comp.malwareMatch.reason}
                              </span>
                            )}
                            {vulnTotal > 0 ? (
                              <div className="flex gap-1">
                                {Array.from(new Set(comp.vulnerabilities.map((v) => v.severity))).map((sev) => (
                                  <Badge key={sev as string} severity={String(sev).toLowerCase()} uppercase>
                                    {String(sev)}
                                  </Badge>
                                ))}
                              </div>
                            ) : comp.malwareMatch ? (
                              <span className="flex items-center text-caption text-red-400 font-medium">
                                <AlertTriangle size={14} className="mr-1 shrink-0" /> Supply-chain threat (no CVE row)
                              </span>
                            ) : (
                              <span className="flex items-center text-caption text-emerald-500 font-medium">
                                <CheckCircle2 size={14} className="mr-1" /> Secure
                              </span>
                            )}
                            {canExpand && (
                              isExpanded ? <ChevronDown size={18} className="text-muted" /> : <ChevronRight size={18} className="text-muted-2 group-hover:text-muted transition-colors" />
                            )}
                          </div>
                        </div>

                        {/* Detailed Expansion */}
                        {isExpanded && canExpand && (
                          <div className="p-5 bg-base/40 border-t border-border animate-in slide-in-from-top-2 duration-200">
                            {comp.malwareMatch && (
                              <div className="mb-6 rounded-xl border border-red-800/60 bg-red-950/25 p-4">
                                <div className="flex items-center gap-2 text-caption font-bold text-red-200 uppercase tracking-widest mb-2">
                                  <AlertTriangle size={14} className="text-red-400" />
                                  Malware DB match
                                </div>
                                <p className="text-body text-text">
                                  <span className="font-mono text-text">{comp.name}@{comp.version}</span>
                                  {' '}is listed in the Fortuna malware package database.
                                </p>
                                <dl className="mt-3 grid gap-1 text-caption text-muted">
                                  <div><dt className="inline text-muted">Reason: </dt><dd className="inline font-medium text-text">{comp.malwareMatch.reason}</dd></div>
                                  {comp.malwareMatch.malwareFamily ? (
                                    <div><dt className="inline text-muted">Family: </dt><dd className="inline text-text">{comp.malwareMatch.malwareFamily}</dd></div>
                                  ) : null}
                                  <div><dt className="inline text-muted">Confidence: </dt><dd className="inline text-text">{comp.malwareMatch.confidence}</dd></div>
                                </dl>
                              </div>
                            )}
                            {vulnTotal > 0 && (
                             <>
                             <div className="flex items-center mb-4 text-caption font-bold text-muted uppercase tracking-widest">
                               <AlertTriangle size={12} className="mr-2 text-orange-500" />
                               Detected Security Vulnerabilities ({vulnTotal})
                             </div>
                             <div className="space-y-4">
                                {comp.vulnerabilities.map(vuln => (
                                  <div key={vuln.id} className={`bg-surface/80 border rounded-xl p-4 ${getSeverityBorder(vuln.severity)}`}>
                                    <div className="flex items-start justify-between mb-3">
                                      <div className="flex flex-col">
                                        <div className="flex items-center">
                                          <span className="text-base font-mono font-bold text-text tracking-tight">{vuln.id}</span>
                                          <a href={`https://www.cve.org/CVERecord?id=${encodeURIComponent(vuln.id)}`} target="_blank" rel="noopener noreferrer" className="ml-2 text-muted-2 hover:text-brand transition-colors">
                                            <ExternalLink size={14} />
                                          </a>
                                        </div>
                                        <div className="mt-1">
                                          <Badge severity={String(vuln.severity).toLowerCase()} uppercase>
                                            {vuln.severity}
                                          </Badge>
                                        </div>
                                      </div>
                                      <div className="bg-base/80 border border-border px-3 py-1.5 rounded-lg text-center">
                                        <div className="text-caption text-muted font-bold uppercase">CVSS Score</div>
                                        <div className={`text-lg font-black ${vuln.cvssScore >= 7 ? 'text-red-500' : vuln.cvssScore >= 4 ? 'text-orange-500' : 'text-yellow-500'}`}>
                                          {vuln.cvssScore.toFixed(1)}
                                        </div>
                                      </div>
                                    </div>
                                    
                                    <div className="mb-4">
                                      <h5 className="text-caption font-bold text-muted mb-1 uppercase tracking-tighter">Description</h5>
                                      <p className="text-body text-text leading-relaxed bg-base/40 p-3 rounded-lg border border-border/50">
                                        {vuln.description}
                                      </p>
                                    </div>

                                    {vuln.fixedVersion ? (
                                      <div className="flex items-center bg-emerald-500/10 border border-emerald-500/20 rounded-lg px-3 py-2">
                                        <CheckCircle2 size={14} className="text-emerald-500 mr-2 shrink-0" />
                                        <span className="text-caption font-medium text-emerald-400">
                                          Remediation available: Update to version <span className="font-bold font-mono underline decoration-dotted">{vuln.fixedVersion}</span> or higher.
                                        </span>
                                      </div>
                                    ) : (
                                      <div className="flex items-center bg-yellow-500/10 border border-yellow-500/20 rounded-lg px-3 py-2">
                                        <Info size={14} className="text-yellow-500 mr-2 shrink-0" />
                                        <span className="text-caption font-medium text-yellow-400">
                                          No fix version is listed for this finding. Monitor vendor security advisories.
                                        </span>
                                      </div>
                                    )}
                                  </div>
                                ))}
                             </div>
                             </>
                            )}
                          </div>
                        )}
                      </div>
                    );
                  }) : (
                    <div className="p-8 text-center text-muted border border-dashed border-border rounded-xl">
                      No components found matching your search.
                    </div>
                  )}
                </div>
              </Card>
            </div>
          ) : (
            <div className="h-full flex flex-col items-center justify-center text-muted bg-surface/30 border border-dashed border-border rounded-xl p-12 text-center min-h-[400px]">
              <Package size={48} className="mb-4 opacity-20" />
              <h3 className="text-lg font-medium text-muted">No Pod Selected</h3>
              <p className="text-body max-w-xs mx-auto mt-2 leading-relaxed">Select a pod from the monitored list to visualize its deep software composition and security risk profile.</p>
            </div>
          )}
        </div>
      </div>
    </PageLayout>
  );
};
