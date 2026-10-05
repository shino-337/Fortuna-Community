import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { BarChart3, Download, FileText, RefreshCw } from 'lucide-react';
import { api } from '../../lib/api';
import type { Report } from '../../types';
import { Card } from '../../design-system/components/Card';
import { PageEmpty, PageError } from '../../design-system/components/PageStatus';
import { Button } from '../ui/Button';
import { downloadText, toCsv } from '../../lib/download';
import { UI_TABLE, UI_TD, UI_TH, UI_TR, UI_THEAD_STICKY } from '../../lib/tableChrome';

const RANGE_OPTIONS = [1, 3, 7, 30] as const;
type RangeDays = (typeof RANGE_OPTIONS)[number];

/** Resource/action counts from audit_logs (GET /audit/reports) for a time window. Requires system.audit.read. */
export const AuditAggregatesPanel: React.FC = () => {
  const [rangeDays, setRangeDays] = useState<RangeDays>(1);
  const [reports, setReports] = useState<Report[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const requestRef = useRef(0);

  const load = useCallback(() => {
    // A slower response for a previous range must not overwrite the current range.
    const seq = ++requestRef.current;
    setLoading(true);
    setError(null);
    api
      .getReportsStrict({ hours: rangeDays * 24 })
      .then((data) => {
        if (seq !== requestRef.current) return;
        setReports(data);
      })
      .catch((err) => {
        if (seq !== requestRef.current) return;
        setError(err instanceof Error ? err.message : 'Could not load audit aggregates.');
        setReports([]);
      })
      .finally(() => {
        if (seq === requestRef.current) setLoading(false);
      });
  }, [rangeDays]);

  useEffect(() => {
    load();
  }, [load]);

  const rangeLabel = `${rangeDays} day${rangeDays === 1 ? '' : 's'}`;
  const total = useMemo(() => reports.reduce((sum, r) => sum + Number(r.count ?? 0), 0), [reports]);
  const byResource = useMemo(() => topCounts(reports, (r) => r.resource), [reports]);
  const byAction = useMemo(() => topCounts(reports, (r) => r.action), [reports]);

  const exportCsv = () => {
    const content = toCsv([
      ['resource', 'action', 'count'],
      ...reports.map((r) => [r.resource || '', r.action || '', String(r.count ?? 0)]),
    ]);
    downloadText(content, `audit-aggregates-${rangeDays}d-${new Date().toISOString().replace(/[:.]/g, '-')}.csv`, 'text/csv;charset=utf-8');
  };

  return (
    <div className="grid gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-caption text-muted">
          {loading ? 'Loading…' : `${total.toLocaleString('en-US')} audited action(s) in the last ${rangeLabel}.`}
        </p>
        <div className="flex flex-wrap items-center gap-2">
          <div className="inline-flex rounded-lg border border-border bg-base/60 p-1" role="group" aria-label="Audit window">
            {RANGE_OPTIONS.map((days) => (
              <button
                key={days}
                type="button"
                onClick={() => setRangeDays(days)}
                aria-pressed={rangeDays === days}
                className={`rounded-md px-3 py-1.5 text-caption font-semibold transition-colors ${
                  rangeDays === days ? 'bg-surface-2 text-text shadow-sm' : 'text-muted hover:text-text'
                }`}
              >
                {days} day{days === 1 ? '' : 's'}
              </button>
            ))}
          </div>
          <Button variant="secondary" size="sm" type="button" onClick={load} disabled={loading}>
            <RefreshCw className="mr-1.5 h-3.5 w-3.5" /> Refresh
          </Button>
          <Button variant="secondary" size="sm" type="button" onClick={exportCsv} disabled={loading || reports.length === 0}>
            <Download className="mr-1.5 h-3.5 w-3.5" /> CSV
          </Button>
        </div>
      </div>

      {error ? (
        <PageError title="Could not load audit aggregates" description={error} className="py-6" action={<Button variant="secondary" size="sm" onClick={load}>Retry</Button>} />
      ) : !loading && reports.length === 0 ? (
        <PageEmpty title="No audit activity" description={`No audited actions in the last ${rangeLabel}.`} className="py-6" />
      ) : (
        <div className="grid gap-4 xl:grid-cols-[minmax(0,0.45fr)_minmax(0,0.55fr)]">
          <Card className="p-4">
            <div className="grid gap-4 md:grid-cols-2">
              <BarList title="Top resources" items={byResource} />
              <BarList title="Top actions" items={byAction} />
            </div>
          </Card>
          <Card className="p-0 overflow-hidden">
            <div className="flex items-center justify-between gap-3 border-b border-border px-4 py-3">
              <h3 className="fortuna-card-title">Resource / action counts</h3>
              <BarChart3 className="h-4 w-4 text-muted" />
            </div>
            <div className="ui-table-scroll max-h-[60vh]">
              <table className={UI_TABLE}>
                <thead className={UI_THEAD_STICKY}>
                  <tr>
                    <th className={UI_TH}>Resource</th>
                    <th className={UI_TH}>Action</th>
                    <th className={`${UI_TH} text-right`}>Count</th>
                  </tr>
                </thead>
                <tbody>
                  {reports.map((report) => (
                    <tr key={report.id} className={UI_TR}>
                      <td className={UI_TD}>
                        <div className="flex items-center font-medium text-text">
                          <FileText className="w-4 h-4 mr-3 text-muted shrink-0" />
                          {report.resource || 'unknown'}
                        </div>
                      </td>
                      <td className={`${UI_TD} text-muted`}>{report.action || 'unknown'}</td>
                      <td className={`${UI_TD} text-right text-text font-mono tabular-nums`}>{report.count ?? 0}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Card>
        </div>
      )}
    </div>
  );
};

function topCounts(reports: Report[], key: (r: Report) => string | undefined): Array<[string, number]> {
  const map = new Map<string, number>();
  for (const report of reports) {
    const k = (key(report) || 'unknown').trim() || 'unknown';
    map.set(k, (map.get(k) ?? 0) + Number(report.count ?? 0));
  }
  return [...map.entries()].sort((a, b) => b[1] - a[1]).slice(0, 6);
}

const BarList: React.FC<{ title: string; items: Array<[string, number]> }> = ({ title, items }) => {
  const max = Math.max(1, ...items.map(([, count]) => count));
  return (
    <div>
      <h3 className="mb-2 text-caption font-semibold text-text">{title}</h3>
      <div className="space-y-2">
        {items.map(([label, count]) => (
          <div key={label} className="grid gap-1">
            <div className="flex items-center justify-between gap-2 text-caption">
              <span className="min-w-0 truncate text-muted" title={label}>{label}</span>
              <span className="font-mono tabular-nums text-text">{count}</span>
            </div>
            <div className="h-2 overflow-hidden rounded-full bg-surface-2">
              <div className="h-full rounded-full bg-brand" style={{ width: `${Math.max(4, Math.round((count / max) * 100))}%` }} />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};
