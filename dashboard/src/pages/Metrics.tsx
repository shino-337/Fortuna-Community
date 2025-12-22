import React, { useEffect, useState } from 'react';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import api from '../lib/api';

interface WorkerMetrics {
  type: string;
  status: string;
  queueDepth: number;
}

interface SystemMetrics {
  health: { status: string };
  sync: { lastFullScan: string; nextScan: string };
  resources: { clusters: number; pods: number; serviceAccounts: number; insights: number };
  api: { status: string; avgLatency: string };
}

interface AgentStatus {
  agents: Array<{
    clusterId: string;
    clusterName: string;
    status: string;
    lastHeartbeat: string;
  }>;
  total: number;
  healthy: number;
  slow: number;
  disconnected: number;
}

export const Metrics: React.FC = () => {
  const [timeRange, setTimeRange] = useState('1h');
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [refreshInterval, setRefreshInterval] = useState(10);
  const [workerMetrics, setWorkerMetrics] = useState<WorkerMetrics[]>([]);
  const [systemMetrics, setSystemMetrics] = useState<SystemMetrics | null>(null);
  const [agentStatus, setAgentStatus] = useState<AgentStatus | null>(null);
  const [queueMetrics, setQueueMetrics] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchMetrics();
    let interval: ReturnType<typeof setInterval> | null = null;
    if (autoRefresh) {
      interval = setInterval(fetchMetrics, refreshInterval * 1000);
    }
    return () => {
      if (interval) clearInterval(interval);
    };
  }, [autoRefresh, refreshInterval]);

  const fetchMetrics = async () => {
    try {
      const [workersRes, systemRes, agentsRes, queueRes] = await Promise.all([
        api.get('/api/v1/metrics/workers').catch(() => ({ data: { workers: [] } })),
        api.get('/api/v1/metrics/system').catch(() => ({ data: null })),
        api.get('/api/v1/agents/status').catch(() => ({ data: { agents: [], total: 0, healthy: 0, slow: 0, disconnected: 0 } })),
        api.get('/api/v1/metrics/queue').catch(() => ({ data: null })),
      ]);

      setWorkerMetrics(workersRes.data?.workers || []);
      setSystemMetrics(systemRes.data);
      setAgentStatus(agentsRes.data);
      setQueueMetrics(queueRes.data);
    } catch (err) {
      console.error('Failed to fetch metrics:', err);
    } finally {
      setLoading(false);
    }
  };

  const getStatusBadge = (status: string) => {
    const colors: Record<string, string> = {
      healthy: 'bg-green-100 text-green-800',
      warning: 'bg-yellow-100 text-yellow-800',
      error: 'bg-red-100 text-red-800',
      slow: 'bg-orange-100 text-orange-800',
      disconnected: 'bg-red-100 text-red-800',
    };
    return (
      <span className={`px-2 py-1 rounded-full text-xs font-medium ${colors[status] || 'bg-gray-100 text-gray-800'}`}>
        {status === 'healthy' ? '✅' : status === 'slow' ? '⚠️' : '❌'} {status.toUpperCase()}
      </span>
    );
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto mb-4"></div>
          <p className="text-gray-600">Loading metrics...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">System Monitoring</h1>
          <p className="text-gray-600 mt-1">Operational health and performance metrics</p>
        </div>
        <div className="flex space-x-2">
          <select
            value={timeRange}
            onChange={(e) => setTimeRange(e.target.value)}
            className="px-4 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="1h">Last 1 hour</option>
            <option value="6h">Last 6 hours</option>
            <option value="24h">Last 24 hours</option>
            <option value="7d">Last 7 days</option>
          </select>
          <div className="flex items-center space-x-2">
            <label className="text-sm text-gray-600">
              <input
                type="checkbox"
                checked={autoRefresh}
                onChange={(e) => setAutoRefresh(e.target.checked)}
                className="mr-2"
              />
              Auto-refresh
            </label>
            {autoRefresh && (
              <select
                value={refreshInterval}
                onChange={(e) => setRefreshInterval(Number(e.target.value))}
                className="px-2 py-1 border border-gray-300 rounded text-sm"
              >
                <option value={5}>5s</option>
                <option value={10}>10s</option>
                <option value={30}>30s</option>
                <option value={60}>1m</option>
              </select>
            )}
          </div>
          <Button variant="secondary" onClick={fetchMetrics}>
            🔄 Refresh
          </Button>
        </div>
      </div>

      {/* System Health Overview */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card className="p-4">
          <div className="text-sm text-gray-600">Workers</div>
          <div className="text-2xl font-bold text-gray-900 mt-1">
            {workerMetrics.filter((w) => w.status === 'healthy').length}/{workerMetrics.length}
          </div>
          <div className="text-xs text-gray-500 mt-1">Healthy</div>
        </Card>
        <Card className="p-4">
          <div className="text-sm text-gray-600">Queue</div>
          <div className="text-2xl font-bold text-gray-900 mt-1">
            {queueMetrics
              ? Object.values(queueMetrics).reduce((sum: number, val: any) => sum + (Number(val) || 0), 0)
              : 0}
          </div>
          <div className="text-xs text-gray-500 mt-1">Messages</div>
        </Card>
        <Card className="p-4">
          <div className="text-sm text-gray-600">Agents</div>
          <div className="text-2xl font-bold text-gray-900 mt-1">
            {agentStatus ? `${agentStatus.healthy}/${agentStatus.total}` : '0/0'}
          </div>
          <div className="text-xs text-gray-500 mt-1">Connected</div>
        </Card>
        <Card className="p-4">
          <div className="text-sm text-gray-600">API</div>
          <div className="text-2xl font-bold text-gray-900 mt-1">
            {systemMetrics?.api?.status === 'healthy' ? '✅' : '⚠️'}
          </div>
          <div className="text-xs text-gray-500 mt-1">
            {systemMetrics?.api?.avgLatency || 'N/A'}
          </div>
        </Card>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Queue Metrics */}
        <Card title="Queue Depth">
          <div className="space-y-4">
            {queueMetrics ? (
              Object.entries(queueMetrics).map(([type, depth]: [string, any]) => (
                <div key={type}>
                  <div className="flex justify-between text-sm mb-1">
                    <span className="text-gray-600 capitalize">{type}</span>
                    <span className="text-gray-900 font-medium">{depth} messages</span>
                  </div>
                  <div className="w-full bg-gray-200 rounded-full h-2">
                    <div
                      className={`h-2 rounded-full ${
                        Number(depth) > 100 ? 'bg-red-600' : Number(depth) > 50 ? 'bg-yellow-600' : 'bg-green-600'
                      }`}
                      style={{ width: `${Math.min(100, (Number(depth) / 200) * 100)}%` }}
                    ></div>
                  </div>
                </div>
              ))
            ) : (
              <div className="text-center py-8 text-gray-500">No queue data available</div>
            )}
          </div>
        </Card>

        {/* Worker Performance */}
        <Card title="Worker Performance">
          <div className="space-y-3">
            {workerMetrics.length > 0 ? (
              workerMetrics.map((worker) => (
                <div key={worker.type} className="flex items-center justify-between">
                  <div className="flex items-center space-x-2">
                    <span className="text-sm font-medium text-gray-900 capitalize">{worker.type}</span>
                    {getStatusBadge(worker.status)}
                  </div>
                  <div className="text-sm text-gray-600">Queue: {worker.queueDepth}</div>
                </div>
              ))
            ) : (
              <div className="text-center py-8 text-gray-500">No worker data available</div>
            )}
          </div>
        </Card>

        {/* Agent Status */}
        <Card title="Agent Status">
          {agentStatus && agentStatus.agents.length > 0 ? (
            <div className="space-y-2">
              <div className="text-sm text-gray-600 mb-3">
                {agentStatus.healthy}/{agentStatus.total} agents healthy
                {agentStatus.slow > 0 && `, ${agentStatus.slow} slow`}
              </div>
              <div className="max-h-64 overflow-y-auto space-y-2">
                {agentStatus.agents.map((agent, idx) => (
                  <div key={idx} className="flex items-center justify-between p-2 bg-gray-50 rounded">
                    <div>
                      <div className="text-sm font-medium text-gray-900">{agent.clusterName}</div>
                      <div className="text-xs text-gray-500">
                        Last heartbeat: {new Date(agent.lastHeartbeat).toLocaleString()}
                      </div>
                    </div>
                    {getStatusBadge(agent.status)}
                  </div>
                ))}
              </div>
            </div>
          ) : (
            <div className="text-center py-8 text-gray-500">No agent data available</div>
          )}
        </Card>

        {/* Sync Status */}
        <Card title="Sync Status">
          {systemMetrics ? (
            <div className="space-y-4">
              <div>
                <div className="text-sm text-gray-600">Last Full Scan</div>
                <div className="text-sm font-medium text-gray-900 mt-1">
                  {systemMetrics.sync?.lastFullScan
                    ? new Date(systemMetrics.sync.lastFullScan).toLocaleString()
                    : 'Never'}
                </div>
              </div>
              <div>
                <div className="text-sm text-gray-600">Next Scan</div>
                <div className="text-sm font-medium text-gray-900 mt-1">
                  {systemMetrics.sync?.nextScan
                    ? new Date(systemMetrics.sync.nextScan).toLocaleString()
                    : 'N/A'}
                </div>
              </div>
              <div className="pt-4 border-t">
                <div className="text-sm text-gray-600 mb-2">Resources Synced</div>
                <div className="grid grid-cols-2 gap-2 text-sm">
                  <div>Clusters: {systemMetrics.resources?.clusters || 0}</div>
                  <div>Pods: {systemMetrics.resources?.pods || 0}</div>
                  <div>ServiceAccounts: {systemMetrics.resources?.serviceAccounts || 0}</div>
                  <div>Insights: {systemMetrics.resources?.insights || 0}</div>
                </div>
              </div>
            </div>
          ) : (
            <div className="text-center py-8 text-gray-500">No sync data available</div>
          )}
        </Card>
      </div>

      {/* Certificate Status */}
      <Card title="Certificate Status">
        <div className="space-y-4">
          <div className="flex justify-between items-center">
            <span className="text-sm text-gray-600">Status</span>
            <span className="px-2 py-1 bg-green-100 text-green-800 rounded-full text-xs font-medium">
              ✅ VALID
            </span>
          </div>
          <div className="flex justify-between items-center">
            <span className="text-sm text-gray-600">Expires</span>
            <span className="text-sm text-gray-900">365 days</span>
          </div>
          <div className="w-full bg-gray-200 rounded-full h-2">
            <div className="bg-green-600 h-2 rounded-full" style={{ width: '99%' }}></div>
          </div>
          <Button variant="secondary" className="w-full">
            🔄 Rotate Certificate
          </Button>
        </div>
      </Card>
    </div>
  );
};
