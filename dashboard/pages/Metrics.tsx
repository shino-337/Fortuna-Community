import React, { useCallback, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { Card } from '../components/ui/Card';
import { Certificate, Agent, ErrorLog, SyncStatus } from '../types';
import { Lock, Radio, RefreshCw, CheckCircle, Clock, Download, List, History, AlertCircle, FileText } from 'lucide-react';
import { Button } from '../components/ui/Button';

export const Monitoring: React.FC = () => {
  const navigate = useNavigate();
  const [certs, setCerts] = useState<Certificate[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [errorLogs, setErrorLogs] = useState<ErrorLog[]>([]);
  const [syncStatus, setSyncStatus] = useState<SyncStatus | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchData = useCallback(async () => {
    const [certData, agentData, logData, syncData] = await Promise.all([
      api.getCertificates(),
      api.getAgents(),
      api.getErrorLogs({ page: 1, pageSize: 10 }),
      api.getSyncStatus()
    ]);
    setCerts(certData);
    setAgents(agentData);
    setErrorLogs(logData.logs);
    setSyncStatus(syncData);
    setLoading(false);
  }, []);

  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.METRICS));
  usePolling(fetchData, intervalMs);

  if (loading) return <div className="text-slate-500 p-8">Loading Operations Center...</div>;

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center flex-wrap gap-4">
        <div>
           <h1 className="text-2xl font-bold text-white">Monitoring (Agent & System Health)</h1>
           <p className="text-slate-400">Operations center for health, agents, and infrastructure.</p>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <Button variant="secondary" size="sm" onClick={() => navigate('/audit')}>
            <History className="w-4 h-4 mr-1.5" /> Audit Logs
          </Button>
          <Button variant="secondary" size="sm" onClick={() => navigate('/error-logs')}>
            <AlertCircle className="w-4 h-4 mr-1.5" /> Error Logs
          </Button>
          <Button variant="secondary" size="sm" onClick={() => navigate('/certificates')}>
            <Lock className="w-4 h-4 mr-1.5" /> Certificates
          </Button>
          <Button variant="secondary" size="sm" onClick={() => navigate('/reports')}>
            <FileText className="w-4 h-4 mr-1.5" /> Reports
          </Button>
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
      
      {/* System Health Overview – all from real API (Agents, Sync from DB) */}
      <div className="grid grid-cols-2 md:grid-cols-2 gap-4">
           <div className="bg-slate-900 border border-slate-800 p-4 rounded-lg">
               <div className="flex justify-between items-start mb-2">
                   <div className="text-slate-400 text-sm font-medium">Agents</div>
                   <Radio className="w-5 h-5 text-emerald-500" />
               </div>
               <div className="text-2xl font-bold text-white">{agents.length}</div>
               <div className="text-xs text-emerald-400 mt-1">From API (agents table)</div>
           </div>
           <div className="bg-slate-900 border border-slate-800 p-4 rounded-lg">
               <div className="flex justify-between items-start mb-2">
                   <div className="text-slate-400 text-sm font-medium">Sync</div>
                   <CheckCircle className="w-5 h-5 text-emerald-500" />
               </div>
               <div className="text-2xl font-bold text-white">{syncStatus?.resources.pods ?? 0}</div>
               <div className="text-xs text-slate-400 mt-1">Pods synced (from DB)</div>
           </div>
      </div>

      <div className="grid lg:grid-cols-5 gap-6">
           {/* Left Column: Sync Status (no Workers/Queue/Latency – removed) */}
           <div className="lg:col-span-3 space-y-6">
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
                   <Button variant="secondary" size="sm" className="w-full mt-2" onClick={() => navigate('/certificates')}>View Certificates</Button>
               </Card>

               <Card title="Error Logs" actions={<Link to="/error-logs" className="text-xs text-pink-500 hover:text-pink-400">View All</Link>}>
                   <div className="space-y-2">
                       {errorLogs.length > 0 ? (
                         <>
                           {errorLogs.map(log => (
                               <div key={log.id} className="p-2 border-l-2 border-slate-700 pl-3 bg-slate-900/50">
                                   <div className="flex items-center justify-between text-xs mb-1">
                                       <span className="font-mono text-slate-500">{log.time}</span>
                                       <span className={`font-bold ${log.level === 'ERROR' ? 'text-red-500' : log.level === 'WARN' ? 'text-yellow-500' : 'text-sky-500'}`}>{log.level}</span>
                                   </div>
                                   <p className="text-xs text-slate-300 line-clamp-2">{log.message}</p>
                                   {log.source && <span className="text-xs text-slate-500 mt-1 block">{log.source}</span>}
                               </div>
                           ))}
                           <Link to="/error-logs" className="w-full text-center text-xs text-slate-500 hover:text-white mt-2 flex items-center justify-center">
                               <List className="w-3 h-3 mr-1" /> View Full Logs
                           </Link>
                         </>
                       ) : (
                         <p className="text-slate-500 text-sm">No error logs. View <Link to="/error-logs" className="text-pink-500 hover:underline">Error Logs</Link> for full list.</p>
                       )}
                   </div>
               </Card>
           </div>
      </div>
    </div>
  );
};