import React, { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { AuditLog } from '../types';
import { Card } from '../components/ui/Card';
import { Download, ArrowLeft } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Pagination } from '../components/Pagination';

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];

export const Audit: React.FC = () => {
  const navigate = useNavigate();
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [loading, setLoading] = useState(true);

  const fetchLogs = useCallback(async () => {
    setLoading(true);
    const data = await api.getAuditLogs({ page, pageSize });
    setLogs(data.logs);
    setTotal(data.total);
    setLoading(false);
  }, [page, pageSize]);

  useEffect(() => {
    fetchLogs();
  }, [fetchLogs]);

  return (
    <PageLayout
      title="Audit Logs"
      description="Track system activities and user actions."
      actions={
        <div className="flex items-center gap-2 flex-wrap">
          <Button variant="secondary" size="sm" onClick={() => navigate('/monitoring')}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Back to Monitoring
          </Button>
          <Button variant="secondary">
            <Download className="w-4 h-4 mr-2" />
            Export CSV
          </Button>
        </div>
      }
    >
      <Card className="p-0 overflow-hidden">
        <div className="ui-table-scroll">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-muted uppercase bg-muted/50 border-b border-border sticky top-0 z-10">
              <tr>
                <th className="px-6 py-4 font-medium">Timestamp</th>
                <th className="px-6 py-4 font-medium">Actor</th>
                <th className="px-6 py-4 font-medium">Action</th>
                <th className="px-6 py-4 font-medium">Resource</th>
                <th className="px-6 py-4 font-medium">Status</th>
                <th className="px-6 py-4 font-medium">Details</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {loading ? (
                <tr>
                  <td colSpan={6} className="px-6 py-8 text-center text-muted">Loading...</td>
                </tr>
              ) : (
              logs.map((log) => (
                <tr key={log.id} className="hover:bg-muted/30 transition-colors">
                  <td className="px-6 py-4 font-mono text-muted whitespace-nowrap">
                    {log.timestamp}
                  </td>
                  <td className="px-6 py-4 font-medium text-text">{log.actor}</td>
                  <td className="px-6 py-4 text-muted">{log.action}</td>
                  <td className="px-6 py-4 text-muted font-mono text-xs">{log.resource}</td>
                  <td className="px-6 py-4">
                    <span className={`
                      inline-flex items-center px-2 py-0.5 rounded text-xs font-medium capitalize
                      ${log.status === 'success' ? 'text-emerald-400 bg-emerald-500/10' : ''}
                      ${log.status === 'failure' ? 'text-red-400 bg-red-500/10' : ''}
                      ${log.status === 'denied' ? 'text-orange-400 bg-orange-500/10' : ''}
                    `}>
                      {log.status}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-muted truncate max-w-xs" title={log.details}>
                    {log.details}
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