import React, { useCallback, useEffect, useState } from 'react';
import { api } from '../lib/api';
import { ErrorLog } from '../types';
import { Card } from '../components/ui/Card';
import { PageLayout } from '../components/PageLayout';
import { Pagination } from '../components/Pagination';
import { AlertCircle, RefreshCw, Filter } from 'lucide-react';
import { Button } from '../components/ui/Button';

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];
const LEVEL_OPTIONS = ['', 'ERROR', 'WARN', 'INFO'];
const SOURCE_OPTIONS = ['', 'core', 'agent', 'worker'];

export const ErrorLogs: React.FC = () => {
  const [logs, setLogs] = useState<ErrorLog[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [level, setLevel] = useState('');
  const [source, setSource] = useState('');
  const [loading, setLoading] = useState(true);

  const fetchLogs = useCallback(async () => {
    setLoading(true);
    const data = await api.getErrorLogs({
      page,
      pageSize,
      ...(level && { level }),
      ...(source && { source }),
    });
    setLogs(data.logs);
    setTotal(data.total);
    setLoading(false);
  }, [page, pageSize, level, source]);

  useEffect(() => {
    fetchLogs();
  }, [fetchLogs]);

  const formatTime = (t: string) => {
    try {
      const d = new Date(t);
      return d.toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'medium' });
    } catch {
      return t;
    }
  };

  const levelClass = (l: string) => {
    switch (l?.toUpperCase()) {
      case 'ERROR':
        return 'text-red-400 bg-red-500/10';
      case 'WARN':
        return 'text-amber-400 bg-amber-500/10';
      case 'INFO':
        return 'text-sky-400 bg-sky-500/10';
      default:
        return 'text-slate-400 bg-slate-500/10';
    }
  };

  return (
    <PageLayout
      title="Error Logs"
      description="System and application error logs from Core and Agent (error_logs table)."
      actions={
        <Button variant="secondary" onClick={fetchLogs} disabled={loading}>
          <RefreshCw className={`w-4 h-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      }
    >
      {/* Filters */}
      <div className="flex flex-wrap items-center gap-4 mb-6">
        <div className="flex items-center gap-2 text-slate-400 text-sm">
          <Filter className="w-4 h-4" />
          <span>Filter</span>
        </div>
        <select
          value={level}
          onChange={(e) => { setLevel(e.target.value); setPage(1); }}
          className="bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:ring-2 focus:ring-pink-500 outline-none"
        >
          {LEVEL_OPTIONS.map((opt) => (
            <option key={opt} value={opt}>
              {opt || 'All levels'}
            </option>
          ))}
        </select>
        <select
          value={source}
          onChange={(e) => { setSource(e.target.value); setPage(1); }}
          className="bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:ring-2 focus:ring-pink-500 outline-none"
        >
          {SOURCE_OPTIONS.map((opt) => (
            <option key={opt} value={opt}>
              {opt || 'All sources'}
            </option>
          ))}
        </select>
      </div>

      <Card className="overflow-hidden">
        <div className="overflow-x-auto max-h-[calc(100vh-20rem)] overflow-y-auto">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-slate-400 uppercase bg-slate-950/30 border-b border-slate-800 sticky top-0 z-10">
              <tr>
                <th className="px-6 py-4 font-medium whitespace-nowrap">Time</th>
                <th className="px-6 py-4 font-medium whitespace-nowrap">Level</th>
                <th className="px-6 py-4 font-medium whitespace-nowrap">Source</th>
                <th className="px-6 py-4 font-medium">Message</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800">
              {loading ? (
                <tr>
                  <td colSpan={4} className="px-6 py-12 text-center text-slate-500">
                    Loading...
                  </td>
                </tr>
              ) : logs.length === 0 ? (
                <tr>
                  <td colSpan={4} className="px-6 py-12 text-center text-slate-500">
                    <AlertCircle className="w-10 h-10 mx-auto mb-2 text-slate-600" />
                    <p>No error logs found.</p>
                    <p className="text-xs mt-1">Logs are written by Core/Agent when errors occur.</p>
                  </td>
                </tr>
              ) : (
                logs.map((log) => (
                  <tr key={log.id} className="hover:bg-slate-800/50 transition-colors">
                    <td className="px-6 py-4 font-mono text-slate-500 whitespace-nowrap text-xs">
                      {formatTime(log.time)}
                    </td>
                    <td className="px-6 py-4">
                      <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${levelClass(log.level)}`}>
                        {log.level}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-slate-400 font-mono text-xs">
                      {log.source ?? '—'}
                    </td>
                    <td className="px-6 py-4 text-slate-300 break-all max-w-2xl" title={log.message}>
                      {log.message}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </Card>

      {(logs.length > 0 || total > 0) && (
        <Pagination
          page={page}
          pageSize={pageSize}
          total={total}
          onPageChange={setPage}
          onPageSizeChange={(size) => { setPageSize(size); setPage(1); }}
          pageSizeOptions={PAGE_SIZE_OPTIONS}
          itemLabel="entries"
        />
      )}
    </PageLayout>
  );
};
