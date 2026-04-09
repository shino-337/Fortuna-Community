import React, { useMemo } from 'react';
import type { PodNetworkConnectionItem, PodNetworkTopDestinationItem } from '../types';
import { formatDateTime } from '../lib/display';

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
  if (src === ip || src === '0.0.0.0' || src === '::') return 'outbound';
  return 'inbound';
}

/* ──────────────── component ──────────────── */

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
    return <p className="text-slate-500 text-sm py-4">Loading summary…</p>;
  }

  if (connections.length === 0 && topDestinations.length === 0) {
    return null; // nothing to show — parent will render empty state
  }

  return (
    <div className="space-y-4">
      {/* ── KPI cards ── */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <Kpi label="Total connections" value={connections.length} />
        <Kpi label="Unique remotes" value={uniqueRemotes} />
        <Kpi label="Outbound" value={directions.outbound} accent="sky" />
        <Kpi label="Inbound" value={directions.inbound} accent="pink" />
      </div>

      {/* ── Direction + Protocol mini-bars ── */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        {/* Direction bar */}
        <div className="bg-slate-900/40 border border-slate-800 rounded-lg p-3">
          <p className="text-[11px] text-slate-500 mb-2 uppercase tracking-wider">Direction breakdown</p>
          <MiniBar
            items={[
              { label: 'Outbound', value: directions.outbound, color: '#38bdf8' },
              { label: 'Inbound', value: directions.inbound, color: '#f472b6' },
              { label: 'Listen', value: directions.listen, color: '#64748b' },
            ]}
          />
        </div>

        {/* Protocol bar */}
        <div className="bg-slate-900/40 border border-slate-800 rounded-lg p-3">
          <p className="text-[11px] text-slate-500 mb-2 uppercase tracking-wider">Protocol breakdown</p>
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
        <div>
          <p className="text-xs text-slate-500 mb-2 uppercase tracking-wider">
            Top destinations (aggregated, 24 h)
          </p>
          <div className="rounded-lg border border-slate-800 bg-slate-950/30 overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-800 bg-slate-900/95 text-left text-xs uppercase tracking-wide text-slate-500">
                  <th className="px-3 py-2">Remote</th>
                  <th className="px-3 py-2">Proto</th>
                  <th className="px-3 py-2 text-right">Observations</th>
                  <th className="px-3 py-2 text-right">Buckets</th>
                  <th className="px-3 py-2">Last seen</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800">
                {topDestinations.map((d, idx) => (
                  <tr key={`${d.destIp}:${d.destPort}:${d.protocol}-${idx}`} className="hover:bg-muted/30">
                    <td className="px-3 py-2 font-mono text-xs text-slate-300 whitespace-nowrap">
                      {d.destIp}:{d.destPort}
                    </td>
                    <td className="px-3 py-2 text-xs">{d.protocol ?? '—'}</td>
                    <td className="px-3 py-2 text-right tabular-nums text-slate-200">{d.observationCount}</td>
                    <td className="px-3 py-2 text-right tabular-nums text-slate-400">
                      {d.distinctBucketCount ?? '—'}
                    </td>
                    <td className="px-3 py-2 text-xs text-slate-500 whitespace-nowrap">
                      {d.lastObservedAt ? formatDateTime(d.lastObservedAt) : '—'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
};

/* ──────────────── sub-components ──────────────── */

const PROTO_COLORS = ['#38bdf8', '#f472b6', '#a78bfa', '#34d399', '#fbbf24', '#fb923c'];

function Kpi({ label, value, accent }: { label: string; value: number; accent?: 'sky' | 'pink' }) {
  const textColor =
    accent === 'sky' ? 'text-sky-400' : accent === 'pink' ? 'text-pink-400' : 'text-white';
  return (
    <div className="bg-slate-900/40 border border-slate-800 rounded-lg px-3 py-2">
      <p className="text-[11px] text-slate-500 mb-0.5">{label}</p>
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
    return <p className="text-xs text-slate-600">No data</p>;
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
      <div className="flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-slate-400">
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
