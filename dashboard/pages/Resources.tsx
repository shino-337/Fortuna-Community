import React, { useState, useEffect, useMemo, useCallback } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { Card } from '../components/ui/Card';
import { api } from '../lib/api';
import { useClusterStore } from '../store/clusterStore';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';
import { K8sResource, PodWithRisk } from '../types';
import { Box, UserCog, Scroll, Key, RefreshCw, Search } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../components/PageLayout';
import { Pagination } from '../components/Pagination';
import { PageEmpty } from '../components/PageEmpty';
import { getSeverityBadgeClass, getPodStatusBadgeClass } from '../lib/severity';

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];
const TAB_IDS = ['Pod', 'ServiceAccount', 'Role', 'RoleBinding'] as const;
type TabId = (typeof TAB_IDS)[number];

export const Resources: React.FC = () => {
  const navigate = useNavigate();
  const selectedClusterId = useClusterStore((s) => s.selectedClusterId);
  const [searchParams, setSearchParams] = useSearchParams();
  const tabParam = searchParams.get('tab');
  const namespaceParam = searchParams.get('namespace') ?? '';
  const initialTab: TabId = tabParam && TAB_IDS.includes(tabParam as TabId) ? (tabParam as TabId) : 'Pod';
  const [activeTab, setActiveTab] = useState<TabId>(initialTab);
  const [namespaceFilter, setNamespaceFilter] = useState(namespaceParam);
  const [resources, setResources] = useState<K8sResource[]>([]);
  const [pods, setPods] = useState<PodWithRisk[]>([]);
  const [podsTotal, setPodsTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [searchTerm, setSearchTerm] = useState('');
  const [sortBy, setSortBy] = useState<'name_asc' | 'namespace_asc' | 'risk_desc'>('name_asc');

  useEffect(() => {
    const t = searchParams.get('tab');
    if (t && TAB_IDS.includes(t as TabId)) setActiveTab(t as TabId);
    setNamespaceFilter(searchParams.get('namespace') ?? '');
  }, [searchParams]);

  const fetchResources = useCallback(async () => {
    setLoading(true);
    if (activeTab === 'Pod') {
      const data = await api.getPods({
        page,
        pageSize,
        cluster: selectedClusterId ?? undefined,
        namespace: namespaceFilter || undefined,
      });
      setPods(data.pods);
      setPodsTotal(data.total);
    } else {
      const data = await api.getResources(activeTab, {
        cluster: selectedClusterId ?? undefined,
        namespace: namespaceFilter || undefined,
      });
      setResources(data);
    }
    setLoading(false);
  }, [activeTab, page, pageSize, selectedClusterId, namespaceFilter]);

  useEffect(() => {
    setPage(1);
  }, [activeTab]);

  useEffect(() => {
    setPage(1);
  }, [searchTerm]);

  useEffect(() => {
    fetchResources();
  }, [fetchResources]);

  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.STATS_CLUSTERS));
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);
  usePolling(fetchResources, intervalMs, { refreshTrigger });

  const tabs = [
    { id: 'Pod', label: 'Pods', icon: <Box size={16} /> },
    { id: 'ServiceAccount', label: 'Service Accounts', icon: <UserCog size={16} /> },
    { id: 'Role', label: 'Roles', icon: <Scroll size={16} /> },
    { id: 'RoleBinding', label: 'Role Bindings', icon: <Key size={16} /> },
  ];

  /** Risk level from count for badge (real data only) */
  const podRiskLevel = (count: number): 'critical' | 'high' | 'medium' | 'low' => {
    if (count >= 10) return 'critical';
    if (count >= 4) return 'high';
    if (count >= 1) return 'medium';
    return 'low';
  };

  const renderTableHead = () => {
    if (activeTab === 'Pod') {
      return (
        <tr>
          <th className="px-6 py-4 font-medium">Name</th>
          <th className="px-6 py-4 font-medium">Namespace</th>
          <th className="px-6 py-4 font-medium">Node</th>
          <th className="px-6 py-4 font-medium">Status</th>
          <th className="px-6 py-4 font-medium">Risk</th>
          <th className="px-6 py-4 font-medium text-right">Actions</th>
        </tr>
      );
    }
    return (
      <tr>
        <th className="px-6 py-4 font-medium">Name</th>
        <th className="px-6 py-4 font-medium">Namespace</th>
        <th className="px-6 py-4 font-medium">Kind</th>
        <th className="px-6 py-4 font-medium">Status</th>
        <th className="px-6 py-4 font-medium text-right">Actions</th>
      </tr>
    );
  };

  const filteredPods = useMemo(() => {
    if (activeTab !== 'Pod' || !searchTerm.trim()) return pods;
    const q = searchTerm.trim().toLowerCase();
    return pods.filter((p) =>
      p.name.toLowerCase().includes(q) ||
      p.namespace.toLowerCase().includes(q) ||
      (p.nodeName || '').toLowerCase().includes(q) ||
      p.uid.toLowerCase().includes(q)
    );
  }, [activeTab, pods, searchTerm]);

  const filteredResources = useMemo(() => {
    if (activeTab === 'Pod') return resources;
    if (!searchTerm.trim()) return resources;
    const q = searchTerm.trim().toLowerCase();
    return resources.filter((r) =>
      r.name.toLowerCase().includes(q) ||
      (r.namespace || '').toLowerCase().includes(q) ||
      (r.kind || '').toLowerCase().includes(q)
    );
  }, [activeTab, resources, searchTerm]);

  const sortedPods = useMemo(() => {
    const out = [...filteredPods];
    out.sort((a, b) => {
      switch (sortBy) {
        case 'namespace_asc':
          return a.namespace.localeCompare(b.namespace);
        case 'risk_desc':
          return b.riskCount - a.riskCount;
        case 'name_asc':
        default:
          return a.name.localeCompare(b.name);
      }
    });
    return out;
  }, [filteredPods, sortBy]);

  const sortedResources = useMemo(() => {
    const out = [...filteredResources];
    out.sort((a, b) => {
      switch (sortBy) {
        case 'namespace_asc':
          return (a.namespace || '').localeCompare(b.namespace || '');
        case 'risk_desc':
          return (b.kind || '').localeCompare(a.kind || '');
        case 'name_asc':
        default:
          return a.name.localeCompare(b.name);
      }
    });
    return out;
  }, [filteredResources, sortBy]);

  const paginatedResources = useMemo(() => {
    if (activeTab === 'Pod') return sortedPods;
    const start = (page - 1) * pageSize;
    return sortedResources.slice(start, start + pageSize);
  }, [activeTab, sortedResources, sortedPods, page, pageSize]);

  const totalForPagination = activeTab === 'Pod' ? (searchTerm.trim() ? sortedPods.length : podsTotal) : sortedResources.length;

  const handleResourceView = (resource: K8sResource) => {
    if (resource.kind === 'ServiceAccount') {
      navigate(`/identities/uid/${encodeURIComponent(resource.id)}`);
    } else if ((resource.kind === 'Role' || resource.kind === 'RoleBinding' || resource.kind === 'ClusterRole' || resource.kind === 'ClusterRoleBinding') && resource.clusterId) {
      navigate(`/clusters/${resource.clusterId}`);
    }
  };

  const renderTableRow = (resource: K8sResource) => {
    const canView = resource.kind === 'ServiceAccount' || (resource.clusterId && ['Role', 'RoleBinding', 'ClusterRole', 'ClusterRoleBinding'].includes(resource.kind));
    return (
      <tr
        key={resource.id}
        className={`hover:bg-slate-800/50 transition-colors border-b border-slate-800 last:border-0 ${canView ? 'cursor-pointer' : ''}`}
        onClick={canView ? () => handleResourceView(resource) : undefined}
      >
        <td className="px-6 py-4 font-medium text-white">{resource.name}</td>
        <td className="px-6 py-4 text-slate-400">{resource.namespace}</td>
        <td className="px-6 py-4 text-slate-400">{resource.kind}</td>
        <td className="px-6 py-4">
          <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border ${resource.status === 'Active' || resource.status === 'Running' ? 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20' : 'text-slate-400 bg-slate-800 border-slate-700'}`}>
            {resource.status ?? 'Active'}
          </span>
        </td>
        <td className="px-6 py-4 text-right" onClick={(e) => e.stopPropagation()}>
          {canView ? (
            <button className="text-pink-500 hover:text-pink-400 text-xs font-medium" onClick={() => handleResourceView(resource)}>
              {resource.kind === 'ServiceAccount' ? 'View identity' : 'View cluster'}
            </button>
          ) : (
            <span className="text-slate-500 text-xs">—</span>
          )}
        </td>
      </tr>
    );
  };

  const renderPodRow = (pod: PodWithRisk) => {
    const level = podRiskLevel(pod.riskCount);
    return (
      <tr key={pod.uid} className="hover:bg-slate-800/50 transition-colors border-b border-slate-800 last:border-0 cursor-pointer" onClick={() => navigate(`/resources/pods/${pod.id}`)}>
        <td className="px-6 py-4 font-medium text-white">{pod.name}</td>
        <td className="px-6 py-4 text-slate-400">{pod.namespace}</td>
        <td className="px-6 py-4 text-slate-400 font-mono text-xs">{pod.nodeName ?? '—'}</td>
        <td className="px-6 py-4">
          <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border ${getPodStatusBadgeClass(pod.status)}`}>{pod.status ?? '—'}</span>
        </td>
        <td className="px-6 py-4">
          <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border capitalize ${getSeverityBadgeClass(level)}`}>
            {level}
            {pod.riskCount > 0 && <span className="ml-1 opacity-90">({pod.riskCount})</span>}
          </span>
        </td>
        <td className="px-6 py-4 text-right" onClick={(e) => e.stopPropagation()}>
          <button className="text-pink-500 hover:text-pink-400 text-xs font-medium" onClick={() => navigate(`/resources/pods/${pod.id}`)}>View</button>
        </td>
      </tr>
    );
  };

  return (
    <PageLayout
      title="Resources Explorer"
      description="Inventory of Kubernetes resources and security context."
      actions={
        <>
          <Button variant="secondary" onClick={fetchResources} isLoading={loading}>
            <RefreshCw className="w-4 h-4 mr-2" /> Refresh
          </Button>
        </>
      }
      toolbar={
        <div className="flex flex-col sm:flex-row gap-3 justify-between items-stretch sm:items-center">
          <div className="relative flex-1 max-w-xs">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-500 w-4 h-4" />
            <input
              type="text"
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              placeholder={activeTab === 'Pod' ? 'Search pods in current page...' : 'Search resources...'}
              className="w-full bg-slate-950 border border-slate-700 rounded-lg pl-9 pr-4 py-2 text-sm text-white focus:outline-none focus:border-pink-500"
            />
          </div>
          <select
            value={namespaceFilter}
            onChange={(e) => {
              const v = e.target.value;
              setNamespaceFilter(v);
              setSearchParams((prev) => {
                const next = new URLSearchParams(prev);
                if (v) next.set('namespace', v); else next.delete('namespace');
                return next;
              });
            }}
            className="bg-slate-950 border border-slate-700 rounded-lg px-4 py-2 text-sm text-slate-300 focus:outline-none focus:border-pink-500 sm:w-48"
          >
            <option value="">All Namespaces</option>
            <option value="default">default</option>
            <option value="kube-system">kube-system</option>
          </select>
          <select
            value={sortBy}
            onChange={(e) => setSortBy(e.target.value as typeof sortBy)}
            className="bg-slate-950 border border-slate-700 rounded-lg px-4 py-2 text-sm text-slate-300 focus:outline-none focus:border-pink-500 sm:w-56"
          >
            <option value="name_asc">Sort: Name A-Z</option>
            <option value="namespace_asc">Sort: Namespace A-Z</option>
            <option value="risk_desc">{activeTab === 'Pod' ? 'Sort: Risk high to low' : 'Sort: Kind Z-A'}</option>
          </select>
        </div>
      }
    >
      <Card className="p-0 overflow-hidden">
        <div className="border-b border-slate-800 bg-slate-900/50">
          <nav className="flex overflow-x-auto">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => {
                  setActiveTab(tab.id as any);
                  setSearchParams((prev) => {
                    const next = new URLSearchParams(prev);
                    next.set('tab', tab.id);
                    return next;
                  });
                }}
                className={`
                  flex items-center px-6 py-4 text-sm font-medium border-b-2 transition-colors whitespace-nowrap
                  ${activeTab === tab.id
                    ? 'border-pink-500 text-pink-500 bg-slate-900'
                    : 'border-transparent text-slate-400 hover:text-slate-200 hover:bg-slate-800'
                  }
                `}
              >
                <span className="mr-2">{tab.icon}</span>
                {tab.label}
              </button>
            ))}
          </nav>
        </div>

        <div className="overflow-x-auto max-h-[calc(100vh-22rem)] overflow-y-auto">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-slate-400 uppercase bg-slate-950/30 border-b border-slate-800 sticky top-0 z-10">
              {renderTableHead()}
            </thead>
            <tbody className="divide-y divide-slate-800">
              {activeTab === 'Pod' ? (
                paginatedResources.length > 0 ? (
                  (paginatedResources as PodWithRisk[]).map(renderPodRow)
                ) : (
                  <tr>
                    <td colSpan={6} className="px-4 py-8">
                      <PageEmpty title="No pods found" description="Ensure Core and agents are syncing for the selected scope." className="py-6" />
                    </td>
                  </tr>
                )
              ) : (
                paginatedResources.length > 0 ? (
                  (paginatedResources as K8sResource[]).map(renderTableRow)
                ) : (
                  <tr>
                    <td colSpan={5} className="px-4 py-8">
                      <PageEmpty title="No resources found" description="Try adjusting tab, namespace, or search filters." className="py-6" />
                    </td>
                  </tr>
                )
              )}
            </tbody>
          </table>
        </div>
      </Card>
      {activeTab === 'Pod' && searchTerm.trim() && (
        <p className="text-xs text-slate-500 mt-2">Pod search applies to current loaded page.</p>
      )}
      {(activeTab === 'Pod' ? podsTotal > 0 : resources.length > 0) && (
        <Pagination
          page={page}
          pageSize={pageSize}
          total={totalForPagination}
          onPageChange={setPage}
          onPageSizeChange={(size) => { setPageSize(size); setPage(1); }}
          pageSizeOptions={PAGE_SIZE_OPTIONS}
          itemLabel={activeTab === 'Pod' ? 'pods' : 'resources'}
        />
      )}
    </PageLayout>
  );
};