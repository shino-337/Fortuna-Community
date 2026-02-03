import React, { useCallback, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { StatCard } from '../components/StatCard';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../components/PageLayout';
import { Server, ShieldAlert, Boxes, Radio, ArrowRight, Shield, AlertTriangle, Bell, Info } from 'lucide-react';
import { Cluster, Insight, InsightsSummary, Notification, PodCapabilitySummaryCapability, PodCapabilityTrendPoint } from '../types';
import { useClusterStore } from '../store/clusterStore';
import { useTimeWindowStore } from '../store/timeWindowStore';
import { getSeverityTextClass } from '../lib/severity';
import { STAT_LABELS } from '../constants/labels';
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';

export const Dashboard: React.FC = () => {
  const navigate = useNavigate();
  const selectedClusterId = useClusterStore((s) => s.selectedClusterId);
  const timeWindowMinutes = useTimeWindowStore((s) => s.valueMinutes);
  const sinceMinutes = timeWindowMinutes > 0 ? timeWindowMinutes : undefined;
  const [stats, setStats] = useState({ clusters: 0, insights: 0, critical: 0, pods: 0, agents: 0, affectedPodCount: 0 });
  const [insightsSummary, setInsightsSummary] = useState<InsightsSummary | null>(null);
  const [clusters, setClusters] = useState<Cluster[]>([]);
  const [topRisks, setTopRisks] = useState<Insight[]>([]);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [pceSummary, setPceSummary] = useState<PodCapabilitySummaryCapability[]>([]);
  const [pceTrend, setPceTrend] = useState<PodCapabilityTrendPoint[]>([]);
  const [threatVelocity, setThreatVelocity] = useState<{ date: string; critical: number; high: number; medium: number; low: number }[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    setError(null);
    try {
      // Stats + insights/summary for severity breakdown and affected workloads (scoped by selected cluster when set)
      const [statsData, summaryResult] = await Promise.all([
        api.getStats(selectedClusterId ?? undefined, sinceMinutes),
        api.getInsightsSummary(selectedClusterId ?? undefined, sinceMinutes).catch(() => null),
      ]);
      setStats(statsData);
      if (summaryResult) setInsightsSummary(summaryResult);

      // Rest in parallel – partial failure OK so dashboard still shows stats
      const [clustersResult, risksResult, notesResult, threatResult, pceResult, pceTrendResult] = await Promise.allSettled([
        api.getClusters(),
        api.getRisks({ page: 1, pageSize: 50, clusterId: selectedClusterId ?? undefined, sinceMinutes }),
        api.getNotifications(),
        api.getThreatVelocity(7),
        api.getPceSummaryByCapability(),
        api.getPceTrend(7),
      ]);

      if (clustersResult.status === 'fulfilled') setClusters(clustersResult.value);
      if (risksResult.status === 'fulfilled') {
        const { insights } = risksResult.value;
        setTopRisks(insights.filter((r: Insight) => r.severity === 'critical').slice(0, 4));
      }
      if (notesResult.status === 'fulfilled') setNotifications(notesResult.value.slice(0, 3));
      if (threatResult.status === 'fulfilled') setThreatVelocity(threatResult.value);
      if (pceResult.status === 'fulfilled') setPceSummary(pceResult.value.slice(0, 5));
      if (pceTrendResult.status === 'fulfilled') setPceTrend(pceTrendResult.value);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unable to load dashboard data');
    } finally {
      setLoading(false);
    }
  }, [selectedClusterId, sinceMinutes]);

  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.STATS_CLUSTERS));
  usePolling(fetchData, intervalMs);

  // Build 7-day labels for fallback when API returns empty (chart still shows axis)
  const last7Days = useMemo(() => {
    const out: string[] = [];
    for (let i = 6; i >= 0; i--) {
      const d = new Date();
      d.setDate(d.getDate() - i);
      out.push(d.toISOString().slice(0, 10));
    }
    return out;
  }, []);

  const chartData = useMemo(() => {
    let points: { name: string; risk: number }[];
    if (threatVelocity.length > 0) {
      points = threatVelocity.map((point) => ({
        name: point.date,
        risk: (point.critical ?? 0) + (point.high ?? 0) + (point.medium ?? 0) + (point.low ?? 0),
      }));
    } else {
      points = last7Days.map((date) => ({ name: date, risk: 0 }));
    }
    return points.slice().sort((a, b) => a.name.localeCompare(b.name));
  }, [threatVelocity, last7Days]);

  const chartYDomain = useMemo(() => {
    const maxRisk = chartData.length ? Math.max(...chartData.map((d) => d.risk), 1) : 1;
    return [0, maxRisk] as [number, number];
  }, [chartData]);

  const pceChartData = useMemo(() => {
    let points: { name: string; total: number }[];
    if (pceTrend.length > 0) {
      points = pceTrend.map((point) => ({
        name: point.date,
        total: (point.critical ?? 0) + (point.high ?? 0) + (point.medium ?? 0) + (point.low ?? 0),
      }));
    } else {
      points = last7Days.map((date) => ({ name: date, total: 0 }));
    }
    return points.slice().sort((a, b) => a.name.localeCompare(b.name));
  }, [pceTrend, last7Days]);

  const pceChartYDomain = useMemo(() => {
    const maxTotal = pceChartData.length ? Math.max(...pceChartData.map((d) => d.total), 1) : 1;
    return [0, maxTotal] as [number, number];
  }, [pceChartData]);

  if (loading) return <div className="flex flex-col justify-center items-center h-[60vh] space-y-4">
    <div className="w-12 h-12 border-4 border-pink-500 border-t-transparent rounded-full animate-spin"></div>
    <span className="text-slate-500 font-medium">Initializing Dashboard...</span>
  </div>;

  if (error) return (
    <div className="flex flex-col justify-center items-center h-[60vh] space-y-4">
      <AlertTriangle className="w-12 h-12 text-amber-500" />
      <span className="text-slate-300 font-medium">Dashboard data unavailable</span>
      <p className="text-slate-500 text-sm max-w-md text-center">{error}</p>
      <p className="text-slate-500 text-xs max-w-md text-center">
        Ensure you are logged in (admin / admin123). If using port-forward, run both: Core 8080 and Dashboard 8081.
      </p>
      <Button variant="secondary" onClick={() => window.location.reload()}>Retry</Button>
    </div>
  );

  return (
    <PageLayout
      title="Dashboard"
      description="Real-time security posture across your infrastructure."
      actions={<Button variant="secondary" onClick={() => navigate('/risks')}>View All Risks</Button>}
    >
      {/* Key Metrics – spec: Total Risks (C/H/M/L), Exposed Capabilities, Affected Workloads; click severity → Risk Center (pre-filtered) */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <div onClick={() => navigate('/clusters')} className="cursor-pointer">
          <StatCard
            title={STAT_LABELS.CLUSTERS}
            value={stats.clusters}
            icon={<Server className="w-6 h-6" />}
            color="bg-blue-500/10 text-blue-400"
          />
        </div>
        <div onClick={() => navigate('/risks?severity=critical')} className="cursor-pointer">
          <StatCard
            title="Critical"
            value={insightsSummary?.critical ?? stats.critical}
            icon={<ShieldAlert className="w-6 h-6" />}
            color="bg-red-500/10 text-red-400"
          />
        </div>
        <div onClick={() => navigate('/risks?severity=high')} className="cursor-pointer">
          <StatCard
            title="High"
            value={insightsSummary?.high ?? 0}
            icon={<ShieldAlert className="w-6 h-6" />}
            color="bg-orange-500/10 text-orange-400"
          />
        </div>
        <div onClick={() => navigate('/risks?severity=medium')} className="cursor-pointer">
          <StatCard
            title="Medium"
            value={insightsSummary?.medium ?? 0}
            icon={<ShieldAlert className="w-6 h-6" />}
            color="bg-amber-500/10 text-amber-400"
          />
        </div>
        <div onClick={() => navigate('/risks?severity=low')} className="cursor-pointer">
          <StatCard
            title="Low"
            value={insightsSummary?.low ?? 0}
            icon={<ShieldAlert className="w-6 h-6" />}
            color="bg-slate-500/10 text-slate-400"
          />
        </div>
        <div onClick={() => navigate('/risks')} className="cursor-pointer">
          <StatCard
            title="Affected Workloads"
            value={stats.affectedPodCount ?? 0}
            icon={<Boxes className="w-6 h-6" />}
            color="bg-rose-500/10 text-rose-400"
          />
        </div>
        <div onClick={() => navigate('/resources')} className="cursor-pointer">
          <StatCard
            title={STAT_LABELS.PODS}
            value={stats.pods}
            icon={<Boxes className="w-6 h-6" />}
            color="bg-emerald-500/10 text-emerald-400"
          />
        </div>
        <div onClick={() => navigate('/monitoring')} className="cursor-pointer">
          <StatCard
            title={STAT_LABELS.AGENTS}
            value={stats.agents.toString()}
            icon={<Radio className="w-6 h-6" />}
            color="bg-purple-500/10 text-purple-400"
          />
        </div>
      </div>

      {stats.clusters === 0 && stats.pods === 0 && stats.insights === 0 && (
        <div className="rounded-lg border border-slate-700 bg-slate-900/50 px-4 py-3 text-sm text-slate-400">
          <span className="font-medium text-slate-300">No data yet.</span> Ensure you are logged in (admin / admin123), Core is running, and the dashboard can reach the API (same origin or port-forward to Dashboard so /api/ proxies to Core).
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Top Critical Risks */}
        <div className="lg:col-span-2 space-y-6">
            <div className="flex items-center justify-between">
              <h2 className="text-xl font-bold text-white flex items-center">
                  <Shield className="w-5 h-5 mr-3 text-red-500" /> Critical Risks
              </h2>
              <button onClick={() => navigate('/risks')} className="text-pink-500 text-sm font-medium hover:underline flex items-center">
                View all <ArrowRight size={14} className="ml-1" />
              </button>
            </div>
            
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                {topRisks.map(risk => (
                    <div 
                      key={risk.id} 
                      className="bg-slate-900 border border-slate-800 p-5 rounded-xl hover:border-pink-500/30 hover:bg-slate-800/50 transition-all cursor-pointer group relative overflow-hidden" 
                      onClick={() => navigate('/risks')}
                    >
                        <div className="absolute top-0 right-0 p-3 opacity-10 group-hover:opacity-20 transition-opacity">
                          <ShieldAlert size={60} />
                        </div>
                        <div className="flex justify-between items-start mb-3">
                            <span className="text-red-500 font-bold text-lg">{risk.score}</span>
                            <span className="text-[10px] uppercase font-bold text-slate-500 border border-slate-700 px-2 py-0.5 rounded">Critical</span>
                        </div>
                        <h3 className="text-white font-bold mb-2 group-hover:text-pink-400 transition-colors line-clamp-1">{risk.title}</h3>
                        <p className="text-xs text-slate-400 line-clamp-2 mb-4 h-8">{risk.description}</p>
                        <div className="flex items-center justify-between">
                          <div className="flex -space-x-2">
                            {risk.affectedResources?.slice(0, 3).map((_, i) => (
                              <div key={i} className="w-6 h-6 rounded-full border-2 border-slate-900 bg-slate-700 flex items-center justify-center">
                                <Boxes size={10} className="text-slate-300" />
                              </div>
                            ))}
                          </div>
                          <span className="text-[10px] text-slate-500">{risk.affectedResources?.length || 0} resources</span>
                        </div>
                    </div>
                ))}
            </div>
            
            <Card title="Threat Velocity (7 Days)">
                 <div className="w-full min-w-[280px] bg-slate-800/30 rounded-md border border-slate-700/50" style={{ width: '100%', height: 260, minHeight: 260 }}>
                   <ResponsiveContainer width="100%" height="100%">
                     <AreaChart data={chartData} margin={{ top: 10, right: 10, left: 5, bottom: 0 }}>
                       <defs>
                         <linearGradient id="colorRisk" x1="0" y1="0" x2="0" y2="1">
                           <stop offset="5%" stopColor="#ef4444" stopOpacity={0.3}/>
                           <stop offset="95%" stopColor="#ef4444" stopOpacity={0}/>
                         </linearGradient>
                       </defs>
                       <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#334155" />
                       <XAxis dataKey="name" axisLine={false} tickLine={false} tick={{ fill: '#94a3b8', fontSize: 11 }} />
                       <YAxis domain={chartYDomain} axisLine={false} tickLine={false} tick={{ fill: '#94a3b8', fontSize: 11 }} width={28} />
                       <Tooltip
                         contentStyle={{ backgroundColor: '#0f172a', borderRadius: '12px', border: '1px solid #1e293b', padding: '12px' }}
                         itemStyle={{ color: '#ef4444' }}
                         formatter={(value: number) => [value, 'Risks']}
                         labelFormatter={(label) => `Date: ${label}`}
                       />
                       <Area type="monotone" dataKey="risk" name="Risks" stroke="#ef4444" strokeWidth={2} fillOpacity={1} fill="url(#colorRisk)" isAnimationActive={true} />
                     </AreaChart>
                   </ResponsiveContainer>
                 </div>
                 {chartData.every((d) => d.risk === 0) && (
                   <p className="text-xs text-slate-500 mt-2 px-1">No risks in last 7 days. Log in (admin/admin123); run <code className="text-slate-400">scripts/run-dashboard-data-tests.sh</code> to populate.</p>
                 )}
            </Card>

            <Card title="PCE Trend (7 Days)">
                 <div className="w-full min-w-[280px] bg-slate-800/30 rounded-md border border-slate-700/50" style={{ width: '100%', height: 260, minHeight: 260 }}>
                   <ResponsiveContainer width="100%" height="100%">
                     <AreaChart data={pceChartData} margin={{ top: 10, right: 10, left: 5, bottom: 0 }}>
                       <defs>
                         <linearGradient id="colorPce" x1="0" y1="0" x2="0" y2="1">
                           <stop offset="5%" stopColor="#22d3ee" stopOpacity={0.3}/>
                           <stop offset="95%" stopColor="#22d3ee" stopOpacity={0}/>
                         </linearGradient>
                       </defs>
                       <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#334155" />
                       <XAxis dataKey="name" axisLine={false} tickLine={false} tick={{ fill: '#94a3b8', fontSize: 11 }} />
                       <YAxis domain={pceChartYDomain} axisLine={false} tickLine={false} tick={{ fill: '#94a3b8', fontSize: 11 }} width={28} />
                       <Tooltip
                         contentStyle={{ backgroundColor: '#0f172a', borderRadius: '12px', border: '1px solid #1e293b', padding: '12px' }}
                         itemStyle={{ color: '#22d3ee' }}
                         formatter={(value: number) => [value, 'Capabilities']}
                         labelFormatter={(label) => `Date: ${label}`}
                       />
                       <Area type="monotone" dataKey="total" name="Capabilities" stroke="#22d3ee" strokeWidth={2} fillOpacity={1} fill="url(#colorPce)" isAnimationActive={true} />
                     </AreaChart>
                   </ResponsiveContainer>
                 </div>
                 {pceChartData.every((d) => d.total === 0) && (
                   <p className="text-xs text-slate-500 mt-2 px-1">No PCE data in last 7 days. Run <code className="text-slate-400">scripts/run-dashboard-data-tests.sh</code> or ensure agents sync capabilities.</p>
                 )}
            </Card>
        </div>

        {/* Sidebar Info – filter by global cluster when set */}
        <div className="space-y-6">
            <h2 className="text-xl font-bold text-white">Cluster Health</h2>
            <Card className="p-0 overflow-hidden shadow-xl shadow-black/20">
                <div className="divide-y divide-slate-800">
                    {(selectedClusterId ? clusters.filter((c) => c.id === selectedClusterId) : clusters).map(cluster => (
                        <div key={cluster.id} className="p-5 hover:bg-slate-800/50 transition-colors group cursor-pointer" onClick={() => navigate(`/clusters/${cluster.id}`)}>
                            <div className="flex justify-between items-center mb-3">
                                <div className="flex items-center">
                                  <div className={`w-2 h-2 rounded-full mr-3 animate-pulse ${
                                    cluster.healthScore > 80 ? 'bg-emerald-500' :
                                    cluster.healthScore > 50 ? 'bg-yellow-500' : 'bg-red-500'
                                  }`}></div>
                                  <span className="font-bold text-white group-hover:text-pink-400 transition-colors">{cluster.name || cluster.id}</span>
                                </div>
                                <span className={`text-[10px] px-2 py-0.5 rounded-full uppercase font-bold ${
                                    cluster.healthScore > 80 ? 'text-emerald-400 bg-emerald-500/10' :
                                    cluster.healthScore > 50 ? 'text-yellow-400 bg-yellow-500/10' :
                                    'text-red-400 bg-red-500/10'
                                }`}>
                                    {cluster.healthScore}%
                                </span>
                            </div>
                            <div className="w-full bg-slate-800 rounded-full h-1.5 overflow-hidden">
                                <div 
                                    className={`h-1.5 rounded-full transition-all duration-1000 ${
                                        cluster.healthScore > 80 ? 'bg-emerald-500' :
                                        cluster.healthScore > 50 ? 'bg-yellow-500' : 'bg-red-500'
                                    }`} 
                                    style={{ width: `${cluster.healthScore}%` }}
                                ></div>
                            </div>
                        </div>
                    ))}
                </div>
            </Card>

            <h2 className="text-xl font-bold text-white">Pod Capabilities</h2>
            <Card className="p-5">
                {pceSummary.length === 0 ? (
                    <div className="text-sm text-slate-500">No capability data available.</div>
                ) : (
                    <div className="space-y-3">
                        {pceSummary.map((row) => (
                            <div key={`${row.capabilityId}-${row.severity}`} className="flex items-center justify-between">
                                <div>
                                    <div className="text-sm font-semibold text-white">{row.capabilityId}</div>
                                    <div className={`text-xs ${getSeverityTextClass(row.severity)}`}>{row.severity}</div>
                                </div>
                                <button type="button" onClick={(e) => { e.stopPropagation(); navigate('/capabilities'); }} className="text-sm font-bold text-pink-400 hover:underline">
                                  {row.count}
                                </button>
                            </div>
                        ))}
                    </div>
                )}
            </Card>

            <h2 className="text-xl font-bold text-white">Recent Activity</h2>
            <div className="space-y-3">
                {notifications.map(note => (
                  <div key={note.id} className="bg-slate-900/50 border border-slate-800 rounded-xl p-4 flex items-start space-x-3">
                    <div className={`p-2 rounded-lg shrink-0 ${
                      note.type === 'error' ? 'bg-red-500/10 text-red-400' :
                      note.type === 'success' ? 'bg-emerald-500/10 text-emerald-400' :
                      'bg-blue-500/10 text-blue-400'
                    }`}>
                      {note.type === 'error' ? <ShieldAlert size={16} /> : <Info size={16} />}
                    </div>
                    <div>
                      <h4 className="text-sm font-semibold text-white">{note.title}</h4>
                      <p className="text-xs text-slate-400 mt-1 line-clamp-2">{note.message}</p>
                      <span className="text-[10px] text-slate-600 mt-2 block uppercase">{note.timestamp}</span>
                    </div>
                  </div>
                ))}
            </div>

            <div className="bg-gradient-to-br from-pink-600/20 to-purple-600/20 border border-pink-500/30 rounded-xl p-5 relative overflow-hidden">
                <div className="relative z-10">
                    <h3 className="text-white font-bold text-sm">Automated Triage Active</h3>
                    <p className="text-slate-300 text-xs mt-2">
                        System is currently monitoring for anomalous RBAC patterns. 
                    </p>
                    <button onClick={() => navigate('/monitoring')} className="mt-4 text-xs font-bold text-pink-400 flex items-center hover:text-pink-300">
                      Check Worker Status <ArrowRight size={12} className="ml-1" />
                    </button>
                </div>
                <div className="absolute top-0 right-0 p-4 opacity-10">
                  <Radio size={80} />
                </div>
            </div>
        </div>
      </div>
    </PageLayout>
  );
};