import React, { useMemo, useState } from 'react';
import clsx from 'clsx';
import type { AttackPath } from '../../types';
import { scenarioEnds, scenarioMaxRisk, type GroupedScenario } from '../../lib/attackPathNarrative';
import { pathRiskLevel, type SeverityLevel } from '../../lib/severity';
import { TARGET_GROUP_LABEL, type TargetGroup } from '../../lib/attackPathSteps';
import { scenarioTargetGroup } from './AttackPathWorkspace';

/** Short name for how a scenario moves, from its chain type. */
export function scenarioMove(s: GroupedScenario): string {
  const t = `${s.type} ${s.objective}`.toUpperCase();
  if (t.includes('ESCAPE') || t.includes('NODE')) return 'Container escape';
  if (t.includes('LATERAL') || t.includes('NETWORK')) return 'Lateral movement';
  if (t.includes('TOKEN') || t.includes('SERVICE_ACCOUNT')) return 'Token reuse';
  if (t.includes('DATA') || t.includes('SECRET') || t.includes('EXFIL')) return 'Data access';
  if (t.includes('RBAC') || t.includes('PRIV') || t.includes('ESCALAT') || t.includes('TAKEOVER')) return 'RBAC escalation';
  return 'Other';
}

const LEVEL_RANK: Record<SeverityLevel, number> = { low: 0, medium: 1, high: 2, critical: 3 };
const LEVEL_FILL: Record<SeverityLevel, string> = {
  critical: 'fill-red-500',
  high: 'fill-orange-500',
  medium: 'fill-yellow-400',
  low: 'fill-slate-400',
};

type Row = { ns: string; move: string; target: TargetGroup; level: SeverityLevel | null };
type Col = 0 | 1 | 2;
type SNode = { col: Col; key: string; label: string; count: number; y: number; h: number; outY: number; inY: number };
type SLink = { from: SNode; to: SNode; count: number; level: SeverityLevel | null };

const W = 960;
const H = 360;
const NODE_W = 12;
const GAP = 10;
const COL_X = [0, W / 2 - NODE_W / 2, W - NODE_W];

function maxLevel(a: SeverityLevel | null, b: SeverityLevel | null): SeverityLevel | null {
  if (!a) return b;
  if (!b) return a;
  return LEVEL_RANK[a] >= LEVEL_RANK[b] ? a : b;
}

function buildSankey(rows: Row[]) {
  const nodes = new Map<string, SNode>();
  const add = (col: Col, label: string) => {
    const key = `${col}:${label}`;
    const n = nodes.get(key) ?? { col, key, label, count: 0, y: 0, h: 0, outY: 0, inY: 0 };
    n.count += 1;
    nodes.set(key, n);
    return n;
  };
  const linkMap = new Map<string, SLink>();
  const link = (from: SNode, to: SNode, level: SeverityLevel | null) => {
    const key = `${from.key}->${to.key}`;
    const l = linkMap.get(key) ?? { from, to, count: 0, level: null };
    l.count += 1;
    l.level = maxLevel(l.level, level);
    linkMap.set(key, l);
  };
  for (const r of rows) {
    const a = add(0, r.ns);
    const b = add(1, r.move);
    const c = add(2, TARGET_GROUP_LABEL[r.target]);
    link(a, b, r.level);
    link(b, c, r.level);
  }
  const total = rows.length || 1;
  for (const col of [0, 1, 2] as Col[]) {
    const list = [...nodes.values()].filter((n) => n.col === col).sort((a, b) => b.count - a.count);
    const unit = (H - GAP * Math.max(0, list.length - 1)) / total;
    let y = 0;
    for (const n of list) {
      n.y = y;
      n.h = Math.max(4, n.count * unit);
      n.outY = n.y;
      n.inY = n.y;
      y += n.h + GAP;
    }
  }
  const links = [...linkMap.values()].sort((a, b) => a.from.y - b.from.y || a.to.y - b.to.y);
  const unitOf = (n: SNode) => n.h / n.count;
  const bands = links.map((l) => {
    const h0 = l.count * unitOf(l.from);
    const h1 = l.count * unitOf(l.to);
    const y0 = l.from.outY;
    const y1 = l.to.inY;
    l.from.outY += h0;
    l.to.inY += h1;
    return { l, y0, h0, y1, h1 };
  });
  return { nodes: [...nodes.values()], bands };
}

/**
 * Every scenario in one picture: where it starts (namespace), how it moves, what it reaches.
 * Band width = scenarios; colour = the highest level among them. Choosing a start or a target filters the list.
 */
export const AttackPathOverview: React.FC<{
  scenarios: GroupedScenario[];
  pathById: Map<string, AttackPath>;
  onNamespace: (ns: string) => void;
  onTarget: (group: TargetGroup) => void;
  fullGraph: React.ReactNode;
}> = ({ scenarios, pathById, onNamespace, onTarget, fullGraph }) => {
  const [hover, setHover] = useState<string | null>(null);
  const [showGraph, setShowGraph] = useState(false);
  const rows: Row[] = useMemo(
    () =>
      scenarios.map((s) => {
        const { entry } = scenarioEnds(s, pathById);
        const risk = scenarioMaxRisk(s, pathById);
        return {
          ns: String(entry?.properties?.namespace || '') || '(no namespace)',
          move: scenarioMove(s),
          target: scenarioTargetGroup(s, pathById),
          level: risk === null ? null : pathRiskLevel(risk),
        };
      }),
    [scenarios, pathById],
  );
  const { nodes, bands } = useMemo(() => buildSankey(rows), [rows]);
  const targetByLabel = new Map(Object.entries(TARGET_GROUP_LABEL).map(([k, v]) => [v, k as TargetGroup]));
  const table = useMemo(() => {
    const m = new Map<string, Row & { n: number }>();
    for (const r of rows) {
      const k = `${r.ns}|${r.move}|${r.target}`;
      const e = m.get(k) ?? { ...r, n: 0 };
      e.n += 1;
      e.level = maxLevel(e.level, r.level);
      m.set(k, e);
    }
    return [...m.values()].sort((a, b) => b.n - a.n);
  }, [rows]);

  if (rows.length === 0) return null;

  const pick = (n: SNode) => {
    if (n.col === 0) onNamespace(n.label === '(no namespace)' ? '' : n.label);
    if (n.col === 2) {
      const g = targetByLabel.get(n.label);
      if (g) onTarget(g);
    }
  };

  return (
    <section aria-labelledby="overview-title" className="flex flex-col gap-3 rounded-xl border border-border bg-surface/45 p-4">
      <div className="flex flex-col gap-1">
        <h2 id="overview-title" className="text-body font-semibold text-text">Where paths start, how they move, what they reach</h2>
        <p className="text-caption text-muted">
          {rows.length} {rows.length === 1 ? 'scenario' : 'scenarios'}. Band width = scenarios, colour = highest level among them. Choose a namespace or a target to list its paths.
        </p>
      </div>
      <div className="hidden grid-cols-3 text-meta font-semibold uppercase tracking-wider text-muted sm:grid">
        <span>Entry namespace</span>
        <span className="text-center">Move</span>
        <span className="text-right">Reaches</span>
      </div>
      <svg viewBox={`-4 -4 ${W + 8} ${H + 8}`} className="hidden h-auto w-full sm:block" role="img" aria-label="Attack paths from entry namespace to target">
        {bands.map(({ l, y0, h0, y1, h1 }) => {
          const x0 = COL_X[l.from.col] + NODE_W;
          const x1 = COL_X[l.to.col];
          const mx = (x0 + x1) / 2;
          const id = `${l.from.key}->${l.to.key}`;
          const lit = hover === null || hover === l.from.key || hover === l.to.key || hover === id;
          return (
            <path
              key={id}
              d={`M ${x0} ${y0} C ${mx} ${y0}, ${mx} ${y1}, ${x1} ${y1} L ${x1} ${y1 + h1} C ${mx} ${y1 + h1}, ${mx} ${y0 + h0}, ${x0} ${y0 + h0} Z`}
              className={clsx(l.level ? LEVEL_FILL[l.level] : 'fill-slate-400', lit ? 'opacity-40' : 'opacity-10', 'transition-opacity')}
              onMouseEnter={() => setHover(id)}
              onMouseLeave={() => setHover(null)}
            >
              <title>{`${l.from.label} → ${l.to.label}: ${l.count} ${l.count === 1 ? 'scenario' : 'scenarios'}`}</title>
            </path>
          );
        })}
        {nodes.map((n) => {
          const clickable = n.col !== 1;
          const labelX = n.col === 2 ? COL_X[2] - 8 : COL_X[n.col] + NODE_W + 8;
          return (
            <g
              key={n.key}
              className={clsx(clickable && 'cursor-pointer')}
              onMouseEnter={() => setHover(n.key)}
              onMouseLeave={() => setHover(null)}
              onClick={() => clickable && pick(n)}
              role={clickable ? 'button' : undefined}
              tabIndex={clickable ? 0 : undefined}
              onKeyDown={(e) => {
                if (clickable && (e.key === 'Enter' || e.key === ' ')) {
                  e.preventDefault();
                  pick(n);
                }
              }}
              aria-label={clickable ? `${n.label}: ${n.count} scenarios, show them` : undefined}
            >
              <rect x={COL_X[n.col]} y={n.y} width={NODE_W} height={n.h} rx={3} className="fill-text" />
              <text
                x={labelX}
                y={n.y + n.h / 2}
                dominantBaseline="middle"
                textAnchor={n.col === 2 ? 'end' : 'start'}
                className={clsx('fill-text text-[13px]', clickable && 'underline-offset-2 hover:underline')}
              >
                {n.label}
                <tspan className="fill-muted"> {n.count}</tspan>
              </text>
            </g>
          );
        })}
      </svg>
      <details className="rounded-lg border border-border sm:open:block" open={false}>
        <summary className="cursor-pointer select-none px-3 py-2 text-caption font-semibold text-muted hover:text-text">Table view</summary>
        <div className="overflow-x-auto border-t border-border">
          <table className="w-full text-left text-caption">
            <thead className="text-muted">
              <tr>
                <th className="px-3 py-1.5 font-semibold">Entry namespace</th>
                <th className="px-3 py-1.5 font-semibold">Move</th>
                <th className="px-3 py-1.5 font-semibold">Reaches</th>
                <th className="px-3 py-1.5 font-semibold">Highest level</th>
                <th className="px-3 py-1.5 text-right font-semibold">Scenarios</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border/60 text-text">
              {table.map((r) => (
                <tr key={`${r.ns}|${r.move}|${r.target}`}>
                  <td className="px-3 py-1.5">
                    <button type="button" className="hover:text-brand hover:underline" onClick={() => onNamespace(r.ns === '(no namespace)' ? '' : r.ns)}>
                      {r.ns}
                    </button>
                  </td>
                  <td className="px-3 py-1.5">{r.move}</td>
                  <td className="px-3 py-1.5">
                    <button type="button" className="hover:text-brand hover:underline" onClick={() => onTarget(r.target)}>
                      {TARGET_GROUP_LABEL[r.target]}
                    </button>
                  </td>
                  <td className="px-3 py-1.5 capitalize">{r.level ?? '—'}</td>
                  <td className="px-3 py-1.5 text-right tabular-nums">{r.n}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </details>
      <div className="hidden flex-col gap-2 sm:flex">
        <button type="button" onClick={() => setShowGraph((v) => !v)} className="w-fit text-caption font-semibold text-brand hover:underline">
          {showGraph ? 'Hide the full graph' : 'Show every path as one graph (advanced)'}
        </button>
        {showGraph ? fullGraph : null}
      </div>
    </section>
  );
};
