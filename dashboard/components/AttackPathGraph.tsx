import React, { useEffect, useRef, useState, useId } from 'react';
import * as d3 from 'd3';
import type { AttackPathGraphData } from '../types';

/* ─── types ───────────────────────────────────────────────── */

interface GraphNode extends d3.SimulationNodeDatum {
  id: string;
  label: string;
  type: string;
  risk?: string;
}

interface GraphLink extends d3.SimulationLinkDatum<GraphNode> {
  type?: string;
  value: number;
}

interface AttackPathGraphProps {
  data: AttackPathGraphData;
  width?: number;
  height?: number;
  onNodeClick?: (nodeId: string, type: string) => void;
  className?: string;
}

/* ─── colours ─────────────────────────────────────────────── */

const NODE_COLORS: Record<string, { fill: string; stroke: string }> = {
  pod:                { fill: '#db2777', stroke: '#be185d' },
  serviceaccount:     { fill: '#3b82f6', stroke: '#2563eb' },
  rolebinding:        { fill: '#f59e0b', stroke: '#d97706' },
  clusterrolebinding: { fill: '#f59e0b', stroke: '#d97706' },
  role:               { fill: '#10b981', stroke: '#059669' },
  clusterrole:        { fill: '#ef4444', stroke: '#dc2626' },
};

const RISK_GLOW: Record<string, string> = {
  critical: '#ef4444',
  high:     '#f97316',
  medium:   '#eab308',
  low:      '#22c55e',
};

const EDGE_COLORS: Record<string, string> = {
  USES_SERVICE_ACCOUNT: '#818cf8',
  REFERENCED_BY:        '#fbbf24',
  GRANTS_ROLE:          '#f87171',
};

/* ─── component ───────────────────────────────────────────── */

export const AttackPathGraph: React.FC<AttackPathGraphProps> = ({
  data,
  width: propWidth,
  height: propHeight,
  onNodeClick,
  className = '',
}) => {
  const svgRef = useRef<SVGSVGElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const [dimensions, setDimensions] = useState({ width: propWidth || 900, height: propHeight || 500 });
  const clipId = useId();

  // Responsive sizing
  useEffect(() => {
    if (propWidth && propHeight) {
      setDimensions({ width: propWidth, height: propHeight });
      return;
    }
    const obs = new ResizeObserver((entries) => {
      const { width, height } = entries[0].contentRect;
      if (width > 0 && height > 0) setDimensions({ width, height: Math.max(height, 400) });
    });
    if (containerRef.current) obs.observe(containerRef.current);
    return () => obs.disconnect();
  }, [propWidth, propHeight]);

  // D3 rendering
  useEffect(() => {
    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    if (!data.nodes.length) return;

    const { width, height } = dimensions;

    // Build node/link data
    const nodeMap = new Map<string, GraphNode>();
    data.nodes.forEach((n) => {
      nodeMap.set(n.id, { ...n } as GraphNode);
    });
    const nodes = Array.from(nodeMap.values());

    const links: GraphLink[] = data.links
      .filter((l) => nodeMap.has(l.source as string) && nodeMap.has(l.target as string))
      .map((l) => ({ ...l } as GraphLink));

    // Container group for zoom
    const g = svg.append('g');

    // Zoom
    const zoom = d3.zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.2, 4])
      .on('zoom', (event) => g.attr('transform', event.transform));
    svg.call(zoom);

    // Arrow markers
    const markerTypes = ['USES_SERVICE_ACCOUNT', 'REFERENCED_BY', 'GRANTS_ROLE', 'default'];
    markerTypes.forEach((t) => {
      svg.append('defs').append('marker')
        .attr('id', `arrow-${t}-${clipId}`)
        .attr('viewBox', '0 -5 10 10')
        .attr('refX', 22)
        .attr('refY', 0)
        .attr('markerWidth', 6)
        .attr('markerHeight', 6)
        .attr('orient', 'auto')
        .append('path')
        .attr('d', 'M0,-5L10,0L0,5')
        .attr('fill', EDGE_COLORS[t] || '#64748b');
    });

    // Simulation
    const simulation = d3.forceSimulation<GraphNode>(nodes)
      .force('link', d3.forceLink<GraphNode, GraphLink>(links).id((d) => d.id).distance(120))
      .force('charge', d3.forceManyBody().strength(-400))
      .force('center', d3.forceCenter(width / 2, height / 2))
      .force('collision', d3.forceCollide().radius(40));

    // Links
    const linkGroup = g.append('g').attr('class', 'links');
    const link = linkGroup.selectAll<SVGLineElement, GraphLink>('line')
      .data(links)
      .join('line')
      .attr('stroke', (d) => EDGE_COLORS[(d as any).type] || '#475569')
      .attr('stroke-width', (d) => Math.max(1, Math.min(d.value / 3, 3)))
      .attr('stroke-opacity', 0.7)
      .attr('marker-end', (d) => `url(#arrow-${(d as any).type || 'default'}-${clipId})`);

    // Link labels
    const linkLabel = g.append('g').attr('class', 'link-labels')
      .selectAll<SVGTextElement, GraphLink>('text')
      .data(links)
      .join('text')
      .text((d) => {
        const t = (d as any).type || '';
        return t.replace(/_/g, ' ').toLowerCase();
      })
      .attr('font-size', 9)
      .attr('fill', '#94a3b8')
      .attr('text-anchor', 'middle')
      .attr('dy', -6);

    // Nodes
    const nodeGroup = g.append('g').attr('class', 'nodes');
    const node = nodeGroup.selectAll<SVGGElement, GraphNode>('g')
      .data(nodes)
      .join('g')
      .style('cursor', 'pointer')
      .call(
        d3.drag<SVGGElement, GraphNode>()
          .on('start', (event, d) => {
            if (!event.active) simulation.alphaTarget(0.3).restart();
            d.fx = d.x; d.fy = d.y;
          })
          .on('drag', (event, d) => { d.fx = event.x; d.fy = event.y; })
          .on('end', (event, d) => {
            if (!event.active) simulation.alphaTarget(0);
            d.fx = null; d.fy = null;
          }),
      );

    // Node circles
    node.append('circle')
      .attr('r', (d) => d.type === 'pod' ? 14 : d.type === 'clusterrole' ? 16 : 12)
      .attr('fill', (d) => NODE_COLORS[d.type]?.fill || '#64748b')
      .attr('stroke', (d) => NODE_COLORS[d.type]?.stroke || '#475569')
      .attr('stroke-width', 2);

    // Risk glow for pods
    node.filter((d) => d.risk && d.risk !== 'low')
      .append('circle')
      .attr('r', 20)
      .attr('fill', 'none')
      .attr('stroke', (d) => RISK_GLOW[d.risk || 'low'] || '#22c55e')
      .attr('stroke-width', 1.5)
      .attr('stroke-opacity', 0.5)
      .attr('stroke-dasharray', '4 2');

    // Node labels
    node.append('text')
      .text((d) => truncateLabel(d.label, 18))
      .attr('font-size', 10)
      .attr('fill', '#e2e8f0')
      .attr('text-anchor', 'middle')
      .attr('dy', 28);

    // Type badge (small text above node)
    node.append('text')
      .text((d) => d.type.replace(/binding$/, 'Bind'))
      .attr('font-size', 7)
      .attr('fill', '#94a3b8')
      .attr('text-anchor', 'middle')
      .attr('dy', -20);

    // Click handler
    node.on('click', (_event, d) => {
      onNodeClick?.(d.id, d.type);
    });

    // Tick
    simulation.on('tick', () => {
      link
        .attr('x1', (d) => (d.source as GraphNode).x!)
        .attr('y1', (d) => (d.source as GraphNode).y!)
        .attr('x2', (d) => (d.target as GraphNode).x!)
        .attr('y2', (d) => (d.target as GraphNode).y!);

      linkLabel
        .attr('x', (d) => ((d.source as GraphNode).x! + (d.target as GraphNode).x!) / 2)
        .attr('y', (d) => ((d.source as GraphNode).y! + (d.target as GraphNode).y!) / 2);

      node.attr('transform', (d) => `translate(${d.x},${d.y})`);
    });

    // Auto-fit after simulation settles
    simulation.on('end', () => {
      const bounds = (g.node() as SVGGElement)?.getBBox();
      if (bounds) {
        const padding = 40;
        const scale = Math.min(
          width / (bounds.width + padding * 2),
          height / (bounds.height + padding * 2),
          1.5,
        );
        const tx = width / 2 - (bounds.x + bounds.width / 2) * scale;
        const ty = height / 2 - (bounds.y + bounds.height / 2) * scale;
        svg.transition().duration(500).call(
          zoom.transform,
          d3.zoomIdentity.translate(tx, ty).scale(scale),
        );
      }
    });

    return () => {
      simulation.stop();
    };
  }, [data, dimensions, clipId, onNodeClick]);

  // Empty state
  if (!data.nodes.length) {
    return (
      <div className={`flex items-center justify-center py-16 text-slate-500 ${className}`}>
        <div className="text-center">
          <p className="text-sm font-medium">Không tìm thấy attack path</p>
          <p className="text-xs mt-1 opacity-75">Chưa có dữ liệu RBAC binding nào dẫn đến quyền nhạy cảm.</p>
        </div>
      </div>
    );
  }

  return (
    <div ref={containerRef} className={`relative w-full ${className}`} style={{ minHeight: 400 }}>
      <svg
        ref={svgRef}
        width={dimensions.width}
        height={dimensions.height}
        className="bg-slate-950/50 rounded-lg border border-slate-800"
      />
      {/* Legend */}
      <div className="absolute bottom-3 left-3 flex flex-wrap gap-3 text-[10px] text-slate-400">
        {Object.entries(NODE_COLORS).map(([type, { fill }]) => (
          <span key={type} className="flex items-center gap-1">
            <span className="w-2.5 h-2.5 rounded-full inline-block" style={{ background: fill }} />
            {type.replace(/^(.)/, (c) => c.toUpperCase())}
          </span>
        ))}
      </div>
    </div>
  );
};

function truncateLabel(text: string, max: number): string {
  if (!text) return '';
  return text.length <= max ? text : text.slice(0, max - 1) + '…';
}
