import React, { useCallback, useEffect, useState } from 'react';
import { matchPath, useLocation, useNavigate, useParams } from 'react-router-dom';
import { api } from '../lib/api';
import { Cluster, ClusterOverview, ClusterInventory, ClusterAgent, ClusterSecuritySummary } from '../types';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Tabs } from '../design-system/components/Tabs';
import { Card } from '../design-system/components/Card';
import { Button } from '../components/ui/Button';
import { ArrowLeft, Globe, Layers, Server, Shield } from 'lucide-react';
import clsx from 'clsx';
import { getClusterDisplayName } from '../lib/clusterDisplay';
import { PageEmpty, PageLoading } from '../design-system/components/PageStatus';
import { formatDateTime, getAgentStatusLabel, getConnectionStatusClass, getConnectionStatusLabel } from '../lib/display';
import { UI_TABLE, UI_TD, UI_TH, UI_TR, UI_THEAD_STICKY } from '../lib/tableChrome';

type TabId = 'overview' | 'inventory' | 'agents' | 'security';

export const ClusterDetail: React.FC = () => {
  const params = useParams<{ id: string }>();
  const location = useLocation();
  const id = params.id ?? matchPath({ path: '/clusters/:id', end: true }, location.pathname)?.params.id;
  const navigate = useNavigate();
  const [cluster, setCluster] = useState<Cluster | null>(null);
  const [overview, setOverview] = useState<ClusterOverview | null>(null);
  const [inventory, setInventory] = useState<ClusterInventory | null>(null);
  const [agents, setAgents] = useState<ClusterAgent[]>([]);
  const [securitySummary, setSecuritySummary] = useState<ClusterSecuritySummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [tabLoading, setTabLoading] = useState(false);
  const [activeTab, setActiveTab] = useState<TabId>('overview');

  const fetchCluster = useCallback(async () => {
    if (!id) {
      setLoading(false);
      setCluster(null);
      setOverview(null);
      return;
    }
    setLoading(true);
    setLoadError(null);
    try {
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
    } catch (err) {
      setCluster(null);
      setOverview(null);
      setLoadError(err instanceof Error ? err.message : 'Cluster detail could not be loaded.');
    } finally {
      setLoading(false);
    }
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

  if (loading) {
    return <PageLoading message="Loading cluster detail..." className="min-h-[40dvh]" />;
  }

  if (!id || !cluster) {
    return (
      <PageLayout
        title={loadError ? 'Cluster detail unavailable' : 'Cluster not found'}
        description={loadError ?? 'The cluster may have been removed or you lack access.'}
      >
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
      <div className="mb-4 flex flex-wrap items-center gap-2 text-caption">
        <span className={`px-2.5 py-1 rounded-full font-medium ${getConnectionStatusClass(cluster.connectionStatus)}`}>
          {getConnectionStatusLabel(cluster.connectionStatus)}
        </span>
        <span className="text-muted">Cluster ID:</span>
        <span className="text-text font-mono">{cluster.id}</span>
      </div>
      {/* Context summary – core metrics */}
      <div className="mb-6 grid grid-cols-1 gap-3 min-[420px]:grid-cols-2 lg:grid-cols-4">
        <Card className="p-4">
          <div className="text-muted text-body">Pods</div>
          <div className="text-xl font-bold text-text">{cluster.podCount ?? cluster.pods ?? '—'}</div>
        </Card>
        <Card className="p-4">
          <div className="text-muted text-body">Deployments</div>
          <div className="text-xl font-bold text-text">{cluster.deploymentCount ?? '—'}</div>
        </Card>
        <Card className="p-4">
          <div className="text-muted text-body">Risks</div>
          <div className="text-xl font-bold text-text">{cluster.riskCount ?? '—'}</div>
        </Card>
        <Card className="p-4">
          <div className="text-muted text-body">Agents</div>
          <div className="text-xl font-bold text-text">{cluster.agentCount ?? '—'}</div>
        </Card>
      </div>

      {/* Tabs */}
      <Tabs items={tabs} value={activeTab} onChange={(id) => setActiveTab(id as TabId)} />

      {/* Tab content – real data from Core APIs */}
      {activeTab === 'overview' && (
        <Card className="p-6">
          <h3 className="text-section-title text-text mb-4">Overview</h3>
          <dl className="grid grid-cols-1 md:grid-cols-2 gap-4 text-body">
            <div>
              <dt className="text-muted">Cluster ID</dt>
              <dd className="text-text font-mono">{cluster.id}</dd>
            </div>
            <div>
              <dt className="text-muted">Status</dt>
              <dd className="text-text capitalize">{cluster.status ?? '—'}</dd>
            </div>
            <div>
              <dt className="text-muted">Last sync</dt>
              <dd className="text-text">{formatDateTime(cluster.lastSync)}</dd>
            </div>
            <div>
              <dt className="text-muted">Version</dt>
              <dd className="text-text font-mono">{cluster.version ?? cluster.k8sVersion ?? '—'}</dd>
            </div>
            {overview && (
              <>
                <div>
                  <dt className="text-muted">Pods</dt>
                  <dd className="text-text">{overview.podCount}</dd>
                </div>
                <div>
                  <dt className="text-muted">Nodes</dt>
                  <dd className="text-text">{overview.nodeCount}</dd>
                </div>
                <div>
                  <dt className="text-muted">Namespaces</dt>
                  <dd className="text-text">{overview.namespaceCount}</dd>
                </div>
              </>
            )}
          </dl>
        </Card>
      )}

      {activeTab === 'inventory' && (
        <Card className="p-6">
          <h3 className="text-section-title text-text mb-4">Inventory</h3>
          {tabLoading ? (
            <PageLoading message="Loading inventory..." className="py-8" />
          ) : inventory ? (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div>
                <h4 className="text-muted text-body font-medium mb-2">Nodes ({inventory.nodes.length})</h4>
                <ul className="space-y-1 text-body text-text font-mono max-h-48 overflow-y-auto">
                  {inventory.nodes.length === 0 ? (
                    <li className="text-muted">No nodes</li>
                  ) : (
                    inventory.nodes.map((n) => (
                      <li key={n}>
                        <button
                          type="button"
                          className="hover:text-brand hover:underline text-left w-full"
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
                <h4 className="text-muted text-body font-medium mb-2">Namespaces ({inventory.namespaces.length})</h4>
                <ul className="space-y-1 text-body text-text font-mono max-h-48 overflow-y-auto">
                  {inventory.namespaces.length === 0 ? <li className="text-muted">No namespaces</li> : inventory.namespaces.map((ns) => (
                    <li key={ns}>
                      <button type="button" className="hover:text-brand hover:underline text-left w-full" onClick={() => navigate(`/resources?tab=Pod&namespace=${encodeURIComponent(ns)}`)}>
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
          <h3 className="text-section-title text-text mb-4">Agents ({agents.length})</h3>
          {tabLoading ? (
            <PageLoading message="Loading agents..." className="py-8" />
          ) : agents.length > 0 ? (
            <div className="ui-table-scroll rounded-lg border border-border">
              <table className={UI_TABLE}>
                <thead className={UI_THEAD_STICKY}>
                  <tr>
                    <th className={UI_TH}>Node</th>
                    <th className={UI_TH}>Status</th>
                    <th className={UI_TH}>Last heartbeat</th>
                    <th className={UI_TH}>Version</th>
                  </tr>
                </thead>
                <tbody>
                  {agents.map((a) => (
                    <tr key={a.agentId} className={UI_TR}>
                      <td className={`${UI_TD} font-mono text-text`}>{a.nodeName ?? '—'}</td>
                      <td className={UI_TD}>
                        <span className={a.status === 'healthy' ? 'text-emerald-400' : a.status === 'slow' ? 'text-amber-400' : 'text-red-400'}>
                          {getAgentStatusLabel(a.status)}
                        </span>
                      </td>
                      <td className={`${UI_TD} text-muted`}>{formatDateTime(a.lastHeartbeat)}</td>
                      <td className={`${UI_TD} text-muted`}>{a.version ?? '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <div>
              <p className="text-muted text-body">No agents for this cluster. Agents are matched by node name from pods.</p>
              <Button variant="secondary" className="mt-4" onClick={() => navigate('/monitoring')}>
                View all agents
              </Button>
            </div>
          )}
        </Card>
      )}

      {activeTab === 'security' && (
        <Card className="p-6">
          <h3 className="text-section-title text-text mb-4">Security Summary</h3>
          {tabLoading ? (
            <PageLoading message="Loading security summary..." className="py-8" />
          ) : securitySummary ? (
            <div className="space-y-4">
              <div className="grid grid-cols-1 gap-3 min-[420px]:grid-cols-2 lg:grid-cols-5">
                <div className="bg-surface/50 rounded-lg p-3 border border-border">
                  <div className="text-muted text-caption uppercase">Critical</div>
                  <div className="text-xl font-bold text-red-400">{securitySummary.criticalCount}</div>
                </div>
                <div className="bg-surface/50 rounded-lg p-3 border border-border">
                  <div className="text-muted text-caption uppercase">High</div>
                  <div className="text-xl font-bold text-orange-400">{securitySummary.highCount}</div>
                </div>
                <div className="bg-surface/50 rounded-lg p-3 border border-border">
                  <div className="text-muted text-caption uppercase">Medium</div>
                  <div className="text-xl font-bold text-amber-400">{securitySummary.mediumCount}</div>
                </div>
                <div className="bg-surface/50 rounded-lg p-3 border border-border">
                  <div className="text-muted text-caption uppercase">Low</div>
                  <div className="text-xl font-bold text-muted">{securitySummary.lowCount}</div>
                </div>
                <div className="bg-surface/50 rounded-lg p-3 border border-border">
                  <div className="text-muted text-caption uppercase">Capabilities</div>
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
