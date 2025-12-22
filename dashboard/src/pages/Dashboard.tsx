import React, { useEffect, useState } from 'react';
import { StatCard } from '../components/StatCard';
import { Card } from '../components/ui/Card';
import api from '../lib/api';
import { InsightSummary } from '../types';

export const Dashboard: React.FC = () => {
  const [summary, setSummary] = useState<InsightSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [clusterCount, setClusterCount] = useState(0);
  const [podCount, setPodCount] = useState(0);
  const [queueMetrics, setQueueMetrics] = useState<any>(null);
  const [workerMetrics, setWorkerMetrics] = useState<any[]>([]);

  useEffect(() => {
    const fetchData = async () => {
      try {
        // Fetch all data in parallel
        const [summaryRes, clustersRes, podsRes, queueRes, workersRes] = await Promise.all([
          api.get('/api/v1/insights/summary').catch(() => ({ data: null })),
          api.get('/api/v1/clusters/stats').catch(() => ({ data: null })),
          api.get('/api/v1/pods', { params: { pageSize: 1 } }).catch(() => ({ data: { total: 0 } })),
          api.get('/api/v1/metrics/queue').catch(() => ({ data: null })),
          api.get('/api/v1/metrics/workers').catch(() => ({ data: { workers: [] } })),
        ]);

        // Set insights summary
        if (summaryRes.data) {
          setSummary(summaryRes.data);
        }

        // Set cluster count
        if (clustersRes.data) {
          const clusters = clustersRes.data?.clusters || clustersRes.data || [];
          const clusterArray = Array.isArray(clusters) ? clusters : [];
          setClusterCount(clusterArray.length);
        } else {
          // Fallback: try basic clusters endpoint
          try {
            const fallbackRes = await api.get('/api/v1/clusters');
            const clusters = fallbackRes.data?.clusters || fallbackRes.data || [];
            setClusterCount(Array.isArray(clusters) ? clusters.length : 0);
          } catch (e) {
            console.error('Failed to fetch clusters:', e);
            setClusterCount(0);
          }
        }

        // Set pod count - use total from pods API (most accurate)
        const podsTotal = podsRes.data?.total || 0;
        if (podsTotal > 0) {
          setPodCount(podsTotal);
        } else if (clustersRes.data) {
          // Fallback: calculate from clusters stats
          const clusters = clustersRes.data?.clusters || clustersRes.data || [];
          const clusterArray = Array.isArray(clusters) ? clusters : [];
          const totalPods = clusterArray.reduce((sum: number, cluster: any) => {
            return sum + (parseInt(cluster.podCount) || 0);
          }, 0);
          setPodCount(totalPods);
        }

        // Set queue metrics
        if (queueRes.data) {
          setQueueMetrics(queueRes.data);
        }

        // Set worker metrics
        if (workersRes.data?.workers) {
          setWorkerMetrics(workersRes.data.workers);
        }
      } catch (err) {
        console.error('Failed to fetch dashboard data:', err);
      } finally {
        setLoading(false);
      }
    };

    fetchData();
    const interval = setInterval(fetchData, 10000); // Refresh every 10 seconds for real-time updates
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">Dashboard Overview</h1>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <StatCard
          title="Clusters"
          value={clusterCount}
          subtitle="Active"
          loading={loading}
        />
        <StatCard
          title="Insights"
          value={summary?.total || 0}
          subtitle={`${summary?.critical || 0} Critical`}
          loading={loading}
        />
        <StatCard
          title="Pods"
          value={podCount}
          subtitle="Monitored"
          loading={loading}
        />
        <StatCard
          title="Critical Issues"
          value={summary?.critical || 0}
          subtitle="Requires attention"
          loading={loading}
        />
      </div>

      {/* Insights by Severity */}
      {summary && (
        <Card title="Insights by Severity">
          <div className="grid grid-cols-4 gap-4">
            <div className="text-center">
              <div className="text-2xl font-bold text-red-600">{summary.critical}</div>
              <div className="text-sm text-gray-600">Critical</div>
            </div>
            <div className="text-center">
              <div className="text-2xl font-bold text-orange-600">{summary.high}</div>
              <div className="text-sm text-gray-600">High</div>
            </div>
            <div className="text-center">
              <div className="text-2xl font-bold text-yellow-600">{summary.medium}</div>
              <div className="text-sm text-gray-600">Medium</div>
            </div>
            <div className="text-center">
              <div className="text-2xl font-bold text-green-600">{summary.low}</div>
              <div className="text-sm text-gray-600">Low</div>
            </div>
          </div>
        </Card>
      )}

      {/* System Health */}
      <Card title="System Health">
        <div className="space-y-4">
          {queueMetrics && (
            <div>
              <div className="flex justify-between mb-1">
                <span className="text-sm text-gray-600">Queue Depth</span>
                <span className="text-sm text-gray-600">
                  {Object.values(queueMetrics).reduce((sum: number, val: any) => {
                    const num = typeof val === 'number' ? val : 0;
                    return sum + num;
                  }, 0)} messages
                </span>
              </div>
              <div className="w-full bg-gray-200 rounded-full h-2">
                {(() => {
                  const totalQueue = Object.values(queueMetrics).reduce((sum: number, val: any) => {
                    const num = typeof val === 'number' ? val : 0;
                    return sum + num;
                  }, 0);
                  const percentage = Math.min(100, (totalQueue / 200) * 100); // Assuming 200 as max
                  const color = totalQueue > 100 ? 'bg-red-600' : totalQueue > 50 ? 'bg-yellow-600' : 'bg-green-600';
                  return <div className={`${color} h-2 rounded-full`} style={{ width: `${percentage}%` }}></div>;
                })()}
              </div>
              <div className="text-xs text-gray-500 mt-1">
                Normalizer: {queueMetrics.normalizer || 0} | 
                Correlator: {queueMetrics.correlator || 0} | 
                Risk: {queueMetrics.risk || 0}
              </div>
            </div>
          )}
          {workerMetrics.length > 0 && (
            <div>
              <div className="flex justify-between mb-1">
                <span className="text-sm text-gray-600">Worker Load</span>
                <span className="text-sm text-gray-600">
                  {workerMetrics.filter((w: any) => w.status === 'healthy').length}/{workerMetrics.length} healthy
                </span>
              </div>
              <div className="w-full bg-gray-200 rounded-full h-2">
                {(() => {
                  const healthyCount = workerMetrics.filter((w: any) => w.status === 'healthy').length;
                  const percentage = workerMetrics.length > 0 ? (healthyCount / workerMetrics.length) * 100 : 0;
                  const color = percentage >= 80 ? 'bg-green-600' : percentage >= 50 ? 'bg-yellow-600' : 'bg-red-600';
                  return <div className={`${color} h-2 rounded-full`} style={{ width: `${percentage}%` }}></div>;
                })()}
              </div>
              <div className="text-xs text-gray-500 mt-1">
                {workerMetrics.map((w: any) => `${w.type}: ${w.status}`).join(' | ')}
              </div>
            </div>
          )}
          {!queueMetrics && !workerMetrics.length && (
            <div className="text-center py-4 text-gray-500 text-sm">
              Loading system metrics...
            </div>
          )}
        </div>
      </Card>
    </div>
  );
};
