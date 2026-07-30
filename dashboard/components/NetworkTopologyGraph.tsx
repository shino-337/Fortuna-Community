import React, { useCallback, useEffect, useId, useMemo, useRef, useState } from 'react';
import * as d3 from 'd3';
import type {
  NetworkActivityConnectionRow,
  NetworkActivityDestinationRow,
  NetworkActivityTalkerRow,
} from '../types';
import type { GraphTrustContext, GraphTrustPosture } from '../lib/graphTrustSemantics';
import {
  buildGraphTrustPosture,
  classifyEdgeTrust,
  classifyNodeTrust,
} from '../lib/graphTrustSemantics';
import { edgeTrustVisual, nodeTrustVisual } from '../lib/graphTrustRendering';
import { GraphTrustOverlay } from './GraphTrustOverlay';
import { ATTACK_NODE_COLORS, GRAPH_EDGE_COLORS, GRAPH_THEME } from '../lib/graphTheme';

/* ─── types ───────────────────────────────────────────────────── */

interface NodeDatum extends d3.SimulationNodeDatum {
  id: string;
  /** Line 1: pod or destination name (avoid UID as primary when a name exists) */
  line1: string;
  /** Line 2: ns + uid hint (pod) or external / destination ns */
  line2: string;
  /** Full tooltip label */
  fullLabel: string;
  kind: 'pod' | 'dest';
  destType?: 'internal_workload' | 'internal_service' | 'external_endpoint';
  weight: number;
  namespace?: string;
  ownerLabel?: string;
}

interface LinkDatum extends d3.SimulationLinkDatum<NodeDatum> {
  value: number;
}

export interface NetworkTopologySelection {
  id: string;
  kind: 'pod' | 'dest';
  typeLabel: string;
  line1: string;
  line2: string;
  fullLabel: string;
  namespace?: string;
  ownerLabel?: string;
  weight: number;
}

/** Particles along edges — visual only, not NetworkPolicy */
interface ParticleDatum {
  linkIndex: number;
  phase: number;
  speed: number;
}

export interface NetworkTopologyGraphProps {
  destinations: NetworkActivityDestinationRow[];
  talkers: NetworkActivityTalkerRow[];
  /** Real connection rows (same API filters); preferred for pod→destination edges matching namespace/filters. */
  connections?: NetworkActivityConnectionRow[];
  /** Pods with activity (talkers) not in the current edge set — add orphan pod affordances to the graph. */
  supplementTalkers?: NetworkActivityTalkerRow[];
  maxNodes?: number;
  onNodeClick?: (nodeId: string, kind: 'pod' | 'dest') => void;
  showCompactLegend?: boolean;
  className?: string;
  /** Traffic simulation: green dots along edges (decorative). Off by default for analyst mode. */
  showTrafficParticles?: boolean;
  /** uid → name from GET /inventory/pods (fill when edges/talkers lack podName from JOIN). */
  podNamesByUid?: Readonly<Record<string, string>>;
  /** Trust / uncertainty semantics (parity with AttackPathGraph). */
  graphTrust?: GraphTrustContext;
  showTrustOverlay?: boolean;
  onSelectionChange?: (selection: NetworkTopologySelection | null) => void;
}

/* ─── helpers ─────────────────────────────────────────────────── */

const MAX_LINKS_PER_DESTINATION = 5;

/** Network topology uses the same graph color language as AttackPathGraph. */
const NETWORK_TOPOLOGY_PALETTE = {
  podFill: ATTACK_NODE_COLORS.pod.fill,
  podStroke: ATTACK_NODE_COLORS.pod.stroke,
  internalFill: ATTACK_NODE_COLORS.role.fill,
  internalStroke: ATTACK_NODE_COLORS.role.stroke,
  externalFill: ATTACK_NODE_COLORS.secret.fill,
  externalStroke: ATTACK_NODE_COLORS.secret.stroke,
  edgeDefault: GRAPH_EDGE_COLORS.NETWORK_REACH,
  edgeInternal: GRAPH_EDGE_COLORS.NETWORK_REACH,
  edgeExternal: GRAPH_EDGE_COLORS.NETWORK_REACH_SOFT,
  edgeMuted: GRAPH_THEME.border,
  edgeFocus: GRAPH_THEME.text,
  particle: GRAPH_EDGE_COLORS.NETWORK_REACH,
  particleCore: GRAPH_THEME.white,
} as const;

const COLOR_POD_FILL = NETWORK_TOPOLOGY_PALETTE.podFill;
const COLOR_POD_STROKE = NETWORK_TOPOLOGY_PALETTE.podStroke;
const COLOR_DEST_INTERNAL_FILL = NETWORK_TOPOLOGY_PALETTE.internalFill;
const COLOR_DEST_INTERNAL_STROKE = NETWORK_TOPOLOGY_PALETTE.internalStroke;
const COLOR_DEST_EXTERNAL_FILL = NETWORK_TOPOLOGY_PALETTE.externalFill;
const COLOR_DEST_EXTERNAL_STROKE = NETWORK_TOPOLOGY_PALETTE.externalStroke;
/** Observed traffic flow (aggregate API, not NetworkPolicy allow/deny). */
const COLOR_LINK_DIM = NETWORK_TOPOLOGY_PALETTE.edgeMuted;
const COLOR_LINK_HOVER = NETWORK_TOPOLOGY_PALETTE.edgeFocus;
const COLOR_PARTICLE = NETWORK_TOPOLOGY_PALETTE.particle;
const COLOR_PARTICLE_CORE = NETWORK_TOPOLOGY_PALETTE.particleCore;
const BG = GRAPH_THEME.surfaceDeep;

/** Cap particle count for light rAF (~2 particles per edge, max 55 edges) */
const MAX_LINK_INDICES_FOR_PARTICLES = 55;

/**
 * Node radius (SVG units before zoom-to-fit).
 * Kept compact for dense topology screens; fitTopologyToView still frames the full graph + labels.
 */
const NODE_RADIUS_MIN = 6;
const NODE_RADIUS_MAX = 17;
/** Force collision padding = radius + pad (avoid overlapping labels/nodes). */
const NODE_COLLISION_EXTRA = 22;
/** Bbox padding when fitting: circle + two lines of text below. */
const NODE_FIT_PADDING = 42;
const NODE_STROKE_DEFAULT = 1.7;
const NODE_STROKE_SELECTED = 2.8;
const PRIMARY_LABEL_LIMIT = 84;
const SECONDARY_LABEL_LIMIT = 44;

function destinationIsInternal(node: NodeDatum): boolean {
  if (node.kind !== 'dest') return false;
  return (
    node.destType === 'internal_workload' ||
    node.destType === 'internal_service' ||
    Boolean(node.namespace && node.namespace.trim() && node.line2 !== 'external')
  );
}

function nodeTypeLabel(node: NodeDatum): string {
  if (node.kind === 'pod') return 'Pod source';
  if (node.destType === 'internal_service') return 'Cluster service';
  if (node.destType === 'internal_workload') return 'Pod endpoint';
  return destinationIsInternal(node) ? 'Internal endpoint' : 'External endpoint';
}

function nodeBaseFill(node: NodeDatum): string {
  if (node.kind === 'pod') return COLOR_POD_FILL;
  return destinationIsInternal(node) ? COLOR_DEST_INTERNAL_FILL : COLOR_DEST_EXTERNAL_FILL;
}

function nodeBaseStroke(node: NodeDatum): string {
  if (node.kind === 'pod') return COLOR_POD_STROKE;
  return destinationIsInternal(node) ? COLOR_DEST_INTERNAL_STROKE : COLOR_DEST_EXTERNAL_STROKE;
}

function endpointNode(endpoint: LinkDatum['source'] | LinkDatum['target']): NodeDatum | null {
  return typeof endpoint === 'object' && endpoint != null && 'id' in endpoint
    ? endpoint as NodeDatum
    : null;
}

function endpointId(endpoint: LinkDatum['source'] | LinkDatum['target']): string | null {
  if (typeof endpoint === 'string') return endpoint;
  return endpointNode(endpoint)?.id ?? null;
}

function linkDestinationNode(link: LinkDatum, nodesById?: ReadonlyMap<string, NodeDatum>): NodeDatum | null {
  const target = endpointNode(link.target);
  if (target?.kind === 'dest') return target;
  const source = endpointNode(link.source);
  if (source?.kind === 'dest') return source;

  const targetId = endpointId(link.target);
  if (targetId) {
    const node = nodesById?.get(targetId);
    if (node?.kind === 'dest') return node;
  }

  const sourceId = endpointId(link.source);
  if (sourceId) {
    const node = nodesById?.get(sourceId);
    if (node?.kind === 'dest') return node;
  }

  return null;
}

function linkBaseStroke(link: LinkDatum, nodesById?: ReadonlyMap<string, NodeDatum>): string {
  const dest = linkDestinationNode(link, nodesById);
  if (!dest) return NETWORK_TOPOLOGY_PALETTE.edgeDefault;
  return destinationIsInternal(dest)
    ? NETWORK_TOPOLOGY_PALETTE.edgeInternal
    : NETWORK_TOPOLOGY_PALETTE.edgeExternal;
}

function mixGraphColor(from: string, to: string, t: number): string {
  return d3.interpolateRgb.gamma(2.2)(from, to)(Math.max(0, Math.min(1, t)));
}

function linkStrokeColor(
  link: LinkDatum,
  maxLinkVal: number,
  nodesById?: ReadonlyMap<string, NodeDatum>,
): string {
  const base = linkBaseStroke(link, nodesById);
  const norm = linkWeightNorm(link.value, maxLinkVal);
  if (norm < 0.18) return mixGraphColor(COLOR_LINK_DIM, base, 0.42);
  return mixGraphColor(base, COLOR_LINK_HOVER, 0.08 + 0.18 * norm);
}

function linkFocusStrokeColor(link: LinkDatum, nodesById?: ReadonlyMap<string, NodeDatum>): string {
  return mixGraphColor(linkBaseStroke(link, nodesById), COLOR_LINK_HOVER, 0.36);
}

function linkStrokeWithTrust(
  link: LinkDatum,
  maxLinkVal: number,
  nodesById?: ReadonlyMap<string, NodeDatum>,
  trustStyle?: { stroke: string; trustKind?: string },
): string {
  const semanticStroke = linkStrokeColor(link, maxLinkVal, nodesById);
  if (!trustStyle) return semanticStroke;
  if (trustStyle.trustKind === 'hidden_by_policy' || trustStyle.trustKind === 'hidden_by_scope') {
    return trustStyle.stroke;
  }
  return semanticStroke;
}

function linkDasharrayWithTrust(trustStyle?: { trustKind?: string; strokeDasharray?: string | null }): string | null {
  if (!trustStyle) return null;
  if (trustStyle.trustKind === 'hidden_by_policy' || trustStyle.trustKind === 'hidden_by_scope') {
    return trustStyle.strokeDasharray ?? '2 6';
  }
  // Runtime network topology is built from observed connection rows. Keep visible flow links solid;
  // trust posture is shown in the overlay/status instead of making observed edges look inferred.
  return null;
}

function truncateGraphLabel(text: string, max = 26): string {
  const t = text.trim();
  if (t.length <= max) return t;
  return `${t.slice(0, max - 1)}…`;
}

/** First 8 characters + … if longer — always shown on the secondary line for pods. */
function uidFootprint(uid: string): string {
  if (!uid) return '—';
  if (uid.length <= 8) return uid;
  return `${uid.slice(0, 8)}…`;
}

type PodMetaPatch = {
  namespace?: string;
  name?: string;
  ownerKind?: string;
  ownerName?: string;
};

/** Merge metadata by uid: prefer non-empty podName/namespace from any row (avoid missing name on first line). */
function mergePodMeta(
  map: Map<string, PodMetaPatch>,
  uid: string,
  patch: PodMetaPatch,
): void {
  const prev = map.get(uid);
  const nameMerged = (patch.name ?? '').trim() || (prev?.name ?? '').trim();
  const namespaceMerged = (patch.namespace ?? '').trim() || (prev?.namespace ?? '').trim();
  const ownerKindMerged = (patch.ownerKind ?? '').trim() || (prev?.ownerKind ?? '').trim();
  const ownerNameMerged = (patch.ownerName ?? '').trim() || (prev?.ownerName ?? '').trim();
  map.set(uid, {
    name: nameMerged || undefined,
    namespace: namespaceMerged || undefined,
    ownerKind: ownerKindMerged || undefined,
    ownerName: ownerNameMerged || undefined,
  });
}

/** Pod node line 1: pod identity only. Owner/controller stays in secondary metadata. */
function podNodeLine1(
  name: string | undefined,
  uid: string,
): string {
  const n = (name ?? '').trim();
  if (n) return truncateGraphLabel(n, 28);
  return truncateGraphLabel(`Pod ${uidFootprint(uid)}`, 28);
}

function podNodeFullLabel(
  name: string | undefined,
  ns: string,
  uid: string,
  ownerKind?: string,
  ownerName?: string,
): string {
  const n = (name ?? '').trim();
  const parts: string[] = [];
  if (n) parts.push(n);
  const ok = (ownerKind ?? '').trim();
  const on = (ownerName ?? '').trim();
  if (ok || on) parts.push(`owner ${[ok, on].filter(Boolean).join('/')}`);
  parts.push(`uid ${uid}`);
  if ((ns ?? '').trim()) parts.push(`ns ${ns.trim()}`);
  return parts.join(' · ');
}

function buildGraph(
  destinations: NetworkActivityDestinationRow[],
  talkers: NetworkActivityTalkerRow[],
  maxNodes: number,
  podNamesByUid?: Readonly<Record<string, string>>,
): { nodes: NodeDatum[]; links: LinkDatum[] } {
  const nodeMap = new Map<string, NodeDatum>();
  const links: LinkDatum[] = [];

  const topTalkers = talkers.slice(0, Math.min(talkers.length, Math.floor(maxNodes * 0.5)));
  for (const t of topTalkers) {
    const id = `pod:${t.podUid}`;
    if (!nodeMap.has(id)) {
      const name =
        (t.podName ?? '').trim() || (podNamesByUid?.[t.podUid] ?? '').trim();
      const ns = (t.namespace ?? '').trim() || '—';
      const line1 = podNodeLine1(name || undefined, t.podUid);
      const line2 = `ns: ${ns} · ${uidFootprint(t.podUid)}`;
      const fullLabel = podNodeFullLabel(name || undefined, ns, t.podUid, t.ownerKind, t.ownerName);
      nodeMap.set(id, {
        id,
        line1,
        line2,
        fullLabel,
        kind: 'pod',
        weight: t.observationCount,
        namespace: t.namespace,
        ownerLabel: t.ownerKind && t.ownerName ? `${t.ownerKind}/${t.ownerName}` : undefined,
      });
    }
  }

  const topDests = destinations.slice(0, Math.min(destinations.length, Math.floor(maxNodes * 0.5)));
  for (const d of topDests) {
    const destKey = `dest:${d.destIp}:${d.destPort}/${d.protocol ?? 'tcp'}`;
    if (!nodeMap.has(destKey)) {
      const hasWorkload = Boolean((d.destWorkloadName ?? '').trim());
      const hasService = Boolean((d.destServiceName ?? '').trim());
      const displayName = hasService
        ? d.destServiceName
        : hasWorkload
          ? d.destWorkloadName
          : d.destIp;
      const line1 = truncateGraphLabel(
        `${displayName}:${d.destPort}`,
        28,
      );
      const ns = ((hasService ? d.destServiceNamespace : d.destWorkloadNamespace) ?? '').trim();
      const line2 = ns ? `${hasService ? 'svc' : 'ns'}: ${ns}` : 'external';
      const fqdn = (d.destServiceFqdn ?? '').trim();
      const fullLabel = hasService
        ? `${fqdn || `${d.destServiceName}.${d.destServiceNamespace}.svc`} · ${d.destIp}:${d.destPort}/${d.protocol ?? 'tcp'}`
        : hasWorkload
          ? `${d.destWorkloadName} · ${d.destIp}:${d.destPort}/${d.protocol ?? 'tcp'}`
        : `${d.destIp}:${d.destPort}/${d.protocol ?? 'tcp'}`;
      nodeMap.set(destKey, {
        id: destKey,
        line1,
        line2,
        fullLabel,
        kind: 'dest',
        destType: hasService ? 'internal_service' : hasWorkload ? 'internal_workload' : 'external_endpoint',
        weight: d.observationCount,
        namespace: hasService ? d.destServiceNamespace : d.destWorkloadNamespace,
      });
    }
  }

  const podIds = topTalkers.map((t) => `pod:${t.podUid}`);
  for (const d of topDests) {
    const destKey = `dest:${d.destIp}:${d.destPort}/${d.protocol ?? 'tcp'}`;
    const podsToLink = podIds.slice(0, Math.min(podIds.length, MAX_LINKS_PER_DESTINATION));
    for (const podId of podsToLink) {
      links.push({
        source: podId,
        target: destKey,
        value: Math.min(nodeMap.get(podId)?.weight ?? 1, nodeMap.get(destKey)?.weight ?? 1),
      });
    }
  }

  return { nodes: Array.from(nodeMap.values()), links };
}

const EDGE_SEP = '\x1f';
const MAX_AGGREGATED_LINKS = 320;

/** Aggregate connection rows or edges (each row may have observationCount > 1); keep top pod/destination by weight. */
function buildGraphFromConnections(
  rows: NetworkActivityConnectionRow[],
  maxNodes: number,
  supplementTalkers?: NetworkActivityTalkerRow[],
  podNamesByUid?: Readonly<Record<string, string>>,
): { nodes: NodeDatum[]; links: LinkDatum[] } {
  type EdgeRec = {
    podUid: string;
    destIp: string;
    destPort: number;
    protocol: string;
    weight: number;
  };
  const edgeMap = new Map<string, EdgeRec>();
  const podMeta = new Map<string, PodMetaPatch>();
  const destMeta = new Map<
    string,
    {
      workloadName?: string;
      workloadNamespace?: string;
      serviceName?: string;
      serviceNamespace?: string;
      serviceFqdn?: string;
    }
  >();

  for (const r of rows) {
    const uid = (r.podUid ?? '').trim();
    const dip = (r.destIp ?? '').trim();
    const dport = r.destPort;
    if (!uid || !dip || dport == null || Number.isNaN(Number(dport))) continue;
    const proto = ((r.protocol ?? 'tcp') as string).trim() || 'tcp';
    const k = `${uid}${EDGE_SEP}${dip}${EDGE_SEP}${dport}${EDGE_SEP}${proto}`;
    const rowW =
      typeof r.observationCount === 'number' && r.observationCount > 0
        ? Number(r.observationCount)
        : 1;
    const cur = edgeMap.get(k);
    if (cur) cur.weight += rowW;
    else
      edgeMap.set(k, {
        podUid: uid,
        destIp: dip,
        destPort: Number(dport),
        protocol: proto,
        weight: rowW,
      });
    const destKey = `dest:${dip}:${Number(dport)}/${proto}`;
    const existingDest = destMeta.get(destKey) ?? {};
    destMeta.set(destKey, {
      workloadName: existingDest.workloadName || (r.destWorkloadName ?? '').trim() || undefined,
      workloadNamespace:
        existingDest.workloadNamespace || (r.destWorkloadNamespace ?? '').trim() || undefined,
      serviceName: existingDest.serviceName || (r.destServiceName ?? '').trim() || undefined,
      serviceNamespace:
        existingDest.serviceNamespace || (r.destServiceNamespace ?? '').trim() || undefined,
      serviceFqdn: existingDest.serviceFqdn || (r.destServiceFqdn ?? '').trim() || undefined,
    });

    const rowPodLabel =
      (r.podName ?? '').trim() || (r.containerName ?? '').trim() || undefined;
    mergePodMeta(podMeta, uid, {
      namespace: r.namespace,
      name: rowPodLabel,
      ownerKind: r.ownerKind,
      ownerName: r.ownerName,
    });
  }

  if (edgeMap.size === 0) {
    return { nodes: [], links: [] };
  }

  const podWeight = new Map<string, number>();
  const destWeight = new Map<string, number>();
  const destTuple = new Map<string, { destIp: string; destPort: number; protocol: string }>();

  for (const e of edgeMap.values()) {
    const destKey = `dest:${e.destIp}:${e.destPort}/${e.protocol}`;
    podWeight.set(e.podUid, (podWeight.get(e.podUid) ?? 0) + e.weight);
    destWeight.set(destKey, (destWeight.get(destKey) ?? 0) + e.weight);
    if (!destTuple.has(destKey)) {
      destTuple.set(destKey, { destIp: e.destIp, destPort: e.destPort, protocol: e.protocol });
    }
  }

  if (supplementTalkers?.length) {
    for (const t of supplementTalkers) {
      const uid = (t.podUid ?? '').trim();
      if (!uid) continue;
      mergePodMeta(podMeta, uid, {
        namespace: t.namespace,
        name: t.podName,
        ownerKind: t.ownerKind,
        ownerName: t.ownerName,
      });
    }
  }

  if (podNamesByUid) {
    for (const uid of podWeight.keys()) {
      const inv = (podNamesByUid[uid] ?? '').trim();
      if (!inv) continue;
      const cur = podMeta.get(uid);
      if ((cur?.name ?? '').trim()) continue;
      mergePodMeta(podMeta, uid, { name: inv });
    }
  }

  const maxPods = Math.max(1, Math.floor(maxNodes * 0.5));
  const maxDests = Math.max(1, Math.floor(maxNodes * 0.5));
  const topPodUids = [...podWeight.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, maxPods)
    .map(([u]) => u);
  const topDestKeys = [...destWeight.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, maxDests)
    .map(([k]) => k);

  const podSet = new Set(topPodUids);
  const destSet = new Set(topDestKeys);

  const nodeMap = new Map<string, NodeDatum>();

  for (const uid of topPodUids) {
    const id = `pod:${uid}`;
    const meta = podMeta.get(uid) ?? {};
    const name = (meta.name ?? '').trim();
    const ns = (meta.namespace ?? '').trim() || '—';
    const w = podWeight.get(uid) ?? 1;
    const ownerLabel =
      meta.ownerKind && meta.ownerName ? `${meta.ownerKind}/${meta.ownerName}` : undefined;
    const line1 = podNodeLine1(name || undefined, uid);
    const line2 = `ns: ${ns} · ${uidFootprint(uid)}`;
    nodeMap.set(id, {
      id,
      line1,
      line2,
      fullLabel: podNodeFullLabel(name || undefined, ns, uid, meta.ownerKind, meta.ownerName),
      kind: 'pod',
      weight: w,
      namespace: meta.namespace,
      ownerLabel,
    });
  }

  for (const destKey of topDestKeys) {
    const t = destTuple.get(destKey);
    if (!t) continue;
    const w = destWeight.get(destKey) ?? 1;
    const meta = destMeta.get(destKey) ?? {};
    const hasService = Boolean((meta.serviceName ?? '').trim());
    const hasWorkload = Boolean((meta.workloadName ?? '').trim());
    const displayName = hasService
      ? meta.serviceName
      : hasWorkload
        ? meta.workloadName
        : t.destIp;
    const ns = hasService ? meta.serviceNamespace : meta.workloadNamespace;
    const line1 = truncateGraphLabel(`${displayName}:${t.destPort}`, 28);
    const fullLabel = hasService
      ? `${meta.serviceFqdn || `${meta.serviceName}.${meta.serviceNamespace}.svc`} · ${t.destIp}:${t.destPort}/${t.protocol}`
      : hasWorkload
        ? `${meta.workloadName} · ${t.destIp}:${t.destPort}/${t.protocol}`
        : `${t.destIp}:${t.destPort}/${t.protocol}`;
    nodeMap.set(destKey, {
      id: destKey,
      line1,
      line2: ns ? `${hasService ? 'svc' : 'ns'}: ${ns}` : 'external',
      fullLabel,
      kind: 'dest',
      destType: hasService ? 'internal_service' : hasWorkload ? 'internal_workload' : 'external_endpoint',
      weight: w,
      namespace: ns,
    });
  }

  const linkCandidates: LinkDatum[] = [];
  for (const e of edgeMap.values()) {
    const destKey = `dest:${e.destIp}:${e.destPort}/${e.protocol}`;
    if (!podSet.has(e.podUid) || !destSet.has(destKey)) continue;
    linkCandidates.push({
      source: `pod:${e.podUid}`,
      target: destKey,
      value: e.weight,
    });
  }
  linkCandidates.sort((a, b) => b.value - a.value);
  const links = linkCandidates.slice(0, MAX_AGGREGATED_LINKS);

  return { nodes: Array.from(nodeMap.values()), links };
}

/**
 * Normalize edge weight to [0, 1] — sqrt so weak edges stay distinguishable and strong edges do not saturate the range.
 */
function linkWeightNorm(value: number, maxLinkVal: number): number {
  const m = Math.max(maxLinkVal, 1e-9);
  return Math.min(1, Math.sqrt(Math.max(0, value) / m));
}

function linkStyleScales(maxLinkVal: number) {
  const maxV = Math.max(maxLinkVal, 1);
  const linkWidthScale = d3.scaleSqrt().domain([0, maxV]).range([0.65, 4.15]);
  const linkOpacityScale = d3.scaleSqrt().domain([0, maxV]).range([0.2, 0.88]);
  return { maxV, linkWidthScale, linkOpacityScale };
}

function nodeSymbolType(node: NodeDatum): d3.SymbolType {
  if (node.kind === 'pod') return d3.symbolCircle;
  return destinationIsInternal(node) ? d3.symbolSquare : d3.symbolDiamond;
}

function nodeSymbolPath(node: NodeDatum, radius: number): string {
  const area = Math.PI * radius * radius;
  return d3.symbol().type(nodeSymbolType(node)).size(area)() ?? '';
}

function linkCurvePath(d: LinkDatum, curve = 0.12): string {
  const s = d.source as NodeDatum;
  const t = d.target as NodeDatum;
  const sx = s.x ?? 0;
  const sy = s.y ?? 0;
  const tx = t.x ?? 0;
  const ty = t.y ?? 0;
  const mx = (sx + tx) / 2;
  const my = (sy + ty) / 2;
  const dx = tx - sx;
  const dy = ty - sy;
  const len = Math.hypot(dx, dy) || 1;
  const off = len * curve;
  const cx = mx + (-dy / len) * off;
  const cy = my + (dx / len) * off;
  return `M${sx},${sy} Q${cx},${cy} ${tx},${ty}`;
}

/* ─── component ───────────────────────────────────────────────── */

export const NetworkTopologyGraph: React.FC<NetworkTopologyGraphProps> = ({
  destinations,
  talkers,
  connections,
  supplementTalkers,
  maxNodes = 80,
  onNodeClick,
  showCompactLegend = true,
  className = '',
  showTrafficParticles = false,
  podNamesByUid,
  graphTrust,
  showTrustOverlay = false,
  onSelectionChange,
}) => {
  const svgRef = useRef<SVGSVGElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const uid = useId().replace(/\W/g, '');
  const arrowId = `arrow-${uid}`;
  const glowId = `glow-${uid}`;

  const [dimensions, setDimensions] = useState({ width: 900, height: 500 });
  const [tooltip, setTooltip] = useState<{ x: number; y: number; content: string } | null>(null);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const selectedNodeIdRef = useRef<string | null>(null);
  selectedNodeIdRef.current = selectedNodeId;

  const { nodes, links } = useMemo(() => {
    if (connections && connections.length > 0) {
      return buildGraphFromConnections(connections, maxNodes, supplementTalkers, podNamesByUid);
    }
    return buildGraph(destinations, talkers, maxNodes, podNamesByUid);
  }, [connections, destinations, talkers, supplementTalkers, maxNodes, podNamesByUid]);

  const sourceEntityCount = useMemo(() => {
    const ids = new Set<string>();
    if (connections && connections.length > 0) {
      for (const row of connections) {
        const uid = (row.podUid ?? '').trim();
        const dip = (row.destIp ?? '').trim();
        const dport = row.destPort;
        const proto = ((row.protocol ?? 'tcp') as string).trim() || 'tcp';
        if (uid) ids.add(`pod:${uid}`);
        if (dip && dport != null) ids.add(`dest:${dip}:${dport}/${proto}`);
      }
      for (const talker of supplementTalkers ?? []) {
        const uid = (talker.podUid ?? '').trim();
        if (uid) ids.add(`pod:${uid}`);
      }
      return ids.size;
    }

    for (const talker of talkers) {
      const uid = (talker.podUid ?? '').trim();
      if (uid) ids.add(`pod:${uid}`);
    }
    for (const dest of destinations) {
      const dip = (dest.destIp ?? '').trim();
      if (dip && dest.destPort != null) {
        ids.add(`dest:${dip}:${dest.destPort}/${dest.protocol ?? 'tcp'}`);
      }
    }
    return ids.size;
  }, [connections, destinations, supplementTalkers, talkers]);

  useEffect(() => {
    if (!selectedNodeId) return;
    if (!nodes.some((n) => n.id === selectedNodeId)) {
      setSelectedNodeId(null);
    }
  }, [nodes, selectedNodeId]);

  useEffect(() => {
    if (!onSelectionChange) return;
    if (!selectedNodeId) {
      onSelectionChange(null);
      return;
    }
    const node = nodes.find((n) => n.id === selectedNodeId);
    if (!node) {
      onSelectionChange(null);
      return;
    }
    onSelectionChange({
      id: node.id,
      kind: node.kind,
      typeLabel: nodeTypeLabel(node),
      line1: node.line1,
      line2: node.line2,
      fullLabel: node.fullLabel,
      namespace: node.namespace,
      ownerLabel: node.ownerLabel,
      weight: node.weight,
    });
  }, [selectedNodeId, nodes, onSelectionChange]);

  const selectNode = useCallback((node: NodeDatum) => {
    const wasSelected = selectedNodeIdRef.current === node.id;
    setSelectedNodeId(wasSelected ? null : node.id);
    if (onNodeClick && node.kind === 'pod' && !wasSelected) {
      onNodeClick(node.id.replace(/^(pod|dest):/, ''), node.kind);
    }
  }, [onNodeClick]);

  const graphIsSummarized =
    sourceEntityCount > nodes.length || nodes.length >= maxNodes || links.length >= MAX_AGGREGATED_LINKS;

  const topologyTrust = useMemo(() => {
    if (!graphTrust) return null;
    const ctx: GraphTrustContext = { ...graphTrust, clustered: graphIsSummarized };
    const posture = buildGraphTrustPosture(ctx, nodes.length, links.length);
    const linkStyles = links.map(() => {
      const kind = classifyEdgeTrust('NETWORK_REACH', ctx);
      return edgeTrustVisual(kind, ctx.semanticMode, 'NETWORK_REACH');
    });
    const nodeStyles = nodes.map((n) => {
      const kind = classifyNodeTrust(
        { id: n.id, type: n.kind === 'pod' ? 'pod' : 'external' },
        ctx,
      );
      const fill = nodeBaseFill(n);
      const stroke = nodeBaseStroke(n);
      const trust = nodeTrustVisual(kind, fill, stroke);
      return {
        ...trust,
        fill,
        stroke,
        strokeDasharray: null,
      };
    });
    return { posture, linkStyles, nodeStyles };
  }, [graphTrust, graphIsSummarized, nodes, links]);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width, height } = entry.contentRect;
        if (width > 0 && height > 0) {
          // Do not enforce min 360px height — avoids SVG taller than flex container (browser overflow).
          setDimensions({ width, height: Math.max(1, height) });
        }
      }
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  useEffect(() => {
    if (!svgRef.current || nodes.length === 0) return;

    const { width, height } = dimensions;
    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    const maxWeight = d3.max(nodes, (d) => d.weight) ?? 1;
    const rScale = d3.scaleSqrt().domain([0, maxWeight]).range([NODE_RADIUS_MIN, NODE_RADIUS_MAX]);
    const maxLinkVal = d3.max(links, (d) => d.value) ?? 1;
    const { linkWidthScale, linkOpacityScale } = linkStyleScales(maxLinkVal);

    const nodesCopy: NodeDatum[] = nodes.map((n) => ({ ...n }));
    const linksCopy: LinkDatum[] = links.map((l) => ({ ...l }));
    const nodesById = new Map(nodesCopy.map((n) => [n.id, n]));
    const denseGraph = nodesCopy.length > PRIMARY_LABEL_LIMIT || linksCopy.length > 160;
    const veryDenseGraph = nodesCopy.length > 110 || linksCopy.length > 260;
    const showPrimaryLabels = !denseGraph;
    const showSecondaryLabels = nodesCopy.length <= SECONDARY_LABEL_LIMIT && linksCopy.length <= 90;

    const nCount = Math.max(nodesCopy.length, 1);
    const diag = Math.sqrt(Math.max(1, width * height));
    const density = Math.sqrt(nCount);
    const linkDistance = Math.min(veryDenseGraph ? 156 : 210, Math.max(64, (diag / 12) * (1 + density * 0.05)));
    const chargeStrength = -Math.min(veryDenseGraph ? 360 : 460, Math.max(130, diag * 0.2 * (1 + density * 0.025)));
    const collisionPad = Math.min(veryDenseGraph ? 30 : 40, Math.max(18, NODE_COLLISION_EXTRA + density * 0.9));

    const simulation = d3
      .forceSimulation(nodesCopy)
      .force(
        'link',
        d3
          .forceLink<NodeDatum, LinkDatum>(linksCopy)
          .id((d) => d.id)
          .distance(linkDistance)
          .strength(0.28),
      )
      .force('charge', d3.forceManyBody().strength(chargeStrength))
      .force('center', d3.forceCenter(width / 2, height / 2))
      .force('x', d3.forceX(width / 2).strength(0.045))
      .force('y', d3.forceY(height / 2).strength(0.045))
      .force('collision', d3.forceCollide<NodeDatum>().radius((d) => rScale(d.weight) + collisionPad))
      .alphaDecay(veryDenseGraph ? 0.09 : 0.065)
      .velocityDecay(veryDenseGraph ? 0.32 : 0.24);

    const defs = svg.append('defs');

    const gridPattern = defs
      .append('pattern')
      .attr('id', `${uid}-grid`)
      .attr('width', 36)
      .attr('height', 36)
      .attr('patternUnits', 'userSpaceOnUse');
    gridPattern
      .append('path')
      .attr('d', 'M36 0H0V36')
      .attr('fill', 'none')
      .attr('stroke', GRAPH_THEME.border)
      .attr('stroke-opacity', 0.18)
      .attr('stroke-width', 1);

    const glowFilter = defs
      .append('filter')
      .attr('id', glowId)
      .attr('x', '-60%')
      .attr('y', '-60%')
      .attr('width', '220%')
      .attr('height', '220%');
    glowFilter.append('feGaussianBlur').attr('stdDeviation', 2.8).attr('result', 'coloredBlur');
    const glowMerge = glowFilter.append('feMerge');
    glowMerge.append('feMergeNode').attr('in', 'coloredBlur');
    glowMerge.append('feMergeNode').attr('in', 'SourceGraphic');

    defs
      .append('marker')
      .attr('id', arrowId)
      .attr('viewBox', '0 -5 10 10')
      .attr('refX', 24)
      .attr('refY', 0)
      .attr('markerWidth', 6.2)
      .attr('markerHeight', 6.2)
      .attr('orient', 'auto')
      .append('path')
      .attr('d', 'M0,-4L8,0L0,4')
      .attr('fill', 'context-stroke');

    const gRoot = svg.append('g').attr('data-topology-root', '1');

    const zoom = d3
      .zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.08, 5])
      .on('zoom', (event) => {
        gRoot.attr('transform', event.transform);
      });
    svg.call(zoom);

    const pad = veryDenseGraph ? 38 : 56;
    const fitTopologyToView = () => {
      if (!svgRef.current || nodesCopy.length === 0) return;
      let minX = Infinity;
      let minY = Infinity;
      let maxX = -Infinity;
      let maxY = -Infinity;
      for (const n of nodesCopy) {
        const x = n.x ?? 0;
        const y = n.y ?? 0;
        const r = rScale(n.weight) + (showPrimaryLabels ? NODE_FIT_PADDING : 16);
        minX = Math.min(minX, x - r);
        maxX = Math.max(maxX, x + r);
        minY = Math.min(minY, y - r);
        maxY = Math.max(maxY, y + r + (showPrimaryLabels ? 34 : 12));
      }
      if (!Number.isFinite(minX) || !Number.isFinite(minY)) return;
      const bw = Math.max(maxX - minX, 64);
      const bh = Math.max(maxY - minY, 64);
      const k = Math.min((width - 2 * pad) / bw, (height - 2 * pad) / bh, veryDenseGraph ? 3.2 : 5);
      if (!Number.isFinite(k) || k <= 0) return;
      const cx = (minX + maxX) / 2;
      const cy = (minY + maxY) / 2;
      const tx = width / 2 - k * cx;
      const ty = height / 2 - k * cy;
      d3.select(svgRef.current)
        .transition()
        .duration(380)
        .call(zoom.transform, d3.zoomIdentity.translate(tx, ty).scale(k));
    };

    gRoot
      .append('rect')
      .attr('width', width)
      .attr('height', height)
      .attr('fill', `url(#${uid}-grid)`)
      .style('cursor', 'grab')
      .lower()
      .on('click', (event) => {
        event.stopPropagation();
        setSelectedNodeId(null);
      });

    const linkG = gRoot.append('g').attr('fill', 'none');

    const linkHaloPath = linkG
      .selectAll('path.topology-link-halo')
      .data(linksCopy)
      .join('path')
      .attr('class', 'topology-link-halo')
      .attr('stroke', (d, i) => linkStrokeWithTrust(d, maxLinkVal, nodesById, topologyTrust?.linkStyles[i]))
      .attr('stroke-width', (d) => linkWidthScale(d.value) + 2)
      .attr('stroke-opacity', (d) => 0.03 + 0.1 * linkWeightNorm(d.value, maxLinkVal))
      .attr('stroke-linecap', 'round')
      .attr('stroke-linejoin', 'round')
      .style('pointer-events', 'none');

    const linkPath = linkG
      .selectAll('path.topology-link')
      .data(linksCopy)
      .join('path')
      .attr('class', 'topology-link')
      .attr('stroke', (d, i) => linkStrokeWithTrust(d, maxLinkVal, nodesById, topologyTrust?.linkStyles[i]))
      .attr('stroke-width', (d, i) =>
        topologyTrust
          ? Math.max(topologyTrust.linkStyles[i]?.strokeWidth ?? 0, linkWidthScale(d.value))
          : linkWidthScale(d.value),
      )
      .attr('stroke-opacity', (d, i) =>
        topologyTrust
          ? Math.max(topologyTrust.linkStyles[i]?.strokeOpacity ?? 0, linkOpacityScale(d.value))
          : linkOpacityScale(d.value),
      )
      .attr('stroke-dasharray', (_d, i) => linkDasharrayWithTrust(topologyTrust?.linkStyles[i]))
      .attr('stroke-linecap', 'round')
      .attr('stroke-linejoin', 'round')
      .attr('marker-end', `url(#${arrowId})`);

    const linkCountForParticles = Math.min(linksCopy.length, MAX_LINK_INDICES_FOR_PARTICLES);
    const particleData: ParticleDatum[] = [];
    if (showTrafficParticles && linkCountForParticles > 0) {
      const speedMul = (linkIdx: number) => {
        const v = linksCopy[linkIdx]?.value ?? 0;
        const n = linkWeightNorm(v, maxLinkVal);
        return 0.42 + 1.65 * n;
      };
      for (let i = 0; i < linkCountForParticles; i++) {
        const mul = speedMul(i);
        particleData.push({
          linkIndex: i,
          phase: Math.random(),
          speed: (0.00011 + Math.random() * 0.00009) * mul,
        });
        particleData.push({
          linkIndex: i,
          phase: 0.35 + Math.random() * 0.3,
          speed: (0.00009 + Math.random() * 0.00007) * mul,
        });
      }
    }

    const particleG = gRoot
      .append('g')
      .attr('class', 'topology-particles')
      .attr('fill', 'none')
      .style('pointer-events', 'none');

    const particleSel =
      showTrafficParticles && particleData.length > 0
        ? particleG
            .selectAll<SVGCircleElement, ParticleDatum>('circle')
            .data(particleData)
            .join('circle')
            .attr('r', (d) => {
              const v = linksCopy[d.linkIndex]?.value ?? 0;
              return 2.05 + 1.35 * linkWeightNorm(v, maxLinkVal);
            })
            .attr('fill', COLOR_PARTICLE_CORE)
            .attr('stroke', COLOR_PARTICLE)
            .attr('stroke-width', (d) => {
              const v = linksCopy[d.linkIndex]?.value ?? 0;
              return 0.45 + 0.45 * linkWeightNorm(v, maxLinkVal);
            })
            .attr('opacity', (d) => {
              const v = linksCopy[d.linkIndex]?.value ?? 0;
              return 0.58 + 0.32 * linkWeightNorm(v, maxLinkVal);
            })
        : null;

    const nodeLayer = gRoot.append('g').attr('class', 'topology-nodes');
    const node = nodeLayer
      .selectAll<SVGGElement, NodeDatum>('g')
      .data(nodesCopy)
      .join('g')
      .attr('class', 'topology-node')
      .attr('data-node-id', (d) => d.id)
      .attr('tabindex', 0)
      .attr('role', 'button')
      .attr('aria-label', (d) => `${nodeTypeLabel(d)} ${d.fullLabel}. ${d.weight} observations. Press Enter to select.`)
      .style('cursor', 'pointer')
      .call(
        d3
          .drag<SVGGElement, NodeDatum>()
          .on('start', (event, d) => {
            if (!event.active) simulation.alphaTarget(0.28).restart();
            d.fx = d.x;
            d.fy = d.y;
          })
          .on('drag', (event, d) => {
            d.fx = event.x;
            d.fy = event.y;
          })
          .on('end', (event, d) => {
            if (!event.active) simulation.alphaTarget(0);
            d.fx = null;
            d.fy = null;
          }),
      );

    node
      .append('path')
      .attr('class', 'topology-node-shape')
      .attr('data-node-id', (d) => d.id)
      .attr('d', (d) => nodeSymbolPath(d, rScale(d.weight)))
      .attr('fill', (d, i) =>
        topologyTrust ? topologyTrust.nodeStyles[i]?.fill : nodeBaseFill(d),
      )
      .attr('fill-opacity', (d, i) =>
        topologyTrust?.nodeStyles[i]?.fillOpacity ?? (d.kind === 'pod' ? 0.78 : destinationIsInternal(d) ? 0.7 : 0.88),
      )
      .attr('stroke', (d, i) =>
        topologyTrust
          ? topologyTrust.nodeStyles[i]?.stroke
          : nodeBaseStroke(d),
      )
      .attr('stroke-width', (d, i) => topologyTrust?.nodeStyles[i]?.strokeWidth ?? NODE_STROKE_DEFAULT)
      .attr('stroke-dasharray', null);

    node
      .append('path')
      .attr('class', 'topology-node-ring')
      .attr('d', (d) => nodeSymbolPath(d, rScale(d.weight) + 4))
      .attr('fill', 'none')
      .attr('stroke', (d) => nodeBaseStroke(d))
      .attr('stroke-opacity', veryDenseGraph ? 0.1 : 0.18)
      .attr('stroke-width', veryDenseGraph ? 0.7 : 0.9)
      .style('pointer-events', 'none');

    node
      .append('circle')
      .attr('class', 'topology-node-hit-target')
      .attr('r', (d) => Math.max(18, rScale(d.weight) + 8))
      .attr('fill', 'transparent')
      .style('pointer-events', 'all');

    if (showPrimaryLabels) {
      node
        .append('text')
        .attr('text-anchor', 'middle')
        .attr('pointer-events', 'none')
        .each(function (d) {
          const r = rScale(d.weight);
          const g = d3.select(this);
          const max1 = width < 520 ? 18 : width < 768 ? 22 : 28;
          const max2 = width < 520 ? 28 : 36;
          const line1 = truncateGraphLabel(d.line1, max1);
          const line2 = truncateGraphLabel(d.line2, max2);
          g.append('tspan')
            .attr('x', 0)
            .attr('dy', r + 17)
            .attr('fill', GRAPH_THEME.text)
            .attr('font-size', width < 520 ? '10px' : '11px')
            .attr('font-weight', '650')
            .attr('paint-order', 'stroke')
            .attr('stroke', GRAPH_THEME.surfaceDeep)
            .attr('stroke-width', 3)
            .attr('stroke-linejoin', 'round')
            .text(line1);
          if (showSecondaryLabels) {
            g.append('tspan')
              .attr('x', 0)
              .attr('dy', 13)
              .attr('fill', GRAPH_THEME.muted)
              .attr('font-size', width < 520 ? '9px' : '10px')
              .attr('paint-order', 'stroke')
              .attr('stroke', GRAPH_THEME.surfaceDeep)
              .attr('stroke-width', 3)
              .attr('stroke-linejoin', 'round')
              .text(line2);
          }
        });
    }

    const applyHighlight = (focusId: string | null) => {
      const fadeOthers = focusId != null;
      node.style('opacity', (d) => {
        if (!fadeOthers) return 1;
        const connected = linksCopy.some((l) => {
          const s = typeof l.source === 'object' ? (l.source as NodeDatum).id : l.source;
          const t = typeof l.target === 'object' ? (l.target as NodeDatum).id : l.target;
          return s === focusId && d.id === t || t === focusId && d.id === s;
        });
        return d.id === focusId || connected ? 1 : 0.12;
      });
      linkHaloPath
        .attr('stroke', (l, i) => {
          const s = typeof l.source === 'object' ? (l.source as NodeDatum).id : l.source;
          const t = typeof l.target === 'object' ? (l.target as NodeDatum).id : l.target;
          if (!focusId) return linkStrokeWithTrust(l, maxLinkVal, nodesById, topologyTrust?.linkStyles[i]);
          return s === focusId || t === focusId ? linkFocusStrokeColor(l, nodesById) : COLOR_LINK_DIM;
        })
        .attr('stroke-opacity', (l) => {
          const s = typeof l.source === 'object' ? (l.source as NodeDatum).id : l.source;
          const t = typeof l.target === 'object' ? (l.target as NodeDatum).id : l.target;
          const base = 0.03 + 0.1 * linkWeightNorm(l.value, maxLinkVal);
          if (!focusId) return base;
          return s === focusId || t === focusId ? Math.min(0.26, base + 0.12) : 0.025;
        });

      linkPath
        .attr('stroke', (l, i) => {
          const s = typeof l.source === 'object' ? (l.source as NodeDatum).id : l.source;
          const t = typeof l.target === 'object' ? (l.target as NodeDatum).id : l.target;
          if (!focusId) return linkStrokeWithTrust(l, maxLinkVal, nodesById, topologyTrust?.linkStyles[i]);
          return s === focusId || t === focusId ? linkFocusStrokeColor(l, nodesById) : COLOR_LINK_DIM;
        })
        .attr('stroke-opacity', (l) => {
          const s = typeof l.source === 'object' ? (l.source as NodeDatum).id : l.source;
          const t = typeof l.target === 'object' ? (l.target as NodeDatum).id : l.target;
          if (!focusId) return linkOpacityScale(l.value);
          return s === focusId || t === focusId ? Math.max(0.62, linkOpacityScale(l.value)) : 0.06;
        });

      if (particleSel) {
        particleSel.attr('opacity', (d) => {
          const l = linksCopy[d.linkIndex];
          const base =
            l != null ? 0.58 + 0.32 * linkWeightNorm(l.value, maxLinkVal) : 0.5;
          if (!focusId) return base;
          if (!l) return 0.06;
          const s = typeof l.source === 'object' ? (l.source as NodeDatum).id : l.source;
          const t = typeof l.target === 'object' ? (l.target as NodeDatum).id : l.target;
          const hit = s === focusId || t === focusId;
          return hit ? Math.min(0.98, base + 0.12) : 0.05;
        });
      }
    };

    node
      .on('mouseenter', (event, d) => {
        const [x, y] = d3.pointer(event, svgRef.current);
        const parts: string[] = [`${nodeTypeLabel(d)}: ${d.fullLabel}`];
        if (d.namespace) parts.push(`Namespace: ${d.namespace}`);
        if (d.ownerLabel) parts.push(`Owner: ${d.ownerLabel}`);
        parts.push(`Observations: ${d.weight}`);
        setTooltip({ x, y: y - 10, content: parts.join('\n') });
        applyHighlight(d.id);
      })
      .on('mouseleave', () => {
        setTooltip(null);
        applyHighlight(selectedNodeIdRef.current);
      })
      .on('focus', (_event, d) => {
        applyHighlight(d.id);
      })
      .on('blur', () => {
        applyHighlight(selectedNodeIdRef.current);
      })
      .on('keydown', (event, d) => {
        if (event.key !== 'Enter' && event.key !== ' ') return;
        event.preventDefault();
        event.stopPropagation();
        selectNode(d);
      })
      .on('click', (event, d) => {
        event.stopPropagation();
        selectNode(d);
      });

    simulation.on('tick', () => {
      linkHaloPath.attr('d', (d) => linkCurvePath(d));
      linkPath.attr('d', (d) => linkCurvePath(d));
      node.attr('transform', (d) => `translate(${d.x ?? 0},${d.y ?? 0})`);
    });

    simulation.on('end', () => {
      window.requestAnimationFrame(() => fitTopologyToView());
    });
    const fitTimer = window.setTimeout(() => fitTopologyToView(), 1100);

    let rafId = 0;
    if (particleSel && particleData.length > 0) {
      let lastT = performance.now();
      const step = (now: number) => {
        const dt = Math.min(now - lastT, 48);
        lastT = now;
        const pathEls = linkPath.nodes() as SVGPathElement[];
        particleSel.each(function (d) {
          d.phase = (d.phase + d.speed * dt) % 1;
          const pathEl = pathEls[d.linkIndex];
          if (!pathEl) return;
          const plen = pathEl.getTotalLength();
          if (!Number.isFinite(plen) || plen < 2) return;
          const pt = pathEl.getPointAtLength(d.phase * plen);
          d3.select(this).attr('cx', pt.x).attr('cy', pt.y);
        });
        rafId = requestAnimationFrame(step);
      };
      rafId = requestAnimationFrame(step);
    }

    return () => {
      window.clearTimeout(fitTimer);
      if (rafId) cancelAnimationFrame(rafId);
      simulation.stop();
    };
  }, [nodes, links, dimensions, selectNode, arrowId, glowId, showTrafficParticles, topologyTrust]);

  useEffect(() => {
    const svgEl = svgRef.current;
    if (!svgEl || nodes.length === 0) return;
    const root = d3.select(svgEl).select('[data-topology-root]');
    if (root.empty()) return;

    const focusId = selectedNodeId;
    root.selectAll<SVGGElement, NodeDatum>('g.topology-node').each(function (d) {
      const on = d.id === focusId;
      d3.select(this)
        .select<SVGPathElement>('path.topology-node-shape')
        .attr('stroke-width', on ? NODE_STROKE_SELECTED : NODE_STROKE_DEFAULT)
        .attr('filter', on ? `url(#${glowId})` : null);
      d3.select(this)
        .select<SVGPathElement>('path.topology-node-ring')
        .attr('stroke-opacity', on ? 0.42 : 0.18)
        .attr('stroke-width', on ? 1.6 : 0.9);
      if (on) d3.select(this).raise();
    });

    const fadeOthers = focusId != null;
    root.selectAll<SVGGElement, NodeDatum>('g.topology-node').style('opacity', function (d) {
      if (!fadeOthers) return 1;
      const connected = links.some((l) => {
        const s = typeof l.source === 'string' ? l.source : (l.source as NodeDatum).id;
        const t = typeof l.target === 'string' ? l.target : (l.target as NodeDatum).id;
        return (s === focusId && d.id === t) || (t === focusId && d.id === s);
      });
      return d.id === focusId || connected ? 1 : 0.12;
    });

    const maxL = d3.max(links, (x) => x.value) ?? 1;
    const { linkOpacityScale: loScale } = linkStyleScales(maxL);
    const nodesById = new Map(nodes.map((n) => [n.id, n]));

    root.selectAll<SVGPathElement, LinkDatum>('path.topology-link-halo').each(function (_, index) {
      const path = d3.select(this);
      const l = path.datum() as LinkDatum;
      const s = typeof l.source === 'object' ? (l.source as NodeDatum).id : String(l.source);
      const t = typeof l.target === 'object' ? (l.target as NodeDatum).id : String(l.target);
      if (!fadeOthers) {
        path
          .attr('stroke', linkStrokeWithTrust(l, maxL, nodesById, topologyTrust?.linkStyles[index]))
          .attr('stroke-opacity', 0.03 + 0.1 * linkWeightNorm(l.value, maxL));
        return;
      }
      const hit = s === focusId || t === focusId;
      const base = 0.03 + 0.1 * linkWeightNorm(l.value, maxL);
      path.attr('stroke', hit ? linkFocusStrokeColor(l, nodesById) : COLOR_LINK_DIM).attr('stroke-opacity', hit ? Math.min(0.26, base + 0.12) : 0.025);
    });

    root.selectAll<SVGPathElement, LinkDatum>('path.topology-link').each(function (_, index) {
      const path = d3.select(this);
      const l = path.datum() as LinkDatum;
      const s = typeof l.source === 'object' ? (l.source as NodeDatum).id : String(l.source);
      const t = typeof l.target === 'object' ? (l.target as NodeDatum).id : String(l.target);
      if (!fadeOthers) {
        path
          .attr('stroke', linkStrokeWithTrust(l, maxL, nodesById, topologyTrust?.linkStyles[index]))
          .attr('stroke-opacity', loScale(l.value));
        return;
      }
      const hit = s === focusId || t === focusId;
      path.attr('stroke', hit ? linkFocusStrokeColor(l, nodesById) : COLOR_LINK_DIM).attr('stroke-opacity', hit ? Math.max(0.62, loScale(l.value)) : 0.06);
    });

    root.select('g.topology-particles').selectAll<SVGCircleElement, ParticleDatum>('circle').each(function () {
      const d = d3.select(this).datum() as ParticleDatum;
      const l = links[d.linkIndex];
      if (!l) {
        d3.select(this).attr('opacity', 0.05);
        return;
      }
      const s = typeof l.source === 'string' ? l.source : (l.source as NodeDatum).id;
      const t = typeof l.target === 'string' ? l.target : (l.target as NodeDatum).id;
      const baseOp = 0.58 + 0.32 * linkWeightNorm(l.value, maxL);
      if (!fadeOthers) {
        d3.select(this).attr('opacity', baseOp);
        return;
      }
      const hit = s === focusId || t === focusId;
      d3.select(this).attr('opacity', hit ? Math.min(0.98, baseOp + 0.12) : 0.05);
    });
  }, [selectedNodeId, nodes, glowId, links, topologyTrust]);

  if (nodes.length === 0) {
    return (
      <div className="flex items-center justify-center py-12 text-muted text-body">
        Not enough data to show topology. Need Top destinations and Top talkers (or connection edges) loaded.
      </div>
    );
  }

  return (
    <div ref={containerRef} className={`relative w-full h-full min-h-0 ${className}`.trim()}>
      {showCompactLegend && (
        <div className="absolute left-2 top-2 z-10 flex max-w-[calc(100%-10rem)] flex-wrap gap-x-3 gap-y-1 rounded-lg border border-border/80 bg-base/90 px-2.5 py-1.5 text-caption text-muted shadow-sm backdrop-blur">
          <span className="flex items-center gap-1.5">
            <span className="inline-block w-3 h-3 rounded-full shrink-0" style={{ background: COLOR_POD_FILL }} />
            Pod source
          </span>
          <span className="flex items-center gap-1.5">
            <span
              className="inline-block h-3 w-3 shrink-0 rounded-[2px]"
              style={{ background: COLOR_DEST_INTERNAL_FILL, opacity: 0.7 }}
            />
            Cluster service / pod endpoint
          </span>
          <span className="flex items-center gap-1.5">
            <span
              className="inline-block h-3 w-3 shrink-0 rotate-45 border bg-base"
              style={{ borderColor: COLOR_DEST_EXTERNAL_STROKE }}
            />
            External endpoint
          </span>
          <span className="flex items-center gap-1.5">
            <span className="inline-block h-px w-5 shrink-0" style={{ background: NETWORK_TOPOLOGY_PALETTE.edgeInternal }} />
            Internal flow
          </span>
          <span className="flex items-center gap-1.5">
            <span className="inline-block h-px w-5 shrink-0" style={{ background: NETWORK_TOPOLOGY_PALETTE.edgeExternal }} />
            External flow
          </span>
          <span className="flex items-center gap-1.5">
            <span className="inline-flex w-7 shrink-0 items-center gap-0.5">
              <span className="inline-block h-px w-3 rounded-full bg-muted/50" />
              <span className="inline-block h-[3px] w-3 rounded-full" style={{ background: NETWORK_TOPOLOGY_PALETTE.edgeInternal }} />
            </span>
            Low/high traffic
          </span>
          {showTrafficParticles && (
            <span className="text-muted-2 w-full">Particles are decorative activity cues, not live packets.</span>
          )}
          <span className="text-muted-2">Click/focus a node to isolate its flows</span>
        </div>
      )}

      {graphIsSummarized && (
        <div className="absolute right-2 top-2 z-10 max-w-[min(100%,18rem)] rounded-lg border border-amber-500/30 bg-amber-950/30 px-2.5 py-1.5 text-caption text-amber-100 shadow-sm backdrop-blur">
          Showing top {nodes.length} of {sourceEntityCount} entities. Tables keep full evidence.
        </div>
      )}

      {showTrustOverlay && topologyTrust ? (
        <div className="absolute bottom-2 left-2 z-10 max-w-md pointer-events-none">
          <GraphTrustOverlay posture={topologyTrust.posture} showLegend />
        </div>
      ) : null}

      {tooltip && (
        <div
          className="absolute z-20 rounded-lg border border-border bg-surface/95 backdrop-blur px-3 py-2 text-caption text-text pointer-events-none whitespace-pre-line shadow-lg max-w-xs"
          style={{ left: tooltip.x + 12, top: tooltip.y }}
        >
          {tooltip.content}
        </div>
      )}

      <svg
        ref={svgRef}
        width={dimensions.width}
        height={dimensions.height}
        style={{ background: BG, borderRadius: 8 }}
        role="img"
        aria-label={`Network topology graph: ${nodes.filter(n => n.kind === 'pod').length} pods, ${nodes.filter(n => n.kind === 'dest').length} destinations, ${links.length} connections. Interactive: drag nodes, scroll to zoom.`}
      >
        <title>{`Network topology: ${nodes.length} nodes, ${links.length} connections`}</title>
      </svg>
      <details className="mt-3 rounded-lg border border-border bg-surface/80 p-3 text-caption text-muted">
        <summary className="cursor-pointer text-body font-semibold text-text">Keyboard topology outline</summary>
        <div className="mt-3 grid gap-3 md:grid-cols-2">
          <div>
            <h3 className="text-caption font-semibold text-text">Entities</h3>
            <ul className="mt-2 space-y-1">
              {nodes.slice(0, 12).map((node) => (
                <li key={node.id}>
                  <button
                    type="button"
                    className="text-left text-muted transition-colors hover:text-text focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/60"
                    onClick={() => selectNode(node)}
                  >
                    <span className="font-medium text-text">{node.fullLabel}</span>
                    <span className="text-muted"> · {node.kind}{node.namespace ? ` · ${node.namespace}` : ''}</span>
                  </button>
                </li>
              ))}
            </ul>
          </div>
          <div>
            <h3 className="text-caption font-semibold text-text">Observed connections</h3>
            <ol className="mt-2 space-y-1">
              {links.slice(0, 12).map((link, index) => {
                const source = typeof link.source === 'object' ? link.source : nodes.find((n) => n.id === String(link.source));
                const target = typeof link.target === 'object' ? link.target : nodes.find((n) => n.id === String(link.target));
                return (
                  <li key={`${source?.id ?? 'source'}-${target?.id ?? 'target'}-${index}`}>
                    <span className="font-medium text-text">{source?.fullLabel ?? 'Source'}</span>
                    <span> to </span>
                    <span className="font-medium text-text">{target?.fullLabel ?? 'Destination'}</span>
                    <span className="text-muted"> · {link.value} flow{link.value === 1 ? '' : 's'}</span>
                  </li>
                );
              })}
            </ol>
          </div>
        </div>
        {nodes.length > 12 || links.length > 12 ? (
          <p className="mt-3 text-muted-2">Showing the first 12 entities and connections. Use filters or table rows for the full evidence set.</p>
        ) : null}
      </details>
    </div>
  );
};
