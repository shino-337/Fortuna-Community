import React, { useMemo } from 'react';
import type { PodNetworkConnectionItem, PodNetworkTopDestinationItem } from '../types';
import { formatDateTime } from '../lib/display';
import { UI_TABLE, UI_THEAD_STICKY, UI_TH_COMPACT, UI_TR, UI_TD_COMPACT_TIGHT } from '../lib/tableChrome';
import { GRAPH_THEME } from '../lib/graphTheme';

/* ──────────────── types ──────────────── */

export interface PodNetworkSummaryProps {
  connections: PodNetworkConnectionItem[];
  topDestinations: PodNetworkTopDestinationItem[];
  podIP?: string;
  loading?: boolean;
}

interface ProtocolBreakdown {
  protocol: string;
  count: number;
}

interface DirectionBreakdown {
  inbound: number;
  outbound: number;
  listen: number;
}

/* ──────────────── helpers ──────────────── */

function computeDirection(
  conn: PodNetworkConnectionItem,
  podIP: string | undefined,
): 'inbound' | 'outbound' | 'listen' {
  const st = (conn.state ?? '').toUpperCase();
  if (st === 'LISTEN' || st.includes('LISTEN')) return 'listen';
  const ip = (podIP ?? '').trim();
  if (!ip) return 'outbound';
  const src = (conn.sourceIp ?? '').trim();
  const dst = (conn.destIp ?? '').trim();
  const sourcePort = Number(conn.sourcePort ?? 0);
  const destPort = Number(conn.destPort ?? 0);
  if (src === '0.0.0.0' || src === '::') return 'listen';
  if (src === ip && sourcePort > 0 && sourcePort < 32768 && destPort >= 32768) {
    return 'inbound';
  }
  if (dst === ip) return 'inbound';
  if (src === ip) return 'outbound';
  return 'inbound';
}

/* ──────────────── constants ──────────────── */

const PROTO_COLORS = [GRAPH_THEME.info, GRAPH_THEME.brand, GRAPH_THEME.indigo, GRAPH_THEME.success, GRAPH_THEME.warning, GRAPH_THEME.high];

/* ──────────────── component ──────────────── */

/** Summary panel for a pod's network activity — KPIs, direction/protocol breakdowns, and top destinations. */
export const PodNetworkSummary: React.FC<PodNetworkSummaryProps> = ({
  connections,
  topDestinations,
  podIP,
  loading,
}) => {
  const directions = useMemo<DirectionBreakdown>(() => {
    const r: DirectionBreakdown = { inbound: 0, outbound: 0, listen: 0 };
    for (const c of connections) {
      const d = computeDirection(c, podIP);
      r[d] += 1;
    }
    return r;
  }, [connections, podIP]);

  const protocols = useMemo<ProtocolBreakdown[]>(() => {
    const m = new Map<string, number>();
    for (const c of connections) {
      const p = (c.protocol ?? 'unknown').toLowerCase();
      m.set(p, (m.get(p) ?? 0) + 1);
    }
    return Array.from(m, ([protocol, count]) => ({ protocol, count })).sort(
      (a, b) => b.count - a.count,
    );
  }, [connections]);

  const uniqueRemotes = useMemo(() => {
    const s = new Set<string>();
    for (const c of connections) {
      const dir = computeDirection(c, podIP);
      if (dir === 'listen') continue;
      const ip = dir === 'outbound' ? c.destIp : c.sourceIp;
      if (ip) s.add(ip);
    }
    return s.size;
  }, [connections, podIP]);

  if (loading) {
    return <p className="text-muted-2 text-body py-4">Loading summary…</p>;
  }

  if (connections.length === 0 && topDestinations.length === 0) {
    return null; // nothing to show — parent will render empty state
  }

  return (
    <div className="space-y-4 min-w-0">
      {/* ── KPI cards ── */}
      <div className="grid min-w-0 grid-cols-1 gap-3 min-[420px]:grid-cols-2 lg:grid-cols-4">
        <Kpi label="Observed sockets" value={connections.length} />
        <Kpi label="Unique remotes" value={uniqueRemotes} />
        <Kpi label="Outbound" value={directions.outbound} accent="sky" />
        <Kpi label="Inbound" value={directions.inbound} accent="brand" />
      </div>

      {/* ── Direction + Protocol mini-bars ── */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        {/* Direction bar */}
        <div className="bg-surface/40 border border-border rounded-lg p-3">
          <p className="text-caption text-muted-2 mb-2 uppercase tracking-wider">Direction breakdown</p>
          <MiniBar
            items={[
              { label: 'Outbound', value: directions.outbound, color: GRAPH_THEME.info },
              { label: 'Inbound', value: directions.inbound, color: GRAPH_THEME.brand },
              { label: 'Listen', value: directions.listen, color: GRAPH_THEME.muted2 },
            ]}
          />
        </div>

        {/* Protocol bar */}
        <div className="bg-surface/40 border border-border rounded-lg p-3">
          <p className="text-caption text-muted-2 mb-2 uppercase tracking-wider">Protocol breakdown</p>
          <MiniBar
            items={protocols.map((p, i) => ({
              label: p.protocol,
              value: p.count,
              color: PROTO_COLORS[i % PROTO_COLORS.length],
            }))}
          />
        </div>
      </div>

      {/* ── Top destinations ── */}
      {topDestinations.length > 0 && (
        <div className="min-w-0">
          <p className="text-caption text-muted-2 mb-2 uppercase tracking-wider">
            Top remote endpoints (aggregated, 24 h)
          </p>
          <div className="rounded-lg border border-border bg-base/30 overflow-hidden -mx-1 sm:mx-0">
            <div className="ui-table-scroll max-h-[min(50dvh,28rem)]">
              <table className={`${UI_TABLE} min-w-[640px]`}>
                <thead className={UI_THEAD_STICKY}>
                  <tr>
                    <th className={UI_TH_COMPACT}>Remote</th>
                    <th className={UI_TH_COMPACT}>Proto</th>
                    <th className={`${UI_TH_COMPACT} text-right`}>Observations</th>
                    <th className={`${UI_TH_COMPACT} text-right`}>Buckets</th>
                    <th className={UI_TH_COMPACT}>Last seen</th>
                  </tr>
                </thead>
                <tbody>
                  {topDestinations.map((d, idx) => {
                    const remote = `${d.destIp}:${d.destPort}`;
                    return (
                      <tr
                        key={`${d.destIp}:${d.destPort}:${d.protocol}-${idx}`}
                        className={UI_TR}
                      >
                        <td
                          className={`${UI_TD_COMPACT_TIGHT} font-mono max-w-[14rem] sm:max-w-[18rem] truncate align-middle`}
                          title={remote}
                        >
                          {remote}
                        </td>
                        <td className={`${UI_TD_COMPACT_TIGHT} align-middle whitespace-nowrap`}>
                          {d.protocol ?? '—'}
                        </td>
                        <td className={`${UI_TD_COMPACT_TIGHT} text-right tabular-nums align-middle`}>
                          {d.observationCount}
                        </td>
                        <td className={`${UI_TD_COMPACT_TIGHT} text-right tabular-nums text-muted align-middle`}>
                          {d.distinctBucketCount ?? '—'}
                        </td>
                        <td className={`${UI_TD_COMPACT_TIGHT} text-muted-2 whitespace-nowrap align-middle`}>
                          {d.lastObservedAt ? formatDateTime(d.lastObservedAt) : '—'}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

/* ──────────────── sub-components ──────────────── */

function Kpi({ label, value, accent }: { label: string; value: number; accent?: 'sky' | 'brand' }) {
  const textColor =
    accent === 'sky' ? 'text-sky-400' : accent === 'brand' ? 'text-brand' : 'text-text';
  return (
    <div className="bg-surface/40 border border-border rounded-lg px-3 py-2">
      <p className="text-caption text-muted-2 mb-0.5">{label}</p>
      <p className={`text-lg font-semibold tabular-nums ${textColor}`}>{value}</p>
    </div>
  );
}

interface MiniBarItem {
  label: string;
  value: number;
  color: string;
}

function MiniBar({ items }: { items: MiniBarItem[] }) {
  const total = items.reduce((s, i) => s + i.value, 0);
  if (total === 0) {
    return <p className="text-caption text-muted-2">No data</p>;
  }
  return (
    <div>
      {/* Stacked bar */}
      <div className="flex h-3 rounded-full overflow-hidden mb-2">
        {items.map(
          (it) =>
            it.value > 0 && (
              <div
                key={it.label}
                className="h-full"
                style={{
                  width: `${(it.value / total) * 100}%`,
                  backgroundColor: it.color,
                  minWidth: 2,
                }}
                title={`${it.label}: ${it.value}`}
              />
            ),
        )}
      </div>
      {/* Labels */}
      <div className="flex flex-wrap gap-x-3 gap-y-1 text-caption text-muted">
        {items.map(
          (it) =>
            it.value > 0 && (
              <span key={it.label} className="flex items-center gap-1">
                <span
                  className="inline-block w-2 h-2 rounded-full"
                  style={{ backgroundColor: it.color }}
                />
                {it.label}: {it.value}
              </span>
            ),
        )}
      </div>
    </div>
  );
}
