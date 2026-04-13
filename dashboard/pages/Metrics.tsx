import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';
import { Card } from '../components/ui/Card';
import { Certificate, Agent, ErrorLog, SyncStatus, WorkerStatus, PipelineHealth } from '../types';
import { Lock, Radio, RefreshCw, Download, History, AlertCircle, FileText, AlertTriangle, Activity, Layers, Shield, Target, Zap } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PageLoading } from '../components/PageLoading';
import { PageEmpty } from '../components/PageEmpty';
import { formatDateTime } from '../lib/display';

export const Monitoring: React.FC = () => {
  const navigate = useNavigate();
  const [certs, setCerts] = useState<Certificate[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [errorLogs, setErrorLogs] = useState<ErrorLog[]>([]);
  const [syncStatus, setSyncStatus] = useState<SyncStatus | null>(null);
  const [workerStatus, setWorkerStatus] = useState<WorkerStatus[]>([]);
  // Phase 3.3: Pipeline health
  const [pipelineHealth, setPipelineHealth] = useState<PipelineHealth | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    setError(null);
    try {
      const [certData, agentData, logData, syncData, workerData] = await Promise.all([
        api.getCertificates(),
        api.getAgents(),
        api.getErrorLogs({ page: 1, pageSize: 10 }),
        api.getSyncStatus(),
        api.getWorkerStatus(),
      ]);
      setCerts(certData);
      setAgents(agentData);
      setErrorLogs(logData.logs);
      setSyncStatus(syncData);
      setWorkerStatus(workerData);

      // Phase 3.3: load pipeline health (non-critical, failure OK)
      const ph = await api.getPipelineHealth().catch(() => null);
      if (ph) setPipelineHealth(ph);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Không thể tải dữ liệu monitoring');
    } finally {
      setLoading(false);
    }
  }, []);

  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.METRICS));
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);
  usePolling(fetchData, intervalMs, { refreshTrigger });
  useEffect(() => { fetchData(); }, [fetchData]);

  const lastHeartbeat = useMemo(() => {
    if (agents.length === 0) return null;
    const timestamps = agents
      .map((a) => new Date(a.lastHeartbeat).getTime())
      .filter((v) => Number.isFinite(v));
    if (timestamps.length === 0) return null;
    return new Date(Math.max(...timestamps)).toISOString();
  }, [agents]);

  const exportPayload = () => {
    const data = {
      exportedAt: new Date().toISOString(),
      agents,
      certificates: certs,
      syncStatus,
      recentErrorLogs: errorLogs,
    };
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `monitoring-export-${new Date().toISOString().replace(/[:.]/g, '-')}.json`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  };

  if (loading) return <PageLoading message="Loading operations monitoring..." className="min-h-[40vh]" />;

  return (
    <PageLayout
      title="Monitoring"
      description="Agent health, synchronization status, certificates, and operational errors."
      actions={
        <div className="flex items-center gap-2 flex-wrap">
          <Button variant="secondary" onClick={fetchData}>
            <RefreshCw className="w-4 h-4 mr-2" /> Refresh
          </Button>
          <Button variant="secondary" onClick={exportPayload}>
            <Download className="w-4 h-4 mr-2" /> Export JSON
          </Button>
        </div>
      }
      toolbar={
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
      }
    >
      {error && (
        <div className="mb-4 flex items-center gap-3 p-3 rounded-lg border border-red-800 bg-red-950/40 text-red-300">
          <AlertTriangle className="w-4 h-4 shrink-0 text-red-400" />
          <span className="text-sm flex-1">{error}</span>
          <Button variant="secondary" size="sm" onClick={fetchData}>
            <RefreshCw className="w-3 h-3 mr-1.5" /> Retry
          </Button>
        </div>
      )}
      <div className="grid grid-cols-1 sm:grid-cols-4 gap-4 mb-6">
        <Card variant="panel">
          <div className="space-y-1">
            <div className="ui-micro-label">Agents</div>
            <div className="text-2xl font-bold text-white">{agents.length}</div>
          </div>
        </Card>
        <Card variant="panel">
          <div className="space-y-1">
            <div className="ui-micro-label">Pods synced</div>
            <div className="text-2xl font-bold text-white">{syncStatus?.resources.pods ?? 0}</div>
          </div>
        </Card>
        <Card variant="panel">
          <div className="space-y-1">
            <div className="ui-micro-label">Service accounts synced</div>
            <div className="text-2xl font-bold text-white">{syncStatus?.resources.sas ?? 0}</div>
          </div>
        </Card>
        <Card variant="panel">
          <div className="space-y-1">
            <div className="ui-micro-label">Latest agent heartbeat</div>
            <div className="text-sm font-medium text-slate-200">{lastHeartbeat ? formatDateTime(lastHeartbeat) : 'N/A'}</div>
          </div>
        </Card>
      </div>

      <div className="grid lg:grid-cols-5 gap-6">
        <div className="lg:col-span-3 space-y-6">
          <Card title="Synchronization">
            <div className="grid sm:grid-cols-2 gap-4 text-sm">
              <div className="rounded-lg border border-slate-800 bg-slate-950/50 p-4">
                <div className="text-slate-500 mb-1">Last full scan</div>
                <div className="text-white">{formatDateTime(syncStatus?.lastScan)}</div>
              </div>
              <div className="rounded-lg border border-slate-800 bg-slate-950/50 p-4">
                <div className="text-slate-500 mb-1">Next scheduled scan</div>
                <div className="text-white">{formatDateTime(syncStatus?.nextScan)}</div>
              </div>
              <div className="rounded-lg border border-slate-800 bg-slate-950/50 p-4 sm:col-span-2">
                <div className="text-slate-500 mb-2">Synced resources</div>
                <div className="grid grid-cols-2 gap-2 text-slate-300">
                  <div>Pods: {syncStatus?.resources.pods ?? 0}</div>
                  <div>ServiceAccounts: {syncStatus?.resources.sas ?? 0}</div>
                  <div>Roles: {syncStatus?.resources.roles ?? 0}</div>
                  <div>Bindings: {syncStatus?.resources.bindings ?? 0}</div>
                </div>
              </div>
            </div>
          </Card>

          <Card title="Recent Error Logs" actions={<Link to="/error-logs" className="text-xs text-pink-500 hover:text-pink-400">View all</Link>}>
            {errorLogs.length === 0 ? (
              <PageEmpty title="No recent errors" description="Error stream is currently quiet." className="py-8" />
            ) : (
              <div className="space-y-2">
                {errorLogs.map((log) => (
                  <div key={log.id} className="p-3 border border-slate-800 rounded bg-slate-950/40">
                    <div className="flex items-center justify-between text-xs mb-1">
                      <span className="text-slate-500 font-mono">{formatDateTime(log.time)}</span>
                      <span className={`${log.level === 'ERROR' ? 'text-red-400' : log.level === 'WARN' ? 'text-yellow-400' : 'text-sky-400'} font-semibold`}>{log.level}</span>
                    </div>
                    <div className="text-sm text-slate-200">{log.message}</div>
                    {log.source && <div className="text-xs text-slate-500 mt-1">{log.source}</div>}
                  </div>
                ))}
              </div>
            )}
          </Card>
        </div>

        <div className="lg:col-span-2 space-y-6">
          <Card title="Agent Status">
            {agents.length === 0 ? (
              <PageEmpty title="No agents reported" description="Check daemonset and agent connectivity." className="py-8" />
            ) : (
              <div className="space-y-3">
                {agents.slice(0, 10).map((agent) => (
                  <div key={agent.id} className="flex items-center justify-between p-2.5 bg-slate-950/50 rounded-lg border border-slate-800">
                    <div className="flex items-center">
                      <Radio className={`w-4 h-4 mr-3 ${agent.status === 'up' ? 'text-emerald-500' : agent.status === 'down' ? 'text-red-500' : 'text-yellow-500'}`} />
                      <div>
                        <div className="text-sm font-medium text-white">{agent.node}</div>
                        <div className="text-xs text-slate-500">{formatDateTime(agent.lastHeartbeat)}</div>
                      </div>
                    </div>
                    <span className={`text-xs uppercase font-semibold ${agent.status === 'up' ? 'text-emerald-400' : agent.status === 'down' ? 'text-red-400' : 'text-yellow-400'}`}>{agent.status}</span>
                  </div>
                ))}
              </div>
            )}
          </Card>

          <Card title="Certificates" actions={<Link to="/certificates" className="text-xs text-pink-500 hover:text-pink-400">View all</Link>}>
            {certs.length === 0 ? (
              <PageEmpty title="No certificates data" description="Certificate information is unavailable." className="py-8" />
            ) : (
              <div className="space-y-2">
                {certs.slice(0, 5).map((cert) => (
                  <div key={cert.id} className="p-3 rounded-lg border border-slate-800 bg-slate-950/50">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2 text-sm text-white"><Lock className="w-4 h-4 text-emerald-500" />{cert.name}</div>
                      <div className={`text-xs font-semibold ${(cert.daysRemaining ?? 0) <= 30 ? 'text-red-400' : 'text-emerald-400'}`}>{cert.daysRemaining ?? 0} days</div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </Card>
        </div>
      </div>

      <div className="mt-6">
        <Card title="Worker / Queue Status" actions={<Activity className="w-4 h-4 text-slate-500" />}>
          {workerStatus.length === 0 ? (
            <PageEmpty title="No worker data" description="Worker status unavailable." className="py-8" />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-slate-800 text-slate-500 text-xs uppercase">
                    <th className="text-left py-2 pr-4 font-medium">Worker</th>
                    <th className="text-right py-2 px-4 font-medium">Queue Depth</th>
                    <th className="text-right py-2 px-4 font-medium">Active</th>
                    <th className="text-right py-2 px-4 font-medium">Processed</th>
                    <th className="text-right py-2 px-4 font-medium">Failed</th>
                    <th className="text-right py-2 pl-4 font-medium">Status</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/50">
                  {workerStatus.map((w) => (
                    <tr key={w.name} className="hover:bg-slate-800/20">
                      <td className="py-2.5 pr-4 font-mono text-slate-200">{w.name}</td>
                      <td className="py-2.5 px-4 text-right text-slate-300">{w.queueDepth}</td>
                      <td className="py-2.5 px-4 text-right text-slate-300">{w.activeWorkers}</td>
                      <td className="py-2.5 px-4 text-right text-slate-300">{w.processed.toLocaleString()}</td>
                      <td className={`py-2.5 px-4 text-right font-medium ${w.failed > 0 ? 'text-red-400' : 'text-slate-500'}`}>{w.failed}</td>
                      <td className="py-2.5 pl-4 text-right">
                        <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold uppercase ${
                          w.status === 'running' ? 'bg-emerald-900/40 text-emerald-400' :
                          w.status === 'degraded' ? 'bg-yellow-900/40 text-yellow-400' :
                          'bg-slate-800 text-slate-500'
                        }`}>{w.status}</span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      </div>

      {/* Phase 3.3: Pipeline Health section */}
      {pipelineHealth && (
        <div className="mt-6">
          <Card title="Pipeline Health" actions={<Layers className="w-4 h-4 text-slate-500" />}>
            <p className="text-xs text-slate-500 mb-4">Status of each layer in the Unified Risk Pipeline. Data refreshes automatically.</p>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
              {/* Layer 1 */}
              <div className="p-3 rounded-lg bg-slate-950/50 border border-slate-800">
                <div className="flex items-center gap-2 mb-2">
                  <Shield className="w-4 h-4 text-blue-400" />
                  <span className="text-xs font-semibold text-slate-300 uppercase tracking-wide">Layer 1 · Fact Discovery</span>
                </div>
                <div className="space-y-1.5 text-xs text-slate-400">
                  <div className="flex justify-between">
                    <span>Last PCE eval</span>
                    <span className="text-slate-200">{pipelineHealth.layer1.lastPceEval ? formatDateTime(pipelineHealth.layer1.lastPceEval) : '—'}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Last Risk Engine eval</span>
                    <span className="text-slate-200">{pipelineHealth.layer1.lastRiskEngineEval ? formatDateTime(pipelineHealth.layer1.lastRiskEngineEval) : '—'}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Active insights</span>
                    <span className="text-slate-200 font-mono">{pipelineHealth.layer1.insightCount.toLocaleString()}</span>
                  </div>
                </div>
              </div>

              {/* Layer 2 */}
              <div className="p-3 rounded-lg bg-slate-950/50 border border-slate-800">
                <div className="flex items-center gap-2 mb-2">
                  <Zap className="w-4 h-4 text-yellow-400" />
                  <span className="text-xs font-semibold text-slate-300 uppercase tracking-wide">Layer 2 · Runtime</span>
                </div>
                <div className="space-y-1.5 text-xs text-slate-400">
                  <div className="flex justify-between">
                    <span>Last state change</span>
                    <span className="text-slate-200">{pipelineHealth.layer2.lastStateChange ? formatDateTime(pipelineHealth.layer2.lastStateChange) : '—'}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Promotion rules</span>
                    <span className="text-slate-200 font-mono">{pipelineHealth.layer2.activePromotionRules}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className={pipelineHealth.layer2.exploitedCapCount > 0 ? 'text-red-400' : ''}>Exploited caps</span>
                    <span className={`font-mono ${pipelineHealth.layer2.exploitedCapCount > 0 ? 'text-red-400' : 'text-slate-200'}`}>{pipelineHealth.layer2.exploitedCapCount}</span>
                  </div>
                </div>
              </div>

              {/* Layer 3 */}
              <div className="p-3 rounded-lg bg-slate-950/50 border border-slate-800">
                <div className="flex items-center gap-2 mb-2">
                  <Target className="w-4 h-4 text-orange-400" />
                  <span className="text-xs font-semibold text-slate-300 uppercase tracking-wide">Layer 3 · Attack Paths</span>
                </div>
                <div className="space-y-1.5 text-xs text-slate-400">
                  <div className="flex justify-between">
                    <span>Last path computation</span>
                    <span className="text-slate-200">{pipelineHealth.layer3.lastPathComputation ? formatDateTime(pipelineHealth.layer3.lastPathComputation) : '—'}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Total paths</span>
                    <span className="text-slate-200 font-mono">{pipelineHealth.layer3.totalPaths.toLocaleString()}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className={pipelineHealth.layer3.criticalPaths > 0 ? 'text-red-400' : ''}>Critical paths</span>
                    <span className={`font-mono ${pipelineHealth.layer3.criticalPaths > 0 ? 'text-red-400' : 'text-slate-200'}`}>{pipelineHealth.layer3.criticalPaths}</span>
                  </div>
                </div>
              </div>

              {/* Layer 4 */}
              <div className="p-3 rounded-lg bg-slate-950/50 border border-slate-800">
                <div className="flex items-center gap-2 mb-2">
                  <Activity className="w-4 h-4 text-emerald-400" />
                  <span className="text-xs font-semibold text-slate-300 uppercase tracking-wide">Layer 4 · Unified Score</span>
                </div>
                <div className="space-y-1.5 text-xs text-slate-400">
                  <div className="flex justify-between">
                    <span>Last score calc</span>
                    <span className="text-slate-200">{pipelineHealth.layer4.lastScoreCalc ? formatDateTime(pipelineHealth.layer4.lastScoreCalc) : '—'}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Resources scored</span>
                    <span className="text-slate-200 font-mono">{pipelineHealth.layer4.resourcesScored.toLocaleString()}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Avg score</span>
                    <span className="text-slate-200 font-mono">{pipelineHealth.layer4.avgScore.toFixed(1)}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>V3 resources</span>
                    <span className="text-emerald-400 font-mono">{pipelineHealth.layer4.v3Resources.toLocaleString()}</span>
                  </div>
                </div>
              </div>
            </div>
          </Card>
        </div>
      )}
    </PageLayout>
  );
};