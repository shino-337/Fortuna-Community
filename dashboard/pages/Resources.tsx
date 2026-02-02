import React, { useState, useEffect, useMemo } from 'react';
import { Card } from '../components/ui/Card';
import { api } from '../lib/api';
import { K8sResource } from '../types';
import { Box, UserCog, Scroll, Key, RefreshCw, Download, Plus, Search, Filter } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../components/PageLayout';
import { Pagination } from '../components/Pagination';

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];

export const Resources: React.FC = () => {
  const [activeTab, setActiveTab] = useState<'Pod' | 'ServiceAccount' | 'Role' | 'RoleBinding'>('Pod');
  const [resources, setResources] = useState<K8sResource[]>([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  useEffect(() => {
    setPage(1);
    fetchResources();
  }, [activeTab]);

  const fetchResources = async () => {
    setLoading(true);
    const data = await api.getResources(activeTab);
    setResources(data);
    setLoading(false);
  };

  const tabs = [
    { id: 'Pod', label: 'Pods', icon: <Box size={16} /> },
    { id: 'ServiceAccount', label: 'Service Accounts', icon: <UserCog size={16} /> },
    { id: 'Role', label: 'Roles', icon: <Scroll size={16} /> },
    { id: 'RoleBinding', label: 'Role Bindings', icon: <Key size={16} /> },
  ];

  const getRiskBadge = (level?: string, score?: number) => {
    const scoreElement = score !== undefined ? <span className="ml-1 opacity-75">({score})</span> : null;
    switch (level) {
      case 'critical': return <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-red-500/10 text-red-400 border border-red-500/20">🔴 CRITICAL{scoreElement}</span>;
      case 'high': return <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-orange-500/10 text-orange-400 border border-orange-500/20">🟡 HIGH{scoreElement}</span>;
      case 'medium': return <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-yellow-500/10 text-yellow-400 border border-yellow-500/20">🟡 MEDIUM{scoreElement}</span>;
      default: return <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">🟢 LOW{scoreElement}</span>;
    }
  };

  const renderTableHead = () => {
    // Simplified logic for brevity, expands based on tab
    return (
      <tr>
        <th className="px-6 py-4 font-medium">Name</th>
        <th className="px-6 py-4 font-medium">Namespace</th>
        <th className="px-6 py-4 font-medium">
            {activeTab === 'Pod' ? 'Service Account' : activeTab === 'ServiceAccount' ? 'Pods' : 'Rules/Ref'}
        </th>
        <th className="px-6 py-4 font-medium">Status</th>
        <th className="px-6 py-4 font-medium">Risk</th>
        <th className="px-6 py-4 font-medium text-right">Actions</th>
      </tr>
    );
  };

  const paginatedResources = useMemo(() => {
    const start = (page - 1) * pageSize;
    return resources.slice(start, start + pageSize);
  }, [resources, page, pageSize]);

  const renderTableRow = (resource: K8sResource) => (
    <tr key={resource.id} className="hover:bg-slate-800/50 transition-colors border-b border-slate-800 last:border-0">
        <td className="px-6 py-4 font-medium text-white">{resource.name}</td>
        <td className="px-6 py-4 text-slate-400">{resource.namespace}</td>
        <td className="px-6 py-4 text-slate-400 font-mono text-xs">
            {resource.saName || resource.podsCount || resource.rulesCount || resource.roleRef || '-'}
        </td>
        <td className="px-6 py-4">
            <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border ${resource.status === 'Active' || resource.status === 'Running' ? 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20' : 'text-slate-400 bg-slate-800 border-slate-700'}`}>
                {resource.status}
            </span>
        </td>
        <td className="px-6 py-4">{getRiskBadge(resource.riskLevel, resource.riskScore)}</td>
        <td className="px-6 py-4 text-right">
            <button className="text-pink-500 hover:text-pink-400 text-xs font-medium">View</button>
        </td>
    </tr>
  );

  return (
    <PageLayout
      title="Resources Explorer"
      description="Inventory of Kubernetes resources and security context."
      actions={
        <>
          <Button variant="secondary" onClick={fetchResources} isLoading={loading}>
            <RefreshCw className="w-4 h-4 mr-2" /> Refresh
          </Button>
          <Button><Plus className="w-4 h-4 mr-2" /> Add</Button>
        </>
      }
      toolbar={
        <div className="flex flex-col sm:flex-row gap-3 justify-between items-stretch sm:items-center">
          <div className="relative flex-1 max-w-xs">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-500 w-4 h-4" />
            <input type="text" placeholder="Search..." className="w-full bg-slate-950 border border-slate-700 rounded-lg pl-9 pr-4 py-2 text-sm text-white focus:outline-none focus:border-pink-500" />
          </div>
          <select className="bg-slate-950 border border-slate-700 rounded-lg px-4 py-2 text-sm text-slate-300 focus:outline-none focus:border-pink-500 sm:w-48">
            <option>All Namespaces</option>
            <option>default</option>
            <option>kube-system</option>
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
                onClick={() => setActiveTab(tab.id as any)}
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
              {paginatedResources.length > 0 ? paginatedResources.map(renderTableRow) : (
                <tr><td colSpan={6} className="p-8 text-center text-slate-500">No resources found</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </Card>
      {resources.length > 0 && (
        <Pagination
          page={page}
          pageSize={pageSize}
          total={resources.length}
          onPageChange={setPage}
          onPageSizeChange={(size) => { setPageSize(size); setPage(1); }}
          pageSizeOptions={PAGE_SIZE_OPTIONS}
          itemLabel="resources"
        />
      )}
    </PageLayout>
  );
};