import React, { useRef, useEffect, useMemo, useState } from 'react';
import * as d3 from 'd3';
import type {
  NetworkActivityDestinationRow,
  NetworkActivityTalkerRow,
} from '../types';

/* ─── types ───────────────────────────────────────────────────── */

interface NodeDatum extends d3.SimulationNodeDatum {
  id: string;
  label: string;
  /** 'pod' = source pod, 'dest' = destination endpoint */
  kind: 'pod' | 'dest';
  /** Metric used to size the node (observation count or connection count) */
  weight: number;
  namespace?: string;
  ownerLabel?: string;
}

interface LinkDatum extends d3.SimulationLinkDatum<NodeDatum> {
  /** Observations / connections between source and dest */
  value: number;
}

export interface NetworkTopologyGraphProps {
  destinations: NetworkActivityDestinationRow[];
  talkers: NetworkActivityTalkerRow[];
  /** Maximum nodes to render — avoids overwhelming the SVG. Default 80. */
  maxNodes?: number;
  onNodeClick?: (nodeId: string, kind: 'pod' | 'dest') => void;
}

/* ─── helpers ─────────────────────────────────────────────────── */

function buildGraph(
  destinations: NetworkActivityDestinationRow[],
  talkers: NetworkActivityTalkerRow[],
  maxNodes: number,
): { nodes: NodeDatum[]; links: LinkDatum[] } {
  const nodeMap = new Map<string, NodeDatum>();
  const links: LinkDatum[] = [];

  // Add top talker pods (source nodes)
  const topTalkers = talkers.slice(0, Math.min(talkers.length, Math.floor(maxNodes * 0.5)));
  for (const t of topTalkers) {
    const id = `pod:${t.podUid}`;
    if (!nodeMap.has(id)) {
      nodeMap.set(id, {
        id,
        label: t.podName || t.podUid.slice(0, 12),
        kind: 'pod',
        weight: t.observationCount,
        namespace: t.namespace,
        ownerLabel: t.ownerKind && t.ownerName ? `${t.ownerKind}/${t.ownerName}` : undefined,
      });
    }
  }

  // Add top destinations (dest nodes)
  const topDests = destinations.slice(0, Math.min(destinations.length, Math.floor(maxNodes * 0.5)));
  for (const d of topDests) {
    const destKey = `dest:${d.destIp}:${d.destPort}/${d.protocol ?? 'tcp'}`;
    if (!nodeMap.has(destKey)) {
      nodeMap.set(destKey, {
        id: destKey,
        label: d.destWorkloadName
          ? `${d.destWorkloadName}:${d.destPort}`
          : `${d.destIp}:${d.destPort}`,
        kind: 'dest',
        weight: d.observationCount,
        namespace: d.destWorkloadNamespace,
      });
    }
  }

  // Build links: connect each talker to each destination (weighted by min observation)
  // We use a simple heuristic since the API doesn't give per-pod->dest pairs:
  // link each talker to every top destination with weight proportional to both.
  const podIds = topTalkers.map((t) => `pod:${t.podUid}`);
  for (const d of topDests) {
    const destKey = `dest:${d.destIp}:${d.destPort}/${d.protocol ?? 'tcp'}`;
    // Connect to each pod that could reach this dest (top N by observation)
    const podsToLink = podIds.slice(0, Math.min(podIds.length, 5));
    for (const podId of podsToLink) {
      links.push({
        source: podId,
        target: destKey,
        value: Math.min(
          nodeMap.get(podId)?.weight ?? 1,
          nodeMap.get(destKey)?.weight ?? 1,
        ),
      });
    }
  }

  return { nodes: Array.from(nodeMap.values()), links };
}

const COLOR_POD = '#38bdf8'; // sky-400
const COLOR_DEST = '#f472b6'; // pink-400
const COLOR_LINK = '#334155'; // slate-700
const COLOR_LINK_HOVER = '#64748b'; // slate-500
const BG = '#020617'; // slate-950

/* ─── component ───────────────────────────────────────────────── */

export const NetworkTopologyGraph: React.FC<NetworkTopologyGraphProps> = ({
  destinations,
  talkers,
  maxNodes = 80,
  onNodeClick,
}) => {
  const svgRef = useRef<SVGSVGElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const [dimensions, setDimensions] = useState({ width: 900, height: 500 });
  const [tooltip, setTooltip] = useState<{ x: number; y: number; content: string } | null>(null);

  // Build graph data from aggregated views
  const { nodes, links } = useMemo(
    () => buildGraph(destinations, talkers, maxNodes),
    [destinations, talkers, maxNodes],
  );

  // Observe container size
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width, height } = entry.contentRect;
        if (width > 0 && height > 0) {
          setDimensions({ width, height: Math.max(height, 400) });
        }
      }
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  // D3 force simulation
  useEffect(() => {
    if (!svgRef.current || nodes.length === 0) return;

    const { width, height } = dimensions;
    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    // Scale for node radius
    const maxWeight = d3.max(nodes, (d) => d.weight) ?? 1;
    const rScale = d3.scaleSqrt().domain([0, maxWeight]).range([4, 20]);

    // Scale for link width
    const maxLinkVal = d3.max(links, (d) => d.value) ?? 1;
    const linkWidthScale = d3.scaleLinear().domain([0, maxLinkVal]).range([0.5, 3]);

    // Create a copy so D3 doesn't mutate our memoised objects
    const nodesCopy: NodeDatum[] = nodes.map((n) => ({ ...n }));
    const linksCopy: LinkDatum[] = links.map((l) => ({ ...l }));

    const simulation = d3
      .forceSimulation(nodesCopy)
      .force(
        'link',
        d3.forceLink<NodeDatum, LinkDatum>(linksCopy).id((d) => d.id).distance(100).strength(0.3),
      )
      .force('charge', d3.forceManyBody().strength(-200))
      .force('center', d3.forceCenter(width / 2, height / 2))
      .force('collision', d3.forceCollide<NodeDatum>().radius((d) => rScale(d.weight) + 4));

    // Container for zoom
    const g = svg.append('g');

    // Zoom behavior
    const zoom = d3.zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.2, 5])
      .on('zoom', (event) => {
        g.attr('transform', event.transform);
      });
    svg.call(zoom);

    // Arrow marker
    svg
      .append('defs')
      .append('marker')
      .attr('id', 'arrowhead')
      .attr('viewBox', '0 -5 10 10')
      .attr('refX', 20)
      .attr('refY', 0)
      .attr('markerWidth', 6)
      .attr('markerHeight', 6)
      .attr('orient', 'auto')
      .append('path')
      .attr('d', 'M0,-4L8,0L0,4')
      .attr('fill', COLOR_LINK);

    // Links
    const link = g
      .append('g')
      .selectAll('line')
      .data(linksCopy)
      .join('line')
      .attr('stroke', COLOR_LINK)
      .attr('stroke-width', (d) => linkWidthScale(d.value))
      .attr('stroke-opacity', 0.6)
      .attr('marker-end', 'url(#arrowhead)');

    // Node groups
    const node = g
      .append('g')
      .selectAll<SVGGElement, NodeDatum>('g')
      .data(nodesCopy)
      .join('g')
      .style('cursor', 'pointer')
      .call(
        d3
          .drag<SVGGElement, NodeDatum>()
          .on('start', (event, d) => {
            if (!event.active) simulation.alphaTarget(0.3).restart();
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

    // Node circles
    node
      .append('circle')
      .attr('r', (d) => rScale(d.weight))
      .attr('fill', (d) => (d.kind === 'pod' ? COLOR_POD : COLOR_DEST))
      .attr('fill-opacity', 0.85)
      .attr('stroke', (d) => (d.kind === 'pod' ? '#0284c7' : '#db2777'))
      .attr('stroke-width', 1.5);

    // Node labels
    node
      .append('text')
      .text((d) => d.label)
      .attr('dy', (d) => rScale(d.weight) + 12)
      .attr('text-anchor', 'middle')
      .attr('fill', '#94a3b8')
      .attr('font-size', '10px')
      .attr('pointer-events', 'none');

    // Hover effects
    node
      .on('mouseenter', (event, d) => {
        const [x, y] = d3.pointer(event, svgRef.current);
        const parts: string[] = [`${d.kind === 'pod' ? 'Pod' : 'Destination'}: ${d.label}`];
        if (d.namespace) parts.push(`NS: ${d.namespace}`);
        if (d.ownerLabel) parts.push(`Owner: ${d.ownerLabel}`);
        parts.push(`Observations: ${d.weight}`);
        setTooltip({ x, y: y - 10, content: parts.join('\n') });

        // Highlight connected links
        link
          .attr('stroke', (l) => {
            const src = typeof l.source === 'object' ? (l.source as NodeDatum).id : l.source;
            const tgt = typeof l.target === 'object' ? (l.target as NodeDatum).id : l.target;
            return src === d.id || tgt === d.id ? COLOR_LINK_HOVER : COLOR_LINK;
          })
          .attr('stroke-opacity', (l) => {
            const src = typeof l.source === 'object' ? (l.source as NodeDatum).id : l.source;
            const tgt = typeof l.target === 'object' ? (l.target as NodeDatum).id : l.target;
            return src === d.id || tgt === d.id ? 1 : 0.3;
          });
      })
      .on('mouseleave', () => {
        setTooltip(null);
        link.attr('stroke', COLOR_LINK).attr('stroke-opacity', 0.6);
      })
      .on('click', (_event, d) => {
        if (onNodeClick) {
          const rawId = d.id.replace(/^(pod|dest):/, '');
          onNodeClick(rawId, d.kind);
        }
      });

    // Tick
    simulation.on('tick', () => {
      link
        .attr('x1', (d) => ((d.source as NodeDatum).x ?? 0))
        .attr('y1', (d) => ((d.source as NodeDatum).y ?? 0))
        .attr('x2', (d) => ((d.target as NodeDatum).x ?? 0))
        .attr('y2', (d) => ((d.target as NodeDatum).y ?? 0));
      node.attr('transform', (d) => `translate(${d.x ?? 0},${d.y ?? 0})`);
    });

    return () => {
      simulation.stop();
    };
  }, [nodes, links, dimensions, onNodeClick]);

  if (nodes.length === 0) {
    return (
      <div className="flex items-center justify-center py-12 text-slate-500 text-sm">
        Không đủ dữ liệu để hiển thị topology. Cần có dữ liệu từ cả "Top đích" và "Top nguồn".
      </div>
    );
  }

  return (
    <div ref={containerRef} className="relative w-full" style={{ minHeight: 420 }}>
      {/* Legend */}
      <div className="absolute top-2 left-2 z-10 flex gap-4 text-xs text-slate-400 bg-slate-900/80 backdrop-blur rounded-lg px-3 py-1.5 border border-slate-800">
        <span className="flex items-center gap-1.5">
          <span className="inline-block w-3 h-3 rounded-full" style={{ background: COLOR_POD }} />
          Pod (nguồn)
        </span>
        <span className="flex items-center gap-1.5">
          <span className="inline-block w-3 h-3 rounded-full" style={{ background: COLOR_DEST }} />
          Đích (dest)
        </span>
        <span className="text-slate-600">Kéo node · Scroll zoom</span>
      </div>

      {/* Tooltip */}
      {tooltip && (
        <div
          className="absolute z-20 rounded-lg border border-slate-700 bg-slate-900/95 backdrop-blur px-3 py-2 text-xs text-slate-300 pointer-events-none whitespace-pre-line shadow-lg"
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
