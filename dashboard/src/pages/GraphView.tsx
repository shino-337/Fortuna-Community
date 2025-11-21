import React, { useState, useCallback } from 'react'
import { useGraph } from '../hooks/useGraph'
import GraphVisualizationD3 from '../components/Graph/GraphVisualizationD3'
import GraphFilters from '../components/Graph/GraphFilters'
import NodeDetailsPanel from '../components/Graph/NodeDetailsPanel'

/**
 * Enhanced Graph View with 3-panel layout
 * Based on graph-ui-enhancement-proposal.md
 * 
 * Layout:
 * [Filters/Legend Sidebar] | [Graph Canvas] | [Node Details Panel]
 */
const GraphView: React.FC = () => {
  // Filter state
  const [cluster, setCluster] = useState<string>('')
  const [namespace, setNamespace] = useState<string>('')
  const [layout, setLayout] = useState<'force' | 'radial' | 'tree' | 'grid'>('force')
  const [autoFitEnabled, setAutoFitEnabled] = useState<boolean>(true)
  
  // Panel state
  const [leftSidebarCollapsed, setLeftSidebarCollapsed] = useState(false)
  const [rightPanelOpen, setRightPanelOpen] = useState(false)
  
  // Node selection
  const [selectedNode, setSelectedNode] = useState<any>(null)
  
  // Node type filters (all enabled by default)
  const [nodeTypeFilters, setNodeTypeFilters] = useState<Record<string, boolean>>({
    serviceaccount: true,
    role: true,
    clusterrole: true,
    namespace: true,
    cluster: true,
  })
  
  // Connection type filters
  const [connectionTypeFilters, setConnectionTypeFilters] = useState<Record<string, boolean>>({
    binding: true,
    member: true,
    owner: true,
    reference: true,
  })
  
  // Fetch graph data with filters
  const { data, isLoading, error } = useGraph({
    cluster: cluster || undefined,
    namespace: namespace || undefined,
  })
  
  console.log('🎬 [GraphView] Rendering with data:', {
    hasData: !!data,
    nodesCount: data?.nodes?.length || 0,
    edgesCount: data?.edges?.length || 0,
    cluster,
    namespace,
    isLoading
  })
  
  // Handle node selection
  const handleNodeSelect = useCallback((nodeData: any) => {
    setSelectedNode(nodeData)
    setRightPanelOpen(true)
  }, [])
  
  // Handle node type filter toggle
  const handleNodeTypeChange = useCallback((type: string, enabled: boolean) => {
    setNodeTypeFilters(prev => ({ ...prev, [type]: enabled }))
  }, [])
  
  // Handle connection filter toggle
  const handleConnectionTypeChange = useCallback((type: string, enabled: boolean) => {
    setConnectionTypeFilters(prev => ({ ...prev, [type]: enabled }))
  }, [])
  
  // Reset all filters
  const handleResetFilters = useCallback(() => {
    setNodeTypeFilters({
      serviceaccount: true,
      role: true,
      clusterrole: true,
      namespace: true,
      cluster: true,
    })
    setConnectionTypeFilters({
      binding: true,
      member: true,
      owner: true,
      reference: true,
    })
  }, [])
  
  return (
    <div className="flex h-full w-full overflow-hidden bg-gray-50 dark:bg-gray-900">
      {/* Left Sidebar - Filters & Legend */}
      <div
        className={`
          transition-all duration-300 ease-in-out
          ${leftSidebarCollapsed ? 'w-12' : 'w-80'}
          bg-white dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700
          flex flex-col
          flex-shrink-0
        `}
      >
        {/* Collapse toggle */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-700">
          {!leftSidebarCollapsed && (
            <h2 className="text-base font-semibold text-gray-900 dark:text-white">
              Filters & Legend
            </h2>
          )}
          <button
            onClick={() => setLeftSidebarCollapsed(!leftSidebarCollapsed)}
            className="p-1.5 rounded hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-600 dark:text-gray-400 transition-colors"
            title={leftSidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          >
            <svg
              className={`w-5 h-5 transition-transform ${leftSidebarCollapsed ? 'rotate-180' : ''}`}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
          </button>
        </div>
        
        {/* Filters content */}
        {!leftSidebarCollapsed && (
          <div className="flex-1 overflow-y-auto">
            <GraphFilters
              cluster={cluster}
              namespace={namespace}
              onClusterChange={setCluster}
              onNamespaceChange={setNamespace}
              isLoading={isLoading}
              nodeTypeFilters={nodeTypeFilters}
              onNodeTypeChange={handleNodeTypeChange}
              connectionTypeFilters={connectionTypeFilters}
              onConnectionTypeChange={handleConnectionTypeChange}
              layout={layout}
              onLayoutChange={(newLayout) => setLayout(newLayout as any)}
              autoFitEnabled={autoFitEnabled}
              onAutoFitChange={setAutoFitEnabled}
              onResetAdvancedFilters={handleResetFilters}
            />
          </div>
        )}
      </div>
      
      {/* Center - Graph Canvas */}
      <div className="flex-1 flex flex-col overflow-hidden bg-white dark:bg-gray-800">
        {/* Header (matching sidebar headers) */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center gap-3">
            <h2 className="text-base font-semibold text-gray-900 dark:text-white">
              Graph Visualization
            </h2>
            {data && !isLoading && (
              <span className="text-xs font-medium text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded">
                {data.nodes.length} nodes, {data.edges.length} edges
              </span>
            )}
          </div>
          
          <div className="flex items-center gap-2">
            {isLoading && (
              <div className="flex items-center gap-2 text-blue-600 dark:text-blue-400">
                <svg className="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                <span className="text-xs font-medium">Loading...</span>
              </div>
            )}
            {error && (
              <span className="text-xs font-medium text-red-600 dark:text-red-400">
                Error loading data
              </span>
            )}
          </div>
        </div>
        
        {/* Graph visualization - D3 SVG Force Graph */}
        <div className="flex-1 relative">
          <GraphVisualizationD3
            cluster={cluster}
            namespace={namespace}
            data={data}
            isLoading={isLoading}
            error={error}
            nodeTypeFilters={nodeTypeFilters}
            connectionTypeFilters={connectionTypeFilters}
            layout={layout}
            autoFitEnabled={autoFitEnabled}
            onNodeSelect={(nodeId: string, nodeType: string, nodeData: Record<string, any>) => {
              handleNodeSelect({ id: nodeId, type: nodeType, data: nodeData })
            }}
          />
        </div>
      </div>
      
      {/* Right Panel - Node Details */}
      <div
        className={`
          transition-all duration-300 ease-in-out
          ${rightPanelOpen ? 'w-80' : 'w-0'}
          bg-white dark:bg-gray-800 border-l border-gray-200 dark:border-gray-700
          overflow-hidden
          flex-shrink-0
        `}
      >
        {rightPanelOpen && (
          <NodeDetailsPanel
            node={selectedNode}
            onClose={() => setRightPanelOpen(false)}
          />
        )}
      </div>
    </div>
  )
}

export default GraphView
