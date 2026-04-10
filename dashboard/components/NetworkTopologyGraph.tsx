import React, { useEffect, useId, useMemo, useRef, useState } from 'react';
import * as d3 from 'd3';
import type {
  NetworkActivityConnectionRow,
  NetworkActivityDestinationRow,
  NetworkActivityTalkerRow,
} from '../types';

/* ─── types ───────────────────────────────────────────────────── */

interface NodeDatum extends d3.SimulationNodeDatum {
  id: string;
  /** Dòng 1: tên pod / đích (không dùng UID làm dòng chính khi đã có tên) */
  line1: string;
  /** Dòng 2: ns + gợi ý uid (pod) hoặc external / ns đích */
  line2: string;
  /** Tooltip đầy đủ */
  fullLabel: string;
  kind: 'pod' | 'dest';
  weight: number;
  namespace?: string;
  ownerLabel?: string;
}

interface LinkDatum extends d3.SimulationLinkDatum<NodeDatum> {
  value: number;
}

/** Hạt chạy dọc path — chỉ visual, không gắn NetworkPolicy */
interface ParticleDatum {
  linkIndex: number;
  phase: number;
  speed: number;
}

export interface NetworkTopologyGraphProps {
  destinations: NetworkActivityDestinationRow[];
  talkers: NetworkActivityTalkerRow[];
  /** Hàng kết nối thật (cùng bộ lọc API); ưu tiên dùng để vẽ cạnh pod→đích đúng với namespace/lọc. */
  connections?: NetworkActivityConnectionRow[];
  /** Pod có activity (talkers) nhưng không nằm trong tập cạnh hiện tại — thêm nút pod orphan vào đồ thị. */
  supplementTalkers?: NetworkActivityTalkerRow[];
  maxNodes?: number;
  onNodeClick?: (nodeId: string, kind: 'pod' | 'dest') => void;
  showCompactLegend?: boolean;
  className?: string;
  /** Mô phỏng lưu lượng: chấm xanh chạy dọc cạnh (decorative). Mặc định bật. */
  showTrafficParticles?: boolean;
  /** uid → name từ GET /inventory/pods (bù khi edges/talkers không có podName từ JOIN). */
  podNamesByUid?: Readonly<Record<string, string>>;
}

/* ─── helpers ─────────────────────────────────────────────────── */

const MAX_LINKS_PER_DESTINATION = 5;

/** Màu theo spec UX: workload pod vs thực thể ngoài / đích */
const COLOR_POD_FILL = '#db2777';
const COLOR_POD_STROKE = '#be185d';
const COLOR_DEST_FILL = '#64748b';
const COLOR_DEST_STROKE = '#475569';
/** Luồng quan sát (aggregate API — không phải NetworkPolicy allow/deny) */
const COLOR_LINK = '#10b981';
const COLOR_LINK_DIM = '#065f46';
const COLOR_LINK_HOVER = '#34d399';
const COLOR_PARTICLE = '#a7f3d0';
const COLOR_PARTICLE_CORE = '#ecfdf5';
const BG = '#020617';

/** Giới hạn số hạt để rAF nhẹ (~2 hạt / cạnh tối đa 55 cạnh) */
const MAX_LINK_INDICES_FOR_PARTICLES = 55;

/**
 * Bán kính node (đơn vị SVG trước khi zoom “fit”).
 * Tăng so với [5,22] cũ để dễ nhìn; fitTopologyToView vẫn căn toàn bộ graph + nhãn trong khung.
 */
const NODE_RADIUS_MIN = 8;
const NODE_RADIUS_MAX = 30;
/** Đệm va chạm force = bán kính + pad (tránh chồng nhãn/node). */
const NODE_COLLISION_EXTRA = 20;
/** Đệm bbox khi fit: vòng tròn + 2 dòng text bên dưới. */
const NODE_FIT_PADDING = 34;
const NODE_STROKE_DEFAULT = 2;
const NODE_STROKE_SELECTED = 4;

function truncateGraphLabel(text: string, max = 26): string {
  const t = text.trim();
  if (t.length <= max) return t;
  return `${t.slice(0, max - 1)}…`;
}

/** 8 ký tự đầu + … nếu dài hơn — luôn hiển thị trên dòng phụ cho pod. */
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

/** Gộp meta theo uid: ưu tiên podName/namespace không rỗng từ bất kỳ dòng nào (tránh dòng đầu thiếu tên). */
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

/** Dòng 1 node pod: tên pod → owner → fallback uid. */
function podNodeLine1(
  name: string | undefined,
  uid: string,
  ownerKind?: string,
  ownerName?: string,
): string {
  const n = (name ?? '').trim();
  if (n) return truncateGraphLabel(n, 28);
  const ok = (ownerKind ?? '').trim();
  const on = (ownerName ?? '').trim();
  if (ok || on) return truncateGraphLabel([ok, on].filter(Boolean).join('/'), 28);
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
      const line1 = podNodeLine1(name || undefined, t.podUid, t.ownerKind, t.ownerName);
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
      const line1 = truncateGraphLabel(
        hasWorkload ? `${d.destWorkloadName}:${d.destPort}` : `${d.destIp}:${d.destPort}`,
        28,
      );
      const ns = (d.destWorkloadNamespace ?? '').trim();
      const line2 = ns ? `ns: ${ns}` : 'external';
      const fullLabel = hasWorkload
        ? `${d.destWorkloadName} · ${d.destIp}:${d.destPort}/${d.protocol ?? 'tcp'}`
        : `${d.destIp}:${d.destPort}/${d.protocol ?? 'tcp'}`;
      nodeMap.set(destKey, {
        id: destKey,
        line1,
        line2,
        fullLabel,
        kind: 'dest',
        weight: d.observationCount,
        namespace: d.destWorkloadNamespace,
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

/** Gom các dòng connections hoặc edges (mỗi dòng có thể có observationCount > 1); giữ top pod/đích theo trọng số. */
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
      if (!podWeight.has(uid)) {
        podWeight.set(uid, Math.max(1, Number(t.observationCount) || 1));
      }
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
    const line1 = podNodeLine1(name || undefined, uid, meta.ownerKind, meta.ownerName);
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
    const line1 = truncateGraphLabel(`${t.destIp}:${t.destPort}`, 28);
    nodeMap.set(destKey, {
      id: destKey,
      line1,
      line2: 'external',
      fullLabel: `${t.destIp}:${t.destPort}/${t.protocol}`,
      kind: 'dest',
      weight: w,
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
 * Chuẩn hoá trọng số cạnh [0, 1] — sqrt để cạnh yếu vẫn phân biệt được, cạnh mạnh không chiếm hết dải.
 */
function linkWeightNorm(value: number, maxLinkVal: number): number {
  const m = Math.max(maxLinkVal, 1e-9);
  return Math.min(1, Math.sqrt(Math.max(0, value) / m));
}

function linkStyleScales(maxLinkVal: number) {
  const maxV = Math.max(maxLinkVal, 1);
  const linkWidthScale = d3.scaleLinear().domain([0, maxV]).range([0.65, 4.2]);
  const linkOpacityScale = d3.scaleLinear().domain([0, maxV]).range([0.26, 0.92]);
  const linkColorScale = d3
    .scaleLinear<string>()
    .domain([0, maxV])
    .range([COLOR_LINK_DIM, COLOR_LINK])
    .interpolate(d3.interpolateRgb);
  return { maxV, linkWidthScale, linkOpacityScale, linkColorScale };
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
  showTrafficParticles = true,
  podNamesByUid,
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

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width, height } = entry.contentRect;
        if (width > 0 && height > 0) {
          // Không ép min 360px — tránh SVG cao hơn container flex (tràn khung browser).
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
    setSelectedNodeId(null);

    const maxWeight = d3.max(nodes, (d) => d.weight) ?? 1;
    const rScale = d3.scaleSqrt().domain([0, maxWeight]).range([NODE_RADIUS_MIN, NODE_RADIUS_MAX]);
    const maxLinkVal = d3.max(links, (d) => d.value) ?? 1;
    const { linkWidthScale, linkOpacityScale, linkColorScale } = linkStyleScales(maxLinkVal);

    const nodesCopy: NodeDatum[] = nodes.map((n) => ({ ...n }));
    const linksCopy: LinkDatum[] = links.map((l) => ({ ...l }));

    const diag = Math.sqrt(Math.max(1, width * height));
    const linkDistance = Math.min(188, Math.max(88, diag / 12));
    const chargeStrength = -Math.min(560, Math.max(240, diag * 0.44));

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
      .force('collision', d3.forceCollide<NodeDatum>().radius((d) => rScale(d.weight) + NODE_COLLISION_EXTRA))
      .alphaDecay(0.06)
      .velocityDecay(0.22);

    const defs = svg.append('defs');

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
      .attr('fill', COLOR_LINK);

    const gRoot = svg.append('g').attr('data-topology-root', '1');

    const zoom = d3
      .zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.08, 5])
      .on('zoom', (event) => {
        gRoot.attr('transform', event.transform);
      });
    svg.call(zoom);

    const pad = 56;
    const fitTopologyToView = () => {
      if (!svgRef.current || nodesCopy.length === 0) return;
      let minX = Infinity;
      let minY = Infinity;
      let maxX = -Infinity;
      let maxY = -Infinity;
      for (const n of nodesCopy) {
        const x = n.x ?? 0;
        const y = n.y ?? 0;
        const r = rScale(n.weight) + NODE_FIT_PADDING;
        minX = Math.min(minX, x - r);
        maxX = Math.max(maxX, x + r);
        minY = Math.min(minY, y - r);
        maxY = Math.max(maxY, y + r + 34);
      }
      if (!Number.isFinite(minX) || !Number.isFinite(minY)) return;
      const bw = Math.max(maxX - minX, 64);
      const bh = Math.max(maxY - minY, 64);
      const k = Math.min((width - 2 * pad) / bw, (height - 2 * pad) / bh, 5);
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
      .attr('fill', 'transparent')
      .style('cursor', 'grab')
      .lower()
      .on('click', (event) => {
        event.stopPropagation();
        setSelectedNodeId(null);
      });

    const linkG = gRoot.append('g').attr('fill', 'none');

    const linkPath = linkG
      .selectAll('path')
      .data(linksCopy)
      .join('path')
      .attr('stroke', (d) => linkColorScale(d.value))
      .attr('stroke-width', (d) => linkWidthScale(d.value))
      .attr('stroke-opacity', (d) => linkOpacityScale(d.value))
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
      .append('circle')
      .attr('data-node-id', (d) => d.id)
      .attr('r', (d) => rScale(d.weight))
      .attr('fill', (d) => (d.kind === 'pod' ? COLOR_POD_FILL : COLOR_DEST_FILL))
      .attr('fill-opacity', 0.92)
      .attr('stroke', (d) => (d.kind === 'pod' ? COLOR_POD_STROKE : COLOR_DEST_STROKE))
      .attr('stroke-width', NODE_STROKE_DEFAULT);

    node
      .append('text')
      .attr('text-anchor', 'middle')
      .attr('pointer-events', 'none')
      .each(function (d) {
        const r = rScale(d.weight);
        const g = d3.select(this);
        g.append('tspan')
          .attr('x', 0)
          .attr('dy', r + 14)
          .attr('fill', '#e2e8f0')
          .attr('font-size', '12px')
          .attr('font-weight', '500')
          .text(d.line1);
        g.append('tspan')
          .attr('x', 0)
          .attr('dy', 13)
          .attr('fill', '#94a3b8')
          .attr('font-size', '10px')
          .text(d.line2);
      });

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
      linkPath
        .attr('stroke', (l) => {
          const s = typeof l.source === 'object' ? (l.source as NodeDatum).id : l.source;
          const t = typeof l.target === 'object' ? (l.target as NodeDatum).id : l.target;
          if (!focusId) return linkColorScale(l.value);
          return s === focusId || t === focusId ? COLOR_LINK_HOVER : COLOR_LINK_DIM;
        })
        .attr('stroke-opacity', (l) => {
          const s = typeof l.source === 'object' ? (l.source as NodeDatum).id : l.source;
          const t = typeof l.target === 'object' ? (l.target as NodeDatum).id : l.target;
          if (!focusId) return linkOpacityScale(l.value);
          return s === focusId || t === focusId ? 0.95 : 0.08;
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
        const parts: string[] = [`${d.kind === 'pod' ? 'Pod' : 'Đích / ngoài cluster'}: ${d.fullLabel}`];
        if (d.namespace) parts.push(`Namespace: ${d.namespace}`);
        if (d.ownerLabel) parts.push(`Owner: ${d.ownerLabel}`);
        parts.push(`Quan sát: ${d.weight}`);
        setTooltip({ x, y: y - 10, content: parts.join('\n') });
        applyHighlight(d.id);
      })
      .on('mouseleave', () => {
        setTooltip(null);
        applyHighlight(selectedNodeIdRef.current);
      })
      .on('click', (event, d) => {
        event.stopPropagation();
        const wasSelected = selectedNodeIdRef.current === d.id;
        setSelectedNodeId((prev) => (prev === d.id ? null : d.id));
        if (onNodeClick && d.kind === 'pod' && !wasSelected) {
          const rawId = d.id.replace(/^(pod|dest):/, '');
          onNodeClick(rawId, d.kind);
        }
      });

    simulation.on('tick', () => {
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
  }, [nodes, links, dimensions, onNodeClick, arrowId, glowId, showTrafficParticles]);

  useEffect(() => {
    const svgEl = svgRef.current;
    if (!svgEl || nodes.length === 0) return;
    const root = d3.select(svgEl).select('[data-topology-root]');
    if (root.empty()) return;

    const focusId = selectedNodeId;
    root.selectAll<SVGGElement, NodeDatum>('g.topology-node').each(function (d) {
      const on = d.id === focusId;
      d3.select(this)
        .select('circle')
        .attr('stroke-width', on ? NODE_STROKE_SELECTED : NODE_STROKE_DEFAULT)
        .attr('filter', on ? `url(#${glowId})` : null);
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
    const { linkOpacityScale: loScale, linkColorScale: lcScale } = linkStyleScales(maxL);

    root.selectAll<SVGPathElement, LinkDatum>('path').each(function () {
      const path = d3.select(this);
      const l = path.datum() as LinkDatum;
      const s = typeof l.source === 'object' ? (l.source as NodeDatum).id : String(l.source);
      const t = typeof l.target === 'object' ? (l.target as NodeDatum).id : String(l.target);
      if (!fadeOthers) {
        path.attr('stroke', lcScale(l.value)).attr('stroke-opacity', loScale(l.value));
        return;
      }
      const hit = s === focusId || t === focusId;
      path.attr('stroke', hit ? COLOR_LINK_HOVER : COLOR_LINK_DIM).attr('stroke-opacity', hit ? 0.95 : 0.08);
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
  }, [selectedNodeId, nodes.length, glowId, links]);

  if (nodes.length === 0) {
    return (
      <div className="flex items-center justify-center py-12 text-slate-500 text-sm">
        Không đủ dữ liệu để hiển thị topology. Cần có dữ liệu từ cả "Top đích" và "Top nguồn".
      </div>
    );
  }

  return (
    <div ref={containerRef} className={`relative w-full h-full min-h-0 ${className}`.trim()}>
      {showCompactLegend && (
        <div className="absolute top-2 left-2 z-10 flex flex-wrap gap-x-4 gap-y-1 text-xs text-slate-400 bg-slate-900/85 backdrop-blur rounded-lg px-3 py-1.5 border border-slate-800 max-w-[min(100%,28rem)]">
          <span className="flex items-center gap-1.5">
            <span className="inline-block w-3 h-3 rounded-full shrink-0" style={{ background: COLOR_POD_FILL }} />
            Pod (workload)
          </span>
          <span className="flex items-center gap-1.5">
            <span className="inline-block w-3 h-3 rounded-full shrink-0" style={{ background: COLOR_DEST_FILL }} />
            Đích / ngoài
          </span>
          <span className="flex items-center gap-1.5 text-emerald-400/90">
            <span className="inline-block w-4 h-0.5 bg-emerald-500 shrink-0" />
            Cạnh: dày / sáng / hạt nhanh ≈ mức hoạt động
          </span>
          <span className="text-slate-600 w-full sm:w-auto">Zoom · kéo node</span>
        </div>
      )}

      {tooltip && (
        <div
          className="absolute z-20 rounded-lg border border-slate-700 bg-slate-900/95 backdrop-blur px-3 py-2 text-xs text-slate-300 pointer-events-none whitespace-pre-line shadow-lg max-w-xs"
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
      />
    </div>
  );
};
