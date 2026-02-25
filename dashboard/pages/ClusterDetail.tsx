import React, { useCallback, useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { Cluster, ClusterOverview, ClusterInventory, ClusterAgent, ClusterSecuritySummary } from '../types';
import { PageLayout } from '../components/PageLayout';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { ArrowLeft, Globe, Layers, Server, Shield } from 'lucide-react';
import clsx from 'clsx';
import { getClusterDisplayName } from '../lib/clusterDisplay';
import { PageLoading } from '../components/PageLoading';
import { PageEmpty } from '../components/PageEmpty';
import { formatDateTime, getAgentStatusLabel, getConnectionStatusClass, getConnectionStatusLabel } from '../lib/display';

type TabId = 'overview' | 'inventory' | 'agents' | 'security';

export const ClusterDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [cluster, setCluster] = useState<Cluster | null>(null);
  const [overview, setOverview] = useState<ClusterOverview | null>(null);
  const [inventory, setInventory] = useState<ClusterInventory | null>(null);
  const [agents, setAgents] = useState<ClusterAgent[]>([]);
  const [securitySummary, setSecuritySummary] = useState<ClusterSecuritySummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [tabLoading, setTabLoading] = useState(false);
  const [activeTab, setActiveTab] = useState<TabId>('overview');

  const fetchCluster = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    const [clusterData, statsList, overviewData] = await Promise.all([
      api.getCluster(id),
      api.getClustersStats().catch(() => []),
      api.getClusterOverview(id).catch(() => null),
    ]);
    const fromStats = Array.isArray(statsList) ? statsList.find((c) => c.id === id) : null;
    if (clusterData && fromStats) {
      setCluster({ ...clusterData, podCount: fromStats.podCount, deploymentCount: fromStats.deploymentCount, riskCount: fromStats.riskCount, agentCount: fromStats.agentCount, connectionStatus: fromStats.connectionStatus });
    } else {
      setCluster(clusterData ?? null);
    }
    setOverview(overviewData ?? null);
    setLoading(false);
  }, [id]);

  const fetchTabData = useCallback(async (tab: TabId) => {
    if (!id) return;
    setTabLoading(true);
    try {
      if (tab === 'inventory') {
        const data = await api.getClusterInventory(id);
        setInventory(data ?? null);
      } else if (tab === 'agents') {
        const { agents: list } = await api.getClusterAgents(id);
        setAgents(list);
      } else if (tab === 'security') {
        const data = await api.getClusterSecuritySummary(id);
        setSecuritySummary(data ?? null);
      }
    } finally {
      setTabLoading(false);
    }
  }, [id]);

  useEffect(() => {
    fetchCluster();
  }, [fetchCluster]);

  useEffect(() => {
    if (id && activeTab !== 'overview') fetchTabData(activeTab);
  }, [id, activeTab, fetchTabData]);

  if (loading || !id) {
    return <PageLoading message="Loading cluster detail..." className="min-h-[40vh]" />;
  }

  if (!cluster) {
    return (
      <PageLayout title="Cluster not found" description="The cluster may have been removed or you lack access.">
        <Button variant="secondary" onClick={() => navigate('/clusters')}>
          <ArrowLeft className="w-4 h-4 mr-2" /> Back to Clusters
        </Button>
      </PageLayout>
    );
  }

  const tabs: { id: TabId; label: string; icon: React.ReactNode }[] = [
    { id: 'overview', label: 'Overview', icon: <Globe className="w-4 h-4" /> },
    { id: 'inventory', label: 'Inventory', icon: <Layers className="w-4 h-4" /> },
    { id: 'agents', label: 'Agents', icon: <Server className="w-4 h-4" /> },
    { id: 'security', label: 'Security Summary', icon: <Shield className="w-4 h-4" /> },
  ];

  return (
    <PageLayout
      title={getClusterDisplayName(cluster)}
      description={cluster.distribution ? `${cluster.distribution} · ${cluster.version ?? cluster.k8sVersion ?? ''}` : cluster.version ?? cluster.k8sVersion ?? undefined}
      actions={
        <Button variant="secondary" onClick={() => navigate('/clusters')}>
          <ArrowLeft className="w-4 h-4 mr-2" /> Back to Clusters
        </Button>
      }
    >
      <div className="mb-4 flex flex-wrap items-center gap-2 text-xs">
        <span className={`px-2.5 py-1 rounded-full font-medium ${getConnectionStatusClass(cluster.connectionStatus)}`}>
          {getConnectionStatusLabel(cluster.connectionStatus)}
        </span>
        <span className="text-slate-500">Cluster ID:</span>
        <span className="text-slate-300 font-mono">{cluster.id}</span>
      </div>
      {/* Context summary – core metrics */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <Card className="p-4">
          <div className="text-slate-400 text-sm">Pods</div>
          <div className="text-xl font-bold text-white">{cluster.podCount ?? cluster.pods ?? '—'}</div>
        </Card>
        <Card className="p-4">
          <div className="text-slate-400 text-sm">Deployments</div>
          <div className="text-xl font-bold text-white">{cluster.deploymentCount ?? '—'}</div>
        </Card>
        <Card className="p-4">
          <div className="text-slate-400 text-sm">Risks</div>
          <div className="text-xl font-bold text-white">{cluster.riskCount ?? '—'}</div>
        </Card>
        <Card className="p-4">
          <div className="text-slate-400 text-sm">Agents</div>
          <div className="text-xl font-bold text-white">{cluster.agentCount ?? '—'}</div>
        </Card>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 p-1 bg-slate-900/80 rounded-lg border border-slate-800 w-fit mb-6">
        {tabs.map(({ id: tabId, label, icon }) => (
          <button
            key={tabId}
            onClick={() => setActiveTab(tabId)}
            className={clsx(
              'flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors',
              activeTab === tabId ? 'bg-pink-600 text-white shadow' : 'text-slate-400 hover:bg-slate-800 hover:text-white'
            )}
          >
            {icon}
            {label}
          </button>
        ))}
      </div>

      {/* Tab content – real data from Core APIs */}
      {activeTab === 'overview' && (
        <Card className="p-6">
          <h3 className="text-lg font-semibold text-white mb-4">Overview</h3>
          <dl className="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
            <div>
              <dt className="text-slate-500">Cluster ID</dt>
              <dd className="text-white font-mono">{cluster.id}</dd>
            </div>
            <div>
              <dt className="text-slate-500">Status</dt>
              <dd className="text-white capitalize">{cluster.status ?? '—'}</dd>
            </div>
            <div>
              <dt className="text-slate-500">Last sync</dt>
              <dd className="text-slate-300">{formatDateTime(cluster.lastSync)}</dd>
            </div>
            <div>
              <dt className="text-slate-500">Version</dt>
              <dd className="text-slate-300 font-mono">{cluster.version ?? cluster.k8sVersion ?? '—'}</dd>
            </div>
            {overview && (
              <>
                <div>
                  <dt className="text-slate-500">Pods</dt>
                  <dd className="text-white">{overview.podCount}</dd>
                </div>
                <div>
                  <dt className="text-slate-500">Nodes</dt>
                  <dd className="text-white">{overview.nodeCount}</dd>
                </div>
                <div>
                  <dt className="text-slate-500">Namespaces</dt>
                  <dd className="text-white">{overview.namespaceCount}</dd>
                </div>
              </>
            )}
          </dl>
        </Card>
      )}

      {activeTab === 'inventory' && (
        <Card className="p-6">
          <h3 className="text-lg font-semibold text-white mb-4">Inventory</h3>
          {tabLoading ? (
            <PageLoading message="Loading inventory..." className="py-8" />
          ) : inventory ? (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div>
                <h4 className="text-slate-400 text-sm font-medium mb-2">Nodes ({inventory.nodes.length})</h4>
                <ul className="space-y-1 text-sm text-slate-300 font-mono max-h-48 overflow-y-auto">
                  {inventory.nodes.length === 0 ? (
                    <li className="text-slate-500">No nodes</li>
                  ) : (
                    inventory.nodes.map((n) => (
                      <li key={n}>
                        <button
                          type="button"
                          className="hover:text-pink-400 hover:underline text-left w-full"
                          onClick={() => id && navigate(`/clusters/${id}/nodes/${encodeURIComponent(n)}`)}
                        >
                          {n}
                        </button>
                      </li>
                    ))
                  )}
                </ul>
              </div>
              <div>
                <h4 className="text-slate-400 text-sm font-medium mb-2">Namespaces ({inventory.namespaces.length})</h4>
                <ul className="space-y-1 text-sm text-slate-300 font-mono max-h-48 overflow-y-auto">
                  {inventory.namespaces.length === 0 ? <li className="text-slate-500">No namespaces</li> : inventory.namespaces.map((ns) => (
                    <li key={ns}>
                      <button type="button" className="hover:text-pink-400 hover:underline text-left w-full" onClick={() => navigate(`/resources?tab=Pod&namespace=${encodeURIComponent(ns)}`)}>
                        {ns}
                      </button>
                    </li>
                  ))}
                </ul>
              </div>
            </div>
          ) : (
            <PageEmpty title="No inventory data" description="No nodes or namespaces returned for this cluster." className="py-8" />
          )}
        </Card>
      )}

      {activeTab === 'agents' && (
        <Card className="p-6">
          <h3 className="text-lg font-semibold text-white mb-4">Agents ({agents.length})</h3>
          {tabLoading ? (
            <PageLoading message="Loading agents..." className="py-8" />
          ) : agents.length > 0 ? (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead className="text-slate-400 border-b border-slate-800">
                  <tr>
                    <th className="text-left py-2">Node</th>
                    <th className="text-left py-2">Status</th>
                    <th className="text-left py-2">Last heartbeat</th>
                    <th className="text-left py-2">Version</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800">
                  {agents.map((a) => (
                    <tr key={a.agentId}>
                      <td className="py-2 font-mono text-white">{a.nodeName ?? '—'}</td>
                      <td className="py-2">
                        <span className={a.status === 'healthy' ? 'text-emerald-400' : a.status === 'slow' ? 'text-amber-400' : 'text-red-400'}>
                          {getAgentStatusLabel(a.status)}
                        </span>
                      </td>
                      <td className="py-2 text-slate-400">{formatDateTime(a.lastHeartbeat)}</td>
                      <td className="py-2 text-slate-400">{a.version ?? '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <div>
              <p className="text-slate-500 text-sm">No agents for this cluster. Agents are matched by node name from pods.</p>
              <Button variant="secondary" className="mt-4" onClick={() => navigate('/monitoring')}>
                View all agents
              </Button>
            </div>
          )}
        </Card>
      )}

      {activeTab === 'security' && (
        <Card className="p-6">
          <h3 className="text-lg font-semibold text-white mb-4">Security Summary</h3>
          {tabLoading ? (
            <PageLoading message="Loading security summary..." className="py-8" />
          ) : securitySummary ? (
            <div className="space-y-4">
              <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
                <div className="bg-slate-900/50 rounded-lg p-3 border border-slate-800">
                  <div className="text-slate-400 text-xs uppercase">Critical</div>
                  <div className="text-xl font-bold text-red-400">{securitySummary.criticalCount}</div>
                </div>
                <div className="bg-slate-900/50 rounded-lg p-3 border border-slate-800">
                  <div className="text-slate-400 text-xs uppercase">High</div>
                  <div className="text-xl font-bold text-orange-400">{securitySummary.highCount}</div>
                </div>
                <div className="bg-slate-900/50 rounded-lg p-3 border border-slate-800">
                  <div className="text-slate-400 text-xs uppercase">Medium</div>
                  <div className="text-xl font-bold text-amber-400">{securitySummary.mediumCount}</div>
                </div>
                <div className="bg-slate-900/50 rounded-lg p-3 border border-slate-800">
                  <div className="text-slate-400 text-xs uppercase">Low</div>
                  <div className="text-xl font-bold text-slate-400">{securitySummary.lowCount}</div>
                </div>
                <div className="bg-slate-900/50 rounded-lg p-3 border border-slate-800">
                  <div className="text-slate-400 text-xs uppercase">Capabilities</div>
                  <div className="text-xl font-bold text-cyan-400">{securitySummary.capabilityCount}</div>
                </div>
              </div>
              <Button variant="secondary" onClick={() => navigate('/risks')}>
                View all risks
              </Button>
            </div>
          ) : (
            <div>
              <PageEmpty title="No security summary data" description="Severity totals are unavailable for this cluster scope." className="py-2" />
              <Button variant="secondary" className="mt-4" onClick={() => navigate('/risks')}>
                View all risks
              </Button>
            </div>
          )}
        </Card>
      )}
    </PageLayout>
  );
};
