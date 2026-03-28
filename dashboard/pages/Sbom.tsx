import React, { useCallback, useRef, useState } from 'react';
import { api } from '../lib/api';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { PodSbom, PodSbomSummary } from '../types';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Package, Search, Filter, ChevronRight, ChevronDown, Info, ExternalLink, Box, AlertTriangle, CheckCircle2, Download } from 'lucide-react';
import { exportSbomAsCsv, exportSbomAsCycloneDxJson, exportSbomAsJson, exportSbomAsSpdxJson } from '../lib/exportSbom';
import { getSeverityBadgeClass, getSeverityBorderClass } from '../lib/severity';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { useNavigate } from 'react-router-dom';
import { SbomMetaBadges } from '../components/SbomMetaBadges';

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
  const initialSelectionDone = useRef(false);

  const fetchSbomList = useCallback(async () => {
    setLoadError(null);
    const params =
      podNameFilter.trim() || namespaceFilter.trim()
        ? { podName: podNameFilter.trim() || undefined, namespace: namespaceFilter.trim() || undefined }
        : undefined;
    try {
      const data = await api.getSbomList(params);
      setSbomList(data);
      if (data.length > 0 && !initialSelectionDone.current) {
        initialSelectionDone.current = true;
        setSelectedPod(data[0]);
        setDetailLoading(true);
        const detail = await api.getPodSbom(data[0].podId);
        setSelectedDetail(detail || null);
        setDetailLoading(false);
      }
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : 'Failed to load SBOM list. Please log in or check connection.');
      setSbomList([]);
    }
    setLoading(false);
  }, [podNameFilter, namespaceFilter]);

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
    const detail = await api.getPodSbom(pod.podId);
    setSelectedDetail(detail || null);
    setDetailLoading(false);
  };

  const getSeverityBadge = (severity: string) => {
    return <span className={`px-2 py-0.5 rounded ui-micro-label border ${getSeverityBadgeClass(severity)}`}>{severity}</span>;
  };

  const getSeverityBorder = (severity: string) => {
    return getSeverityBorderClass(severity);
  };

  const filteredComponents = selectedDetail?.components.filter(c =>
    c.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
    c.vulnerabilities.some(v => v.id.toLowerCase().includes(searchTerm.toLowerCase()))
  ) || [];

  if (loading) return (
    <div className="flex flex-col justify-center items-center h-[60vh] space-y-4">
      <div className="w-12 h-12 border-4 border-pink-500 border-t-transparent rounded-full animate-spin"></div>
      <span className="text-slate-500 font-medium">Scanning SBOM Repositories...</span>
    </div>
  );

  return (
    <PageLayout
      title="SBOM & Vulnerability Analysis"
      description="Deep analysis of container image components and CVEs."
    >
      {loadError && (
        <div className="flex items-center gap-2 rounded-lg border border-amber-500/50 bg-amber-500/10 px-4 py-3 text-amber-200 text-sm">
          <AlertTriangle className="w-5 h-5 shrink-0" />
          <span>{loadError}</span>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 min-h-0">
        {/* Pod List: 1/3 width, scrollable */}
        <div className="flex flex-col min-h-0 lg:max-h-[calc(100vh-14rem)]">
          <h2 className="text-sm font-semibold text-slate-500 uppercase tracking-wider mb-3 shrink-0">Monitored Pods</h2>
          <div className="space-y-3 shrink-0">
            <div className="flex flex-col gap-2">
              <input
                type="text"
                placeholder="Filter by pod name..."
                value={podNameFilter}
                onChange={(e) => setPodNameFilter(e.target.value)}
                className="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white placeholder:text-slate-500 focus:ring-1 focus:ring-pink-500 outline-none"
              />
              <input
                type="text"
                placeholder="Filter by namespace..."
                value={namespaceFilter}
                onChange={(e) => setNamespaceFilter(e.target.value)}
                className="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white placeholder:text-slate-500 focus:ring-1 focus:ring-pink-500 outline-none"
              />
            </div>
            {!loading && sbomList.length === 0 && !loadError && (
              <p className="text-slate-500 text-sm py-4">No pods with SBOM data yet. Pods appear after the agent scans them (1–3 min after pod is Running).</p>
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
                  selectedPod?.podId === pod.podId 
                  ? 'bg-pink-600/10 border-pink-500 shadow-lg shadow-pink-900/10' 
                  : 'bg-slate-900 border-slate-800 hover:border-slate-700'
                }`}
              >
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center">
                    <Box className={`w-4 h-4 mr-2 transition-colors ${selectedPod?.podId === pod.podId ? 'text-pink-500' : 'text-slate-400 group-hover:text-slate-300'}`} />
                    <span className="font-bold text-white text-sm">{pod.podName}</span>
                  </div>
                  <ChevronRight size={14} className={`transition-transform duration-200 ${selectedPod?.podId === pod.podId ? 'text-pink-500 rotate-90 lg:rotate-0' : 'text-slate-600'}`} />
                </div>
                <div className="text-xs text-slate-500 truncate mb-1 font-mono">{pod.image}</div>
                <SbomMetaBadges
                  compact
                  className="mb-2"
                  sbomSource={pod.sbomSource}
                  confidence={pod.confidence}
                  goVersion={pod.goVersion}
                />
                <div className="text-[10px] text-slate-500 mb-2">
                  <span title="Pod creation time">Created: {createdLabel}</span>
                  <span className="mx-1.5">·</span>
                  <span title="Pod phase (synced)">Phase: {statusLabel}</span>
                </div>
                <div className="flex gap-2">
                  {summary.critical > 0 && (
                    <span className="bg-red-500/10 text-red-400 px-2 py-0.5 rounded text-[10px] font-bold">
                      {summary.critical} Critical
                    </span>
                  )}
                  {summary.high > 0 && (
                    <span className="bg-orange-500/10 text-orange-400 px-2 py-0.5 rounded text-[10px] font-bold">
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
        <div className="lg:col-span-2 flex flex-col min-h-0 lg:max-h-[calc(100vh-14rem)] overflow-y-auto space-y-6">
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
                  selectedDetail && (selectedDetail.components?.length ?? 0) > 0 ? (
                    <div className="flex items-center gap-2">
                      <Button size="sm" variant="secondary" onClick={() => exportSbomAsCsv(selectedDetail)} title="Download SBOM as CSV">
                        <Download size={14} className="mr-2" /> Export CSV
                      </Button>
                      <Button size="sm" variant="secondary" onClick={() => exportSbomAsJson(selectedDetail)} title="Download SBOM as JSON">
                        <Download size={14} className="mr-2" /> Export JSON
                      </Button>
                      <Button size="sm" variant="secondary" onClick={() => exportSbomAsSpdxJson(selectedDetail)} title="Download SBOM as SPDX JSON">
                        <Download size={14} className="mr-2" /> Export SPDX
                      </Button>
                      <Button size="sm" variant="secondary" onClick={() => exportSbomAsCycloneDxJson(selectedDetail)} title="Download SBOM as CycloneDX JSON">
                        <Download size={14} className="mr-2" /> Export CycloneDX
                      </Button>
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
                    </div>
                  ) : null
                }
              >
                {detailLoading && (
                  <div className="flex items-center text-sm text-slate-500 mb-4">
                    <div className="w-4 h-4 border-2 border-pink-500 border-t-transparent rounded-full animate-spin mr-2"></div>
                    Loading SBOM detail...
                  </div>
                )}
                {!detailLoading && (selectedDetail || selectedPod) && (
                  <SbomMetaBadges
                    className="mb-4"
                    sbomSource={selectedDetail?.sbomSource ?? selectedPod?.sbomSource}
                    confidence={selectedDetail?.confidence ?? selectedPod?.confidence}
                    goVersion={selectedDetail?.goVersion ?? selectedPod?.goVersion}
                  />
                )}
                <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-6 bg-slate-950 p-4 rounded-lg border border-slate-800">
                  <div className="flex items-center space-x-6">
                    <div className="text-center">
                      <div className="ui-micro-label">Components</div>
                      <div className="text-xl font-bold text-white">{selectedDetail?.components.length || 0}</div>
                    </div>
                    <div className="h-8 w-px bg-slate-800"></div>
                    <div className="text-center">
                      <div className="ui-micro-label">Vulnerabilities</div>
                      <div className="text-xl font-bold text-red-500">
                        {selectedDetail?.components.reduce((acc, c) => acc + c.vulnerabilities.length, 0) || 0}
                      </div>
                    </div>
                  </div>
                  <div className="relative flex-1 max-w-xs">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-500 w-4 h-4" />
                    <input 
                      type="text" 
                      placeholder="Search packages or CVEs..." 
                      value={searchTerm}
                      onChange={(e) => setSearchTerm(e.target.value)}
                      className="w-full bg-slate-900 border border-slate-700 rounded-lg pl-9 pr-4 py-2 text-sm text-white focus:ring-1 focus:ring-pink-500 outline-none placeholder:text-slate-600 transition-all"
                    />
                  </div>
                </div>

                <div className="space-y-4">
                  {filteredComponents.length > 0 ? filteredComponents.map(comp => {
                    const compKey = String(comp.id);
                    const isExpanded = expandedComponents.has(compKey);
                    const vulnTotal = comp.vulnerabilities.length;
                    
                    return (
                      <div key={compKey} className={`bg-slate-900/50 border rounded-lg overflow-hidden transition-all duration-200 ${isExpanded ? 'border-slate-600' : 'border-slate-800'}`}>
                        {/* Summary Header */}
                        <div 
                          className="p-4 flex justify-between items-center cursor-pointer hover:bg-slate-900/80 transition-colors group"
                          onClick={() => toggleExpand(compKey)}
                        >
                          <div className="flex items-center space-x-3">
                            <div className={`p-2 rounded-lg ${vulnTotal > 0 ? 'bg-red-500/10 text-red-400' : 'bg-emerald-500/10 text-emerald-400'}`}>
                              <Package size={18} />
                            </div>
                            <div>
                              <div className="flex items-center">
                                <span className="font-bold text-slate-100">{comp.name}</span>
                                <span className="ml-2 text-xs text-slate-500 bg-slate-800 px-1.5 py-0.5 rounded font-mono">v{comp.version}</span>
                              </div>
                              <div className="text-[10px] text-slate-500 uppercase mt-1 tracking-wider">{comp.type} {comp.language ? `• ${comp.language}` : ''}</div>
                            </div>
                          </div>
                          <div className="flex items-center space-x-4">
                            {vulnTotal > 0 ? (
                              <div className="flex gap-1">
                                {/* Fix for line 180: cast 'sev' (inferred as unknown) to string for getSeverityBadge */}
                                {Array.from(new Set(comp.vulnerabilities.map(v => v.severity))).map((sev) => (
                                  <div key={sev as string}>{getSeverityBadge(sev as string)}</div>
                                ))}
                              </div>
                            ) : (
                              <span className="flex items-center text-xs text-emerald-500 font-medium">
                                <CheckCircle2 size={14} className="mr-1" /> Secure
                              </span>
                            )}
                            {vulnTotal > 0 && (
                              isExpanded ? <ChevronDown size={18} className="text-slate-400" /> : <ChevronRight size={18} className="text-slate-600 group-hover:text-slate-400 transition-colors" />
                            )}
                          </div>
                        </div>

                        {/* Detailed Expansion */}
                        {isExpanded && vulnTotal > 0 && (
                          <div className="p-5 bg-slate-950/40 border-t border-slate-800 animate-in slide-in-from-top-2 duration-200">
                             <div className="flex items-center mb-4 text-xs font-bold text-slate-500 uppercase tracking-widest">
                               <AlertTriangle size={12} className="mr-2 text-orange-500" />
                               Detected Security Vulnerabilities ({vulnTotal})
                             </div>
                             <div className="space-y-4">
                                {comp.vulnerabilities.map(vuln => (
                                  <div key={vuln.id} className={`bg-slate-900/80 border border-slate-800 rounded-xl p-4 border-l-4 ${getSeverityBorder(vuln.severity)}`}>
                                    <div className="flex items-start justify-between mb-3">
                                      <div className="flex flex-col">
                                        <div className="flex items-center">
                                          <span className="text-base font-mono font-bold text-white tracking-tight">{vuln.id}</span>
                                          <a href={`https://nvd.nist.gov/vuln/detail/${vuln.id}`} target="_blank" rel="noopener noreferrer" className="ml-2 text-slate-600 hover:text-pink-500 transition-colors">
                                            <ExternalLink size={14} />
                                          </a>
                                        </div>
                                        <div className="mt-1">{getSeverityBadge(vuln.severity)}</div>
                                      </div>
                                      <div className="bg-slate-950/80 border border-slate-800 px-3 py-1.5 rounded-lg text-center">
                                        <div className="text-[10px] text-slate-500 font-bold uppercase">CVSS Score</div>
                                        <div className={`text-lg font-black ${vuln.cvssScore >= 7 ? 'text-red-500' : vuln.cvssScore >= 4 ? 'text-orange-500' : 'text-yellow-500'}`}>
                                          {vuln.cvssScore.toFixed(1)}
                                        </div>
                                      </div>
                                    </div>
                                    
                                    <div className="mb-4">
                                      <h5 className="text-xs font-bold text-slate-400 mb-1 uppercase tracking-tighter">Description</h5>
                                      <p className="text-sm text-slate-300 leading-relaxed bg-slate-950/40 p-3 rounded-lg border border-slate-800/50">
                                        {vuln.description}
                                      </p>
                                    </div>

                                    {vuln.fixedVersion ? (
                                      <div className="flex items-center bg-emerald-500/10 border border-emerald-500/20 rounded-lg px-3 py-2">
                                        <CheckCircle2 size={14} className="text-emerald-500 mr-2 shrink-0" />
                                        <span className="text-xs font-medium text-emerald-400">
                                          Remediation available: Update to version <span className="font-bold font-mono underline decoration-dotted">{vuln.fixedVersion}</span> or higher.
                                        </span>
                                      </div>
                                    ) : (
                                      <div className="flex items-center bg-yellow-500/10 border border-yellow-500/20 rounded-lg px-3 py-2">
                                        <Info size={14} className="text-yellow-500 mr-2 shrink-0" />
                                        <span className="text-xs font-medium text-yellow-400">
                                          No fix version currently listed in NVD records. Monitor vendor security advisories.
                                        </span>
                                      </div>
                                    )}
                                  </div>
                                ))}
                             </div>
                          </div>
                        )}
                      </div>
                    );
                  }) : (
                    <div className="p-8 text-center text-slate-500 border border-dashed border-slate-800 rounded-xl">
                      No components found matching your search.
                    </div>
                  )}
                </div>
              </Card>
            </div>
          ) : (
            <div className="h-full flex flex-col items-center justify-center text-slate-500 bg-slate-900/30 border border-dashed border-slate-800 rounded-xl p-12 text-center min-h-[400px]">
              <Package size={48} className="mb-4 opacity-20" />
              <h3 className="text-lg font-medium text-slate-400">No Pod Selected</h3>
              <p className="text-sm max-w-xs mx-auto mt-2 leading-relaxed">Select a pod from the monitored list to visualize its deep software composition and security risk profile.</p>
            </div>
          )}
        </div>
      </div>
    </PageLayout>
  );
};
