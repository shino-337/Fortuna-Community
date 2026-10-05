import React, { useCallback, useEffect, useRef, useState } from 'react';
import { RefreshCw } from 'lucide-react';
import { api, getAvailabilityIssue } from '../../lib/api';
import type { AuditLog } from '../../types';
import { Card } from '../../design-system/components/Card';
import { FilterBar } from '../../design-system/components/FilterBar';
import { PageEmpty, PageError, PageLoading } from '../../design-system/components/PageStatus';
import { Button } from '../ui/Button';
import { Pagination } from '../Pagination';
import { formatDateTime } from '../../lib/display';
import { UI_TABLE, UI_TD, UI_TH, UI_TR, UI_THEAD_STICKY } from '../../lib/tableChrome';

/** Platform audit log (GET /audit/logs), paged on the server. Requires system.audit.read. */
export const PlatformAuditLogPanel: React.FC = () => {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [resource, setResource] = useState('');
  const [action, setAction] = useState('');
  const [debounced, setDebounced] = useState({ resource: '', action: '' });
  const requestRef = useRef(0);

  useEffect(() => {
    const t = window.setTimeout(() => setDebounced({ resource: resource.trim(), action: action.trim() }), 300);
    return () => window.clearTimeout(t);
  }, [resource, action]);

  const fetchPage = useCallback(async () => {
    // Only the newest request may update the table.
    const seq = ++requestRef.current;
    setLoading(true);
    try {
      const data = await api.getAuditLogs({
        page,
        pageSize,
        resource: debounced.resource || undefined,
        action: debounced.action || undefined,
      });
      if (seq !== requestRef.current) return;
      setLogs(data.logs);
      setTotal(Number.isFinite(data.total) ? data.total : 0);
      setError(null);
    } catch (e) {
      if (seq !== requestRef.current) return;
      setLogs([]);
      setTotal(0);
      setError(getAvailabilityIssue(e, 'Platform audit logs').description);
    } finally {
      if (seq === requestRef.current) setLoading(false);
    }
  }, [page, pageSize, debounced]);

  useEffect(() => {
    void fetchPage();
  }, [fetchPage]);

  const filtersActive = resource.trim() !== '' || action.trim() !== '';
  const resetFilters = () => {
    setResource('');
    setAction('');
    setPage(1);
  };

  return (
    <Card
      variant="panel"
      contentClassName="p-0"
      title="Platform audit log"
      actions={
        <Button variant="secondary" size="sm" type="button" onClick={() => void fetchPage()} disabled={loading}>
          <RefreshCw className="w-4 h-4 mr-1.5" /> Refresh
        </Button>
      }
    >
      <div className="border-b border-border px-4 py-3">
        <FilterBar
          embedded
          reset={{ onReset: resetFilters, active: filtersActive }}
          search={{
            value: action,
            onChange: (v) => {
              setAction(v);
              setPage(1);
            },
            placeholder: 'Action (exact, e.g. delete)',
          }}
          namespace={{
            inputMode: true,
            value: resource,
            onChange: (v) => {
              setResource(v);
              setPage(1);
            },
            options: [],
            placeholder: 'Resource (exact, e.g. users)',
            title: 'Filter by resource',
          }}
        />
      </div>
      {loading && logs.length === 0 ? (
        <PageLoading className="py-8 px-3" />
      ) : error ? (
        <PageError title="Audit logs unavailable" description={error} className="py-8 px-3" />
      ) : logs.length === 0 ? (
        <PageEmpty
          title="No audit entries"
          description={filtersActive ? 'No entries match these filters.' : 'No platform audit records yet.'}
          className="py-8 px-3"
        />
      ) : (
        <>
          <div className="ui-table-scroll">
            <table className={UI_TABLE}>
              <thead className={UI_THEAD_STICKY}>
                <tr>
                  <th className={UI_TH}>Time</th>
                  <th className={UI_TH}>Actor</th>
                  <th className={UI_TH}>Action</th>
                  <th className={UI_TH}>Resource</th>
                  <th className={UI_TH}>Status</th>
                </tr>
              </thead>
              <tbody>
                {logs.map((log) => (
                  <tr key={log.id || `${log.timestamp}-${log.action}`} className={UI_TR}>
                    <td className={`${UI_TD} font-mono text-caption text-muted`}>{formatDateTime(log.timestamp)}</td>
                    <td className={`${UI_TD} text-text font-medium`}>{log.actor || log.user || 'system'}</td>
                    <td className={`${UI_TD} text-muted`}>{log.action}</td>
                    <td className={`${UI_TD} text-muted font-mono text-caption`}>{log.resource}</td>
                    <td className={UI_TD}>{log.status}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <Pagination
            page={page}
            pageSize={pageSize}
            total={total}
            onPageChange={setPage}
            onPageSizeChange={(size) => {
              setPageSize(size);
              setPage(1);
            }}
            pageSizeOptions={[10, 20, 50]}
            itemLabel="entries"
          />
        </>
      )}
    </Card>
  );
};
