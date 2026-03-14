import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';
import { Card } from '../components/ui/Card';
import { Certificate, Agent, ErrorLog, SyncStatus } from '../types';
import { Lock, Radio, RefreshCw, Download, History, AlertCircle, FileText } from 'lucide-react';
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
  const [loading, setLoading] = useState(true);

  const fetchData = useCallback(async () => {
    try {
      const [certData, agentData, logData, syncData] = await Promise.all([
        api.getCertificates(),
        api.getAgents(),
        api.getErrorLogs({ page: 1, pageSize: 10 }),
        api.getSyncStatus(),
      ]);
      setCerts(certData);
      setAgents(agentData);
      setErrorLogs(logData.logs);
      setSyncStatus(syncData);
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
      <div className="grid grid-cols-1 sm:grid-cols-4 gap-4 mb-6">
        <Card variant="panel">
          <div className="space-y-1">
            <div className="text-slate-500 text-[10px] font-semibold uppercase tracking-wider">Agents</div>
            <div className="text-2xl font-bold text-white">{agents.length}</div>
          </div>
        </Card>
        <Card variant="panel">
          <div className="space-y-1">
            <div className="text-slate-500 text-[10px] font-semibold uppercase tracking-wider">Pods synced</div>
            <div className="text-2xl font-bold text-white">{syncStatus?.resources.pods ?? 0}</div>
          </div>
        </Card>
        <Card variant="panel">
          <div className="space-y-1">
            <div className="text-slate-500 text-[10px] font-semibold uppercase tracking-wider">Service accounts synced</div>
            <div className="text-2xl font-bold text-white">{syncStatus?.resources.sas ?? 0}</div>
          </div>
        </Card>
        <Card variant="panel">
          <div className="space-y-1">
            <div className="text-slate-500 text-[10px] font-semibold uppercase tracking-wider">Latest agent heartbeat</div>
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
    </PageLayout>
  );
};