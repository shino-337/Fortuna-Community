import React, { useEffect, useState, useCallback } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { AttackPathGraph } from '../components/AttackPathGraph';
import { api } from '../lib/api';
import type { AttackPathGraphData, AttackPathSummary, AttackPath } from '../types';
import { useClusterStore } from '../store/clusterStore';
import {
  AlertTriangle,
  Shield,
  Network,
  ChevronRight,
  RefreshCw,
  Target,
  Layers,
  TrendingUp,
} from 'lucide-react';

/* ─── helpers ─────────────────────────────────────────────── */

const riskBadgeClasses: Record<string, string> = {
  critical: 'bg-red-500/20 text-red-400 border-red-500/40',
  high:     'bg-orange-500/20 text-orange-400 border-orange-500/40',
  medium:   'bg-yellow-500/20 text-yellow-400 border-yellow-500/40',
  low:      'bg-green-500/20 text-green-400 border-green-500/40',
};

function riskLabel(score: number): string {
  if (score >= 9) return 'critical';
  if (score >= 7) return 'high';
  if (score >= 4) return 'medium';
  return 'low';
}

/* ─── main component ──────────────────────────────────────── */

export const AttackPaths: React.FC = () => {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { selectedClusterId } = useClusterStore();

  const [graphData, setGraphData] = useState<AttackPathGraphData>({ nodes: [], links: [] });
  const [summary, setSummary] = useState<AttackPathSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

  // Selected pod for detail
  const podUidParam = searchParams.get('podUid') || '';
  const [selectedPodPaths, setSelectedPodPaths] = useState<AttackPath[]>([]);
  const [selectedPodLoading, setSelectedPodLoading] = useState(false);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [graph, sum] = await Promise.all([
        api.getAttackPathsGraph(),
        api.getAttackPathsSummary(selectedClusterId || undefined),
      ]);
      setGraphData(graph);
      setSummary(sum);
    } catch {
      /* empty */
    } finally {
      setLoading(false);
    }
  }, [selectedClusterId]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Fetch pod-specific paths
  useEffect(() => {
    if (!podUidParam) {
      setSelectedPodPaths([]);
      return;
    }
    setSelectedPodLoading(true);
    api.getAttackPathsForPod(podUidParam).then(setSelectedPodPaths).finally(() => setSelectedPodLoading(false));
  }, [podUidParam]);

  const handleRefresh = async () => {
    setRefreshing(true);
    await fetchData();
    setRefreshing(false);
  };

  const handleNodeClick = (nodeId: string, type: string) => {
    if (type === 'pod') {
      navigate(`/resources/pods/uid/${encodeURIComponent(nodeId)}`);
    }
  };

  /* ─── render ─────────────────────────────────────────────── */

  return (
    <PageLayout
      title="Attack Paths"
      description="Trực quan hoá đường tấn công từ Pod đến tài nguyên nhạy cảm (cluster-admin, Secrets, …)."
      actions={
        <button
          onClick={handleRefresh}
          disabled={refreshing}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-slate-700 bg-slate-800/60 text-sm text-slate-300 hover:bg-slate-700/80 transition disabled:opacity-50"
        >
          <RefreshCw size={14} className={refreshing ? 'animate-spin' : ''} />
          Làm mới
        </button>
      }
    >
      {/* ─── Summary cards ─────────────────────────────────── */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <StatCard
          icon={<Network size={18} className="text-pink-400" />}
          label="Tổng Attack Paths"
          value={summary?.totalPaths ?? '–'}
          loading={loading}
        />
        <StatCard
          icon={<AlertTriangle size={18} className="text-red-400" />}
          label="Critical"
          value={summary?.criticalPaths ?? '–'}
          loading={loading}
          accent="red"
        />
        <StatCard
          icon={<TrendingUp size={18} className="text-orange-400" />}
          label="High"
          value={summary?.highPaths ?? '–'}
          loading={loading}
          accent="orange"
        />
        <StatCard
          icon={<Shield size={18} className="text-yellow-400" />}
          label="Medium"
          value={summary?.mediumPaths ?? '–'}
          loading={loading}
          accent="yellow"
        />
      </div>

      {/* ─── Graph ─────────────────────────────────────────── */}
      <section className="mb-6">
        <h2 className="text-sm font-semibold text-slate-300 mb-2 flex items-center gap-1.5">
          <Layers size={14} /> Đồ thị Attack Path
        </h2>
        {loading ? (
          <div className="flex items-center justify-center py-20">
            <div className="w-8 h-8 border-4 border-pink-500 border-t-transparent rounded-full animate-spin" />
          </div>
        ) : (
          <AttackPathGraph
            data={graphData}
            onNodeClick={handleNodeClick}
            className="h-[500px]"
          />
        )}
      </section>

      {/* ─── Top risky pods ────────────────────────────────── */}
      {summary && summary.topPods && summary.topPods.length > 0 && (
        <section className="mb-6">
          <h2 className="text-sm font-semibold text-slate-300 mb-2 flex items-center gap-1.5">
            <Target size={14} /> Pods rủi ro cao nhất
          </h2>
          <div className="overflow-x-auto rounded-lg border border-slate-800">
            <table className="w-full text-sm text-left text-slate-400">
              <thead className="bg-slate-900/80 text-slate-500 text-xs uppercase">
                <tr>
                  <th className="px-4 py-2">Pod</th>
                  <th className="px-4 py-2">Namespace</th>
                  <th className="px-4 py-2">Service Account</th>
                  <th className="px-4 py-2">Risk</th>
                  <th className="px-4 py-2">Lý do</th>
                  <th className="px-4 py-2"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800">
                {summary.topPods.map((pod) => {
                  const level = riskLabel(pod.risk_score);
                  return (
                    <tr key={pod.uid} className="hover:bg-slate-800/40 transition-colors">
                      <td className="px-4 py-2 font-medium text-white">{pod.name || pod.uid.slice(0, 12)}</td>
                      <td className="px-4 py-2">{pod.namespace}</td>
                      <td className="px-4 py-2">{pod.service_account_name}</td>
                      <td className="px-4 py-2">
                        <span className={`px-2 py-0.5 rounded text-[10px] font-semibold border ${riskBadgeClasses[level]}`}>
                          {level.toUpperCase()} ({pod.risk_score.toFixed(1)})
                        </span>
                      </td>
                      <td className="px-4 py-2 text-xs max-w-xs truncate">{pod.risk_reason}</td>
                      <td className="px-4 py-2">
                        <button
                          onClick={() => navigate(`/resources/pods/uid/${encodeURIComponent(pod.uid)}`)}
                          className="text-pink-400 hover:text-pink-300 transition"
                        >
                          <ChevronRight size={14} />
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {/* ─── Target breakdown ──────────────────────────────── */}
      {summary && summary.targetBreakdown && Object.keys(summary.targetBreakdown).length > 0 && (
        <section className="mb-6">
          <h2 className="text-sm font-semibold text-slate-300 mb-2">Target Breakdown</h2>
          <div className="flex flex-wrap gap-2">
            {Object.entries(summary.targetBreakdown)
              .sort(([, a], [, b]) => b - a)
              .map(([role, count]) => (
                <span
                  key={role}
                  className="px-3 py-1 bg-slate-800/60 border border-slate-700 rounded-full text-xs text-slate-300"
                >
                  {role}: <span className="font-semibold text-white">{count}</span>
                </span>
              ))}
          </div>
        </section>
      )}

      {/* ─── Pod detail paths (if podUid query param) ──────── */}
      {podUidParam && (
        <section>
          <h2 className="text-sm font-semibold text-slate-300 mb-2">
            Attack Paths cho pod <code className="text-pink-400">{podUidParam.slice(0, 12)}…</code>
          </h2>
          {selectedPodLoading ? (
            <div className="flex items-center justify-center py-8">
              <div className="w-6 h-6 border-4 border-pink-500 border-t-transparent rounded-full animate-spin" />
            </div>
          ) : selectedPodPaths.length === 0 ? (
            <p className="text-xs text-slate-500">Không có attack path nào cho pod này.</p>
          ) : (
            <div className="space-y-3">
              {selectedPodPaths.map((path, idx) => (
                <PathCard key={idx} path={path} />
              ))}
            </div>
          )}
        </section>
      )}
    </PageLayout>
  );
};

/* ─── sub-components ──────────────────────────────────────── */

const StatCard: React.FC<{
  icon: React.ReactNode;
  label: string;
  value: string | number;
  loading?: boolean;
  accent?: string;
}> = ({ icon, label, value, loading, accent }) => (
  <div className="bg-slate-900/70 border border-slate-800 rounded-lg p-4">
    <div className="flex items-center gap-2 mb-1">
      {icon}
      <span className="text-xs text-slate-500 uppercase tracking-wide">{label}</span>
    </div>
    {loading ? (
      <div className="h-7 w-16 bg-slate-800 rounded animate-pulse mt-1" />
    ) : (
      <p className={`text-2xl font-bold ${accent === 'red' ? 'text-red-400' : accent === 'orange' ? 'text-orange-400' : accent === 'yellow' ? 'text-yellow-400' : 'text-white'}`}>
        {value}
      </p>
    )}
  </div>
);

const PathCard: React.FC<{ path: AttackPath }> = ({ path }) => {
  const level = riskLabel(path.total_risk);
  return (
    <div className="bg-slate-900/70 border border-slate-800 rounded-lg p-4 hover:border-slate-700 transition-colors">
      <div className="flex items-start justify-between mb-2">
        <p className="text-sm text-slate-300">{path.description}</p>
        <span className={`px-2 py-0.5 rounded text-[10px] font-semibold border ${riskBadgeClasses[level]}`}>
          {level.toUpperCase()}
        </span>
      </div>
      <div className="flex items-center gap-1 text-xs text-slate-500 overflow-x-auto pb-1">
        {path.nodes.map((n, i) => (
          <React.Fragment key={n.id}>
            <span className="px-2 py-0.5 bg-slate-800 border border-slate-700 rounded whitespace-nowrap">
              <span className="text-slate-400">{n.type}: </span>
              <span className="text-white">{(n.properties.name as string) || n.id.slice(0, 12)}</span>
            </span>
            {i < path.nodes.length - 1 && (
              <ChevronRight size={12} className="text-slate-600 shrink-0" />
            )}
          </React.Fragment>
        ))}
      </div>
      <div className="flex gap-4 mt-2 text-[10px] text-slate-500">
        <span>Risk: {path.total_risk.toFixed(1)}</span>
        <span>Difficulty: {(path.difficulty * 100).toFixed(0)}%</span>
        <span>Impact: {(path.impact * 100).toFixed(0)}%</span>
        <span>Hops: {path.length}</span>
      </div>
    </div>
  );
};
