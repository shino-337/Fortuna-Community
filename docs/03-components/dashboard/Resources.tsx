
import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card } from '../components/ui/Card';
import { api } from '../lib/api';
import { K8sResource } from '../types';
import { Box, UserCog, Scroll, Key, RefreshCw, Plus, Search } from 'lucide-react';
import { Button } from '../components/ui/Button';

export const Resources: React.FC = () => {
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<'Pod' | 'ServiceAccount' | 'Role' | 'RoleBinding' | 'Node'>('Pod');
  const [resources, setResources] = useState<K8sResource[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
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
    { id: 'Node', label: 'Nodes', icon: <Box size={16} /> },
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

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
           <h1 className="text-2xl font-bold text-white">Resources Explorer</h1>
           <p className="text-slate-400">Inventory of Kubernetes resources and security context.</p>
        </div>
        <div className="flex space-x-2">
            <Button variant="secondary" onClick={fetchResources} isLoading={loading}><RefreshCw className="w-4 h-4 mr-2" /> Refresh</Button>
            <Button><Plus className="w-4 h-4 mr-2" /> Add</Button>
        </div>
      </div>

      <Card className="p-0 overflow-hidden">
        <div className="border-b border-slate-800 bg-slate-900/50">
          <nav className="flex overflow-x-auto">
            {tabs.map((tab) => (
              <button key={tab.id} onClick={() => setActiveTab(tab.id as any)} className={`flex items-center px-6 py-4 text-sm font-medium border-b-2 transition-colors whitespace-nowrap ${activeTab === tab.id ? 'border-pink-500 text-pink-500 bg-slate-900' : 'border-transparent text-slate-400 hover:text-slate-200 hover:bg-slate-800'}`}>
                <span className="mr-2">{tab.icon}</span> {tab.label}
              </button>
            ))}
          </nav>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-slate-400 uppercase bg-slate-950/30 border-b border-slate-800">
              <tr>
                <th className="px-6 py-4 font-medium">Name</th>
                <th className="px-6 py-4 font-medium">Namespace</th>
                <th className="px-6 py-4 font-medium">{activeTab === 'Pod' ? 'Node' : 'Info'}</th>
                <th className="px-6 py-4 font-medium">Status</th>
                <th className="px-6 py-4 font-medium">Risk</th>
                <th className="px-6 py-4 font-medium text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800">
              {resources.map((resource) => (
                <tr key={resource.id} className="hover:bg-slate-800/50 transition-colors cursor-pointer group" onClick={() => navigate(`/resources/${resource.id}`)}>
                    <td className="px-6 py-4 font-medium text-white group-hover:text-pink-500 transition-colors">{resource.name}</td>
                    <td className="px-6 py-4 text-slate-400">{resource.namespace}</td>
                    <td className="px-6 py-4 text-slate-500 font-mono text-xs">{resource.node || resource.podsCount || resource.rulesCount || '-'}</td>
                    <td className="px-6 py-4">
                        <span className={`inline-flex items-center px-2 py-0.5 rounded text-[10px] font-bold uppercase border ${resource.status === 'Running' || resource.status === 'Ready' || resource.status === 'Active' ? 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20' : 'text-slate-400 bg-slate-800 border-slate-700'}`}>
                            {resource.status}
                        </span>
                    </td>
                    <td className="px-6 py-4">{getRiskBadge(resource.riskLevel, resource.riskScore)}</td>
                    <td className="px-6 py-4 text-right">
                        <button className="text-slate-500 group-hover:text-pink-500 transition-colors">View Details</button>
                    </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
    </div>
  );
};
