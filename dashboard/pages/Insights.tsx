import React, { useCallback, useMemo, useState } from 'react';
import { api } from '../lib/api';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { Cluster, Insight, PodCapabilityDetail, PodCapabilitySummarySeverity } from '../types';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../components/PageLayout';
import { Pagination } from '../components/Pagination';
import { Shield, AlertTriangle, Info, CheckCircle, Search, Box, User, ArrowRight } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { getSeverityBadgeClass, getSeverityTextClass } from '../lib/severity';
import { RISK_CENTER_DESCRIPTION } from '../constants/labels';
import { CapabilityMetadataBrowser } from '../components/CapabilityMetadataBrowser';
import { RuntimeSignalsTable } from '../components/RuntimeSignalsTable';

type TabId = 'risks' | 'pce' | 'reference';

export const RiskCenter: React.FC = () => {
  const navigate = useNavigate();
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
  const [filter, setFilter] = useState('all');
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedRisk, setSelectedRisk] = useState<Insight | null>(null);
  const [resolved24h, setResolved24h] = useState<number>(0);
  const [loading, setLoading] = useState(true);
  const [risksPage, setRisksPage] = useState(1);
  const [risksPageSize, setRisksPageSize] = useState(20);

  const fetchData = useCallback(async () => {
    setLoading(true);
    const [risksData, pceData, pceList, clusterList, stats] = await Promise.all([
      api.getRisks({
        page: risksPage,
        pageSize: risksPageSize,
        severity: filter !== 'all' ? filter : undefined,
        search: searchTerm.trim() || undefined,
      }),
      api.getPceSummaryBySeverity(),
      api.getPceCapabilities({ limit: 50 }),
      api.getClusters(),
      api.getStats(),
    ]);
    setRisks(risksData.insights);
    setRisksTotal(risksData.total);
    setPceSummary(pceData);
    setPceDetails(pceList);
    setClusters(clusterList);
    setResolved24h((stats as { resolved24h?: number }).resolved24h ?? 0);
    setLoading(false);
  }, [risksPage, risksPageSize, filter, searchTerm]);

  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.SBOM_RISK_LIST));
  usePolling(fetchData, intervalMs);
  const isFirstFetch = React.useRef(true);
  React.useEffect(() => {
    if (isFirstFetch.current) {
      isFirstFetch.current = false;
      return;
    }
    fetchData();
  }, [fetchData]);

  React.useEffect(() => { setRisksPage(1); }, [filter, searchTerm]);

  const getSeverityIcon = (severity: Insight['severity']) => {
    switch (severity) {
      case 'critical': return <Shield className="w-5 h-5 text-red-500" />;
      case 'high': return <AlertTriangle className="w-5 h-5 text-orange-500" />;
      case 'medium': return <Info className="w-5 h-5 text-yellow-500" />;
      case 'low': return <CheckCircle className="w-5 h-5 text-blue-500" />;
    }
  };

  const stats = {
    total: risksTotal,
    critical: risks.filter(r => r.severity === 'critical').length,
    high: risks.filter(r => r.severity === 'high').length,
    medium: risks.filter(r => r.severity === 'medium').length,
    low: risks.filter(r => r.severity === 'low').length,
  };

  const paginatedRisks = risks;

  if (loading) return <div className="text-slate-500 p-8">Loading Risk Center...</div>;

  const tabs: { id: TabId; label: string }[] = [
    { id: 'risks', label: 'Risks' },
    { id: 'pce', label: 'PCE' },
    { id: 'reference', label: 'Reference' },
  ];

  return (
    <PageLayout
      title="Risk Center"
      description={RISK_CENTER_DESCRIPTION}
    >
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

      {/* ----- Risks tab ----- */}
      {activeTab === 'risks' && (
        <div className="space-y-6">
          {/* Risks summary bar – total from API (matches Dashboard Security Risks); severity counts = this page */}
          <div>
            <p className="text-xs text-slate-500 mb-2 uppercase tracking-wider">
              Total: {risksTotal} vulnerability risks {risksTotal > 0 && '(this page by severity below)'}
            </p>
            <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
              <div className="bg-slate-900 border border-slate-800 p-3 rounded-lg flex items-center justify-between">
                <span className="text-slate-400 text-sm">Critical</span>
                <span className="text-red-500 font-bold text-lg">{stats.critical}</span>
              </div>
              <div className="bg-slate-900 border border-slate-800 p-3 rounded-lg flex items-center justify-between">
                <span className="text-slate-400 text-sm">High</span>
                <span className="text-orange-500 font-bold text-lg">{stats.high}</span>
              </div>
              <div className="bg-slate-900 border border-slate-800 p-3 rounded-lg flex items-center justify-between">
                <span className="text-slate-400 text-sm">Medium</span>
                <span className="text-yellow-500 font-bold text-lg">{stats.medium}</span>
              </div>
              <div className="bg-slate-900 border border-slate-800 p-3 rounded-lg flex items-center justify-between">
                <span className="text-slate-400 text-sm">Low</span>
                <span className="text-blue-500 font-bold text-lg">{stats.low}</span>
              </div>
              <div className="bg-slate-900 border border-slate-800 p-3 rounded-lg flex items-center justify-between">
                <span className="text-slate-400 text-sm">Resolved (24h)</span>
                <span className="text-emerald-500 font-bold text-lg">{resolved24h}</span>
              </div>
            </div>
          </div>

          {/* Filters */}
          <div className="flex flex-col md:flex-row gap-4 md:items-center md:justify-between">
            <div className="flex flex-wrap gap-2">
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
            </div>
            <div className="relative flex-1 md:max-w-xs">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 w-4 h-4" />
              <input
                type="text"
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                placeholder="Search risks, pods, resources..."
                className="pl-9 pr-4 py-2 bg-slate-900 border border-slate-700 rounded-lg text-sm text-white focus:ring-2 focus:ring-pink-500 outline-none w-full placeholder:text-slate-600"
              />
            </div>
          </div>

          {/* Risk list – compact table with pagination */}
          <div className="bg-slate-900 border border-slate-800 rounded-t-lg overflow-hidden">
            <div className="overflow-x-auto max-h-[calc(100vh-22rem)] overflow-y-auto">
              <table className="w-full text-sm">
                <thead className="bg-slate-950 text-slate-400 border-b border-slate-800">
                  <tr>
                    <th className="text-left px-4 py-3 w-10">Severity</th>
                    <th className="text-left px-4 py-3">Title</th>
                    <th className="text-left px-4 py-3 w-20">Score</th>
                    <th className="text-left px-4 py-3 hidden lg:table-cell">Namespace</th>
                    <th className="text-left px-4 py-3 w-24">Status</th>
                    <th className="text-left px-4 py-3 hidden md:table-cell w-28">Date</th>
                    <th className="text-right px-4 py-3 w-40">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {paginatedRisks.length === 0 ? (
                    <tr>
                      <td colSpan={7} className="px-4 py-8 text-center text-slate-500">
                        No risks match the current filters.
                      </td>
                    </tr>
                  ) : (
                    paginatedRisks.map((risk) => (
                      <tr
                        key={risk.id}
                        className="border-b border-slate-800 hover:bg-slate-800/50 transition-colors"
                      >
                        <td className="px-4 py-3">{getSeverityIcon(risk.severity)}</td>
                        <td className="px-4 py-3">
                          <span className="text-white font-medium">{risk.title}</span>
                          <span className="text-slate-500 ml-1 text-xs">({risk.id})</span>
                        </td>
                        <td className="px-4 py-3 text-slate-300">{risk.score}/100</td>
                        <td className="px-4 py-3 text-slate-400 hidden lg:table-cell truncate max-w-[140px]" title={risk.affectedResources?.[0]?.namespace ?? risk.clusterId}>
                          {risk.affectedResources?.[0]?.namespace ?? risk.clusterName ?? risk.clusterId ?? '—'}
                        </td>
                        <td className="px-4 py-3">
                          <span className="uppercase px-2 py-0.5 bg-slate-800 rounded text-xs text-slate-300">
                            {risk.status}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-slate-500 hidden md:table-cell">
                          {new Date(risk.timestamp).toLocaleDateString()}
                        </td>
                        <td className="px-4 py-3 text-right">
                          <button
                            onClick={() => setSelectedRisk(risk)}
                            className="text-pink-400 hover:text-pink-300 text-sm font-medium mr-3"
                          >
                            Details
                          </button>
                          <Button
                            size="sm"
                            variant="secondary"
                            onClick={() => navigate('/attack-paths')}
                          >
                            Attack Path
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
              itemLabel="risks"
            />
          )}
        </div>
      )}

      {/* ----- PCE tab ----- */}
      {activeTab === 'pce' && (
        <div className="space-y-6">
          {/* PCE Summary – capability counts, not risk counts */}
          <div className="bg-slate-900 border border-slate-800 p-4 rounded-lg">
            <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-3">
              PCE capabilities by severity
            </h2>
            {pceSummary.length === 0 ? (
              <p className="text-sm text-slate-500">No capability data available.</p>
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
              Drill-down by pod or capability
            </h2>
            <p className="text-xs text-slate-500 mb-4">
              Filter by cluster, namespace, pod name, or capability ID. Apply to load results; Refresh to fetch latest data.
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
                  Apply
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
                  Refresh
                </Button>
              </div>
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
                      <td colSpan={5} className="px-3 py-4 text-center text-slate-500">No capability records. Set filters and click Apply.</td>
                    </tr>
                  ) : (
                    pceDetails.map((row) => (
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
              Runtime Signals
            </h2>
            <p className="text-xs text-slate-500 mb-4">Real-time security signals from pods.</p>
            <RuntimeSignalsTable />
          </div>
          <div className="bg-slate-900 border border-slate-800 p-6 rounded-lg">
            <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-1 flex items-center gap-2">
              <Shield size={16} className="text-pink-400" />
              Capability Metadata
            </h2>
            <p className="text-xs text-slate-500 mb-4">Standardized capability definitions and severity.</p>
            <CapabilityMetadataBrowser />
          </div>
        </div>
      )}

      {/* Risk Detail Modal – full info when user clicks Details */}
      {selectedRisk && (
        <div className="fixed inset-0 bg-black/50 backdrop-blur-sm z-50 flex items-center justify-center p-4" onClick={() => setSelectedRisk(null)}>
          <div className="bg-slate-900 border border-slate-800 rounded-lg max-w-4xl w-full max-h-[90vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-slate-800 flex justify-between items-start">
              <div>
                <div className="flex items-center space-x-2 mb-2">
                  {getSeverityIcon(selectedRisk.severity)}
                  <h2 className="text-xl font-bold text-white">{selectedRisk.id}: {selectedRisk.title}</h2>
                </div>
                <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-slate-500">
                  <span>Score: <span className="text-slate-300 font-medium">{selectedRisk.score}/100</span></span>
                  <span>Severity: <span className="uppercase">{selectedRisk.severity}</span></span>
                  <span>Status: <span className="uppercase">{selectedRisk.status}</span></span>
                  <span>Detected: {new Date(selectedRisk.timestamp).toLocaleString()}</span>
                </div>
              </div>
              <button onClick={() => setSelectedRisk(null)} className="text-slate-400 hover:text-white text-2xl leading-none">
                ×
              </button>
            </div>
            <div className="p-6 space-y-6">
              <div>
                <h3 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-2">Description</h3>
                <p className="text-slate-300 text-sm leading-relaxed">{selectedRisk.description}</p>
              </div>
              <div>
                <h3 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-2">Affected Resources</h3>
                <div className="flex flex-wrap gap-2">
                  {selectedRisk.affectedResources?.map((res, idx) => (
                    <div key={idx} className="flex items-center bg-slate-950 border border-slate-700 rounded px-3 py-2 text-sm text-slate-300">
                      {res.kind === 'Pod' && <Box className="w-4 h-4 mr-2 text-blue-400" />}
                      {res.kind === 'ServiceAccount' && <User className="w-4 h-4 mr-2 text-green-400" />}
                      {res.kind === 'ClusterRole' && <Shield className="w-4 h-4 mr-2 text-red-400" />}
                      <span className="font-medium">{res.name}</span>
                      <span className="ml-2 text-xs text-slate-500">({res.kind})</span>
                      {res.namespace && <span className="ml-2 text-xs text-slate-500">in {res.namespace}</span>}
                    </div>
                  ))}
                </div>
              </div>
              <div>
                <h3 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-2">Impact</h3>
                <p className="text-slate-300 text-sm">{selectedRisk.impact || 'No impact description available.'}</p>
              </div>
              <div>
                <h3 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-2">Namespace</h3>
                <div className="bg-slate-950 border border-slate-800 rounded p-3 text-sm text-slate-300">
                  {(selectedRisk.affectedResources?.[0]?.namespace ?? selectedRisk.clusterName) && <span>{selectedRisk.affectedResources?.[0]?.namespace ?? selectedRisk.clusterName} </span>}
                  {selectedRisk.clusterId && selectedRisk.clusterId !== (selectedRisk.affectedResources?.[0]?.namespace ?? selectedRisk.clusterName) && <span className="text-slate-500">(cluster: {selectedRisk.clusterId})</span>}
                  {selectedRisk.category && <span className="ml-2 text-slate-500">· {selectedRisk.category}</span>}
                </div>
              </div>
              <div className="flex justify-end gap-3 pt-4 border-t border-slate-800">
                <Button variant="secondary" onClick={() => setSelectedRisk(null)}>Close</Button>
                <Button onClick={() => { setSelectedRisk(null); navigate('/attack-paths'); }}>View Attack Path</Button>
              </div>
            </div>
          </div>
        </div>
      )}
    </PageLayout>
  );
};
