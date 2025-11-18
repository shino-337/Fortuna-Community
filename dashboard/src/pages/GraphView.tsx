import { useState, useEffect, useCallback } from 'react'
import GraphVisualization from '../components/Graph/GraphVisualization'
import GraphFilters from '../components/Graph/GraphFilters'
import NodeDetailsPanel from '../components/Graph/NodeDetailsPanel'

// localStorage keys
const STORAGE_KEY_CLUSTER = 'ksam-graph-filter-cluster'
const STORAGE_KEY_NAMESPACE = 'ksam-graph-filter-namespace'

type LayoutOption = 'cose' | 'fcose' | 'dagre' | 'breadthfirst' | 'circle' | 'concentric' | 'grid'

const GraphView = () => {
  // Load filters from localStorage on mount
  const [cluster, setCluster] = useState<string>(() => {
    if (typeof window !== 'undefined') {
      return localStorage.getItem(STORAGE_KEY_CLUSTER) || ''
    }
    return ''
  })
  const [namespace, setNamespace] = useState<string>(() => {
    if (typeof window !== 'undefined') {
      return localStorage.getItem(STORAGE_KEY_NAMESPACE) || ''
    }
    return ''
  })
  const [isGraphLoading, setIsGraphLoading] = useState(false)

  const [nodeTypeFilters, setNodeTypeFilters] = useState<Record<string, boolean>>({
    serviceaccount: true,
    role: true,
    clusterrole: true,
    namespace: true,
    cluster: true,
  })

  const [connectionTypeFilters, setConnectionTypeFilters] = useState<Record<string, boolean>>({
    binding: true,
    member: true,
    owner: true,
    reference: true,
  })

  const [layout, setLayout] = useState<LayoutOption>('cose')
  const [autoFitEnabled, setAutoFitEnabled] = useState(true)
  const [sidebarCollapsed, setSidebarCollapsed] = useState<boolean>(() => {
    if (typeof window !== 'undefined') {
      const stored = localStorage.getItem('ksam-graph-sidebar-collapsed')
      return stored === 'true'
    }
    return false
  })
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)
  const [selectedNodeType, setSelectedNodeType] = useState<string | null>(null)
  const [selectedNodeData, setSelectedNodeData] = useState<Record<string, any> | null>(null)

  const handleLayoutChange = useCallback((value: LayoutOption) => {
    setLayout(value)
  }, [])

  const handleAutoFitChange = useCallback((value: boolean) => {
    setAutoFitEnabled(value)
  }, [])

  const toggleSidebar = useCallback(() => {
    setSidebarCollapsed((prev) => {
      const newValue = !prev
      if (typeof window !== 'undefined') {
        localStorage.setItem('ksam-graph-sidebar-collapsed', String(newValue))
      }
      return newValue
    })
  }, [])


  const handleNodeTypeChange = useCallback((type: string, value: boolean) => {
    setNodeTypeFilters((prev) => ({
      ...prev,
      [type]: value,
    }))
  }, [])

  const handleConnectionTypeChange = useCallback((type: string, value: boolean) => {
    setConnectionTypeFilters((prev) => ({
      ...prev,
      [type]: value,
    }))
  }, [])

  const resetAdvancedFilters = useCallback(() => {
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
    setLayout('cose')
    setAutoFitEnabled(true)
  }, [])

  // Save to localStorage when filters change
  useEffect(() => {
    if (typeof window !== 'undefined') {
      if (cluster) {
        localStorage.setItem(STORAGE_KEY_CLUSTER, cluster)
      } else {
        localStorage.removeItem(STORAGE_KEY_CLUSTER)
      }
    }
  }, [cluster])

  useEffect(() => {
    if (typeof window !== 'undefined') {
      if (namespace) {
        localStorage.setItem(STORAGE_KEY_NAMESPACE, namespace)
      } else {
        localStorage.removeItem(STORAGE_KEY_NAMESPACE)
      }
    }
  }, [namespace])

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900 flex flex-col" style={{ height: '100vh' }}>
      {/* Header */}
      <div className="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-4 py-3 flex-shrink-0">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Graph View</h1>
            <p className="text-sm text-gray-600 dark:text-gray-400">Visualize ServiceAccount relationships and permissions</p>
          </div>
        </div>
      </div>

      {/* Main Content: 3-Panel Layout */}
      <div className="flex-1 flex overflow-hidden" style={{ minHeight: 0 }}>
        {/* Left Sidebar: Filters + Legend (Collapsible) */}
        <div className={`bg-white dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700 transition-all duration-300 ease-in-out ${
          sidebarCollapsed ? 'w-0 overflow-hidden' : 'w-72 flex-shrink-0'
        }`}>
          <div className="h-full overflow-y-auto p-3">
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-base font-semibold text-gray-900 dark:text-white">Filters & Legend</h2>
              <button
                onClick={toggleSidebar}
                className="p-1.5 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors"
                title="Collapse sidebar"
              >
                <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 19l-7-7 7-7m8 14l-7-7 7-7" />
                </svg>
              </button>
            </div>
            
            <GraphFilters
              cluster={cluster}
              namespace={namespace}
              onClusterChange={setCluster}
              onNamespaceChange={setNamespace}
              isLoading={isGraphLoading}
              nodeTypeFilters={nodeTypeFilters}
              onNodeTypeChange={handleNodeTypeChange}
              connectionTypeFilters={connectionTypeFilters}
              onConnectionTypeChange={handleConnectionTypeChange}
              layout={layout}
              onLayoutChange={handleLayoutChange}
              autoFitEnabled={autoFitEnabled}
              onAutoFitChange={handleAutoFitChange}
              onResetAdvancedFilters={resetAdvancedFilters}
            />
          </div>
        </div>

        {/* Collapse Button when sidebar is hidden */}
        {sidebarCollapsed && (
          <button
            onClick={toggleSidebar}
            className="absolute left-0 top-1/2 -translate-y-1/2 z-20 bg-white dark:bg-gray-800 border-r border-t border-b border-gray-200 dark:border-gray-700 rounded-r-lg p-2 shadow-md hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
            title="Expand sidebar"
          >
            <svg className="w-5 h-5 text-gray-600 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 5l7 7-7 7M5 5l7 7-7 7" />
            </svg>
          </button>
        )}

        {/* Center: Graph Canvas */}
        <div className="flex-1 relative overflow-hidden" style={{ minHeight: '600px', height: '100%' }}>
          <GraphVisualization 
            cluster={cluster || undefined} 
            namespace={namespace || undefined} 
            height="100%"
            onLoadingChange={setIsGraphLoading}
            nodeTypeFilters={nodeTypeFilters}
            connectionTypeFilters={connectionTypeFilters}
            layout={layout}
            autoFitEnabled={autoFitEnabled}
            onNodeSelect={(nodeId, nodeType, nodeData) => {
              setSelectedNodeId(nodeId)
              setSelectedNodeType(nodeType)
              setSelectedNodeData(nodeData)
            }}
            selectedNodeId={selectedNodeId}
          />
        </div>

        {/* Right Panel: Node Details (Shows when node is selected) */}
        <NodeDetailsPanel
          nodeId={selectedNodeId}
          nodeType={selectedNodeType}
          nodeData={selectedNodeData}
          onClose={() => {
            setSelectedNodeId(null)
            setSelectedNodeType(null)
            setSelectedNodeData(null)
          }}
        />
      </div>
    </div>
  )
}

export default GraphView
