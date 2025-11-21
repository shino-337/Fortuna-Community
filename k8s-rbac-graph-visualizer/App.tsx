import React, { useState, useEffect } from 'react';
import ForceGraph from './components/ForceGraph';
import SidebarLeft from './components/SidebarLeft';
import SidebarRight from './components/SidebarRight';
import { generateRbacData } from './services/dataGenerator';
import { GraphData, RbacNode, FilterState, NodeType } from './types';

const App: React.FC = () => {
  const [data, setData] = useState<GraphData>({ nodes: [], links: [] });
  const [selectedNode, setSelectedNode] = useState<RbacNode | null>(null);
  const [collapsedLeft, setCollapsedLeft] = useState(false);
  const [containerSize, setContainerSize] = useState({ width: 800, height: 600 });

  // Lists for Sidebar options
  const [availableClusters, setAvailableClusters] = useState<string[]>([]);
  const [availableNamespaces, setAvailableNamespaces] = useState<string[]>([]);
  
  // Filters State
  const [filters, setFilters] = useState<FilterState>({
    showSystem: true,
    nodeTypes: {
      [NodeType.USER]: true,
      [NodeType.GROUP]: true,
      [NodeType.SERVICE_ACCOUNT]: true,
      [NodeType.ROLE]: true,
      [NodeType.CLUSTER_ROLE]: true,
      [NodeType.ROLE_BINDING]: true,
      [NodeType.CLUSTER_ROLE_BINDING]: true,
    },
    clusters: {},
    namespaces: {}
  });

  useEffect(() => {
    // Load mock data
    const rbacData = generateRbacData();
    
    // Extract metadata for filters
    const uniqueClusters = Array.from(new Set(rbacData.nodes.map(n => n.clusterName || 'Unknown'))).sort();
    const uniqueNamespaces = Array.from(new Set(rbacData.nodes.map(n => n.namespace || 'Cluster Scope'))).sort();

    setAvailableClusters(uniqueClusters);
    setAvailableNamespaces(uniqueNamespaces);

    // Initialize filters to all true
    setFilters(prev => ({
        ...prev,
        clusters: uniqueClusters.reduce((acc, curr) => ({ ...acc, [curr]: true }), {}),
        namespaces: uniqueNamespaces.reduce((acc, curr) => ({ ...acc, [curr]: true }), {})
    }));

    setData(rbacData);
  }, []);

  useEffect(() => {
    const handleResize = () => {
      const container = document.getElementById('graph-container');
      if (container) {
        setContainerSize({
          width: container.clientWidth,
          height: container.clientHeight,
        });
      }
    };

    window.addEventListener('resize', handleResize);
    handleResize(); // Initial calculation

    // Small delay to ensure layout is settled
    setTimeout(handleResize, 100);

    return () => window.removeEventListener('resize', handleResize);
  }, [collapsedLeft, selectedNode]); // Recalculate when panels change

  const handleNodeClick = (node: RbacNode) => {
      // Check if node is empty (deselect)
      if (!node.id) {
          setSelectedNode(null);
          return;
      }
      setSelectedNode(node);
  };

  const nodeFilter = (node: RbacNode): boolean => {
    // 1. Node Type
    if (!filters.nodeTypes[node.type]) return false;

    // 2. Cluster
    // If node has no cluster name (unlikely with current generator), we default to showing it if 'Unknown' is checked or just show it.
    const clusterKey = node.clusterName || 'Unknown';
    if (filters.clusters[clusterKey] === false) return false;

    // 3. Namespace
    // Nodes without namespace (ClusterRole, etc) map to 'Cluster Scope' key
    const nsKey = node.namespace || 'Cluster Scope';
    if (filters.namespaces[nsKey] === false) return false;

    return true;
  };

  return (
    <div className="flex h-screen bg-slate-900 text-slate-200 font-sans overflow-hidden">
      
      {/* Left Sidebar */}
      <SidebarLeft 
        collapsed={collapsedLeft} 
        setCollapsed={setCollapsedLeft}
        filters={filters}
        setFilters={setFilters}
        availableClusters={availableClusters}
        availableNamespaces={availableNamespaces}
      />

      {/* Main Canvas Area */}
      <main className="flex-1 flex flex-col relative h-full">
        {/* Toolbar / Header */}
        <header className="h-14 bg-slate-900 border-b border-slate-700 flex items-center px-6 justify-between shrink-0 z-10">
            <div className="flex items-center gap-2">
                <div className="w-8 h-8 bg-blue-600 rounded flex items-center justify-center text-white font-bold">K8s</div>
                <h1 className="text-lg font-semibold text-slate-100">RBAC Visualizer</h1>
            </div>
            <div className="text-sm text-slate-500">
                {data.nodes.filter(nodeFilter).length} Nodes • {data.links.filter(l => {
                    // Quick visual count approximation; exact filtering happens inside ForceGraph but this is good for UI feedback
                     const s = typeof l.source === 'object' ? (l.source as RbacNode) : data.nodes.find(n => n.id === l.source);
                     const t = typeof l.target === 'object' ? (l.target as RbacNode) : data.nodes.find(n => n.id === l.target);
                     return s && t && nodeFilter(s) && nodeFilter(t);
                }).length} Links
            </div>
        </header>

        {/* Graph Container */}
        <div id="graph-container" className="flex-1 relative overflow-hidden bg-slate-950">
             <ForceGraph 
                data={data}
                width={containerSize.width}
                height={containerSize.height}
                onNodeClick={handleNodeClick}
                selectedNodeId={selectedNode?.id || null}
                filter={nodeFilter}
             />
             {/* Floating Legend Hint if Leftbar collapsed */}
             {collapsedLeft && (
                <div className="absolute top-4 left-4 bg-slate-800/80 p-2 rounded backdrop-blur-sm text-xs text-slate-400 pointer-events-none select-none">
                    Filters
                </div>
             )}
        </div>
      </main>

      {/* Right Sidebar (Detail Inspector) */}
      {selectedNode && (
          <SidebarRight node={selectedNode} />
      )}
    </div>
  );
};

export default App;