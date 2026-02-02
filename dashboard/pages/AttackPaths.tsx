import React, { useEffect, useRef, useState } from 'react';
import * as d3 from 'd3';
import { Card } from '../components/ui/Card';
import { RefreshCw, Info } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../components/PageLayout';
import { api } from '../lib/api';

// Types for D3 Graph
interface Node extends d3.SimulationNodeDatum {
  id: string;
  group: number;
  label: string;
  type: 'internet' | 'loadbalancer' | 'service' | 'pod' | 'database';
  risk?: string;
}

interface Link extends d3.SimulationLinkDatum<Node> {
  source: string | Node;
  target: string | Node;
  value: number;
}

export const AttackPaths: React.FC = () => {
  const svgRef = useRef<SVGSVGElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const [version, setVersion] = useState(0); // Trigger re-render
  const [tooltip, setTooltip] = useState<{x: number, y: number, data: Node} | null>(null);
  const [graphData, setGraphData] = useState<{ nodes: Node[]; links: Link[] }>({ nodes: [], links: [] });
  const [loading, setLoading] = useState(true);

  // Fetch attack paths graph from API
  useEffect(() => {
    const fetchGraphData = async () => {
      try {
        setLoading(true);
        const data = await api.getAttackPathsGraph();
        // Transform API data to D3 format with groups
        const nodes: Node[] = data.nodes.map((node, idx) => ({
          id: node.id,
          label: node.label,
          type: node.type as Node['type'],
          risk: node.risk as Node['risk'],
          group: idx % 5 + 1, // Simple grouping based on index
        }));
        const links: Link[] = data.links.map(link => ({
          source: link.source,
          target: link.target,
          value: link.value,
        }));
        setGraphData({ nodes, links });
      } catch (err) {
        console.error('Failed to fetch attack paths graph:', err);
        setGraphData({ nodes: [], links: [] });
      } finally {
        setLoading(false);
      }
    };
    fetchGraphData();
  }, []);

  useEffect(() => {
    if (!svgRef.current || !containerRef.current || loading || graphData.nodes.length === 0) return;

    const width = containerRef.current.clientWidth;
    const height = 600;

    // Clear previous SVG content
    d3.select(svgRef.current).selectAll("*").remove();

    const svg = d3.select(svgRef.current)
      .attr("width", width)
      .attr("height", height)
      .attr("viewBox", [0, 0, width, height]);

    // Create a copy of the data to avoid mutation issues with React StrictMode
    const nodes = graphData.nodes.map(d => ({...d}));
    const links = graphData.links.map(d => ({...d}));

    const simulation = d3.forceSimulation(nodes)
      .force("link", d3.forceLink(links).id((d: any) => d.id).distance(100))
      .force("charge", d3.forceManyBody().strength(-400))
      .force("center", d3.forceCenter(width / 2, height / 2))
      .force("collide", d3.forceCollide().radius(50));

    // Define markers for arrows
    svg.append("defs").selectAll("marker")
      .data(["end"])
      .enter().append("marker")
      .attr("id", "arrow")
      .attr("viewBox", "0 -5 10 10")
      .attr("refX", 25)
      .attr("refY", 0)
      .attr("markerWidth", 6)
      .attr("markerHeight", 6)
      .attr("orient", "auto")
      .append("path")
      .attr("fill", "#64748b")
      .attr("d", "M0,-5L10,0L0,5");

    const link = svg.append("g")
      .attr("stroke", "#475569")
      .attr("stroke-opacity", 0.6)
      .selectAll("line")
      .data(links)
      .join("line")
      .attr("stroke-width", 2)
      .attr("marker-end", "url(#arrow)");

    const node = svg.append("g")
      .attr("stroke", "#1e293b")
      .attr("stroke-width", 1.5)
      .selectAll("g")
      .data(nodes)
      .join("g")
      .attr("cursor", "pointer");

    // Node circles
    node.append("circle")
      .attr("r", 15)
      .attr("fill", (d) => {
        if (d.type === 'internet') return '#64748b';
        if (d.type === 'loadbalancer') return '#3b82f6';
        if (d.type === 'service') return '#8b5cf6';
        if (d.type === 'database') return '#ef4444';
        return '#10b981'; // pod
      })
      .call(drag(simulation) as any);
      
    // Risk indicator ring for high/critical risks
    node.filter(d => d.risk === 'critical' || d.risk === 'high')
      .append("circle")
      .attr("r", 19)
      .attr("fill", "none")
      .attr("stroke", "#ef4444")
      .attr("stroke-width", 2)
      .attr("stroke-opacity", 0.8)
      .attr("stroke-dasharray", "3,3");

    // Node Labels
    node.append("text")
      .attr("x", 20)
      .attr("y", 5)
      .text(d => d.label)
      .clone(true).lower()
      .attr("fill", "none")
      .attr("stroke", "#0f172a")
      .attr("stroke-width", 3);
      
    node.append("text")
      .attr("x", 20)
      .attr("y", 5)
      .text(d => d.label)
      .attr("fill", "#cbd5e1")
      .attr("stroke", "none")
      .attr("font-size", "12px")
      .attr("font-weight", "bold");

    // Interaction Logic
    node
      .on("mouseenter", (event, d) => {
        // Dim all
        link.transition().duration(200).attr("stroke-opacity", 0.1);
        node.transition().duration(200).attr("opacity", 0.3);

        // Highlight current
        const currentNode = d3.select(event.currentTarget);
        currentNode.transition().duration(200).attr("opacity", 1);
        
        // Find neighbors
        const linkedEdges = link.filter((l: any) => l.source.id === d.id || l.target.id === d.id);
        
        linkedEdges
            .transition().duration(200)
            .attr("stroke-opacity", 1)
            .attr("stroke", "#db2777"); // pink-600

        linkedEdges.each((l: any) => {
             const neighborId = l.source.id === d.id ? l.target.id : l.source.id;
             node.filter((n: any) => n.id === neighborId).transition().duration(200).attr("opacity", 1);
        });

        // Set Tooltip
        const rect = containerRef.current?.getBoundingClientRect();
        if (rect) {
             setTooltip({
                 x: d.x!,
                 y: d.y!,
                 data: d
             });
        }
      })
      .on("mouseleave", (event, d) => {
          // Reset styles
          link.transition().duration(200).attr("stroke-opacity", 0.6).attr("stroke", "#475569");
          node.transition().duration(200).attr("opacity", 1);
          setTooltip(null);
      });

    simulation.on("tick", () => {
      link
        .attr("x1", (d: any) => d.source.x)
        .attr("y1", (d: any) => d.source.y)
        .attr("x2", (d: any) => d.target.x)
        .attr("y2", (d: any) => d.target.y);

      node
        .attr("transform", (d: any) => `translate(${d.x},${d.y})`);
    });

    function drag(simulation: d3.Simulation<Node, undefined>) {
      function dragstarted(event: any) {
        if (!event.active) simulation.alphaTarget(0.3).restart();
        event.subject.fx = event.subject.x;
        event.subject.fy = event.subject.y;
        setTooltip(null); // Hide tooltip on drag
      }
      
      function dragged(event: any) {
        event.subject.fx = event.x;
        event.subject.fy = event.y;
      }
      
      function dragended(event: any) {
        if (!event.active) simulation.alphaTarget(0);
        event.subject.fx = null;
        event.subject.fy = null;
      }
      
      return d3.drag()
        .on("start", dragstarted)
        .on("drag", dragged)
        .on("end", dragended);
    }

    return () => {
      simulation.stop();
    };
  }, [version, loading, graphData]);

  return (
    <PageLayout
      title="Attack Path Visualization"
      description="Interactive graph of potential vulnerability chains."
      actions={
        <Button variant="secondary" onClick={() => setVersion(v => v + 1)}>
          <RefreshCw className="w-4 h-4 mr-2" />
          Reset Layout
        </Button>
      }
    >
      <Card className="p-0 overflow-hidden bg-slate-900 border-slate-800">
        <div ref={containerRef} className="w-full h-[600px] border-b border-slate-800 relative bg-slate-950/50">
          {loading ? (
            <div className="flex items-center justify-center h-full">
              <div className="text-center">
                <div className="w-12 h-12 border-4 border-pink-500 border-t-transparent rounded-full animate-spin mx-auto mb-4"></div>
                <span className="text-slate-400">Loading attack paths graph...</span>
              </div>
            </div>
          ) : graphData.nodes.length === 0 ? (
            <div className="flex items-center justify-center h-full">
              <div className="text-center text-slate-400">
                <Info className="w-12 h-12 mx-auto mb-4 opacity-50" />
                <p>No attack paths data available.</p>
                <p className="text-sm mt-2">Attack paths will appear here when detected.</p>
              </div>
            </div>
          ) : (
            <svg ref={svgRef} className="w-full h-full cursor-grab active:cursor-grabbing"></svg>
          )}
          
          {/* Tooltip */}
          {tooltip && (
              <div 
                className="absolute z-10 pointer-events-none transition-all duration-75" 
                style={{ 
                    left: tooltip.x, 
                    top: tooltip.y, 
                    transform: 'translate(-50%, -130%)' 
                }}
              >
                  <div className="bg-slate-900 border border-slate-700 p-3 rounded-lg shadow-xl w-48 backdrop-blur-sm bg-opacity-95">
                      <div className="flex items-center justify-between mb-1">
                        <span className="font-bold text-white text-sm">{tooltip.data.label}</span>
                        {tooltip.data.risk && (
                            <span className={`text-[10px] uppercase font-bold px-1.5 py-0.5 rounded ${
                                tooltip.data.risk === 'critical' ? 'bg-red-900/50 text-red-400' :
                                tooltip.data.risk === 'high' ? 'bg-orange-900/50 text-orange-400' :
                                tooltip.data.risk === 'medium' ? 'bg-yellow-900/50 text-yellow-400' :
                                'bg-emerald-900/50 text-emerald-400'
                            }`}>
                                {tooltip.data.risk}
                            </span>
                        )}
                      </div>
                      <div className="text-xs text-slate-400 capitalize mb-2">{tooltip.data.type}</div>
                      <div className="text-xs text-slate-500 border-t border-slate-800 pt-2 flex items-start">
                          <Info className="w-3 h-3 mr-1 mt-0.5" />
                          <span>Click to view detailed node analysis.</span>
                      </div>
                  </div>
                  {/* Arrow */}
                  <div className="w-0 h-0 border-l-8 border-l-transparent border-r-8 border-r-transparent border-t-8 border-t-slate-700 absolute left-1/2 -translate-x-1/2 -bottom-2"></div>
              </div>
          )}

          <div className="absolute bottom-4 left-4 bg-slate-900/90 backdrop-blur p-4 rounded-lg border border-slate-700 shadow-sm text-xs space-y-2 text-slate-300 pointer-events-none select-none">
            <div className="font-semibold mb-2 text-white">Legend</div>
            <div className="flex items-center"><span className="w-3 h-3 rounded-full bg-slate-500 mr-2"></span> Internet</div>
            <div className="flex items-center"><span className="w-3 h-3 rounded-full bg-blue-500 mr-2"></span> Load Balancer</div>
            <div className="flex items-center"><span className="w-3 h-3 rounded-full bg-purple-500 mr-2"></span> Service</div>
            <div className="flex items-center"><span className="w-3 h-3 rounded-full bg-emerald-500 mr-2"></span> Pod</div>
            <div className="flex items-center"><span className="w-3 h-3 rounded-full bg-red-500 mr-2"></span> Database</div>
          </div>
        </div>
      </Card>
    </PageLayout>
  );
};