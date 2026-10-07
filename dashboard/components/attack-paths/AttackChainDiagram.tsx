import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import clsx from 'clsx';
import type { AttackPath, AttackPathNode, AttackStep } from '../../types';
import { NODE_KIND_LABEL, nodeDisplayName, nodeKind, techniqueStepIndex } from '../../lib/attackPathSteps';

const CARD_W_MAX = 184;
const CARD_W_MIN = 120;
const CARD_H = 58;
const COL_GAP_MAX = 56;
const COL_GAP_MIN = 32;
const ROW_H = 78;
const HEAD_H = 22;

type LaidNode = { node: AttackPathNode; col: number; y: number; entry: boolean; target: boolean };
type LaidEdge = { key: string; from: LaidNode; to: LaidNode; step?: number; observed: boolean };

/**
 * Lays the paths out in columns from entry (left) to target (right): every hop sits one column after the
 * hop before it, shared objects are drawn once, and each path keeps its own row.
 */
export function layoutChain(
  paths: AttackPath[],
  steps: AttackStep[],
  stepsFor: (path: AttackPath) => AttackStep[] = () => steps,
): { nodes: LaidNode[]; edges: LaidEdge[]; cols: number; rows: number } {
  const depth = new Map<string, number>();
  const byId = new Map<string, AttackPathNode>();
  const rowsOf = new Map<string, number[]>();
  const entries = new Set<string>();
  const targets = new Set<string>();
  const pairs: Array<[string, string]> = [];

  paths.forEach((p, row) => {
    const nodes = Array.isArray(p.nodes) ? p.nodes : [];
    nodes.forEach((n, i) => {
      if (!byId.has(n.id)) byId.set(n.id, n);
      depth.set(n.id, Math.max(depth.get(n.id) ?? 0, i));
      rowsOf.set(n.id, [...(rowsOf.get(n.id) ?? []), row]);
      if (i > 0) pairs.push([nodes[i - 1].id, n.id]);
    });
    if (nodes[0]) entries.add(nodes[0].id);
    if (nodes.length > 1) targets.add(nodes[nodes.length - 1].id);
  });

  // Longest-path layering, so a hop is always right of the one before it (bounded in case of cycles).
  for (let pass = 0; pass < byId.size; pass++) {
    let changed = false;
    for (const [a, b] of pairs) {
      const need = (depth.get(a) ?? 0) + 1;
      if (a !== b && (depth.get(b) ?? 0) < need && need < byId.size + 1) {
        depth.set(b, need);
        changed = true;
      }
    }
    if (!changed) break;
  }
  const maxDepth = Math.max(0, ...depth.values());
  for (const t of targets) if (!entries.has(t)) depth.set(t, maxDepth);

  // Within a column, sit each object at the average row of its paths, then push apart overlaps.
  const columns = new Map<number, LaidNode[]>();
  for (const [id, node] of byId) {
    const rows = rowsOf.get(id) ?? [0];
    const laid: LaidNode = {
      node,
      col: depth.get(id) ?? 0,
      y: rows.reduce((a, b) => a + b, 0) / rows.length,
      entry: entries.has(id),
      target: targets.has(id) && !entries.has(id),
    };
    columns.set(laid.col, [...(columns.get(laid.col) ?? []), laid]);
  }
  let rows = Math.max(1, paths.length);
  for (const list of columns.values()) {
    list.sort((a, b) => a.y - b.y);
    for (let i = 1; i < list.length; i++) list[i].y = Math.max(list[i].y, list[i - 1].y + 1);
    rows = Math.max(rows, Math.ceil(list[list.length - 1].y + 1));
  }
  const laidById = new Map<string, LaidNode>();
  for (const list of columns.values()) for (const n of list) laidById.set(n.node.id, n);

  const edges: LaidEdge[] = [];
  const seen = new Map<string, LaidEdge>();
  for (const p of paths) {
    // Whether a step was seen at runtime is per chain variant, so read it from this path's own chain.
    const own = stepsFor(p);
    const pathEdges = Array.isArray(p.edges) && p.edges.length > 0
      ? p.edges
      : (p.nodes ?? []).slice(1).map((n, i) => ({ type: '', source: p.nodes[i].id, target: n.id }));
    for (const e of pathEdges) {
      const from = laidById.get(e.source);
      const to = laidById.get(e.target);
      const key = `${e.source}->${e.target}`;
      if (!from || !to) continue;
      const ownStep = techniqueStepIndex(e.type, own);
      const observed = Boolean(ownStep && own[ownStep - 1]?.runtime_observed);
      const prev = seen.get(key);
      if (prev) {
        prev.observed = prev.observed || observed;
        continue;
      }
      const edge: LaidEdge = { key, from, to, step: techniqueStepIndex(e.type, steps), observed };
      seen.set(key, edge);
      edges.push(edge);
    }
  }
  return { nodes: [...laidById.values()], edges, cols: maxDepth + 1, rows };
}

function columnTitle(col: number, cols: number, nodes: LaidNode[]): string {
  if (col === 0) return 'Entry';
  if (col === cols - 1) return 'Target';
  const kinds = nodes.filter((n) => n.col === col).map((n) => nodeKind(n.node));
  if (kinds.every((k) => k === 'service_account')) return 'Identity';
  if (kinds.every((k) => k === 'role_binding' || k === 'cluster_role_binding')) return 'Binding';
  if (kinds.every((k) => k === 'capability' || k === 'attack_step')) return 'Weakness';
  return 'Access';
}

/**
 * The selected scenario as a left-to-right chain: entry → identity → access → target. Solid links were
 * seen at runtime, dashed ones are inferred; the number on a link is the step that crosses it.
 */
export const AttackChainDiagram: React.FC<{
  paths: AttackPath[];
  steps: AttackStep[];
  highlightedStepIndex: number | null;
  onStepClick: (step: number) => void;
  hrefFor: (node: AttackPathNode) => string | null;
  /** The steps of the chain variant a path belongs to; decides which links were seen at runtime. */
  stepsFor?: (path: AttackPath) => AttackStep[];
}> = ({ paths, steps, highlightedStepIndex, onStepClick, hrefFor, stepsFor }) => {
  const layout = useMemo(() => layoutChain(paths, steps, stepsFor), [paths, steps, stepsFor]);
  const boxRef = useRef<HTMLDivElement>(null);
  const [boxWidth, setBoxWidth] = useState(0);
  useEffect(() => {
    const el = boxRef.current;
    if (!el || typeof ResizeObserver === 'undefined') return;
    const ro = new ResizeObserver(([entry]) => setBoxWidth(entry.contentRect.width));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  if (layout.nodes.length === 0) return null;
  // Cards shrink to fit the pane; below the minimum the diagram scrolls sideways instead.
  const fitWith = (gap: number) => (boxWidth > 0 ? Math.floor((boxWidth - (layout.cols - 1) * gap) / layout.cols) : CARD_W_MAX);
  const COL_GAP = fitWith(COL_GAP_MAX) >= 150 ? COL_GAP_MAX : COL_GAP_MIN;
  const CARD_W = Math.max(CARD_W_MIN, Math.min(CARD_W_MAX, fitWith(COL_GAP)));
  const width = layout.cols * CARD_W + (layout.cols - 1) * COL_GAP;
  const height = HEAD_H + layout.rows * ROW_H - (ROW_H - CARD_H);
  const x = (col: number) => col * (CARD_W + COL_GAP);
  const y = (row: number) => HEAD_H + row * ROW_H;
  const anyHighlight = highlightedStepIndex !== null;

  return (
    <figure className="m-0 flex flex-col gap-2">
      <div className="overflow-x-auto rounded-lg border border-border bg-base p-3">
        <div ref={boxRef} className="w-full" />
        <div className="relative" style={{ width, height }} role="img" aria-label={`Attack chain with ${layout.nodes.length} objects`}>
          {Array.from({ length: layout.cols }, (_, col) => (
            <span key={col} className="absolute top-0 text-meta font-semibold uppercase tracking-wider text-muted" style={{ left: x(col), width: CARD_W }}>
              {columnTitle(col, layout.cols, layout.nodes)}
            </span>
          ))}
          <svg className="pointer-events-none absolute inset-0 overflow-visible" width={width} height={height} aria-hidden>
            {layout.edges.map((e) => {
              const x1 = x(e.from.col) + CARD_W;
              const y1 = y(e.from.y) + CARD_H / 2;
              const x2 = x(e.to.col);
              const y2 = y(e.to.y) + CARD_H / 2;
              const mid = (x1 + x2) / 2;
              const active = anyHighlight && e.step === highlightedStepIndex;
              return (
                <g key={e.key} className={clsx(active ? 'text-brand' : e.observed ? 'text-emerald-400' : 'text-muted', anyHighlight && !active && 'opacity-30')}>
                  <path
                    d={`M ${x1} ${y1} C ${mid} ${y1}, ${mid} ${y2}, ${x2 - 6} ${y2}`}
                    fill="none"
                    stroke="currentColor"
                    strokeWidth={active ? 3 : 2}
                    strokeDasharray={e.observed ? undefined : '5 4'}
                  />
                  <path d={`M ${x2 - 7} ${y2 - 4} L ${x2} ${y2} L ${x2 - 7} ${y2 + 4}`} fill="none" stroke="currentColor" strokeWidth={2} />
                </g>
              );
            })}
          </svg>
          {layout.edges
            .filter((e) => e.step)
            .map((e) => {
              const cx = (x(e.from.col) + CARD_W + x(e.to.col)) / 2;
              const cy = (y(e.from.y) + y(e.to.y)) / 2 + CARD_H / 2;
              const active = highlightedStepIndex === e.step;
              return (
                <button
                  key={`step-${e.key}`}
                  type="button"
                  onClick={() => onStepClick(e.step!)}
                  aria-pressed={active}
                  aria-label={`Step ${e.step}: ${steps[e.step! - 1]?.name ?? ''}`}
                  title={steps[e.step! - 1]?.name}
                  className={clsx(
                    'absolute flex h-6 w-6 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full border font-mono text-meta font-semibold',
                    active ? 'border-brand bg-brand text-white' : 'border-border bg-surface text-text hover:border-brand',
                  )}
                  style={{ left: cx, top: cy }}
                >
                  {e.step}
                </button>
              );
            })}
          {layout.nodes.map((n) => {
            const href = hrefFor(n.node);
            const kind = nodeKind(n.node);
            const body = (
              <>
                <span className="flex items-center justify-between gap-2 text-meta uppercase tracking-wider text-muted">
                  <span className="truncate">{NODE_KIND_LABEL[kind] ?? kind}</span>
                  {n.entry ? <span className="shrink-0 font-semibold text-brand">Start</span> : null}
                  {n.target ? <span className="shrink-0 font-semibold text-red-300">Target</span> : null}
                </span>
                <span className="block truncate text-body font-medium text-text">{nodeDisplayName(n.node)}</span>
              </>
            );
            const cls = clsx(
              'absolute flex flex-col justify-center gap-0.5 rounded-lg border bg-surface px-3',
              n.target ? 'border-red-500/50' : n.entry ? 'border-brand/50' : 'border-border',
              href && 'hover:border-brand',
            );
            const style = { left: x(n.col), top: y(n.y), width: CARD_W, height: CARD_H };
            return href ? (
              <Link key={n.node.id} to={href} className={cls} style={style} title={nodeDisplayName(n.node)}>
                {body}
              </Link>
            ) : (
              <div key={n.node.id} className={cls} style={style} title={nodeDisplayName(n.node)}>
                {body}
              </div>
            );
          })}
        </div>
      </div>
      <figcaption className="flex flex-wrap items-center gap-x-4 gap-y-1 text-caption text-muted">
        <span className="inline-flex items-center gap-1.5">
          <svg width="22" height="6" aria-hidden><line x1="0" y1="3" x2="22" y2="3" className="text-emerald-400" stroke="currentColor" strokeWidth="2" /></svg>
          seen at runtime
        </span>
        <span className="inline-flex items-center gap-1.5">
          <svg width="22" height="6" aria-hidden><line x1="0" y1="3" x2="22" y2="3" className="text-muted" stroke="currentColor" strokeWidth="2" strokeDasharray="5 4" /></svg>
          inferred from configuration
        </span>
        <span>Number = step</span>
      </figcaption>
    </figure>
  );
};
