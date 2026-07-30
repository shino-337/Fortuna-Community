import React, { useCallback, useEffect, useMemo, useRef, useState, useId } from 'react';
import { ChevronUp, ChevronDown, Maximize2, Minus, Move, Plus } from 'lucide-react';
import * as d3 from 'd3';
import type { AttackPathGraphData } from '../types';
import type { GraphSemanticMode } from '../lib/persona';
import { graphEdgeVisual, graphNodeLabelMax } from '../lib/graphSemantics';
import {
  buildGraphTrustPosture,
  classifyEdgeTrust,
  classifyNodeTrust,
  type GraphTrustContext,
} from '../lib/graphTrustSemantics';
import { edgeTrustVisual, nodeTrustVisual } from '../lib/graphTrustRendering';
import { GraphTrustOverlay } from './GraphTrustOverlay';
import { ATTACK_NODE_COLORS, GRAPH_EDGE_COLORS, GRAPH_THEME } from '../lib/graphTheme';

/* ─── types ───────────────────────────────────────────────── */

interface GraphNode {
  id: string;
  label: string;
  type: string;
  risk?: string;
  isStart?: boolean;
  isEnd?: boolean;
  stepIndex?: number;
  x?: number;
  y?: number;
  vx?: number;
  vy?: number;
  fx?: number | null;
  fy?: number | null;
  index?: number;
  pathIds?: string[];
  pathId?: string;
}

interface GraphLink {
  source: GraphNode | string;
  target: GraphNode | string;
  type?: string;
  value: number;
  stepIndex?: number;
  _curveOffset?: number;
  pathIds?: string[];
}

interface AttackPathGraphProps {
  data: AttackPathGraphData;
  width?: number;
  height?: number;
  onNodeClick?: (nodeId: string, type: string) => void;
  highlightedNodeIds?: Set<string>;
  highlightedEdgeKeys?: Set<string>;
  focusNodeIds?: Set<string>;
  /** 1-based technique step index — emphasizes matching edges without rebuilding layout */
  highlightedStepIndex?: number | null;
  className?: string;
  /** Layer 2 graph semantics (persona-adaptive). */
  semanticMode?: GraphSemanticMode;
  /** Native graph trust layer (Phase 1). */
  graphTrust?: GraphTrustContext;
  showTrustOverlay?: boolean;
}

function edgeKeyFromDatum(d: GraphLink): string {
  const s = typeof d.source === 'object' ? (d.source as GraphNode).id : String(d.source);
  const t = typeof d.target === 'object' ? (d.target as GraphNode).id : String(d.target);
  return `${s}->${t}`;
}

function endpointId(value: GraphLink['source'] | null | undefined, fallback: string): string {
  if (!value) return fallback;
  return typeof value === 'object' ? (value as GraphNode).id : String(value);
}

function pathIdsFromDatum(value: { pathIds?: string[]; pathId?: unknown } | null | undefined): string[] {
  if (!value) return [];
  const ids = new Set<string>();
  if (Array.isArray(value.pathIds)) {
    for (const id of value.pathIds) {
      const s = String(id ?? '').trim();
      if (s) ids.add(s);
    }
  }
  const legacy = String(value.pathId ?? '').trim();
  if (legacy) ids.add(legacy);
  return Array.from(ids);
}

function mergePathIds(a?: string[], b?: string[]): string[] | undefined {
  const ids = new Set<string>();
  for (const id of a ?? []) {
    const s = String(id ?? '').trim();
    if (s) ids.add(s);
  }
  for (const id of b ?? []) {
    const s = String(id ?? '').trim();
    if (s) ids.add(s);
  }
  return ids.size > 0 ? Array.from(ids) : undefined;
}

function curvedPath(d: GraphLink): string {
  const sx = (d.source as GraphNode).x!;
  const sy = (d.source as GraphNode).y!;
  const tx = (d.target as GraphNode).x!;
  const ty = (d.target as GraphNode).y!;
  const mx = (sx + tx) / 2;
  const my = (sy + ty) / 2;
  const dx = tx - sx;
  const dy = ty - sy;
  const len = Math.sqrt(dx * dx + dy * dy) || 1;
  const nx = -dy / len;
  const ny = dx / len;
  const off = d._curveOffset ?? 0;
  const cx = mx + nx * off;
  const cy = my + ny * off;
  return `M ${sx} ${sy} Q ${cx} ${cy} ${tx} ${ty}`;
}

function labelAnchor(d: GraphLink): [number, number] {
  const sx = (d.source as GraphNode).x!;
  const sy = (d.source as GraphNode).y!;
  const tx = (d.target as GraphNode).x!;
  const ty = (d.target as GraphNode).y!;
  const mx = (sx + tx) / 2;
  const my = (sy + ty) / 2;
  const dx = tx - sx;
  const dy = ty - sy;
  const len = Math.sqrt(dx * dx + dy * dy) || 1;
  const nx = -dy / len;
  const ny = dx / len;
  const off = d._curveOffset ?? 0;
  const cx = mx + nx * off;
  const cy = my + ny * off;
  const lx = 0.25 * sx + 0.5 * cx + 0.25 * tx;
  const ly = 0.25 * sy + 0.5 * cy + 0.25 * ty;
  return [lx, ly];
}

function assignCurveOffsets(rawLinks: AttackPathGraphData['links']): Map<number, number> {
  const pairFirstIndex = new Map<string, number>();
  const pairCount = new Map<string, number>();
  rawLinks.forEach((l) => {
    const k = `${l.source}->${l.target}`;
    pairCount.set(k, (pairCount.get(k) || 0) + 1);
  });
  const pairSeen = new Map<string, number>();
  const offsetByIndex = new Map<number, number>();
  rawLinks.forEach((l, i) => {
    const k = `${l.source}->${l.target}`;
    const total = pairCount.get(k) || 1;
    const idx = pairSeen.get(k) ?? 0;
    pairSeen.set(k, idx + 1);
    const spread = 20;
    const offset = total === 1 ? 0 : (idx - (total - 1) / 2) * spread;
    offsetByIndex.set(i, offset);
  });
  return offsetByIndex;
}

/* ─── colours ─────────────────────────────────────────────── */

const NODE_COLORS = ATTACK_NODE_COLORS;

const NODE_LEGEND_TYPES = [
  'path_entry',
  'pod',
  'service_account',
  'role_binding',
  'cluster_role_binding',
  'role',
  'cluster_role',
  'capability',
  'attack_step',
  'node',
  'secret',
] as const;

const RISK_GLOW: Record<string, string> = {
  critical: GRAPH_THEME.critical,
  high: GRAPH_THEME.high,
  medium: GRAPH_THEME.medium,
  low: GRAPH_THEME.success,
};

const EDGE_DESCRIPTIONS: Record<string, string> = {
  PATH_ENTRY: 'Synthetic marker for one concrete attack path entry.',
  SERVICE_ACCOUNT_ACCESS: 'Pod can use service account credentials.',
  RBAC_BINDING: 'Service account is linked to RBAC binding.',
  GRANTS_ROLE: 'Binding grants role/cluster role permissions.',
  NETWORK_REACH: 'Observed network path (high confidence).',
  NETWORK_REACH_SOFT: 'Soft allow path (new pod fallback, lower confidence).',
  CONTAINER_ESCAPE: 'Potential container escape primitive is available.',
  HOST_ACCESS: 'Path can touch host-level surface.',
  LATERAL_MOVE: 'Path supports movement to another workload/node.',
  HAS_ATTACK_STEP: 'Runtime attack step associated with this path.',
  CAN_STEAL_CREDENTIALS: 'Path includes credential theft step.',
};

const HUMAN_EDGE_LABELS: Record<string, string> = {
  PATH_ENTRY: 'starts at',
  SERVICE_ACCOUNT_ACCESS: 'harvests token',
  RBAC_BINDING: 'binds to',
  GRANTS_ROLE: 'escalates to',
  NETWORK_REACH: 'network reach',
  NETWORK_REACH_SOFT: 'potential reach',
  CONTAINER_ESCAPE: 'escapes container',
  ESC_HOSTPATH_NODE: 'escapes via hostPath',
  ESC_HOSTPID: 'escapes via hostPID',
  HOST_ACCESS: 'accesses node',
  LATERAL_MOVE: 'moves laterally',
  HAS_ATTACK_STEP: 'executes step',
  CAN_STEAL_CREDENTIALS: 'steals creds',
};

function edgeStrokeColor(d: GraphLink): string {
  const t = String((d as GraphLink).type || '').toUpperCase().trim();
  return GRAPH_EDGE_COLORS[t] || GRAPH_THEME.muted2;
}

function normalizeNodeType(type: string | undefined): string {
  return String(type || 'unknown').toLowerCase().trim().replace(/-/g, '_');
}

function nodeColorsForType(type: string | undefined): { fill: string; stroke: string } {
  const normalized = normalizeNodeType(type);
  return NODE_COLORS[normalized] || { fill: GRAPH_THEME.muted2, stroke: GRAPH_THEME.border };
}

function nodeLegendStyle(type: string): React.CSSProperties {
  const color = nodeColorsForType(type);
  const normalized = normalizeNodeType(type);
  const base: React.CSSProperties = {
    width: 11,
    height: 11,
    display: 'inline-block',
    background: color.fill,
    border: `1.5px solid ${color.stroke}`,
    flexShrink: 0,
  };
  if (normalized === 'pod') return { ...base, borderRadius: 3 };
  if (normalized === 'node') return { ...base, clipPath: 'polygon(50% 0%, 92% 25%, 92% 75%, 50% 100%, 8% 75%, 8% 25%)' };
  if (normalized.includes('binding')) return { ...base, transform: 'rotate(45deg)', borderRadius: 2 };
  if (normalized.includes('role')) return { ...base, clipPath: 'polygon(50% 0%, 92% 18%, 86% 68%, 50% 100%, 14% 68%, 8% 18%)' };
  return { ...base, borderRadius: 999 };
}

function attackEdgeVisual(
  lk: GraphLink,
  semanticMode: GraphSemanticMode,
  renderTrustCtx: GraphTrustContext,
): {
  stroke: string;
  strokeWidth: number;
  strokeOpacity: number;
  strokeDasharray: string | null;
} {
  const hasStepIndex = lk.stepIndex !== undefined;
  const persona = graphEdgeVisual(semanticMode, lk.type, hasStepIndex);
  const trustKind = classifyEdgeTrust(lk.type, renderTrustCtx, hasStepIndex);
  const trust = edgeTrustVisual(trustKind, semanticMode, lk.type, hasStepIndex);
  return {
    stroke: edgeStrokeColor(lk),
    strokeWidth: hasStepIndex ? Math.max(persona.strokeWidth, trust.strokeWidth, 2.4) : Math.max(persona.strokeWidth, trust.strokeWidth, 1.25),
    strokeOpacity: Math.min(0.96, Math.max(persona.strokeOpacity, trust.strokeOpacity)),
    strokeDasharray: trust.strokeDasharray ?? persona.strokeDasharray,
  };
}

const MARKER_REF_X = 22;

/* ─── LOD clustering ──────────────────────────────────────── */

const LOD_THRESHOLD = 200;

/**
 * When node count exceeds LOD_THRESHOLD, collapse non-critical nodes
 * into type-based cluster super-nodes. This keeps entry/end/attack-step
 * nodes at full fidelity while reducing DOM element count from 500+ to ~20-30.
 */
function clusterForLOD(
  rawNodes: AttackPathGraphData['nodes'],
  rawLinks: AttackPathGraphData['links'],
): { nodes: AttackPathGraphData['nodes']; links: AttackPathGraphData['links']; clustered: boolean; clusterCounts: Map<string, number> } {
  if (rawNodes.length <= LOD_THRESHOLD) {
    return { nodes: rawNodes, links: rawLinks, clustered: false, clusterCounts: new Map() };
  }

  // Preserve critical nodes: start, end, and attack_step
  const criticalIds = new Set<string>();
  const clusterCounts = new Map<string, number>();
  const nodeToCluster = new Map<string, string>();

  rawNodes.forEach((n) => {
    const type = (n.type || 'unknown').toLowerCase().replace(/-/g, '_');
    if (n.isStart || n.isEnd || type === 'attack_step') {
      criticalIds.add(n.id);
    } else {
      const clusterId = `__cluster_${type}`;
      nodeToCluster.set(n.id, clusterId);
      clusterCounts.set(type, (clusterCounts.get(type) || 0) + 1);
    }
  });

  // Build super-nodes for each type cluster
  const clusterNodes: AttackPathGraphData['nodes'] = [];
  const seenClusters = new Set<string>();
  for (const [type, count] of clusterCounts) {
    const clusterId = `__cluster_${type}`;
    if (!seenClusters.has(clusterId)) {
      seenClusters.add(clusterId);
      const pathIds = rawNodes
        .filter((n) => nodeToCluster.get(n.id) === clusterId)
        .flatMap((n) => pathIdsFromDatum(n));
      clusterNodes.push({
        id: clusterId,
        label: `${count} ${type.replace(/_/g, ' ')}${count > 1 ? 's' : ''}`,
        type,
        pathIds: Array.from(new Set(pathIds)),
      });
    }
  }

  // Keep critical nodes
  const preservedNodes = rawNodes.filter((n) => criticalIds.has(n.id));
  const mergedNodes = [...preservedNodes, ...clusterNodes];
  const mergedNodeIds = new Set(mergedNodes.map((n) => n.id));

  // Remap links: replace clustered node references with cluster super-node IDs
  const seenLinks = new Set<string>();
  const mergedLinks: AttackPathGraphData['links'] = [];
  rawLinks.forEach((l) => {
    const src = nodeToCluster.get(l.source as string) ?? l.source;
    const tgt = nodeToCluster.get(l.target as string) ?? l.target;
    if (!mergedNodeIds.has(src as string) || !mergedNodeIds.has(tgt as string)) return;
    if (src === tgt) return; // skip self-loops within a cluster
    const key = `${src}->${tgt}-${l.type || ''}`;
    if (seenLinks.has(key)) {
      const existing = mergedLinks.find((m) => `${m.source}->${m.target}-${m.type || ''}` === key);
      if (existing) existing.pathIds = mergePathIds(existing.pathIds, l.pathIds);
      return;
    }
    seenLinks.add(key);
    mergedLinks.push({ ...l, source: src, target: tgt });
  });

  return { nodes: mergedNodes, links: mergedLinks, clustered: true, clusterCounts };
}

/* ─── component ───────────────────────────────────────────── */

/**
 * Renders an interactive force-directed attack path graph with D3. Supports LOD clustering for large graphs, persona-adaptive semantics (exploitability/crownjewel), and a native trust overlay layer. Includes zoom/pan controls, node drag, click callbacks, and accessibility labels.
 */
export const AttackPathGraph: React.FC<AttackPathGraphProps> = ({
  /** Graph data containing nodes and links */
  data,
  /** Explicit width override; otherwise uses container size (min 900) */
  width: propWidth,
  /** Explicit height override; otherwise uses container size (min 520) */
  height: propHeight,
  /** Callback when a node is clicked outside of drag operations. Returns [nodeId, type]. */
  onNodeClick,
  /** Set of node IDs to highlight visually without affecting simulation layout */
  highlightedNodeIds,
  /** Set of edge keys (source->target) to highlight; highlights step-index edges if present */
  highlightedEdgeKeys,
  focusNodeIds,
  /** Highlights the specified technique step index in red with thicker strokes. Null or <=0 disables highlighting. */
  highlightedStepIndex,
  className = '',
  semanticMode = 'exploitability',
  graphTrust,
  showTrustOverlay = true,
}) => {
  const trustCtx: GraphTrustContext = useMemo(
    () => ({
      semanticMode: graphTrust?.semanticMode ?? semanticMode,
      visibilityState: graphTrust?.visibilityState,
      telemetry: graphTrust?.telemetry,
      clustered: graphTrust?.clustered,
      crownJewelIds: graphTrust?.crownJewelIds,
      runtimeConfirmedNodeIds: graphTrust?.runtimeConfirmedNodeIds,
    }),
    [graphTrust, semanticMode],
  );

  const svgRef = useRef<SVGSVGElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const zoomBehaviorRef = useRef<d3.ZoomBehavior<SVGSVGElement, unknown> | null>(null);
  const fitGraphRef = useRef<((duration?: number) => void) | null>(null);
  const hadFocusRef = useRef(false);
  const [dimensions, setDimensions] = useState({ width: propWidth || 900, height: propHeight || 520 });
  const [legendOpen, setLegendOpen] = useState(false);
  const [forceFull, setForceFull] = useState(false);
  const [focusedNodeId, setFocusedNodeId] = useState<string | null>(null);
  const isLODPreview = !forceFull && data.nodes.length > LOD_THRESHOLD;
  const trustPosture = useMemo(
    () => buildGraphTrustPosture({ ...trustCtx, clustered: isLODPreview }, data.nodes.length, data.links.length),
    [trustCtx, data.nodes.length, data.links.length, isLODPreview],
  );
  const clipId = useId();
  const glowFilterId = `attack-node-glow-${clipId}`;
  const graphEpochRef = useRef(0);

  useEffect(() => {
    if (propWidth && propHeight) {
      setDimensions({ width: propWidth, height: propHeight });
      return;
    }
    const obs = new ResizeObserver((entries) => {
      const { width, height } = entries[0].contentRect;
      if (width > 0 && height > 0) setDimensions({ width, height: Math.max(height, 360) });
    });
    if (containerRef.current) obs.observe(containerRef.current);
    return () => obs.disconnect();
  }, [propWidth, propHeight]);

  const zoomBy = useCallback((factor: number) => {
    const svgEl = svgRef.current;
    const zoom = zoomBehaviorRef.current;
    if (!svgEl || !zoom) return;
    d3.select(svgEl).transition().duration(180).call(zoom.scaleBy, factor);
  }, []);

  const fitGraph = useCallback(() => {
    fitGraphRef.current?.(260);
  }, []);

  // Build simulation + geometry only when data/layout changes — NOT on highlight changes
  useEffect(() => {
    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();
    setFocusedNodeId(null);

    if (!data.nodes.length) return;

    // LOD clustering for large graphs
    const lod = forceFull ? { nodes: data.nodes, links: data.links, clustered: false, clusterCounts: new Map<string, number>() } : clusterForLOD(data.nodes, data.links);
    const renderTrustCtx: GraphTrustContext = { ...trustCtx, clustered: lod.clustered };
    const effectiveData = { nodes: lod.nodes, links: lod.links };

    graphEpochRef.current += 1;
    const epoch = graphEpochRef.current;

    const { width, height } = dimensions;
    const showDenseLabels = effectiveData.links.length <= 42 && effectiveData.nodes.length <= 55;

    const nodeMap = new Map<string, GraphNode>();
    effectiveData.nodes.forEach((n) => {
      nodeMap.set(n.id, { ...n } as GraphNode);
    });
    const nodes = Array.from(nodeMap.values());

    const rawFiltered = effectiveData.links.filter((l) => nodeMap.has(l.source as string) && nodeMap.has(l.target as string));
    const curveMap = assignCurveOffsets(rawFiltered);

    const links: GraphLink[] = rawFiltered.map((l, i) => ({
      ...l,
      _curveOffset: curveMap.get(i) ?? 0,
    })) as GraphLink[];

    const nCount = Math.max(nodes.length, 1);
    const viewportDiagonal = Math.sqrt(width * height);
    const linkDistance = Math.max(112, Math.min(260, viewportDiagonal / Math.max(4.2, Math.sqrt(nCount))));
    const collideRadius = Math.max(42, Math.min(76, 560 / Math.sqrt(nCount)));
    const chargeStrength = Math.max(-980, Math.min(-360, -82 * Math.sqrt(nCount)));
    const labelMax = graphNodeLabelMax(semanticMode, width);

    const defs = svg.append('defs');
    const gridSize = width < 640 ? 28 : 36;
    const gridId = `attack-grid-${clipId}`;
    defs
      .append('pattern')
      .attr('id', gridId)
      .attr('width', gridSize)
      .attr('height', gridSize)
      .attr('patternUnits', 'userSpaceOnUse')
      .append('path')
      .attr('d', `M ${gridSize} 0 L 0 0 0 ${gridSize}`)
      .attr('fill', 'none')
      .attr('stroke', GRAPH_THEME.muted)
      .attr('stroke-opacity', 0.08)
      .attr('stroke-width', 1);
    const glow = defs.append('filter').attr('id', glowFilterId).attr('x', '-70%').attr('y', '-70%').attr('width', '240%').attr('height', '240%');
    glow.append('feGaussianBlur').attr('stdDeviation', 3).attr('result', 'blur');
    glow.append('feMerge').selectAll('feMergeNode').data(['blur', 'SourceGraphic']).join('feMergeNode').attr('in', (d) => d);
    svg
      .append('rect')
      .attr('width', width)
      .attr('height', height)
      .attr('rx', 12)
      .attr('fill', `url(#${gridId})`);

    const g = svg.append('g').attr('class', 'zoom-root');

    const zoom = d3
      .zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.18, 5])
      .on('zoom', (event) => g.attr('transform', event.transform.toString()));
    zoomBehaviorRef.current = zoom;
    svg.call(zoom).on('dblclick.zoom', null);

    const markerTypes = [
      'PATH_ENTRY',
      'SERVICE_ACCOUNT_ACCESS',
      'RBAC_BINDING',
      'GRANTS_ROLE',
      'NETWORK_REACH',
      'NETWORK_REACH_SOFT',
      'CONTAINER_ESCAPE',
      'ESC_HOSTPATH_NODE',
      'ESC_HOSTPID',
      'HOST_ACCESS',
      'LATERAL_MOVE',
      'HAS_ATTACK_STEP',
      'CAN_STEAL_CREDENTIALS',
      'default',
    ];
    markerTypes.forEach((t) => {
      defs
        .append('marker')
        .attr('id', `arrow-${t}-${clipId}`)
        .attr('viewBox', '0 -5 10 10')
        .attr('refX', MARKER_REF_X)
        .attr('refY', 0)
        .attr('markerWidth', 6)
        .attr('markerHeight', 6)
        .attr('orient', 'auto')
        .append('path')
        .attr('d', 'M0,-5L10,0L0,5')
        .attr('fill', GRAPH_EDGE_COLORS[t] || GRAPH_THEME.muted);
    });

    const simulation = d3
      .forceSimulation<GraphNode>(nodes)
      .force('link', d3.forceLink<GraphNode, GraphLink>(links).id((d) => d.id).distance(linkDistance))
      .force('charge', d3.forceManyBody().strength(chargeStrength))
      .force('center', d3.forceCenter(width / 2, height / 2))
      .force('collision', d3.forceCollide().radius(collideRadius));

    const linkGroup = g.append('g').attr('class', 'links');
    const link = linkGroup
      .selectAll('path')
      .data(links)
      .join('path')
      .attr('fill', 'none')
      .attr('data-edge-key', (d) => edgeKeyFromDatum(d))
      .attr('data-step-index', (d) => ((d as GraphLink).stepIndex ?? ''))
      .attr('stroke', (d) => {
        return attackEdgeVisual(d as GraphLink, semanticMode, renderTrustCtx).stroke;
      })
      .attr('stroke-width', (d) => {
        return attackEdgeVisual(d as GraphLink, semanticMode, renderTrustCtx).strokeWidth;
      })
      .attr('stroke-opacity', (d) => {
        return attackEdgeVisual(d as GraphLink, semanticMode, renderTrustCtx).strokeOpacity;
      })
      .attr('stroke-dasharray', (d) => {
        return attackEdgeVisual(d as GraphLink, semanticMode, renderTrustCtx).strokeDasharray;
      })
      .attr('marker-end', (d) => `url(#arrow-${(d as GraphLink).type || 'default'}-${clipId})`);
    link.append('title').text((d) => {
      const t = (d as GraphLink).type || 'UNKNOWN';
      return `${t}: ${EDGE_DESCRIPTIONS[t] || 'Attack-path relationship edge.'}`;
    });

    const labelLinks = links.filter((l) => l.stepIndex !== undefined || showDenseLabels);
    const linkLabel = g
      .append('g')
      .attr('class', 'link-labels')
      .selectAll('g')
      .data(labelLinks)
      .join('g')
      .attr('class', 'link-label-group')
      .attr('data-edge-key', (d) => edgeKeyFromDatum(d));

    linkLabel
      .append('text')
      .text((d) => {
        const t = (d as GraphLink).type || '';
        const key = t.toUpperCase().trim();
        const lab = HUMAN_EDGE_LABELS[key] || t.replace(/_/g, ' ').toLowerCase();
        const step = (d as GraphLink).stepIndex;
        return step !== undefined ? `${step}. ${lab}` : lab;
      })
      .attr('font-size', 9)
      .attr('font-weight', (d) => ((d as GraphLink).stepIndex !== undefined ? 'bold' : 'normal'))
      .attr('fill', (d) => edgeStrokeColor(d))
      .attr('stroke', GRAPH_THEME.surfaceDeep)
      .attr('stroke-width', 3)
      .attr('stroke-linejoin', 'round')
      .style('paint-order', 'stroke fill')
      .attr('text-anchor', 'middle')
      .attr('dy', -6);

    const nodeGroup = g.append('g').attr('class', 'nodes');
    let nodeWasDragged = false;
    const node = nodeGroup
      .selectAll('g')
      .data(nodes)
      .join('g')
      .attr('data-node-id', (d) => d.id)
      .attr('role', 'button')
      .attr('tabindex', 0)
      .attr('aria-label', (d) => `Inspect ${d.label || d.id}, ${d.type.replace(/_/g, ' ')}`)
      .style('cursor', 'grab')
      .on('keydown.keyboard-node', (event: KeyboardEvent, d) => {
        if (event.key !== 'Enter' && event.key !== ' ') return;
        event.preventDefault();
        event.stopPropagation();
        onNodeClick?.(d.id, d.type);
      })
      .on('mouseenter.node-focus', (_event, d) => {
        setFocusedNodeId(d.id);
      })
      .on('mouseleave.node-focus', () => {
        setFocusedNodeId(null);
      })
      .on('focus.node-focus', (_event, d) => {
        setFocusedNodeId(d.id);
      })
      .on('blur.node-focus', () => {
        setFocusedNodeId(null);
      })
      .attr('opacity', 1)
      .call(
        d3
          .drag<SVGGElement, GraphNode>()
          .on('start', (event, d) => {
            nodeWasDragged = false;
            if (!event.active) simulation.alphaTarget(0.3).restart();
            d.fx = d.x;
            d.fy = d.y;
            d3.select(event.sourceEvent?.currentTarget ?? null).style('cursor', 'grabbing');
          })
          .on('drag', (event, d) => {
            nodeWasDragged = true;
            d.fx = event.x;
            d.fy = event.y;
          })
          .on('end', (event, d) => {
            if (!event.active) simulation.alphaTarget(0);
            d.fx = null;
            d.fy = null;
            d3.select(event.sourceEvent?.currentTarget ?? null).style('cursor', 'grab');
            window.setTimeout(() => {
              nodeWasDragged = false;
            }, 0);
          }),
      );

    node.each(function (d) {
      const el = d3.select(this);
      const { fill: baseFill, stroke: baseStroke } = nodeColorsForType(d.type);
      const nTrust = classifyNodeTrust(d, renderTrustCtx);
      const nVis = nodeTrustVisual(nTrust, baseFill, baseStroke);
      const color = nVis.fill;
      const strokeColor = nVis.stroke;
      const type = normalizeNodeType(d.type);

      if (type === 'path_entry') {
        el.append('circle')
          .attr('r', 10)
          .attr('fill', color)
          .attr('stroke', strokeColor)
          .attr('stroke-width', 2.5);
        el.append('path')
          .attr('d', 'M -3 -5 L 5 0 L -3 5 Z')
          .attr('fill', GRAPH_THEME.white)
          .attr('fill-opacity', 0.95);
      } else if (type === 'pod') {
        el.append('rect')
          .attr('x', -14)
          .attr('y', -14)
          .attr('width', 28)
          .attr('height', 28)
          .attr('rx', 4)
          .attr('ry', 4)
          .attr('fill', color)
          .attr('stroke', strokeColor)
          .attr('stroke-width', nVis.strokeWidth)
          .attr('stroke-dasharray', nVis.strokeDasharray)
          .attr('fill-opacity', nVis.fillOpacity);
      } else if (type === 'node') {
        el.append('polygon')
          .attr('points', '0,-16 13.8,-8 13.8,8 0,16 -13.8,8 -13.8,-8')
          .attr('fill', color)
          .attr('stroke', strokeColor)
          .attr('stroke-width', 2);
      } else if (type.includes('binding')) {
        el.append('rect')
          .attr('x', -12)
          .attr('y', -12)
          .attr('width', 24)
          .attr('height', 24)
          .attr('rx', 3)
          .attr('ry', 3)
          .attr('transform', 'rotate(45)')
          .attr('fill', color)
          .attr('stroke', strokeColor)
          .attr('stroke-width', 2);
      } else if (type.includes('role')) {
        el.append('path')
          .attr(
            'd',
            'M 0 -16 C 10 -16 16 -12 16 -12 C 16 -12 16 2 16 6 C 16 12 0 20 0 20 C 0 20 -16 12 -16 6 C -16 2 -16 -12 -16 -12 C -16 -12 -10 -16 0 -16 Z',
          )
          .attr('fill', color)
          .attr('stroke', strokeColor)
          .attr('stroke-width', 2);
      } else {
        el.append('circle').attr('r', 14).attr('fill', color).attr('stroke', strokeColor).attr('stroke-width', 2);
      }
    });

    node.each(function (d) {
      const nTrust = classifyNodeTrust(d, renderTrustCtx);
      const { fill: baseFill, stroke: baseStroke } = nodeColorsForType(d.type);
      const nVis = nodeTrustVisual(nTrust, baseFill, baseStroke);
      if (nVis.glowColor) {
        d3.select(this)
          .append('circle')
          .attr('r', 26)
          .attr('fill', 'none')
          .attr('stroke', nVis.glowColor)
          .attr('stroke-width', 2)
          .attr('stroke-opacity', 0.55)
          .attr('stroke-dasharray', nTrust === 'runtime_confirmed' ? null : '4 2')
          .attr('filter', `url(#${glowFilterId})`);
      } else if (d.risk && d.risk !== 'low') {
        d3.select(this)
          .append('circle')
          .attr('r', 26)
          .attr('fill', 'none')
          .attr('stroke', RISK_GLOW[d.risk || 'low'] || GRAPH_THEME.success)
          .attr('stroke-width', 1.5)
          .attr('stroke-opacity', 0.5)
          .attr('stroke-dasharray', '4 2');
      }
    });

    node
      .append('text')
      .text((d) => truncateLabel(d.label, labelMax))
      .attr('font-size', width < 480 ? 9 : 10)
      .attr('fill', GRAPH_THEME.text)
      .attr('stroke', GRAPH_THEME.surfaceDeep)
      .attr('stroke-width', 3)
      .attr('stroke-linejoin', 'round')
      .style('paint-order', 'stroke fill')
      .attr('text-anchor', 'middle')
      .attr('dy', 28);

    const badgeGroup = node
      .filter((d) => d.isStart || d.isEnd)
      .append('g')
      .attr('class', 'role-badge')
      .attr('transform', 'translate(0, -36)');

    badgeGroup
      .append('rect')
      .attr('fill', (d) => (d.isStart ? GRAPH_THEME.info : GRAPH_THEME.critical))
      .attr('rx', 3)
      .attr('ry', 3)
      .attr('width', (d) => (d.isStart ? 34 : 28))
      .attr('height', 14)
      .attr('x', (d) => (d.isStart ? -17 : -14))
      .attr('y', 0);

    badgeGroup
      .append('text')
      .text((d) => (d.isStart ? 'START' : 'END'))
      .attr('font-size', 8)
      .attr('font-weight', 'bold')
      .attr('fill', GRAPH_THEME.white)
      .attr('text-anchor', 'middle')
      .attr('dy', 10);

    node
      .append('text')
      .text((d) => (normalizeNodeType(d.type) === 'path_entry' ? 'path marker' : d.type.replace(/binding$/, 'Bind')))
      .attr('font-size', 7)
      .attr('fill', GRAPH_THEME.muted)
      .attr('stroke', GRAPH_THEME.surfaceDeep)
      .attr('stroke-width', 2)
      .attr('stroke-linejoin', 'round')
      .style('paint-order', 'stroke fill')
      .attr('text-anchor', 'middle')
      .attr('dy', (d) => (d.isStart || d.isEnd ? -48 : -22));

    node.append('title').text(
      (d) => `${d.label}\nType: ${d.type.replace(/_/g, ' ')}\nPath signal: ${(d.risk || 'low').toUpperCase()}`,
    );

    node
      .filter((d) => normalizeNodeType(d.type) === 'path_entry')
      .append('circle')
      .attr('class', 'path-entry-hit-target')
      .attr('r', 22)
      .attr('fill', 'transparent')
      .style('pointer-events', 'all')
      .style('cursor', 'pointer')
      .on('click.focus-path', (event, d) => {
        event.preventDefault();
        event.stopPropagation();
        onNodeClick?.(d.id, d.type);
      });

    let pathEntryPointerDown: { id: string; x: number; y: number } | null = null;
    node
      .on('pointerdown.focus-path', (event, d) => {
        if (normalizeNodeType(d.type) !== 'path_entry') return;
        const [x, y] = d3.pointer(event, svgRef.current);
        pathEntryPointerDown = { id: d.id, x, y };
      })
      .on('pointerup.focus-path', (event, d) => {
        if (normalizeNodeType(d.type) !== 'path_entry' || !pathEntryPointerDown || pathEntryPointerDown.id !== d.id) return;
        const [x, y] = d3.pointer(event, svgRef.current);
        const moved = Math.hypot(x - pathEntryPointerDown.x, y - pathEntryPointerDown.y);
        pathEntryPointerDown = null;
        if (moved > 4) return;
        event.preventDefault();
        event.stopPropagation();
        onNodeClick?.(d.id, d.type);
      });

    node.on('click', (event, d) => {
      if (nodeWasDragged) {
        event.preventDefault();
        event.stopPropagation();
        return;
      }
      if (normalizeNodeType(d.type) === 'path_entry') return;
      onNodeClick?.(d.id, d.type);
    });

    simulation.on('tick', () => {
      const margin = width < 640 ? 42 : 58;
      nodes.forEach((d) => {
        if (d.x != null) d.x = Math.max(margin, Math.min(width - margin, d.x));
        if (d.y != null) d.y = Math.max(margin, Math.min(height - margin, d.y));
      });

      link.attr('d', (d) => curvedPath(d as GraphLink));

      linkLabel.attr('transform', (d) => {
        const [lx, ly] = labelAnchor(d as GraphLink);
        return `translate(${lx},${ly})`;
      });

      node.attr('transform', (d) => `translate(${d.x},${d.y})`);
    });

    const fitToView = (duration = 420) => {
      if (epoch !== graphEpochRef.current) return;
      const bounds = (g.node() as SVGGElement)?.getBBox();
      if (bounds) {
        const padding = width < 640 ? 58 : 72;
        const scale = Math.min(
          width / (bounds.width + padding * 2),
          height / (bounds.height + padding * 2),
          1.35,
        );
        const tx = width / 2 - (bounds.x + bounds.width / 2) * scale;
        const ty = height / 2 - (bounds.y + bounds.height / 2) * scale;
        svg.transition().duration(duration).call(zoom.transform, d3.zoomIdentity.translate(tx, ty).scale(scale));
      }
    };
    fitGraphRef.current = fitToView;
    const fitTimer = window.setTimeout(() => fitToView(260), 900);
    simulation.on('end', () => fitToView(500));

    return () => {
      window.clearTimeout(fitTimer);
      simulation.stop();
      if (epoch === graphEpochRef.current) {
        fitGraphRef.current = null;
      }
    };
  }, [data, dimensions, clipId, onNodeClick, forceFull, semanticMode, trustCtx]);

  // Update highlight styles without tearing down the simulation
  useEffect(() => {
    const svgEl = svgRef.current;
    if (!svgEl || !data.nodes.length) return;

    const root = d3.select(svgEl).select('g.zoom-root');
    if (root.empty()) return;

    const activeNodeIds =
      highlightedNodeIds && highlightedNodeIds.size > 0
        ? highlightedNodeIds
        : focusNodeIds && focusNodeIds.size > 0
          ? focusNodeIds
          : undefined;
    const hasPathHL =
      (activeNodeIds && activeNodeIds.size > 0) ||
      (highlightedNodeIds && highlightedNodeIds.size > 0) ||
      (highlightedEdgeKeys && highlightedEdgeKeys.size > 0);
    const stepIdx = highlightedStepIndex != null && highlightedStepIndex > 0 ? highlightedStepIndex : null;
    const focusId = focusedNodeId;
    const interactionNodeIds = new Set<string>();
    const focusEdgeKeys = new Set<string>();

    if (focusId) {
      const renderedNodes = new Map<string, GraphNode>();
      root
        .select('.nodes')
        .selectAll<SVGGElement, GraphNode>('g[data-node-id]')
        .each(function (d) {
          renderedNodes.set(d.id, d);
        });
      const focusPathIds = new Set(pathIdsFromDatum(renderedNodes.get(focusId)));

      if (focusPathIds.size > 0) {
        for (const nodeDatum of renderedNodes.values()) {
          if (pathIdsFromDatum(nodeDatum).some((pathId) => focusPathIds.has(pathId))) {
            interactionNodeIds.add(nodeDatum.id);
          }
        }
      } else {
        const adjacency = new Map<string, Set<string>>();
        root
          .select('.links')
          .selectAll<SVGPathElement, GraphLink>('path[data-edge-key]')
          .each(function (d) {
            const sourceId = endpointId(d.source, '');
            const targetId = endpointId(d.target, '');
            if (!sourceId || !targetId) return;
            if (!adjacency.has(sourceId)) adjacency.set(sourceId, new Set());
            if (!adjacency.has(targetId)) adjacency.set(targetId, new Set());
            adjacency.get(sourceId)!.add(targetId);
            adjacency.get(targetId)!.add(sourceId);
          });
        const queue = [focusId];
        interactionNodeIds.add(focusId);
        for (let i = 0; i < queue.length; i += 1) {
          const current = queue[i];
          for (const next of adjacency.get(current) ?? []) {
            if (interactionNodeIds.has(next)) continue;
            interactionNodeIds.add(next);
            queue.push(next);
          }
        }
      }

      interactionNodeIds.add(focusId);
      root
        .select('.links')
        .selectAll<SVGPathElement, GraphLink>('path[data-edge-key]')
        .each(function (d) {
          const sourceId = endpointId(d.source, '');
          const targetId = endpointId(d.target, '');
          const pathHit =
            focusPathIds.size > 0 &&
            pathIdsFromDatum(d).some((pathId) => focusPathIds.has(pathId));
          const componentHit =
            focusPathIds.size === 0 &&
            sourceId &&
            targetId &&
            interactionNodeIds.has(sourceId) &&
            interactionNodeIds.has(targetId);
          if (!pathHit && !componentHit) return;
          focusEdgeKeys.add(edgeKeyFromDatum(d));
          if (sourceId) interactionNodeIds.add(sourceId);
          if (targetId) interactionNodeIds.add(targetId);
        });
    }

    root
      .select('.nodes')
      .selectAll<SVGGElement, GraphNode>('g[data-node-id]')
      .attr('opacity', function () {
        const id = this.getAttribute('data-node-id') || '';
        if (focusId) return interactionNodeIds.has(id) ? 1 : 0.12;
        if (!hasPathHL) return 1;
        return activeNodeIds?.has(id) ? 1 : 0.16;
      })
      .attr('filter', function () {
        const id = this.getAttribute('data-node-id') || '';
        if (focusId && id === focusId) return `url(#${glowFilterId})`;
        return hasPathHL && activeNodeIds?.has(id) ? `url(#${glowFilterId})` : null;
      });

    root
      .select('.links')
      .selectAll<SVGPathElement, GraphLink>('path[data-edge-key]')
      .attr('stroke-opacity', function (d) {
        const key = edgeKeyFromDatum(d);
        const step = d.stepIndex;
        let op = 0.75;
        if (focusId) {
          op = focusEdgeKeys.has(key) ? 1 : 0.06;
        } else if (hasPathHL) {
          op = highlightedEdgeKeys?.has(key) ? 1 : 0.14;
        }
        if (stepIdx != null && step === stepIdx) {
          if (focusId) {
            op = focusEdgeKeys.has(key) ? 1 : 0.06;
          } else {
            op = hasPathHL ? (highlightedEdgeKeys?.has(key) ? 1 : 0.14) : 1;
          }
        }
        return op;
      })
      .attr('stroke-width', function (d) {
        const key = edgeKeyFromDatum(d);
        const step = d.stepIndex;
        let w = step !== undefined ? 2.25 : 1.1;
        if (stepIdx != null && step === stepIdx) {
          w = Math.max(w, 4);
        }
        if (focusId) {
          return focusEdgeKeys.has(key) ? Math.max(w, 3.25) : Math.min(w, 1.1);
        }
        if (hasPathHL && highlightedEdgeKeys && !highlightedEdgeKeys.has(key)) {
          w = Math.min(w, 1.2);
        }
        return w;
      });

    root
      .select('.link-labels')
      .selectAll<SVGGElement, GraphLink>('g[data-edge-key]')
      .attr('opacity', function (d) {
        const key = edgeKeyFromDatum(d);
        const step = d.stepIndex;
        if (focusId) {
          return focusEdgeKeys.has(key) ? 1 : 0.08;
        }
        if (hasPathHL) {
          return highlightedEdgeKeys?.has(key) ? 1 : 0.14;
        }
        if (stepIdx != null && step === stepIdx) return 1;
        if (stepIdx != null) return 0.25;
        return 1;
      });
  }, [data.nodes.length, data.links.length, highlightedNodeIds, highlightedEdgeKeys, focusNodeIds, highlightedStepIndex, focusedNodeId, glowFilterId]);

  useEffect(() => {
    const svgEl = svgRef.current;
    const zoom = zoomBehaviorRef.current;
    if (!svgEl || !zoom || !data.nodes.length) return;

    if (!focusNodeIds || focusNodeIds.size === 0) {
      if (hadFocusRef.current) {
        hadFocusRef.current = false;
        fitGraphRef.current?.(260);
      }
      return;
    }

    const selected = Array.from(svgEl.querySelectorAll<SVGGElement>('g.zoom-root .nodes g[data-node-id]'))
      .filter((el) => focusNodeIds.has(el.getAttribute('data-node-id') || ''));
    if (selected.length === 0) return;

    let minX = Infinity;
    let minY = Infinity;
    let maxX = -Infinity;
    let maxY = -Infinity;
    selected.forEach((el) => {
      const box = el.getBBox();
      minX = Math.min(minX, box.x);
      minY = Math.min(minY, box.y);
      maxX = Math.max(maxX, box.x + box.width);
      maxY = Math.max(maxY, box.y + box.height);
    });
    if (!Number.isFinite(minX) || !Number.isFinite(minY) || !Number.isFinite(maxX) || !Number.isFinite(maxY)) return;

    hadFocusRef.current = true;
    const width = dimensions.width;
    const height = dimensions.height;
    const padding = width < 640 ? 72 : 96;
    const boxWidth = Math.max(1, maxX - minX);
    const boxHeight = Math.max(1, maxY - minY);
    const scale = Math.max(
      0.35,
      Math.min(width / (boxWidth + padding * 2), height / (boxHeight + padding * 2), 2.2),
    );
    const tx = width / 2 - (minX + boxWidth / 2) * scale;
    const ty = height / 2 - (minY + boxHeight / 2) * scale;
    d3.select(svgEl)
      .transition()
      .duration(300)
      .call(zoom.transform, d3.zoomIdentity.translate(tx, ty).scale(scale));
  }, [data.nodes.length, dimensions, focusNodeIds]);

  if (!data.nodes.length) {
    return (
      <div className={`flex items-center justify-center py-16 text-muted ${className}`}>
        <div className="text-center">
          <p className="text-body font-medium">No attack path found</p>
          <p className="text-caption mt-1 opacity-75">No RBAC binding data leading to a sensitive privilege yet.</p>
        </div>
      </div>
    );
  }

  const isLOD = !forceFull && data.nodes.length > LOD_THRESHOLD;

  const foldedTypeSummary = useMemo(() => {
    if (!isLOD) return '';
    const { clusterCounts } = clusterForLOD(data.nodes, data.links);
    return [...clusterCounts.entries()]
      .sort((a, b) => b[1] - a[1])
      .slice(0, 4)
      .map(([type, count]) => `${count} ${type.replace(/_/g, ' ')}`)
      .join(', ');
  }, [data.links, data.nodes, isLOD]);

  const ariaLabel = `Attack path graph with ${data.nodes.length} nodes and ${data.links.length} edges. ` +
    `${data.nodes.filter(n => n.isStart).length} entry points, ` +
    `${data.nodes.filter(n => n.isEnd).length} targets. ` +
    (isLOD
      ? 'Showing clustered view. Some intermediate identity and relationship edges are summarized.'
      : 'Interactive: drag nodes, scroll to zoom.');
  const outlineNodes = data.nodes.slice(0, 12);
  const outlineNodeLabels = new Map(data.nodes.map((node) => [node.id, node.label || node.id]));
  const outlineLinks = data.links.slice(0, 12);

  return (
    <div ref={containerRef} className={`relative w-full ${className}`} style={{ minHeight: 400 }}>
      {showTrustOverlay ? (
        <GraphTrustOverlay posture={trustPosture} className="absolute top-3 left-3 z-30 max-w-xs hidden md:block" />
      ) : null}
      {isLOD && (
        <div className="absolute top-2 left-1/2 -translate-x-1/2 z-30 flex max-w-[min(100%,42rem)] items-center gap-2 rounded-lg border border-amber-500/30 bg-amber-950/80 backdrop-blur px-3 py-1.5 text-caption text-amber-200 shadow-lg">
          <span>
            Clustered view ({data.nodes.length} raw nodes): entry, target, and attack-step nodes stay exact; folded types include
            {foldedTypeSummary ? ` ${foldedTypeSummary}.` : ' service accounts, roles, and bindings.'} Verify identity/RBAC edges before containment.
          </span>
          <button
            type="button"
            onClick={() => setForceFull(true)}
            className="text-amber-300 hover:text-amber-100 font-semibold underline underline-offset-2"
          >
            Show all nodes
          </button>
        </div>
      )}
      {forceFull && data.nodes.length > LOD_THRESHOLD && (
        <div className="absolute top-2 left-1/2 -translate-x-1/2 z-30 flex items-center gap-2 rounded-lg border border-border bg-surface/90 backdrop-blur px-3 py-1.5 text-caption text-muted shadow-lg">
          <span>Full graph ({data.nodes.length} nodes) may be slow. Scenario Focus is safer for exact chain review.</span>
          <button
            type="button"
            onClick={() => setForceFull(false)}
            className="text-brand hover:text-text font-semibold underline underline-offset-2"
          >
            Cluster view
          </button>
        </div>
      )}
      <svg
        ref={svgRef}
        width={dimensions.width}
        height={dimensions.height}
        className="rounded-xl border border-border bg-base shadow-inner cursor-grab active:cursor-grabbing select-none touch-none"
        role="img"
        aria-label={ariaLabel}
      >
        <title>{ariaLabel}</title>
      </svg>
      <div className="absolute right-3 top-3 z-30 flex items-center gap-1 rounded-lg border border-border bg-surface/95 p-1 shadow-lg backdrop-blur">
        <button
          type="button"
          onClick={() => zoomBy(1.22)}
          className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted transition-colors hover:bg-surface-2 hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/60"
          aria-label="Zoom in attack path graph"
          title="Zoom in"
        >
          <Plus size={15} />
        </button>
        <button
          type="button"
          onClick={() => zoomBy(0.82)}
          className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted transition-colors hover:bg-surface-2 hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/60"
          aria-label="Zoom out attack path graph"
          title="Zoom out"
        >
          <Minus size={15} />
        </button>
        <button
          type="button"
          onClick={fitGraph}
          className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted transition-colors hover:bg-surface-2 hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/60"
          aria-label="Fit attack path graph to view"
          title="Fit to view"
        >
          <Maximize2 size={15} />
        </button>
      </div>
      <div className="pointer-events-none absolute bottom-3 right-3 z-20 hidden items-center gap-1.5 rounded-lg border border-border/80 bg-base/80 px-2.5 py-1.5 text-caption text-muted shadow-lg backdrop-blur md:flex">
        <Move size={13} />
        Drag nodes · scroll to zoom · Fit resets
      </div>
      <details className="mt-3 rounded-lg border border-border bg-surface/80 p-3 text-caption text-muted">
        <summary className="cursor-pointer text-body font-semibold text-text">Keyboard graph outline</summary>
        <div className="mt-3 grid gap-3 md:grid-cols-2">
          <div>
            <h3 className="text-caption font-semibold text-text">Nodes</h3>
            <ul className="mt-2 space-y-1">
              {outlineNodes.map((node) => (
                <li key={node.id}>
                  <button
                    type="button"
                    className="text-left text-muted transition-colors hover:text-text focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/60"
                    onClick={() => onNodeClick?.(node.id, node.type)}
                  >
                    <span className="font-medium text-text">{node.label || node.id}</span>
                    <span className="text-muted"> · {node.type.replace(/_/g, ' ')}</span>
                  </button>
                </li>
              ))}
            </ul>
          </div>
          <div>
            <h3 className="text-caption font-semibold text-text">Relationships</h3>
            <ol className="mt-2 space-y-1">
              {outlineLinks.map((link, index) => {
                const sourceId = endpointId(link.source, 'source');
                const targetId = endpointId(link.target, 'target');
                return (
                  <li key={`${sourceId}-${targetId}-${index}`}>
                    <span className="font-medium text-text">{outlineNodeLabels.get(sourceId) ?? sourceId}</span>
                    <span> to </span>
                    <span className="font-medium text-text">{outlineNodeLabels.get(targetId) ?? targetId}</span>
                    <span className="text-muted"> · {(link.type ?? 'relationship').replace(/_/g, ' ')}</span>
                  </li>
                );
              })}
            </ol>
          </div>
        </div>
        {data.nodes.length > outlineNodes.length || data.links.length > outlineLinks.length ? (
          <p className="mt-3 text-muted-2">
            Showing the first {outlineNodes.length} nodes and {outlineLinks.length} relationships. Use scenario filters to narrow the graph.
          </p>
        ) : null}
      </details>
      <div className="absolute bottom-2 left-2 right-2 z-20 md:z-auto md:bottom-3 md:left-3 md:right-auto">
        <button
          type="button"
          aria-expanded={legendOpen}
          onClick={() => setLegendOpen((v) => !v)}
          className="md:hidden mb-1 flex items-center gap-1 rounded-lg border border-border bg-surface/95 px-2 py-1 text-micro font-semibold uppercase tracking-wide text-muted shadow-lg pointer-events-auto"
        >
          Node legend
          {legendOpen ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
        </button>
        <div
          className={`flex flex-wrap gap-x-2 gap-y-1 md:gap-2 text-caption text-muted rounded-lg border border-border/80 bg-base/90 p-2 shadow-lg backdrop-blur max-h-[min(36dvh,220px)] overflow-y-auto overscroll-contain md:max-h-none md:max-w-[42rem] ${legendOpen ? 'flex' : 'hidden md:flex'} pointer-events-auto`}
        >
          {NODE_LEGEND_TYPES.map((type) => (
            <span key={type} className="flex items-center gap-0.5 md:gap-1 shrink-0">
              <span style={nodeLegendStyle(type)} />
              {type.replace(/_/g, ' ').replace(/^(.)/, (c) => c.toUpperCase())}
            </span>
          ))}
        </div>
      </div>
    </div>
  );
};

function truncateLabel(text: string, max: number): string {
  if (!text) return '';
  return text.length <= max ? text : text.slice(0, max - 1) + '…';
}
