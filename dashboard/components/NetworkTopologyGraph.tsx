import React, { useCallback, useEffect, useId, useMemo, useRef, useState } from 'react';
import * as d3 from 'd3';
import type { NetworkActivityConnectionRow } from '../types';
import { ATTACK_NODE_COLORS, GRAPH_THEME } from '../lib/graphTheme';

/* ─── types ───────────────────────────────────────────────────── */

interface NodeDatum extends d3.SimulationNodeDatum {
  id: string;
  /** Pod name, or destination name:port. */
  line1: string;
  /** Pod namespace, or the destination's service namespace / "external". */
  line2: string;
  /** Full label for tooltips and the text outline. */
  fullLabel: string;
  kind: 'pod' | 'dest';
  destType?: 'internal_workload' | 'internal_service' | 'external_endpoint';
  weight: number;
  namespace?: string;
  ownerLabel?: string;
}

interface LinkDatum extends d3.SimulationLinkDatum<NodeDatum> {
  value: number;
  /** podUid|destIp|destPort|protocol, the same key the page uses for flagged flows. */
  flowKey: string;
}

export interface NetworkTopologyGraphProps {
  /** Pod → destination groups (`view=edges`). */
  connections: NetworkActivityConnectionRow[];
  /** Flows the page flags as worth a look, keyed podUid|destIp|destPort|protocol. */
  flaggedFlowKeys?: ReadonlySet<string>;
  /** Total flow groups for the current filters, when the page loaded only the first ones. */
  totalFlows?: number;
  maxNodes?: number;
  /** Called with `podUid` for pods and `destIp:destPort/protocol` for destinations. */
  onNodeClick?: (nodeId: string, kind: 'pod' | 'dest') => void;
  /** Sizes the drawing area; the text outline sits below it. */
  className?: string;
  /** uid → name from GET /inventory/pods (fill when edges lack podName). */
  podNamesByUid?: Readonly<Record<string, string>>;
}

/* ─── helpers ─────────────────────────────────────────────────── */

const COLOR_POD_FILL = ATTACK_NODE_COLORS.pod.fill;
const COLOR_POD_STROKE = ATTACK_NODE_COLORS.pod.stroke;
const COLOR_DEST_INTERNAL_FILL = ATTACK_NODE_COLORS.role.fill;
const COLOR_DEST_INTERNAL_STROKE = ATTACK_NODE_COLORS.role.stroke;
/** External endpoints are neutral; amber is kept for flows worth a look. */
const COLOR_DEST_EXTERNAL_FILL = '#475569';
const COLOR_DEST_EXTERNAL_STROKE = '#cbd5e1';
const COLOR_LINK = '#64748b';
const COLOR_FLAGGED = '#f59e0b';
const COLOR_LINK_DIM = GRAPH_THEME.border;
const COLOR_LINK_FOCUS = GRAPH_THEME.text;

const NODE_RADIUS_MIN = 6;
const NODE_RADIUS_MAX = 15;
const NODE_STROKE_DEFAULT = 1.6;
const NODE_STROKE_SELECTED = 2.8;
const MAX_AGGREGATED_LINKS = 320;
/** Above this many nodes only the first label line is drawn. */
const SECONDARY_LABEL_LIMIT = 40;
/** Rough width of one label character at 11px, used to keep labels inside the fitted view. */
const LABEL_CHAR_WIDTH = 6.2;
const OUTLINE_LIMIT = 12;

function destinationIsInternal(node: NodeDatum): boolean {
  return node.kind === 'dest' && node.destType !== 'external_endpoint';
}

function nodeTypeLabel(node: NodeDatum): string {
  if (node.kind === 'pod') return 'Pod';
  if (node.destType === 'internal_service') return 'Service';
  if (node.destType === 'internal_workload') return 'Pod endpoint';
  return 'External';
}

function nodeBaseFill(node: NodeDatum): string {
  if (node.kind === 'pod') return COLOR_POD_FILL;
  return destinationIsInternal(node) ? COLOR_DEST_INTERNAL_FILL : COLOR_DEST_EXTERNAL_FILL;
}

function nodeBaseStroke(node: NodeDatum): string {
  if (node.kind === 'pod') return COLOR_POD_STROKE;
  return destinationIsInternal(node) ? COLOR_DEST_INTERNAL_STROKE : COLOR_DEST_EXTERNAL_STROKE;
}

function endpointId(endpoint: LinkDatum['source'] | LinkDatum['target']): string {
  if (typeof endpoint === 'object' && endpoint != null) return (endpoint as NodeDatum).id;
  return String(endpoint);
}

function truncateGraphLabel(text: string, max: number): string {
  const t = text.trim();
  return t.length <= max ? t : `${t.slice(0, max - 1)}…`;
}

function uidFootprint(uid: string): string {
  return uid.length <= 8 ? uid : `${uid.slice(0, 8)}…`;
}

type PodMeta = { namespace?: string; name?: string; ownerKind?: string; ownerName?: string };
type DestMeta = {
  workloadName?: string;
  workloadNamespace?: string;
  serviceName?: string;
  serviceNamespace?: string;
  serviceFqdn?: string;
};

function firstNonEmpty(...values: Array<string | undefined>): string | undefined {
  for (const v of values) {
    const t = (v ?? '').trim();
    if (t) return t;
  }
  return undefined;
}

/** Aggregate edge rows into pod and destination nodes; keep the busiest pods and destinations. */
function buildGraphFromConnections(
  rows: NetworkActivityConnectionRow[],
  maxNodes: number,
  podNamesByUid?: Readonly<Record<string, string>>,
): { nodes: NodeDatum[]; links: LinkDatum[]; entityCount: number } {
  type EdgeRec = { podUid: string; destKey: string; flowKey: string; weight: number };
  const edgeMap = new Map<string, EdgeRec>();
  const podMeta = new Map<string, PodMeta>();
  const destMeta = new Map<string, DestMeta & { destIp: string; destPort: number; protocol: string }>();

  for (const r of rows) {
    const uid = (r.podUid ?? '').trim();
    const dip = (r.destIp ?? '').trim();
    const dport = Number(r.destPort);
    if (!uid || !dip || r.destPort == null || Number.isNaN(dport)) continue;
    const proto = String(r.protocol ?? 'tcp').trim() || 'tcp';
    const destKey = `${dip}:${dport}/${proto}`;
    const flowKey = `${uid}|${dip}|${dport}|${proto.toLowerCase()}`;
    const w = typeof r.observationCount === 'number' && r.observationCount > 0 ? r.observationCount : 1;
    const cur = edgeMap.get(flowKey);
    if (cur) cur.weight += w;
    else edgeMap.set(flowKey, { podUid: uid, destKey, flowKey, weight: w });

    const d = destMeta.get(destKey);
    destMeta.set(destKey, {
      destIp: dip,
      destPort: dport,
      protocol: proto,
      workloadName: firstNonEmpty(d?.workloadName, r.destWorkloadName),
      workloadNamespace: firstNonEmpty(d?.workloadNamespace, r.destWorkloadNamespace),
      serviceName: firstNonEmpty(d?.serviceName, r.destServiceName),
      serviceNamespace: firstNonEmpty(d?.serviceNamespace, r.destServiceNamespace),
      serviceFqdn: firstNonEmpty(d?.serviceFqdn, r.destServiceFqdn),
    });
    const p = podMeta.get(uid);
    podMeta.set(uid, {
      name: firstNonEmpty(p?.name, r.podName, r.containerName),
      namespace: firstNonEmpty(p?.namespace, r.namespace),
      ownerKind: firstNonEmpty(p?.ownerKind, r.ownerKind),
      ownerName: firstNonEmpty(p?.ownerName, r.ownerName),
    });
  }

  const podWeight = new Map<string, number>();
  const destWeight = new Map<string, number>();
  for (const e of edgeMap.values()) {
    podWeight.set(e.podUid, (podWeight.get(e.podUid) ?? 0) + e.weight);
    destWeight.set(e.destKey, (destWeight.get(e.destKey) ?? 0) + e.weight);
  }
  const entityCount = podWeight.size + destWeight.size;

  const half = Math.max(1, Math.floor(maxNodes / 2));
  const topPods = [...podWeight.entries()].sort((a, b) => b[1] - a[1]).slice(0, half);
  const topDests = [...destWeight.entries()].sort((a, b) => b[1] - a[1]).slice(0, half);

  const nodes: NodeDatum[] = [];
  for (const [uid, weight] of topPods) {
    const meta = podMeta.get(uid) ?? {};
    const name = firstNonEmpty(meta.name, podNamesByUid?.[uid]);
    const ns = meta.namespace ?? '';
    const owner = meta.ownerKind && meta.ownerName ? `${meta.ownerKind}/${meta.ownerName}` : undefined;
    nodes.push({
      id: `pod:${uid}`,
      line1: name ?? `Pod ${uidFootprint(uid)}`,
      line2: ns,
      fullLabel: [name, owner ? `owner ${owner}` : '', `uid ${uid}`].filter(Boolean).join(' · '),
      kind: 'pod',
      weight,
      namespace: ns || undefined,
      ownerLabel: owner,
    });
  }
  for (const [destKey, weight] of topDests) {
    const m = destMeta.get(destKey);
    if (!m) continue;
    const display = m.serviceName ?? m.workloadName ?? m.destIp;
    const ns = m.serviceName ? m.serviceNamespace : m.workloadName ? m.workloadNamespace : undefined;
    const tuple = `${m.destIp}:${m.destPort}/${m.protocol}`;
    nodes.push({
      id: `dest:${destKey}`,
      line1: `${display}:${m.destPort}`,
      line2: ns ?? (m.serviceName || m.workloadName ? '' : 'external'),
      fullLabel: m.serviceName
        ? `${m.serviceFqdn || `${m.serviceName}.${m.serviceNamespace}.svc`} · ${tuple}`
        : m.workloadName
          ? `${m.workloadName} · ${tuple}`
          : tuple,
      kind: 'dest',
      destType: m.serviceName ? 'internal_service' : m.workloadName ? 'internal_workload' : 'external_endpoint',
      weight,
      namespace: ns,
    });
  }

  const podSet = new Set(topPods.map(([u]) => u));
  const destSet = new Set(topDests.map(([k]) => k));
  const links = [...edgeMap.values()]
    .filter((e) => podSet.has(e.podUid) && destSet.has(e.destKey))
    .sort((a, b) => b.weight - a.weight)
    .slice(0, MAX_AGGREGATED_LINKS)
    .map((e) => ({ source: `pod:${e.podUid}`, target: `dest:${e.destKey}`, value: e.weight, flowKey: e.flowKey }));

  return { nodes, links, entityCount };
}

function nodeSymbolPath(node: NodeDatum, radius: number): string {
  const type = node.kind === 'pod' ? d3.symbolCircle : destinationIsInternal(node) ? d3.symbolSquare : d3.symbolDiamond;
  return d3.symbol().type(type).size(Math.PI * radius * radius)() ?? '';
}

/** Slight curve from the source edge to the target edge, so the arrow tip touches the target shape. */
function linkCurvePath(d: LinkDatum, sourceR: number, targetR: number): string {
  const s = d.source as NodeDatum;
  const t = d.target as NodeDatum;
  const sx0 = s.x ?? 0;
  const sy0 = s.y ?? 0;
  const tx0 = t.x ?? 0;
  const ty0 = t.y ?? 0;
  const dx = tx0 - sx0;
  const dy = ty0 - sy0;
  const len = Math.hypot(dx, dy) || 1;
  const ux = dx / len;
  const uy = dy / len;
  const sx = sx0 + ux * sourceR;
  const sy = sy0 + uy * sourceR;
  const tx = tx0 - ux * (targetR + 2);
  const ty = ty0 - uy * (targetR + 2);
  const off = len * 0.08;
  const cx = (sx + tx) / 2 - uy * off;
  const cy = (sy + ty) / 2 + ux * off;
  return `M${sx},${sy} Q${cx},${cy} ${tx},${ty}`;
}

/* ─── component ───────────────────────────────────────────────── */

export const NetworkTopologyGraph: React.FC<NetworkTopologyGraphProps> = ({
  connections,
  flaggedFlowKeys,
  totalFlows,
  maxNodes = 80,
  onNodeClick,
  className = '',
  podNamesByUid,
}) => {
  const svgRef = useRef<SVGSVGElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const uid = useId().replace(/\W/g, '');
  const arrowId = `arrow-${uid}`;
  const arrowFlaggedId = `arrow-flagged-${uid}`;

  const [dimensions, setDimensions] = useState<{ width: number; height: number } | null>(null);
  const dimensionsRef = useRef(dimensions);
  dimensionsRef.current = dimensions;
  const [tooltip, setTooltip] = useState<{ x: number; y: number; content: string } | null>(null);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const selectedNodeIdRef = useRef<string | null>(null);
  selectedNodeIdRef.current = selectedNodeId;
  // Parents pass inline handlers; keep the latest in a ref so a re-render never re-runs the layout.
  const onNodeClickRef = useRef(onNodeClick);
  onNodeClickRef.current = onNodeClick;
  /** Re-paints highlight state; set by the layout effect. */
  const paintRef = useRef<(focusId: string | null) => void>(() => {});
  /** Frames the whole graph; set by the layout effect, re-run on resize. */
  const fitRef = useRef<(animate: boolean) => void>(() => {});

  const { nodes, links, entityCount } = useMemo(
    () => buildGraphFromConnections(connections, maxNodes, podNamesByUid),
    [connections, maxNodes, podNamesByUid],
  );

  const flaggedKey = useMemo(() => [...(flaggedFlowKeys ?? [])].sort().join(','), [flaggedFlowKeys]);
  const flaggedLinks = useMemo(() => {
    const keys = new Set(flaggedKey ? flaggedKey.split(',') : []);
    return new Set(links.filter((l) => keys.has(l.flowKey)).map((l) => l.flowKey));
  }, [links, flaggedKey]);
  const flaggedNodeIds = useMemo(() => {
    const out = new Set<string>();
    for (const l of links) if (flaggedLinks.has(l.flowKey)) out.add(endpointId(l.target));
    return out;
  }, [links, flaggedLinks]);

  useEffect(() => {
    if (selectedNodeId && !nodes.some((n) => n.id === selectedNodeId)) setSelectedNodeId(null);
  }, [nodes, selectedNodeId]);

  const selectNode = useCallback((node: NodeDatum) => {
    const wasSelected = selectedNodeIdRef.current === node.id;
    setSelectedNodeId(wasSelected ? null : node.id);
    if (!wasSelected) onNodeClickRef.current?.(node.id.replace(/^(pod|dest):/, ''), node.kind);
  }, []);

  const hasNodes = nodes.length > 0;
  // The SVG is absolutely positioned inside the measured box, so its size never feeds back into the box.
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      const { width, height } = entries[0].contentRect;
      if (width < 1 || height < 1) return;
      setDimensions((prev) =>
        prev && Math.abs(prev.width - width) < 1 && Math.abs(prev.height - height) < 1 ? prev : { width, height },
      );
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, [hasNodes]);

  const hasSize = dimensions != null;
  const narrow = (dimensions?.width ?? 900) < 640;

  useEffect(() => {
    const dims = dimensionsRef.current;
    if (!svgRef.current || nodes.length === 0 || !dims) return;
    const { width } = dims;
    const svg = d3.select(svgRef.current);
    svg.selectAll(':not(title)').remove();

    const maxWeight = d3.max(nodes, (d) => d.weight) ?? 1;
    const rScale = d3.scaleSqrt().domain([0, maxWeight]).range([NODE_RADIUS_MIN, NODE_RADIUS_MAX]);
    const maxLinkVal = d3.max(links, (d) => d.value) ?? 1;
    const linkWidth = d3.scaleSqrt().domain([0, maxLinkVal]).range([0.8, 4]);
    const linkOpacity = d3.scaleSqrt().domain([0, maxLinkVal]).range([0.35, 0.8]);

    const nodesCopy: NodeDatum[] = nodes.map((n) => ({ ...n }));
    const linksCopy: LinkDatum[] = links.map((l) => ({ ...l }));
    const radius = new Map(nodesCopy.map((n) => [n.id, rScale(n.weight)]));
    const neighbours = new Map<string, Set<string>>();
    for (const l of linksCopy) {
      const s = endpointId(l.source);
      const t = endpointId(l.target);
      if (!neighbours.has(s)) neighbours.set(s, new Set());
      if (!neighbours.has(t)) neighbours.set(t, new Set());
      neighbours.get(s)!.add(t);
      neighbours.get(t)!.add(s);
    }

    // Two columns: pods on the left with labels to their left, destinations on the right with labels to
    // their right. Labels then never cross the flows between the columns.
    const showSecondary = nodesCopy.length <= SECONDARY_LABEL_LIMIT;
    const labelMax = narrow ? 14 : 30;
    const rowGap = showSecondary ? 40 : 28;
    const columnGap = narrow ? 120 : Math.min(520, Math.max(320, width * 0.4));
    const columnX = (n: NodeDatum) => (n.kind === 'pod' ? -columnGap / 2 : columnGap / 2);
    // Seed rows grouped by namespace so the first frame is already readable.
    for (const kind of ['pod', 'dest'] as const) {
      const col = nodesCopy
        .filter((n) => n.kind === kind)
        .sort((a, b) => (a.namespace ?? '~').localeCompare(b.namespace ?? '~') || b.weight - a.weight);
      col.forEach((n, i) => {
        n.x = columnX(n);
        n.y = (i - (col.length - 1) / 2) * rowGap;
      });
    }

    const simulation = d3
      .forceSimulation(nodesCopy)
      .force('link', d3.forceLink<NodeDatum, LinkDatum>(linksCopy).id((d) => d.id).distance(columnGap).strength(0.05))
      .force('x', d3.forceX<NodeDatum>(columnX).strength(1))
      .force('y', d3.forceY(0).strength(0.02))
      .force('collision', d3.forceCollide<NodeDatum>().radius(rowGap / 2).strength(1).iterations(3))
      .alphaDecay(0.08)
      .stop();
    // Settle before the first paint: no jumping nodes, and the frame below fits the final layout.
    simulation.tick(Math.ceil(Math.log(simulation.alphaMin()) / Math.log(1 - simulation.alphaDecay())));
    // Keep the order the forces found (fewer crossings), then snap to evenly spaced rows so labels never overlap.
    for (const kind of ['pod', 'dest'] as const) {
      const col = nodesCopy.filter((n) => n.kind === kind).sort((a, b) => (a.y ?? 0) - (b.y ?? 0));
      col.forEach((n, i) => {
        n.x = columnX(n);
        n.y = (i - (col.length - 1) / 2) * rowGap;
      });
    }

    const defs = svg.append('defs');
    for (const [id, color] of [
      [arrowId, COLOR_LINK],
      [arrowFlaggedId, COLOR_FLAGGED],
    ] as const) {
      defs
        .append('marker')
        .attr('id', id)
        .attr('viewBox', '0 -5 10 10')
        .attr('refX', 8)
        .attr('refY', 0)
        .attr('markerWidth', 5)
        .attr('markerHeight', 5)
        .attr('markerUnits', 'strokeWidth')
        .attr('orient', 'auto')
        .append('path')
        .attr('d', 'M0,-4L8,0L0,4')
        .attr('fill', color);
    }

    const gRoot = svg.append('g').attr('data-topology-root', '1');
    const zoom = d3
      .zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.2, 4])
      .on('zoom', (event) => gRoot.attr('transform', event.transform));
    svg.call(zoom).on('dblclick.zoom', null);
    svg.on('click', () => setSelectedNodeId(null));

    const labelWidth = (n: NodeDatum) =>
      Math.max(truncateGraphLabel(n.line1, labelMax).length, showSecondary ? truncateGraphLabel(n.line2, labelMax).length * 0.9 : 0) *
      LABEL_CHAR_WIDTH;
    fitRef.current = (animate: boolean) => {
      const box = dimensionsRef.current;
      if (!svgRef.current || !box) return;
      let minX = Infinity;
      let minY = Infinity;
      let maxX = -Infinity;
      let maxY = -Infinity;
      for (const n of nodesCopy) {
        const x = n.x ?? 0;
        const y = n.y ?? 0;
        const r = radius.get(n.id) ?? NODE_RADIUS_MIN;
        const lw = labelWidth(n) + r + 8;
        minX = Math.min(minX, n.kind === 'pod' ? x - lw : x - r);
        maxX = Math.max(maxX, n.kind === 'pod' ? x + r : x + lw);
        minY = Math.min(minY, y - Math.max(r, 12));
        maxY = Math.max(maxY, y + Math.max(r, showSecondary ? 18 : 8));
      }
      const pad = 24;
      const k = Math.min((box.width - 2 * pad) / Math.max(maxX - minX, 1), (box.height - 2 * pad) / Math.max(maxY - minY, 1), 1.2);
      // Never shrink text below ~9px; a tall graph scrolls (pans) instead.
      const scale = Math.max(k, 0.75);
      const tx = box.width / 2 - scale * ((minX + maxX) / 2);
      const ty = scale === k ? box.height / 2 - scale * ((minY + maxY) / 2) : pad - scale * minY;
      const transform = d3.zoomIdentity.translate(tx, ty).scale(scale);
      const sel = d3.select(svgRef.current);
      if (animate) sel.transition().duration(250).call(zoom.transform, transform);
      else sel.call(zoom.transform, transform);
    };

    const linkG = gRoot.append('g').attr('fill', 'none');
    const linkPath = linkG
      .selectAll<SVGPathElement, LinkDatum>('path')
      .data(linksCopy)
      .join('path')
      .attr('class', 'topology-link')
      .attr('data-flagged', (d) => (flaggedLinks.has(d.flowKey) ? '1' : null))
      .attr('stroke-linecap', 'round')
      .attr('d', (d) =>
        linkCurvePath(d, radius.get(endpointId(d.source)) ?? 0, radius.get(endpointId(d.target)) ?? 0),
      );
    // Flagged flows draw last so they sit on top.
    linkPath.filter((d) => flaggedLinks.has(d.flowKey)).raise();

    const node = gRoot
      .append('g')
      .attr('class', 'topology-nodes')
      .selectAll<SVGGElement, NodeDatum>('g')
      .data(nodesCopy)
      .join('g')
      .attr('class', 'topology-node')
      .attr('data-node-id', (d) => d.id)
      .attr('data-flagged', (d) => (flaggedNodeIds.has(d.id) ? '1' : null))
      .attr('transform', (d) => `translate(${d.x ?? 0},${d.y ?? 0})`)
      .attr('tabindex', 0)
      .attr('role', 'button')
      .attr('aria-label', (d) => `${nodeTypeLabel(d)} ${d.fullLabel}. Seen ${d.weight} times.${flaggedNodeIds.has(d.id) ? ' Worth a look.' : ''}`)
      .style('cursor', 'pointer');

    node
      .append('circle')
      .attr('r', (d) => Math.max(16, (radius.get(d.id) ?? 0) + 6))
      .attr('fill', 'transparent');
    node
      .append('path')
      .attr('class', 'topology-node-shape')
      .attr('d', (d) => nodeSymbolPath(d, radius.get(d.id) ?? NODE_RADIUS_MIN))
      .attr('fill', nodeBaseFill)
      .attr('fill-opacity', 0.85)
      .attr('stroke', (d) => (flaggedNodeIds.has(d.id) ? COLOR_FLAGGED : nodeBaseStroke(d)));

    const text = node
      .append('text')
      .attr('pointer-events', 'none')
      .attr('text-anchor', (d) => (d.kind === 'pod' ? 'end' : 'start'))
      .attr('x', (d) => ((radius.get(d.id) ?? 0) + 6) * (d.kind === 'pod' ? -1 : 1))
      .attr('paint-order', 'stroke')
      .attr('stroke', GRAPH_THEME.surfaceDeep)
      .attr('stroke-width', 3)
      .attr('stroke-linejoin', 'round');
    text
      .append('tspan')
      .attr('x', (d) => ((radius.get(d.id) ?? 0) + 6) * (d.kind === 'pod' ? -1 : 1))
      .attr('dy', showSecondary ? '-0.1em' : '0.35em')
      .attr('fill', (d) => (flaggedNodeIds.has(d.id) ? COLOR_FLAGGED : GRAPH_THEME.text))
      .attr('font-size', '11px')
      .attr('font-weight', 600)
      .text((d) => truncateGraphLabel(d.line1, labelMax));
    if (showSecondary) {
      text
        .filter((d) => Boolean(d.line2))
        .append('tspan')
        .attr('x', (d) => ((radius.get(d.id) ?? 0) + 6) * (d.kind === 'pod' ? -1 : 1))
        .attr('dy', '1.25em')
        .attr('fill', GRAPH_THEME.muted)
        .attr('font-size', '10px')
        .text((d) => truncateGraphLabel(d.line2, labelMax));
    }

    paintRef.current = (focusId: string | null) => {
      const near = focusId ? neighbours.get(focusId) ?? new Set<string>() : null;
      node.style('opacity', (d) => (!near || d.id === focusId || near.has(d.id) ? 1 : 0.15));
      node
        .select<SVGPathElement>('path.topology-node-shape')
        .attr('stroke-width', (d) => (d.id === selectedNodeIdRef.current ? NODE_STROKE_SELECTED : NODE_STROKE_DEFAULT));
      linkPath.each(function (l) {
        const hit = !focusId || endpointId(l.source) === focusId || endpointId(l.target) === focusId;
        const flagged = flaggedLinks.has(l.flowKey);
        d3.select(this)
          .attr('stroke', !hit ? COLOR_LINK_DIM : flagged ? COLOR_FLAGGED : focusId ? COLOR_LINK_FOCUS : COLOR_LINK)
          .attr('stroke-width', flagged ? Math.max(2, linkWidth(l.value)) : linkWidth(l.value))
          .attr('stroke-opacity', !hit ? 0.12 : flagged ? 0.95 : focusId ? 0.8 : linkOpacity(l.value))
          .attr('stroke-dasharray', flagged ? '6 3' : null)
          .attr('marker-end', hit ? `url(#${flagged ? arrowFlaggedId : arrowId})` : null);
      });
    };
    paintRef.current(selectedNodeIdRef.current);

    node
      .on('mouseenter', (event, d) => {
        const [x, y] = d3.pointer(event, containerRef.current);
        const parts = [`${nodeTypeLabel(d)}: ${d.fullLabel}`];
        if (d.kind === 'pod' && d.namespace) parts.push(`Namespace: ${d.namespace}`);
        parts.push(`Seen ${d.weight.toLocaleString()}×`);
        if (flaggedNodeIds.has(d.id)) parts.push('Worth a look');
        setTooltip({ x, y, content: parts.join('\n') });
        paintRef.current(d.id);
      })
      .on('mouseleave', () => {
        setTooltip(null);
        paintRef.current(selectedNodeIdRef.current);
      })
      .on('focus', (_event, d) => paintRef.current(d.id))
      .on('blur', () => paintRef.current(selectedNodeIdRef.current))
      .on('keydown', (event, d) => {
        if (event.key !== 'Enter' && event.key !== ' ') return;
        event.preventDefault();
        selectNode(d);
      })
      .on('click', (event, d) => {
        event.stopPropagation();
        selectNode(d);
      });

    fitRef.current(false);
    return () => {
      svg.on('.zoom', null).on('click', null);
    };
    // Layout depends on the graph and on the narrow/wide breakpoint; other resizes only re-fit.
  }, [nodes, links, flaggedLinks, flaggedNodeIds, hasSize, narrow, selectNode, arrowId, arrowFlaggedId]);

  useEffect(() => {
    fitRef.current(true);
  }, [dimensions]);

  useEffect(() => {
    paintRef.current(selectedNodeId);
  }, [selectedNodeId]);

  if (nodes.length === 0) {
    return (
      <div className="flex items-center justify-center px-4 py-12 text-center text-body text-muted">
        No flows to draw for these filters.
      </div>
    );
  }

  const shownFlows = links.length;
  const allFlows = Math.max(totalFlows ?? 0, connections.length);
  const summarized = entityCount > nodes.length || shownFlows < allFlows;
  const tooltipLeft = tooltip ? Math.min(tooltip.x + 12, (dimensions?.width ?? 0) - 260) : 0;

  return (
    <div className="flex flex-col">
      <div ref={containerRef} className={`relative min-h-0 overflow-hidden ${className}`.trim()}>
        <svg
          ref={svgRef}
          className="absolute inset-0 h-full w-full touch-none select-none"
          style={{ background: GRAPH_THEME.surfaceDeep }}
          role="img"
          aria-label={`Network map: ${nodes.filter((n) => n.kind === 'pod').length} pods, ${nodes.filter((n) => n.kind === 'dest').length} destinations, ${links.length} flows. Drag to pan, scroll or pinch to zoom.`}
        >
          <title>{`Network map: ${nodes.length} nodes, ${links.length} flows`}</title>
        </svg>
        {tooltip ? (
          <div
            className="pointer-events-none absolute z-20 max-w-[16rem] whitespace-pre-line break-words rounded-lg border border-border bg-surface/95 px-3 py-2 text-caption text-text shadow-lg backdrop-blur"
            style={{ left: Math.max(8, tooltipLeft), top: tooltip.y + 14 }}
          >
            {tooltip.content}
          </div>
        ) : null}
      </div>

      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 border-t border-border px-3 py-2 text-caption text-muted">
        <span className="flex items-center gap-1.5">
          <span className="inline-block h-2.5 w-2.5 rounded-full" style={{ background: COLOR_POD_FILL }} />
          Pod
        </span>
        <span className="flex items-center gap-1.5">
          <span className="inline-block h-2.5 w-2.5 rounded-[2px]" style={{ background: COLOR_DEST_INTERNAL_FILL }} />
          Service or pod
        </span>
        <span className="flex items-center gap-1.5">
          <span className="inline-block h-2 w-2 rotate-45" style={{ background: COLOR_DEST_EXTERNAL_FILL, outline: `1px solid ${COLOR_DEST_EXTERNAL_STROKE}` }} />
          External
        </span>
        {flaggedLinks.size > 0 ? (
          <span className="flex items-center gap-1.5 text-amber-300">
            <span className="inline-block w-5 border-t-2 border-dashed" style={{ borderColor: COLOR_FLAGGED }} />
            Worth a look
          </span>
        ) : null}
        {summarized ? (
          <span className="ml-auto">
            Busiest {nodes.length.toLocaleString()} of {entityCount.toLocaleString()} endpoints
            {allFlows > connections.length ? `, first ${connections.length.toLocaleString()} of ${allFlows.toLocaleString()} flows` : ''}
          </span>
        ) : null}
      </div>

      <details className="border-t border-border px-3 py-2 text-caption text-muted">
        <summary className="cursor-pointer font-semibold text-text">Map as text</summary>
        <div className="mt-2 grid gap-3 md:grid-cols-2">
          <div>
            <h3 className="font-semibold text-text">Endpoints</h3>
            <ul className="mt-1 space-y-1">
              {nodes.slice(0, OUTLINE_LIMIT).map((n) => (
                <li key={n.id}>
                  <button
                    type="button"
                    className="text-left hover:text-text focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/60"
                    onClick={() => selectNode(n)}
                  >
                    <span className="font-medium text-text">{n.line1}</span> · {nodeTypeLabel(n)}
                    {n.namespace ? ` · ${n.namespace}` : ''}
                  </button>
                </li>
              ))}
            </ul>
          </div>
          <div>
            <h3 className="font-semibold text-text">Busiest flows</h3>
            <ol className="mt-1 space-y-1">
              {links.slice(0, OUTLINE_LIMIT).map((l) => {
                const s = nodes.find((n) => n.id === endpointId(l.source));
                const t = nodes.find((n) => n.id === endpointId(l.target));
                return (
                  <li key={l.flowKey}>
                    <span className="text-text">{s?.line1}</span> → <span className="text-text">{t?.line1}</span> · {l.value.toLocaleString()}×
                    {flaggedLinks.has(l.flowKey) ? <span className="text-amber-300"> · worth a look</span> : null}
                  </li>
                );
              })}
            </ol>
          </div>
        </div>
        {nodes.length > OUTLINE_LIMIT || links.length > OUTLINE_LIMIT ? (
          <p className="mt-2 text-muted-2">The table view lists every flow.</p>
        ) : null}
      </details>
    </div>
  );
};
