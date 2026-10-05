import React, { useCallback, useEffect, useRef, useState } from 'react';
import { RefreshCw } from 'lucide-react';
import { api, getAvailabilityIssue } from '../../lib/api';
import type { SecurityActivityItem } from '../../types';
import { Card } from '../../design-system/components/Card';
import { FilterBar } from '../../design-system/components/FilterBar';
import { PageEmpty, PageError, PageLoading } from '../../design-system/components/PageStatus';
import { Button } from '../ui/Button';
import { Pagination } from '../Pagination';
import { formatDateTime } from '../../lib/display';
import { getSeverityBadgeClass } from '../../lib/severity';
import { UI_FILTER_SELECT } from '../../lib/formChrome';
import { UI_TABLE, UI_TD, UI_TH, UI_TR, UI_THEAD_STICKY } from '../../lib/tableChrome';

const SEVERITIES = ['critical', 'high', 'medium', 'low', 'info'] as const;
const RESULTS = ['success', 'deny', 'error'] as const;

/** Security activity timeline (GET /governance/security-activity, append-only). Requires system.audit.read. */
export const SecurityActivityPanel: React.FC = () => {
  const [items, setItems] = useState<SecurityActivityItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(30);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [severity, setSeverity] = useState('');
  const [result, setResult] = useState('');
  const [action, setAction] = useState('');
  const [debouncedAction, setDebouncedAction] = useState('');
  const requestRef = useRef(0);

  useEffect(() => {
    const t = window.setTimeout(() => setDebouncedAction(action.trim()), 300);
    return () => window.clearTimeout(t);
  }, [action]);

  const fetchPage = useCallback(async () => {
    // Overlapping filter changes: only the newest request may update the table.
    const seq = ++requestRef.current;
    setLoading(true);
    try {
      const r = await api.listSecurityActivity({
        limit: pageSize,
        offset: (page - 1) * pageSize,
        severity: severity || undefined,
        result: result || undefined,
        action: debouncedAction || undefined,
      });
      if (seq !== requestRef.current) return;
      setItems(r.items);
      setTotal(r.total);
      setError(null);
    } catch (e) {
      if (seq !== requestRef.current) return;
      setItems([]);
      setTotal(0);
      setError(getAvailabilityIssue(e, 'Security activity').description);
    } finally {
      if (seq === requestRef.current) setLoading(false);
    }
  }, [page, pageSize, severity, result, debouncedAction]);

  useEffect(() => {
    void fetchPage();
  }, [fetchPage]);

  const filtersActive = severity !== '' || result !== '' || action.trim() !== '';
  const resetFilters = () => {
    setSeverity('');
    setResult('');
    setAction('');
    setPage(1);
  };

  return (
    <Card
      variant="panel"
      contentClassName="p-0"
      title="Security activity"
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
            placeholder: 'Action contains, e.g. graph_query',
          }}
          trailing={
            <>
              <select
                aria-label="Severity"
                className={UI_FILTER_SELECT}
                value={severity}
                onChange={(e) => {
                  setSeverity(e.target.value);
                  setPage(1);
                }}
              >
                <option value="">Any severity</option>
                {SEVERITIES.map((s) => (
                  <option key={s} value={s}>
                    {s}
                  </option>
                ))}
              </select>
              <select
                aria-label="Result"
                className={UI_FILTER_SELECT}
                value={result}
                onChange={(e) => {
                  setResult(e.target.value);
                  setPage(1);
                }}
              >
                <option value="">Any result</option>
                {RESULTS.map((r) => (
                  <option key={r} value={r}>
                    {r}
                  </option>
                ))}
              </select>
            </>
          }
        />
      </div>
      {loading && items.length === 0 ? (
        <PageLoading className="py-8 px-3" />
      ) : error ? (
        <PageError title="Security activity unavailable" description={error} className="py-8 px-3" />
      ) : items.length === 0 ? (
        <PageEmpty
          title="No events"
          description={filtersActive ? 'No events match these filters.' : 'No security activity recorded yet.'}
          className="py-8 px-3"
        />
      ) : (
        <>
          <div className="ui-table-scroll">
            <table className={UI_TABLE}>
              <thead className={UI_THEAD_STICKY}>
                <tr>
                  <th className={UI_TH}>Time</th>
                  <th className={UI_TH}>Severity</th>
                  <th className={UI_TH}>Result</th>
                  <th className={UI_TH}>Action</th>
                  <th className={UI_TH}>Actor</th>
                  <th className={UI_TH}>Resource</th>
                  <th className={UI_TH}>Session</th>
                </tr>
              </thead>
              <tbody>
                {items.map((row) => (
                  <tr key={`${row.id ?? row.eventId ?? row.createdAt}-${row.action}`} className={UI_TR}>
                    <td className={`${UI_TD} text-caption whitespace-nowrap`}>
                      {row.createdAt ? formatDateTime(row.createdAt) : '—'}
                    </td>
                    <td className={UI_TD}>
                      <span className={getSeverityBadgeClass((row.severity || 'info').toLowerCase())}>
                        {row.severity || '—'}
                      </span>
                    </td>
                    <td className={`${UI_TD} text-caption`}>{row.result || '—'}</td>
                    <td className={`${UI_TD} font-mono text-caption max-w-[14rem] truncate`} title={row.action}>
                      {row.action || '—'}
                    </td>
                    <td className={`${UI_TD} text-caption`}>
                      {row.actorUsername || '—'}
                      {row.actorUserId != null ? <span className="text-muted"> · uid {row.actorUserId}</span> : null}
                    </td>
                    <td className={`${UI_TD} text-caption max-w-[12rem] truncate`} title={row.resourceType || row.resource}>
                      {row.resourceType || row.resource || '—'}
                      {row.resourceId ? <span className="text-muted"> / {row.resourceId}</span> : null}
                    </td>
                    <td className={`${UI_TD} font-mono text-caption max-w-[8rem] truncate`} title={row.sessionId}>
                      {row.sessionId ? `${row.sessionId.slice(0, 8)}…` : '—'}
                    </td>
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
            pageSizeOptions={[30, 50, 100]}
            itemLabel="events"
          />
        </>
      )}
    </Card>
  );
};
