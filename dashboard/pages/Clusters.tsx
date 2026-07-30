import React, { useCallback, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';
import { Cluster } from '../types';
import { Card } from '../design-system/components/Card';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Pagination } from '../components/Pagination';
import { RefreshCw, MoreHorizontal, Globe } from 'lucide-react';
import clsx from 'clsx';
import { STAT_LABELS } from '../constants/labels';
import { getClusterDisplayName } from '../lib/clusterDisplay';
import { FilterBar } from '../design-system/components/FilterBar';
import { UI_FILTER_SELECT } from '../lib/formChrome';
import { PageEmpty, PageError } from '../design-system/components/PageStatus';
import { getConnectionStatusClass, getConnectionStatusLabel } from '../lib/display';
import { UI_TABLE, UI_TD, UI_TH, UI_TR, UI_THEAD_STICKY } from '../lib/tableChrome';
import { PAGE_TITLES } from '../lib/pageTitles';
import { DataFreshness } from '../components/DataFreshness';

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
  const [error, setError] = useState<string | null>(null);
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null);

  const fetchClusters = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.getClustersStats();
      setClusters(data);
      setError(null);
      setUpdatedAt(new Date());
    } catch {
      setError('Cluster inventory could not be refreshed.');
    } finally {
      setLoading(false);
    }
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
      title={PAGE_TITLES.clusters}
      description={`Manage and monitor clusters. Count matches Dashboard "${STAT_LABELS.CLUSTERS}" (active within 7 days).`}
      actions={
        <Button onClick={fetchClusters} variant="secondary" isLoading={loading}>
          <RefreshCw className="w-4 h-4 mr-2" />
          Refresh
        </Button>
      }
      toolbar={
        <FilterBar
          embedded
          search={{
            value: searchName,
            onChange: (v) => {
              setSearchName(v);
              setPage(1);
            },
            placeholder: 'Search by cluster name…',
            inputClassName: 'min-w-[200px] max-w-none sm:max-w-md',
          }}
          trailing={
            <>
              <DataFreshness updatedAt={updatedAt} loading={loading} error={error} />
              <select
                value={healthFilter}
                onChange={(e) => {
                  setHealthFilter(e.target.value as typeof healthFilter);
                  setPage(1);
                }}
                className={UI_FILTER_SELECT}
              >
                <option value="all">All health</option>
                <option value="connected">Connected</option>
                <option value="degraded">Degraded</option>
                <option value="disconnected">Disconnected</option>
              </select>
              <select
                value={sortBy}
                onChange={(e) => {
                  setSortBy(e.target.value as typeof sortBy);
                  setPage(1);
                }}
                className={`${UI_FILTER_SELECT} sm:min-w-[14rem]`}
              >
                <option value="risk_desc">Sort: Risk high to low</option>
                <option value="agents_desc">Sort: Agents high to low</option>
                <option value="pods_desc">Sort: Pods high to low</option>
                <option value="name_asc">Sort: Name A-Z</option>
              </select>
            </>
          }
        />
      }
    >
      {error && clusters.length === 0 ? (
        <PageError
          title="Could not load clusters"
          description="Cluster inventory is unavailable. Existing data is kept when possible so refresh failures do not look like an empty environment."
          action={<Button variant="secondary" onClick={fetchClusters} isLoading={loading}>Retry clusters</Button>}
        />
      ) : (
      <Card className="p-0 overflow-hidden">
        <div className="ui-table-scroll">
          <table className={UI_TABLE}>
            <thead className={UI_THEAD_STICKY}>
              <tr>
                <th className={UI_TH}>Cluster (id / name)</th>
                <th className={UI_TH}>Connection</th>
                <th className={UI_TH}>Risk Count</th>
                <th className={UI_TH}>Agents</th>
                <th className={UI_TH}>Version / Distribution</th>
                <th className={UI_TH}>Resources</th>
                <th className={`${UI_TH} text-right`}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {paginatedClusters.length === 0 ? (
                <tr>
                  <td colSpan={7} className={`${UI_TD} py-8`}>
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
                  role="link"
                  tabIndex={0}
                  className={`${UI_TR} cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 focus-visible:ring-inset`}
                  onClick={() => navigate(`/clusters/${cluster.id}`)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter' || event.key === ' ') {
                      event.preventDefault();
                      navigate(`/clusters/${cluster.id}`);
                    }
                  }}
                >
                  <td className={`${UI_TD} font-medium text-text`}>
                    <div className="flex items-center">
                        <div className="w-8 h-8 rounded-lg bg-brand/20 flex items-center justify-center mr-3 text-brand">
                            <Globe className="w-4 h-4" />
                        </div>
                        <div>
                          <div>{getClusterDisplayName(cluster)}</div>
                          {(cluster.id?.startsWith('sha256-') || (cluster.name && cluster.name !== cluster.id)) && (
                            <div className="text-caption font-mono text-muted mt-0.5">{cluster.id}</div>
                          )}
                        </div>
                    </div>
                  </td>
                  <td className={UI_TD}>
                    <span className={clsx(
                      "px-2.5 py-0.5 rounded-full text-caption font-medium capitalize",
                      getConnectionStatusClass(cluster.connectionStatus)
                    )}>
                      {getConnectionStatusLabel(cluster.connectionStatus)}
                    </span>
                  </td>
                  <td className={`${UI_TD} text-text`}>
                    {cluster.riskCount != null ? cluster.riskCount : '—'}
                  </td>
                  <td className={`${UI_TD} text-text`}>
                    {cluster.agentCount != null ? cluster.agentCount : '—'}
                  </td>
                  <td className={`${UI_TD} font-mono text-muted`}>
                    <div>{cluster.version ?? cluster.k8sVersion ?? '—'}</div>
                    {cluster.distribution && <div className="text-caption text-muted">{cluster.distribution}</div>}
                  </td>
                  <td className={`${UI_TD} text-muted`}>
                    {cluster.podCount != null || cluster.deploymentCount != null
                      ? `${cluster.podCount ?? 0} Pods / ${cluster.deploymentCount ?? 0} Deployments`
                      : (cluster.nodes != null && cluster.pods != null ? `${cluster.nodes} Nodes / ${cluster.pods} Pods` : '—')}
                  </td>
                  <td className={`${UI_TD} text-right`} onClick={(e) => e.stopPropagation()}>
                    <button
                      type="button"
                      className="inline-flex min-h-10 min-w-10 items-center justify-center rounded-lg text-muted transition-colors hover:bg-surface-2 hover:text-brand focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 sm:min-h-8 sm:min-w-8"
                      aria-label={`Open ${getClusterDisplayName(cluster)} cluster details`}
                      onClick={() => navigate(`/clusters/${cluster.id}`)}
                    >
                        <MoreHorizontal className="w-5 h-5" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
      )}
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
