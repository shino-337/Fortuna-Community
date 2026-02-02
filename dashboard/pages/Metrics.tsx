import React, { useCallback, useState } from 'react';
import { api } from '../lib/api';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { Card } from '../components/ui/Card';
import { Certificate, Agent, QueueMetric, ErrorLog, SyncStatus } from '../types';
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from 'recharts';
import { Lock, Radio, Activity, AlertCircle, RefreshCw, CheckCircle, Clock, Server, Download, List } from 'lucide-react';
import { Button } from '../components/ui/Button';

export const Monitoring: React.FC = () => {
  const [certs, setCerts] = useState<Certificate[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [queueMetrics, setQueueMetrics] = useState<QueueMetric[]>([]);
  const [errorLogs, setErrorLogs] = useState<ErrorLog[]>([]);
  const [syncStatus, setSyncStatus] = useState<SyncStatus | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchData = useCallback(async () => {
    const [certData, agentData, queueData, logData, syncData] = await Promise.all([
      api.getCertificates(),
      api.getAgents(),
      api.getQueueMetrics(),
      api.getErrorLogs(),
      api.getSyncStatus()
    ]);
    setCerts(certData);
    setAgents(agentData);
    setQueueMetrics(queueData);
    setErrorLogs(logData);
    setSyncStatus(syncData);
    setLoading(false);
  }, []);

  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.METRICS));
  usePolling(fetchData, intervalMs);

  if (loading) return <div className="text-slate-500 p-8">Loading Operations Center...</div>;

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
           <h1 className="text-2xl font-bold text-white">System Monitoring</h1>
           <p className="text-slate-400">Operations center for health, performance, and infrastructure.</p>
        </div>
        <div className="flex items-center space-x-3">
             <div className="flex items-center bg-slate-900 px-3 py-1.5 rounded-lg border border-slate-800 text-sm text-slate-400">
                <Clock className="w-4 h-4 mr-2" /> Last 1 hour
             </div>
             <Button variant="secondary" className="text-sm">
                <RefreshCw className="w-4 h-4 mr-2" /> Auto: 10s
             </Button>
             <Button variant="secondary" className="text-sm">
                <Download className="w-4 h-4 mr-2" /> Export
             </Button>
        </div>
      </div>
      
      {/* System Health Overview Cards - all from API or "—" when not available */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
           <div className="bg-slate-900 border border-slate-800 p-4 rounded-lg">
               <div className="flex justify-between items-start mb-2">
                   <div className="text-slate-400 text-sm font-medium">Workers</div>
                   <Server className="w-5 h-5 text-slate-500" />
               </div>
               <div className="text-2xl font-bold text-white">—</div>
               <div className="text-xs text-slate-500 mt-1">API not yet available</div>
           </div>
           <div className="bg-slate-900 border border-slate-800 p-4 rounded-lg">
               <div className="flex justify-between items-start mb-2">
                   <div className="text-slate-400 text-sm font-medium">Queue Depth</div>
                   <Activity className="w-5 h-5 text-blue-500" />
               </div>
               <div className="text-2xl font-bold text-white">
                 {queueMetrics.length > 0
                   ? (() => {
                       const m = queueMetrics[queueMetrics.length - 1] as { normalizer?: number; correlator?: number; risk?: number };
                       const total = (m.normalizer ?? 0) + (m.correlator ?? 0) + (m.risk ?? 0);
                       return total;
                     })()
                   : '—'}
               </div>
               <div className="text-xs text-slate-500 mt-1">{queueMetrics.length > 0 ? 'From API' : 'No data'}</div>
           </div>
           <div className="bg-slate-900 border border-slate-800 p-4 rounded-lg">
               <div className="flex justify-between items-start mb-2">
                   <div className="text-slate-400 text-sm font-medium">Agents</div>
                   <Radio className="w-5 h-5 text-emerald-500" />
               </div>
               <div className="text-2xl font-bold text-white">{agents.length}</div>
               <div className="text-xs text-emerald-400 mt-1">From API</div>
           </div>
           <div className="bg-slate-900 border border-slate-800 p-4 rounded-lg">
               <div className="flex justify-between items-start mb-2">
                   <div className="text-slate-400 text-sm font-medium">API Latency</div>
                   <Activity className="w-5 h-5 text-slate-500" />
               </div>
               <div className="text-2xl font-bold text-white">—</div>
               <div className="text-xs text-slate-500 mt-1">API not yet available</div>
           </div>
      </div>

      <div className="grid lg:grid-cols-5 gap-6">
           {/* Left Column: Metrics & Graphs (60% visually, using col-span-3) */}
           <div className="lg:col-span-3 space-y-6">
                <Card title="Queue Processing Metrics">
                    <div className="h-64 w-full">
                        <ResponsiveContainer width="100%" height="100%">
                        <AreaChart data={queueMetrics}>
                            <defs>
                                <linearGradient id="colorNorm" x1="0" y1="0" x2="0" y2="1">
                                    <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.8}/>
                                    <stop offset="95%" stopColor="#3b82f6" stopOpacity={0}/>
                                </linearGradient>
                                <linearGradient id="colorRisk" x1="0" y1="0" x2="0" y2="1">
                                    <stop offset="5%" stopColor="#ef4444" stopOpacity={0.8}/>
                                    <stop offset="95%" stopColor="#ef4444" stopOpacity={0}/>
                                </linearGradient>
                            </defs>
                            <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#1e293b" />
                            <XAxis dataKey="timestamp" axisLine={false} tickLine={false} tick={{fill: '#94a3b8'}} />
                            <YAxis axisLine={false} tickLine={false} tick={{fill: '#94a3b8'}} />
                            <Tooltip 
                            contentStyle={{ backgroundColor: '#0f172a', borderRadius: '8px', border: '1px solid #1e293b', color: '#f8fafc' }}
                            />
                            <Legend />
                            <Area type="monotone" dataKey="normalizer" stackId="1" stroke="#3b82f6" fill="url(#colorNorm)" name="Normalizer" />
                            <Area type="monotone" dataKey="risk" stackId="1" stroke="#ef4444" fill="url(#colorRisk)" name="Risk Engine" />
                        </AreaChart>
                        </ResponsiveContainer>
                    </div>
                </Card>

                <Card title="Sync Status & Performance">
                    <div className="grid sm:grid-cols-2 gap-6">
                         <div className="space-y-4">
                             <div className="flex justify-between items-center p-3 bg-slate-950/50 border border-slate-800 rounded-lg">
                                 <span className="text-sm text-slate-400">Last Full Scan</span>
                                 <span className="flex items-center text-sm text-white font-medium">
                                     <CheckCircle className="w-4 h-4 text-emerald-500 mr-2" />
                                     {syncStatus?.lastScan}
                                 </span>
                             </div>
                             <div className="flex justify-between items-center p-3 bg-slate-950/50 border border-slate-800 rounded-lg">
                                 <span className="text-sm text-slate-400">Next Scan</span>
                                 <span className="text-sm text-white font-medium">{syncStatus?.nextScan}</span>
                             </div>
                             <div className="flex justify-between items-center p-3 bg-slate-950/50 border border-slate-800 rounded-lg">
                                 <span className="text-sm text-slate-400">API Drift</span>
                                 <span className="flex items-center text-sm text-emerald-400 font-medium">
                                     {syncStatus?.drift ? 'Detected' : 'None'}
                                 </span>
                             </div>
                         </div>
                         <div className="bg-slate-950/30 p-4 rounded-lg border border-slate-800/50">
                             <h4 className="text-xs font-semibold text-slate-500 uppercase mb-3">Synced Resources</h4>
                             <ul className="space-y-2 text-sm">
                                 <li className="flex justify-between">
                                     <span className="text-slate-400">Pods</span>
                                     <span className="text-white font-mono">{syncStatus?.resources.pods}</span>
                                 </li>
                                 <li className="flex justify-between">
                                     <span className="text-slate-400">ServiceAccounts</span>
                                     <span className="text-white font-mono">{syncStatus?.resources.sas}</span>
                                 </li>
                                 <li className="flex justify-between">
                                     <span className="text-slate-400">Roles</span>
                                     <span className="text-white font-mono">{syncStatus?.resources.roles}</span>
                                 </li>
                                 <li className="flex justify-between">
                                     <span className="text-slate-400">Bindings</span>
                                     <span className="text-white font-mono">{syncStatus?.resources.bindings}</span>
                                 </li>
                             </ul>
                             <Button variant="secondary" size="sm" className="w-full mt-4">Manual Sync</Button>
                         </div>
                    </div>
                </Card>
           </div>

           {/* Right Column: Status & Logs (40%) */}
           <div className="lg:col-span-2 space-y-6">
               <Card title="Agent Status" actions={<Button variant="ghost" className="text-xs text-pink-500">View All</Button>}>
                   <div className="space-y-3">
                       {agents.slice(0, 5).map(agent => (
                           <div key={agent.id} className="flex items-center justify-between p-2.5 bg-slate-950/50 rounded-lg border border-slate-800">
                               <div className="flex items-center">
                                   <Radio className={`w-4 h-4 mr-3 ${agent.status === 'up' ? 'text-emerald-500' : 'text-yellow-500'}`} />
                                   <div>
                                       <div className="text-sm font-medium text-white">{agent.node}</div>
                                       <div className="text-xs text-slate-500">{agent.lastHeartbeat}</div>
                                   </div>
                               </div>
                               <span className={`text-xs uppercase font-bold ${agent.status === 'up' ? 'text-emerald-500' : 'text-yellow-500'}`}>
                                   {agent.status}
                               </span>
                           </div>
                       ))}
                   </div>
               </Card>

               <Card title="Certificate Status">
                   {certs.slice(0, 2).map(cert => (
                        <div key={cert.id} className="mb-3 last:mb-0 p-3 bg-slate-950/50 rounded-lg border border-slate-800">
                            <div className="flex justify-between items-start mb-1">
                                <div className="flex items-center">
                                    <Lock className="w-4 h-4 text-emerald-500 mr-2" />
                                    <span className="text-sm font-medium text-white truncate w-32">{cert.name}</span>
                                </div>
                                <span className={`text-xs font-bold ${cert.daysRemaining && cert.daysRemaining < 30 ? 'text-red-400' : 'text-emerald-400'}`}>
                                    {cert.daysRemaining} days
                                </span>
                            </div>
                            <div className="w-full bg-slate-800 rounded-full h-1.5 mt-2">
                                <div className="bg-emerald-500 h-1.5 rounded-full" style={{ width: '90%' }}></div>
                            </div>
                        </div>
                   ))}
                   <Button variant="secondary" size="sm" className="w-full mt-2">Manage Certificates</Button>
               </Card>

               <Card title="Error Logs (Last 1h)">
                   <div className="space-y-2">
                       {errorLogs.length > 0 ? (
                         <>
                           {errorLogs.map(log => (
                               <div key={log.id} className="p-2 border-l-2 border-slate-700 pl-3 bg-slate-900/50">
                                   <div className="flex items-center justify-between text-xs mb-1">
                                       <span className="font-mono text-slate-500">{log.time}</span>
                                       <span className={`font-bold ${log.level === 'ERROR' ? 'text-red-500' : 'text-yellow-500'}`}>{log.level}</span>
                                   </div>
                                   <p className="text-xs text-slate-300 line-clamp-2">{log.message}</p>
                               </div>
                           ))}
                           <button className="w-full text-center text-xs text-slate-500 hover:text-white mt-2 flex items-center justify-center">
                               <List className="w-3 h-3 mr-1" /> View Full Logs
                           </button>
                         </>
                       ) : (
                         <p className="text-slate-500 text-sm">Error log aggregation not yet available.</p>
                       )}
                   </div>
               </Card>
           </div>
      </div>
    </div>
  );
};