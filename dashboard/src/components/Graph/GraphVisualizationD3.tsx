/**
 * Graph Visualization D3 Wrapper
 * Integrates ForceGraphSvg with existing KSAM API data
 * K8s Fortuna Platform
 */

import React, { useRef, useEffect, useState, useMemo, useCallback } from 'react'
import ForceGraphSvg from './ForceGraphSvg'
import { RbacNode, RbacLink, GraphData } from './types'
import { NODE_RADIUS } from './constants'

interface GraphVisualizationD3Props {
  cluster?: string
  namespace?: string
  data?: { nodes: any[]; edges: any[] }
  isLoading?: boolean
  error?: any
  nodeTypeFilters?: Record<string, boolean>
  connectionTypeFilters?: Record<string, boolean>
  layout?: 'force' | 'radial' | 'tree' | 'grid'
  autoFitEnabled?: boolean
  onNodeSelect?: (nodeId: string, nodeType: string, nodeData: Record<string, any>) => void
}

const GraphVisualizationD3: React.FC<GraphVisualizationD3Props> = ({
  data: propData,
  isLoading = false,
  error,
  nodeTypeFilters = {},
  // connectionTypeFilters - TODO: Implement edge filtering
  layout = 'force',
  onNodeSelect,
}) => {
  const containerRef = useRef<HTMLDivElement>(null)
  const [dimensions, setDimensions] = useState({ width: 800, height: 600 })
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)

  // Track container size
  useEffect(() => {
    const updateDimensions = () => {
      if (containerRef.current) {
        const rect = containerRef.current.getBoundingClientRect()
        const width = Math.floor(rect.width) || 800
        const height = Math.floor(rect.height) || 600

        console.log('📐 [GraphVisualizationD3] Container dimensions:', { width, height })

        if (width > 0 && height > 0) {
          setDimensions({ width, height })
        }
      }
    }

    // Initial size check
    setTimeout(updateDimensions, 100)

    // Update on window resize
    const handleResize = () => requestAnimationFrame(updateDimensions)
    window.addEventListener('resize', handleResize)

    // ResizeObserver for container size changes
    let resizeObserver: ResizeObserver | null = null
    if (containerRef.current && typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver(() => requestAnimationFrame(updateDimensions))
      resizeObserver.observe(containerRef.current)
    }

    return () => {
      window.removeEventListener('resize', handleResize)
      resizeObserver?.disconnect()
    }
  }, [])

  // Transform API data to RbacNode/RbacLink format
  const graphData = useMemo<GraphData>(() => {
    if (!propData || !propData.nodes || !propData.edges) {
      return { nodes: [], links: [] }
    }

    console.log('📊 [GraphVisualizationD3] Transforming data...', {
      nodesCount: propData.nodes.length,
      edgesCount: propData.edges.length,
    })

    // Transform nodes
    const nodes: RbacNode[] = propData.nodes.map((node: any) => ({
      id: node.id,
      label: node.label || node.id,
      type: node.type || 'unknown',
      clusterName: node.data?.cluster,
      namespace: node.data?.namespace,
      data: node.data || {},
      radius: NODE_RADIUS[node.type] || 12,
    }))

    // Transform edges to links
    const links: RbacLink[] = propData.edges.map((edge: any) => ({
      source: edge.source,
      target: edge.target,
      type: edge.type || 'unknown',
      data: edge.data || {},
    }))

    console.log('✅ [GraphVisualizationD3] Transformed:', { nodes: nodes.length, links: links.length })

    return { nodes, links }
  }, [propData])

  // Filter function
  const filterNode = useCallback(
    (node: RbacNode): boolean => {
      // Check node type filter
      const nodeType = node.type || 'unknown'
      if (nodeTypeFilters[nodeType] === false) {
        return false
      }

      return true
    },
    [nodeTypeFilters]
  )

  // Handle node click
  const handleNodeClick = useCallback(
    (node: RbacNode) => {
      if (!node.id) {
        // Deselect
        setSelectedNodeId(null)
        return
      }

      console.log('🖱️ [GraphVisualizationD3] Node clicked:', node.id, node.type)
      setSelectedNodeId(node.id)

      if (onNodeSelect) {
        onNodeSelect(node.id, node.type, node.data)
      }
    },
    [onNodeSelect]
  )

  // Loading state
  if (isLoading) {
    return (
      <div className="flex items-center justify-center w-full h-full bg-slate-900">
        <div className="text-gray-400 flex items-center">
          <svg
            className="animate-spin -ml-1 mr-3 h-5 w-5 text-pink-500"
            xmlns="http://www.w3.org/2000/svg"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              className="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              strokeWidth="4"
            ></circle>
            <path
              className="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            ></path>
          </svg>
          Loading graph...
        </div>
      </div>
    )
  }

  // Error state
  if (error) {
    return (
      <div className="flex items-center justify-center w-full h-full bg-slate-900">
        <div className="text-red-400 flex items-center">
          <svg className="h-5 w-5 mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          Error loading graph: {String(error)}
        </div>
      </div>
    )
  }

  // No data state
  if (!propData || graphData.nodes.length === 0) {
    return (
      <div className="flex items-center justify-center w-full h-full bg-slate-900 border border-slate-700 rounded-lg">
        <div className="text-gray-400 flex flex-col items-center">
          <svg
            className="h-12 w-12 mb-3 text-gray-600"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={1.5}
              d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
            />
          </svg>
          <span className="text-base font-medium text-gray-300 mb-1">No nodes to display</span>
          <p className="text-sm text-gray-500 text-center max-w-xs">
            {propData && propData.nodes.length > 0
              ? 'All nodes are filtered out. Adjust your filters to see data.'
              : 'No data available. Try selecting a different cluster or namespace.'}
          </p>
        </div>
      </div>
    )
  }

  return (
    <div ref={containerRef} className="w-full h-full overflow-hidden relative">
      <ForceGraphSvg
        data={graphData}
        width={dimensions.width}
        height={dimensions.height}
        onNodeClick={handleNodeClick}
        selectedNodeId={selectedNodeId}
        filter={filterNode}
        layout={layout}
      />
      
      {/* Floating stats overlay */}
      <div className="absolute bottom-4 right-4 bg-slate-800/90 backdrop-blur-sm border border-slate-700 rounded-lg px-4 py-2 text-xs text-slate-300 shadow-lg pointer-events-none">
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-1.5">
            <div className="w-2 h-2 rounded-full bg-pink-500"></div>
            <span>{graphData.nodes.filter(filterNode).length} Nodes</span>
          </div>
          <div className="flex items-center gap-1.5">
            <div className="w-2 h-2 rounded-full bg-purple-500"></div>
            <span>{graphData.links.length} Links</span>
          </div>
          <div className="flex items-center gap-1.5">
            <svg className="w-3.5 h-3.5 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z" />
            </svg>
            <span className="capitalize">{layout}</span>
          </div>
        </div>
      </div>
    </div>
  )
}

export default GraphVisualizationD3

