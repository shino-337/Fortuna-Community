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
import { Badge } from '../../design-system/components/Badge';
import { SidePanel } from '../SidePanel';
import { When } from '../When';
import { UI_FILTER_SELECT } from '../../lib/formChrome';
import { UI_TABLE, UI_TD, UI_TH, UI_TR, UI_THEAD_STICKY } from '../../lib/tableChrome';

const SEVERITIES = ['critical', 'high', 'medium', 'low'] as const;
const RESULTS = ['success', 'deny', 'error'] as const;
/** Event domains the server tags activity with (the former Investigation timeline filtered by these). */
const DOMAINS = ['auth', 'sessions', 'rbac', 'findings', 'graph', 'export'] as const;

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
  const [domain, setDomain] = useState('');
  const [action, setAction] = useState('');
  const [debouncedAction, setDebouncedAction] = useState('');
  const [selected, setSelected] = useState<SecurityActivityItem | null>(null);
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
        domain: domain || undefined,
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
  }, [page, pageSize, severity, result, debouncedAction, domain]);

  useEffect(() => {
    void fetchPage();
  }, [fetchPage]);

  const filtersActive = severity !== '' || result !== '' || domain !== '' || action.trim() !== '';
  const resetFilters = () => {
    setSeverity('');
    setResult('');
    setDomain('');
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
                aria-label="Domain"
                className={UI_FILTER_SELECT}
                value={domain}
                onChange={(e) => {
                  setDomain(e.target.value);
                  setPage(1);
                }}
              >
                <option value="">Any domain</option>
                {DOMAINS.map((d) => (
                  <option key={d} value={d}>
                    {d}
                  </option>
                ))}
              </select>
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
                    {r === 'deny' ? 'denied' : r}
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
                  <th className={UI_TH}>When</th>
                  <th className={UI_TH}>Actor</th>
                  <th className={UI_TH}>Action</th>
                  <th className={UI_TH}>Resource</th>
                  <th className={UI_TH}>Result</th>
                  <th className={UI_TH}>Severity</th>
                </tr>
              </thead>
              <tbody>
                {items.map((row) => (
                  <tr
                    key={`${row.id ?? row.eventId ?? row.createdAt}-${row.action}`}
                    className={`${UI_TR} cursor-pointer ${selected === row ? 'bg-brand/10' : ''}`}
                    tabIndex={0}
                    onClick={() => setSelected(row)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        setSelected(row);
                      }
                    }}
                  >
                    <td className={`${UI_TD} text-caption text-muted`}>
                      <When iso={row.createdAt} />
                    </td>
                    <td className={`${UI_TD} text-caption`}>
                      <div className="text-text">{row.actorUsername || 'system'}</div>
                      {row.actorRole ? <div className="text-meta text-muted">{row.actorRole}</div> : null}
                    </td>
                    <td className={`${UI_TD} font-mono text-caption max-w-[16rem] truncate`} title={row.action}>
                      {row.action || '—'}
                    </td>
                    <td className={`${UI_TD} text-caption max-w-[14rem] truncate`} title={row.resourceType || row.resource}>
                      {row.resourceType || row.resource || '—'}
                      {row.resourceId ? <span className="text-muted"> {row.resourceId}</span> : null}
                    </td>
                    <td className={UI_TD}>
                      <ResultBadge result={row.result} />
                    </td>
                    <td className={UI_TD}>
                      {row.severity ? (
                        <Badge severity={row.severity.toLowerCase()} uppercase={false} showShape={false} className="capitalize">
                          {row.severity}
                        </Badge>
                      ) : (
                        <span className="text-muted">—</span>
                      )}
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
      <SidePanel
        open={selected != null}
        onClose={() => setSelected(null)}
        title={<span className="font-mono">{selected?.action || 'Event'}</span>}
        subtitle={selected?.createdAt ? formatDateTime(selected.createdAt) : undefined}
      >
        {selected ? <EventDetail row={selected} /> : null}
      </SidePanel>
    </Card>
  );
};

const RESULT_CLASS: Record<string, string> = {
  success: 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20',
  deny: 'text-red-300 bg-red-500/10 border-red-500/30',
  error: 'text-amber-300 bg-amber-500/10 border-amber-500/30',
};

const ResultBadge: React.FC<{ result?: string }> = ({ result }) => {
  if (!result) return <span className="text-muted">—</span>;
  const key = result.toLowerCase();
  return (
    <span className={`px-2 py-0.5 rounded border text-caption font-medium ${RESULT_CLASS[key] ?? 'text-muted bg-surface-2 border-border'}`}>
      {key === 'deny' ? 'denied' : key}
    </span>
  );
};

function prettyJson(raw?: string): string | null {
  if (!raw) return null;
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

/** Everything the event recorded: who, from where, what changed. */
const EventDetail: React.FC<{ row: SecurityActivityItem }> = ({ row }) => {
  const facts: Array<[string, React.ReactNode]> = [
    ['Actor', [row.actorUsername || 'system', row.actorRole].filter(Boolean).join(' · ')],
    ['Result', <ResultBadge key="r" result={row.result} />],
    ['Resource', [row.resourceType || row.resource, row.resourceId].filter(Boolean).join(' ') || '—'],
    ['Target user', row.targetUserId != null ? String(row.targetUserId) : null],
    ['Source IP', row.sourceIp],
    ['Auth method', row.authMethod],
    ['User agent', row.userAgent],
    ['Session', row.sessionId],
    ['Request', row.requestId],
    ['Correlation', row.correlationId],
  ];
  const blocks: Array<[string, string | null]> = [
    ['Before', prettyJson(row.beforeStateJson)],
    ['After', prettyJson(row.afterStateJson)],
    ['Details', prettyJson(row.detailsJson)],
  ];
  return (
    <div className="space-y-5">
      <dl className="grid grid-cols-[auto,1fr] gap-x-4 gap-y-2 text-body">
        {facts
          .filter(([, v]) => v != null && v !== '')
          .map(([k, v]) => (
            <React.Fragment key={k}>
              <dt className="text-muted">{k}</dt>
              <dd className="text-text break-all">{v}</dd>
            </React.Fragment>
          ))}
      </dl>
      {blocks
        .filter(([, v]) => v)
        .map(([k, v]) => (
          <div key={k}>
            <div className="text-caption uppercase tracking-wide text-muted mb-1">{k}</div>
            <pre className="ui-code-scroll rounded border border-border bg-base/70 p-3 text-caption text-text">{v}</pre>
          </div>
        ))}
    </div>
  );
};
