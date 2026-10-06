import React, { useCallback, useEffect, useState } from 'react';
import { Button } from '../../components/ui/Button';
import { api } from '../../lib/api';
import { formatDateTime } from '../../lib/display';
import type { ErrorLog } from '../../types';

const PAGE_SIZE = 25;
const LEVELS = ['', 'error', 'warn', 'info'] as const;

/** The full operational log, newest first, filterable by level and source. */
export const ErrorLogTable: React.FC = () => {
  const [level, setLevel] = useState<string>('');
  const [source, setSource] = useState('');
  const [appliedSource, setAppliedSource] = useState('');
  const [page, setPage] = useState(1);
  const [logs, setLogs] = useState<ErrorLog[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.getErrorLogs({ page, pageSize: PAGE_SIZE, level: level || undefined, source: appliedSource || undefined });
      setLogs(res.logs);
      setTotal(res.total);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [page, level, appliedSource]);

  useEffect(() => {
    void load();
  }, [load]);

  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <section id="operational-error-logs" className="rounded-lg border border-border bg-surface" aria-label="Operational log">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-4 py-3">
        <h2 className="text-body font-semibold text-text">Operational log</h2>
        <form
          className="flex flex-wrap items-center gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            setPage(1);
            setAppliedSource(source.trim());
          }}
        >
          <select
            aria-label="Level"
            value={level}
            onChange={(e) => {
              setPage(1);
              setLevel(e.target.value);
            }}
            className="rounded-md border border-border bg-base px-2 py-1.5 text-caption text-text"
          >
            {LEVELS.map((l) => (
              <option key={l || 'all'} value={l}>
                {l ? l[0].toUpperCase() + l.slice(1) : 'All levels'}
              </option>
            ))}
          </select>
          <input
            aria-label="Source"
            value={source}
            onChange={(e) => setSource(e.target.value)}
            placeholder="Source, e.g. core"
            className="w-36 rounded-md border border-border bg-base px-2 py-1.5 text-caption text-text outline-none focus:border-brand"
          />
          <Button size="sm" variant="secondary" type="submit">
            Filter
          </Button>
        </form>
      </div>
      {error ? (
        <p className="px-4 py-3 text-caption text-amber-200" role="alert">
          The log could not be loaded. {error}
        </p>
      ) : logs.length === 0 ? (
        <p className="px-4 py-3 text-caption text-muted">{loading ? 'Loading…' : 'No log entries match.'}</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[40rem] text-left text-caption">
            <thead className="text-meta uppercase tracking-wider text-muted">
              <tr className="border-b border-border">
                <th scope="col" className="px-4 py-2 font-semibold">Time</th>
                <th scope="col" className="px-4 py-2 font-semibold">Level</th>
                <th scope="col" className="px-4 py-2 font-semibold">Source</th>
                <th scope="col" className="px-4 py-2 font-semibold">Message</th>
              </tr>
            </thead>
            <tbody>
              {logs.map((l) => (
                <tr key={l.id} className="border-b border-border/60 align-top last:border-b-0">
                  <td className="whitespace-nowrap px-4 py-2 font-mono text-muted">{formatDateTime(l.time)}</td>
                  <td className={`px-4 py-2 font-semibold uppercase ${String(l.level).toLowerCase() === 'error' ? 'text-critical' : String(l.level).toLowerCase() === 'warn' ? 'text-amber-300' : 'text-muted'}`}>
                    {l.level}
                  </td>
                  <td className="px-4 py-2 font-mono text-muted">{l.source || 'core'}</td>
                  <td className="px-4 py-2 text-text">{l.message}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {total > PAGE_SIZE ? (
        <div className="flex items-center justify-between gap-3 border-t border-border px-4 py-2 text-caption text-muted">
          <span>
            Page {page} of {pages} · {total} entries
          </span>
          <span className="flex gap-2">
            <Button size="sm" variant="secondary" disabled={page <= 1 || loading} onClick={() => setPage((p) => p - 1)}>
              Newer
            </Button>
            <Button size="sm" variant="secondary" disabled={page >= pages || loading} onClick={() => setPage((p) => p + 1)}>
              Older
            </Button>
          </span>
        </div>
      ) : null}
    </section>
  );
};
