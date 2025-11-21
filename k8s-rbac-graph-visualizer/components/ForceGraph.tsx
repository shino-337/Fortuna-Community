import React, { useEffect, useRef } from 'react';
import * as d3 from 'd3';
import { GraphData, RbacNode, RbacLink } from '../types';
import { NODE_COLORS } from '../constants';

interface ForceGraphProps {
  data: GraphData;
  onNodeClick: (node: RbacNode) => void;
  selectedNodeId: string | null;
  width: number;
  height: number;
  filter: (node: RbacNode) => boolean;
}

const ForceGraph: React.FC<ForceGraphProps> = ({ data, onNodeClick, selectedNodeId, width, height, filter }) => {
  const svgRef = useRef<SVGSVGElement>(null);
  const simulationRef = useRef<d3.Simulation<RbacNode, RbacLink> | null>(null);

  // 1. Prepare Data (Filter)
  // Memoizing this would be better in a larger app, but fine here.
  const filteredNodes = data.nodes.filter(filter);
  const filteredNodeIds = new Set(filteredNodes.map(n => n.id));
  const filteredLinks = data.links.filter(l => {
     const sourceId = typeof l.source === 'object' ? (l.source as RbacNode).id : l.source as string;
     const targetId = typeof l.target === 'object' ? (l.target as RbacNode).id : l.target as string;
     return filteredNodeIds.has(sourceId) && filteredNodeIds.has(targetId);
  });

  // 2. Initialize Simulation & Render Static Elements
  useEffect(() => {
    if (!svgRef.current) return;
    const svg = d3.select(svgRef.current);
    
    // Clear previous render
    svg.selectAll("*").remove(); 

    // --- Defs (Arrowheads) ---
    const defs = svg.append('defs');
    defs.append('marker')
        .attr('id', 'arrowhead')
        .attr('viewBox', '0 -5 10 10')
        .attr('refX', 8) // Slightly offset to not overlap stroke
        .attr('refY', 0)
        .attr('orient', 'auto')
        .attr('markerWidth', 6)
        .attr('markerHeight', 6)
        .attr('xoverflow', 'visible')
        .append('path')
        .attr('d', 'M0,-5L10,0L0,5')
        .attr('fill', '#9ca3af')
        .style('stroke', 'none');

    const g = svg.append('g').attr('class', 'graph-container');

    // --- Zoom ---
    const zoom = d3.zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.1, 4])
      .on('zoom', (event) => {
        g.attr('transform', event.transform);
      });

    svg.call(zoom);

    // --- Simulation Setup ---
    const simulation = d3.forceSimulation<RbacNode, RbacLink>(filteredNodes)
      .force('link', d3.forceLink<RbacNode, RbacLink>(filteredLinks).id(d => d.id).distance(100))
      .force('charge', d3.forceManyBody().strength(-500))
      .force('center', d3.forceCenter(width / 2, height / 2))
      .force('collide', d3.forceCollide().radius(d => (d.radius || 20) + 8)); // Prevent overlap

    simulationRef.current = simulation;

    // --- Draw Links ---
    const link = g.append('g')
      .attr('class', 'links')
      .selectAll('line')
      .data(filteredLinks)
      .join('line')
      .attr('stroke', '#9ca3af')
      .attr('stroke-opacity', 0.6)
      .attr('stroke-width', 1.5)
      .attr('marker-end', 'url(#arrowhead)');

    // --- Draw Nodes ---
    const node = g.append('g')
      .attr('class', 'nodes')
      .selectAll<SVGGElement, RbacNode>('g')
      .data(filteredNodes)
      .join('g')
      .attr('cursor', 'pointer')
      .call(d3.drag<SVGGElement, RbacNode>()
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
        }));

    node.append('circle')
      .attr('r', d => d.radius || 20)
      .attr('fill', d => NODE_COLORS[d.type] || '#cbd5e1')
      .attr('stroke', '#1e293b')
      .attr('stroke-width', 1.5)
      .attr('class', 'node-circle');

    node.append('text')
      .text(d => d.name.length > 15 ? d.name.substring(0, 12) + '...' : d.name)
      .attr('x', d => (d.radius || 20) + 5)
      .attr('y', 5)
      .attr('fill', '#e2e8f0')
      .style('font-size', '12px')
      .style('font-weight', '500')
      .style('pointer-events', 'none')
      .style('text-shadow', '0 1px 3px rgba(0,0,0,0.8)');

    // Click Event
    node.on('click', (event, d) => {
        event.stopPropagation();
        onNodeClick(d);
    });

    // --- Tick Function ---
    simulation.on('tick', () => {
      link
        .attr('x1', d => (d.source as RbacNode).x!)
        .attr('y1', d => (d.source as RbacNode).y!)
        // Calculate intersection for arrow head to touch circle edge
        .attr('x2', d => {
            const target = d.target as RbacNode;
            const source = d.source as RbacNode;
            const dx = target.x! - source.x!;
            const dy = target.y! - source.y!;
            const dist = Math.sqrt(dx * dx + dy * dy);
            if (dist === 0) return target.x!;
            const r = (target.radius || 20) + 5; // Node radius + gap
            return target.x! - (dx * r / dist); 
        })
        .attr('y2', d => {
            const target = d.target as RbacNode;
            const source = d.source as RbacNode;
            const dx = target.x! - source.x!;
            const dy = target.y! - source.y!;
            const dist = Math.sqrt(dx * dx + dy * dy);
            if (dist === 0) return target.y!;
            const r = (target.radius || 20) + 5;
            return target.y! - (dy * r / dist);
        });

      node
        .attr('transform', d => `translate(${d.x},${d.y})`);
    });

    return () => {
      simulation.stop();
    };
  }, [width, height, filteredNodes.length, filteredLinks.length]); // Re-run simulation only on structural changes

  // 3. Handle Selection (Visual Updates Only)
  useEffect(() => {
     if (!svgRef.current) return;
     const svg = d3.select(svgRef.current);
     const g = svg.select('.graph-container');
     
     if (!g.empty()) {
         // Identify connected nodes
         const relatedIds = new Set<string>();
         if (selectedNodeId) {
             relatedIds.add(selectedNodeId);
             filteredLinks.forEach(l => {
                 const sId = typeof l.source === 'object' ? (l.source as RbacNode).id : l.source as string;
                 const tId = typeof l.target === 'object' ? (l.target as RbacNode).id : l.target as string;
                 if (sId === selectedNodeId) relatedIds.add(tId);
                 if (tId === selectedNodeId) relatedIds.add(sId);
             });
         }

         const isNoneSelected = !selectedNodeId;

         // Update Nodes
         g.selectAll('.node-circle')
            .transition().duration(300)
            .attr('opacity', (d: any) => isNoneSelected || relatedIds.has(d.id) ? 1 : 0.2)
            .attr('stroke', (d: any) => d.id === selectedNodeId ? '#ffffff' : (relatedIds.has(d.id) ? '#94a3b8' : '#1e293b'))
            .attr('stroke-width', (d: any) => d.id === selectedNodeId ? 4 : (relatedIds.has(d.id) ? 2.5 : 1.5));

         // Update Links
         g.selectAll('line')
            .transition().duration(300)
            .attr('stroke-opacity', (d: any) => {
                const sId = typeof d.source === 'object' ? d.source.id : d.source;
                const tId = typeof d.target === 'object' ? d.target.id : d.target;
                return isNoneSelected || (relatedIds.has(sId) && relatedIds.has(tId)) ? 1 : 0.1;
            })
            .attr('stroke', (d: any) => {
                 const sId = typeof d.source === 'object' ? d.source.id : d.source;
                 const tId = typeof d.target === 'object' ? d.target.id : d.target;
                 return (relatedIds.has(sId) && relatedIds.has(tId)) && !isNoneSelected ? '#60a5fa' : '#9ca3af';
            });
     }
  }, [selectedNodeId, filteredLinks]);

  return (
    <svg
      ref={svgRef}
      width={width}
      height={height}
      className="w-full h-full bg-slate-900 rounded-lg shadow-inner cursor-move"
      onClick={() => onNodeClick({} as any)}
    />
  );
};

export default ForceGraph;