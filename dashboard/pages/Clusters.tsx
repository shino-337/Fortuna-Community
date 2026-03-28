import React, { useCallback, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';
import { Cluster } from '../types';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Pagination } from '../components/Pagination';
import { RefreshCw, MoreHorizontal, Globe, Search } from 'lucide-react';
import clsx from 'clsx';
import { STAT_LABELS } from '../constants/labels';
import { getClusterDisplayName } from '../lib/clusterDisplay';
import { PageEmpty } from '../components/PageEmpty';
import { getConnectionStatusClass, getConnectionStatusLabel } from '../lib/display';

const PAGE_SIZE_OPTIONS = [10, 20, 50];
const HEALTH_FILTER_OPTIONS = ['all', 'connected', 'degraded', 'disconnected'] as const;

export const Clusters: React.FC = () => {
  const navigate = useNavigate();
  const [clusters, setClusters] = useState<Cluster[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [searchName, setSearchName] = useState('');
  const [healthFilter, setHealthFilter] = useState<'all' | 'connected' | 'degraded' | 'disconnected'>('all');
  const [sortBy, setSortBy] = useState<'name_asc' | 'risk_desc' | 'agents_desc' | 'pods_desc'>('risk_desc');

  const fetchClusters = useCallback(async () => {
    setLoading(true);
    const data = await api.getClustersStats();
    setClusters(data);
    setLoading(false);
  }, []);

  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.STATS_CLUSTERS));
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);
  usePolling(fetchClusters, intervalMs, { refreshTrigger });

  const filteredClusters = useMemo(() => {
    let out = clusters;
    if (searchName.trim()) {
      const q = searchName.trim().toLowerCase();
      out = out.filter((c) => getClusterDisplayName(c).toLowerCase().includes(q));
    }
    if (healthFilter !== 'all') {
      out = out.filter((c) => (c.connectionStatus || '').toLowerCase() === healthFilter);
    }
    out = [...out].sort((a, b) => {
      switch (sortBy) {
        case 'name_asc':
          return getClusterDisplayName(a).localeCompare(getClusterDisplayName(b));
        case 'agents_desc':
          return (b.agentCount ?? -1) - (a.agentCount ?? -1);
        case 'pods_desc':
          return (b.podCount ?? b.pods ?? -1) - (a.podCount ?? a.pods ?? -1);
        case 'risk_desc':
        default:
          return (b.riskCount ?? -1) - (a.riskCount ?? -1);
      }
    });
    return out;
  }, [clusters, searchName, healthFilter, sortBy]);

  const paginatedClusters = useMemo(() => {
    const start = (page - 1) * pageSize;
    return filteredClusters.slice(start, start + pageSize);
  }, [filteredClusters, page, pageSize]);

  return (
    <PageLayout
      title={STAT_LABELS.CLUSTERS}
      description={`Manage and monitor clusters. Count matches Dashboard "${STAT_LABELS.CLUSTERS}" (active within 7 days).`}
      actions={
        <Button onClick={fetchClusters} variant="secondary" isLoading={loading}>
          <RefreshCw className="w-4 h-4 mr-2" />
          Refresh
        </Button>
      }
      toolbar={
        <div className="flex flex-wrap gap-3 items-center">
          <div className="relative flex-1 min-w-[200px] max-w-xs">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-500 w-4 h-4" />
            <input
              type="text"
              value={searchName}
              onChange={(e) => { setSearchName(e.target.value); setPage(1); }}
              placeholder="Search by cluster name..."
              className="w-full bg-slate-950 border border-slate-700 rounded-lg pl-9 pr-4 py-2 text-sm text-white focus:outline-none focus:border-pink-500"
            />
          </div>
          <select
            value={healthFilter}
            onChange={(e) => { setHealthFilter(e.target.value as typeof healthFilter); setPage(1); }}
            className="bg-slate-950 border border-slate-700 rounded-lg px-4 py-2 text-sm text-slate-300 focus:outline-none focus:border-pink-500"
          >
            <option value="all">All health</option>
            <option value="connected">Connected</option>
            <option value="degraded">Degraded</option>
            <option value="disconnected">Disconnected</option>
          </select>
          <select
            value={sortBy}
            onChange={(e) => { setSortBy(e.target.value as typeof sortBy); setPage(1); }}
            className="bg-slate-950 border border-slate-700 rounded-lg px-4 py-2 text-sm text-slate-300 focus:outline-none focus:border-pink-500"
          >
            <option value="risk_desc">Sort: Risk high to low</option>
            <option value="agents_desc">Sort: Agents high to low</option>
            <option value="pods_desc">Sort: Pods high to low</option>
            <option value="name_asc">Sort: Name A-Z</option>
          </select>
        </div>
      }
    >
      <Card className="p-0 overflow-hidden">
        <div className="ui-table-scroll">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-muted uppercase bg-muted/50 border-b border-border">
              <tr>
                <th className="px-6 py-4 font-medium">Cluster (id / name)</th>
                <th className="px-6 py-4 font-medium">Connection</th>
                <th className="px-6 py-4 font-medium">Risk Count</th>
                <th className="px-6 py-4 font-medium">Agents</th>
                <th className="px-6 py-4 font-medium">Version / Distribution</th>
                <th className="px-6 py-4 font-medium">Resources</th>
                <th className="px-6 py-4 font-medium text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800">
              {paginatedClusters.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-6 py-8">
                    <PageEmpty
                      title="No clusters match current filters"
                      description="Try clearing search text or health filter."
                      className="py-6"
                    />
                  </td>
                </tr>
              ) : paginatedClusters.map((cluster) => (
                <tr
                  key={cluster.id}
                  className="hover:bg-muted/30 transition-colors cursor-pointer"
                  onClick={() => navigate(`/clusters/${cluster.id}`)}
                >
                  <td className="px-6 py-4 font-medium text-white">
                    <div className="flex items-center">
                        <div className="w-8 h-8 rounded-lg bg-pink-900/20 flex items-center justify-center mr-3 text-pink-500">
                            <Globe className="w-4 h-4" />
                        </div>
                        <div>
                          <div>{getClusterDisplayName(cluster)}</div>
                          {(cluster.id?.startsWith('sha256-') || (cluster.name && cluster.name !== cluster.id)) && (
                            <div className="text-xs font-mono text-slate-500 mt-0.5">{cluster.id}</div>
                          )}
                        </div>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <span className={clsx(
                      "px-2.5 py-0.5 rounded-full text-xs font-medium capitalize",
                      getConnectionStatusClass(cluster.connectionStatus)
                    )}>
                      {getConnectionStatusLabel(cluster.connectionStatus)}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-slate-300">
                    {cluster.riskCount != null ? cluster.riskCount : '—'}
                  </td>
                  <td className="px-6 py-4 text-slate-300">
                    {cluster.agentCount != null ? cluster.agentCount : '—'}
                  </td>
                  <td className="px-6 py-4 font-mono text-slate-500">
                    <div>{cluster.version ?? cluster.k8sVersion ?? '—'}</div>
                    {cluster.distribution && <div className="text-xs text-slate-500">{cluster.distribution}</div>}
                  </td>
                  <td className="px-6 py-4 text-slate-400">
                    {cluster.podCount != null || cluster.deploymentCount != null
                      ? `${cluster.podCount ?? 0} Pods / ${cluster.deploymentCount ?? 0} Deployments`
                      : (cluster.nodes != null && cluster.pods != null ? `${cluster.nodes} Nodes / ${cluster.pods} Pods` : '—')}
                  </td>
                  <td className="px-6 py-4 text-right" onClick={(e) => e.stopPropagation()}>
                    <button className="text-slate-500 hover:text-pink-500 transition-colors" onClick={() => navigate(`/clusters/${cluster.id}`)}>
                        <MoreHorizontal className="w-5 h-5" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
      {filteredClusters.length > 0 && (
        <Pagination
          page={page}
          pageSize={pageSize}
          total={filteredClusters.length}
          onPageChange={setPage}
          onPageSizeChange={(size) => { setPageSize(size); setPage(1); }}
          pageSizeOptions={PAGE_SIZE_OPTIONS}
          itemLabel="clusters"
        />
      )}
    </PageLayout>
  );
};