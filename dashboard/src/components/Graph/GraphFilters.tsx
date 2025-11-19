import { useState, useEffect, useRef } from 'react'
import { useClusters } from '../../hooks/useClusters'

type LayoutOption = 'cose' | 'fcose' | 'dagre' | 'breadthfirst' | 'circle' | 'concentric' | 'grid'

interface GraphFiltersProps {
  cluster?: string
  namespace?: string
  onClusterChange: (cluster: string) => void
  onNamespaceChange: (namespace: string) => void
  isLoading?: boolean // Loading state from parent
  nodeTypeFilters: Record<string, boolean>
  onNodeTypeChange: (type: string, value: boolean) => void
  connectionTypeFilters: Record<string, boolean>
  onConnectionTypeChange: (type: string, value: boolean) => void
  layout: LayoutOption
  onLayoutChange: (layout: LayoutOption) => void
  autoFitEnabled: boolean
  onAutoFitChange: (enabled: boolean) => void
  onResetAdvancedFilters?: () => void
}

const NODE_TYPE_OPTIONS: Array<{ key: 'serviceaccount' | 'role' | 'clusterrole' | 'namespace' | 'cluster'; label: string }> = [
  { key: 'serviceaccount', label: 'ServiceAccount' },
  { key: 'role', label: 'Role' },
  { key: 'clusterrole', label: 'ClusterRole' },
  { key: 'namespace', label: 'Namespace' },
  { key: 'cluster', label: 'Cluster' },
]

const CONNECTION_TYPE_OPTIONS: Array<{ key: 'binding' | 'member' | 'owner' | 'reference'; label: string }> = [
  { key: 'binding', label: 'Binding' },
  { key: 'member', label: 'Member' },
  { key: 'owner', label: 'Owner' },
  { key: 'reference', label: 'Reference' },
]

const GraphFilters = ({
  cluster,
  namespace,
  onClusterChange,
  onNamespaceChange,
  isLoading = false,
  nodeTypeFilters,
  onNodeTypeChange,
  connectionTypeFilters,
  onConnectionTypeChange,
  layout,
  onLayoutChange,
  autoFitEnabled,
  onAutoFitChange,
  onResetAdvancedFilters,
}: GraphFiltersProps) => {
  const { data: clusters, isLoading: clustersLoading } = useClusters()
  const [localNamespace, setLocalNamespace] = useState(namespace || '')
  const [showAdvancedFilters, setShowAdvancedFilters] = useState(false)
  const [recentNamespaces, setRecentNamespaces] = useState<string[]>([])
  const [suggestedNamespaces, setSuggestedNamespaces] = useState<string[]>([])
  const [showNamespaceSuggestions, setShowNamespaceSuggestions] = useState(false)
  const namespaceInputRef = useRef<HTMLInputElement>(null)
  const suggestionsRef = useRef<HTMLDivElement>(null)
  const debounceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  
  // Common namespaces for suggestions (in a real app, these could be fetched from an API)
  const commonNamespaces = [
    'default', 
    'kube-system', 
    'kube-public', 
    'kube-node-lease', 
    'monitoring',
    'logging',
    'cert-manager',
    'ingress-nginx'
  ]

  // Sync local namespace with prop
  useEffect(() => {
    setLocalNamespace(namespace || '')
  }, [namespace])

  // Add to recent namespaces when changed
  useEffect(() => {
    if (namespace && namespace.trim() !== '') {
      setRecentNamespaces(prev => {
        const newRecentNamespaces = prev.filter(ns => ns !== namespace)
        return [namespace, ...newRecentNamespaces].slice(0, 5) // Keep only last 5
      })
    }
  }, [namespace])

  // Handle namespace input focus
  const handleNamespaceInputFocus = () => {
    // Generate suggestions based on recent and common namespaces
    const suggestions = [
      ...new Set([
        ...recentNamespaces,
        ...commonNamespaces
      ])
    ].filter(ns => ns.toLowerCase().includes(localNamespace.toLowerCase())).slice(0, 6)
    
    setSuggestedNamespaces(suggestions)
    setShowNamespaceSuggestions(suggestions.length > 0)
  }

  // Handle namespace input change and suggestions
  const handleNamespaceInputChange = (value: string) => {
    setLocalNamespace(value)
    
    // Generate suggestions based on input value
    if (value.trim() !== '') {
      const suggestions = [
        ...new Set([
          ...recentNamespaces,
          ...commonNamespaces
        ])
      ].filter(ns => ns.toLowerCase().includes(value.toLowerCase())).slice(0, 6)
      
      setSuggestedNamespaces(suggestions)
      setShowNamespaceSuggestions(suggestions.length > 0)
    } else {
      setShowNamespaceSuggestions(false)
    }
    
    // Clear existing timer
    if (debounceTimerRef.current) {
      clearTimeout(debounceTimerRef.current)
    }
    
    // Set new timer to update after 500ms of no typing
    debounceTimerRef.current = setTimeout(() => {
      onNamespaceChange(value)
    }, 500)
  }

  // Select a suggested namespace
  const selectNamespace = (selected: string) => {
    // Clear any pending debounce timer
    if (debounceTimerRef.current) {
      clearTimeout(debounceTimerRef.current)
      debounceTimerRef.current = null
    }
    
    setLocalNamespace(selected)
    onNamespaceChange(selected)
    setShowNamespaceSuggestions(false)
    namespaceInputRef.current?.focus()
  }

  // Handle click outside suggestions to close them
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        suggestionsRef.current && 
        !suggestionsRef.current.contains(event.target as Node) &&
        namespaceInputRef.current &&
        !namespaceInputRef.current.contains(event.target as Node)
      ) {
        setShowNamespaceSuggestions(false)
      }
    }
    
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  // Clear filters handler
  const handleClearFilters = () => {
    // Clear any pending debounce timer
    if (debounceTimerRef.current) {
      clearTimeout(debounceTimerRef.current)
      debounceTimerRef.current = null
    }
    
    onClusterChange('')
    onNamespaceChange('')
    setLocalNamespace('')
  }

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current)
      }
    }
  }, [])

  // Load recent namespaces from localStorage
  useEffect(() => {
    if (typeof window !== 'undefined') {
      const stored = localStorage.getItem('ksam-recent-namespaces')
      if (stored) {
        try {
          const parsed = JSON.parse(stored)
          if (Array.isArray(parsed)) {
            setRecentNamespaces(parsed)
          }
        } catch (e) {
          // Ignore parse errors
        }
      }
    }
  }, [])

  // Save recent namespaces to localStorage
  useEffect(() => {
    if (typeof window !== 'undefined' && recentNamespaces.length > 0) {
      localStorage.setItem('ksam-recent-namespaces', JSON.stringify(recentNamespaces))
    }
  }, [recentNamespaces])

  return (
    <div className="flex flex-col h-full bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 overflow-hidden">
      {/* Main Filters - Collapsible Card */}
      <div className="border-b border-gray-200 dark:border-gray-700">
        <button
          type="button"
          onClick={() => setShowAdvancedFilters(!showAdvancedFilters)}
          className="w-full px-4 py-3 flex items-center justify-between hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
        >
          <div className="flex items-center">
            <svg className="w-4 h-4 mr-2 text-gray-600 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
            </svg>
            <span className="text-sm font-semibold text-gray-900 dark:text-white">Filters</span>
            {(cluster || namespace) && (
              <span className="ml-2 px-1.5 py-0.5 text-xs font-medium bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200 rounded-full">
                Active
              </span>
            )}
          </div>
          <div className="flex items-center gap-2">
            {isLoading && (
              <div className="flex items-center text-xs text-gray-500 dark:text-gray-400">
                <svg className="animate-spin h-4 w-4 mr-1" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                <span>Loading...</span>
              </div>
            )}
            <svg 
              className={`w-4 h-4 text-gray-500 dark:text-gray-400 transition-transform ${showAdvancedFilters ? 'rotate-180' : ''}`}
              fill="none" 
              viewBox="0 0 24 24" 
              stroke="currentColor"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </div>
        </button>
      </div>

      <div className="flex-shrink-0 p-4">
        <div className="grid grid-cols-1 gap-4">
          {/* Cluster Filter */}
          <div className="relative">
            <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300 mb-1.5 flex items-center">
              <svg className="w-3.5 h-3.5 mr-1.5 text-gray-600 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
              </svg>
              Cluster
              {clustersLoading && (
                <svg className="animate-spin h-3 w-3 ml-2 text-gray-400" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
              )}
            </label>
            <div className="relative">
              <select
                value={cluster || ''}
                onChange={(e) => onClusterChange(e.target.value)}
                disabled={clustersLoading || isLoading}
                className="w-full px-3 py-2 text-xs border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 transition-colors appearance-none disabled:bg-gray-100 dark:disabled:bg-gray-700 disabled:cursor-not-allowed"
              >
                <option value="">All Clusters</option>
                {clusters && Array.isArray(clusters) && clusters.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </select>
              <div className="absolute inset-y-0 right-0 flex items-center pr-2.5 pointer-events-none">
                <svg className="h-3.5 w-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 9l4-4 4 4m0 6l-4 4-4-4" />
                </svg>
              </div>
            </div>
          </div>
          
          {/* Namespace Filter */}
          <div className="relative">
            <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300 mb-1.5 flex items-center">
              <svg className="w-3.5 h-3.5 mr-1.5 text-gray-600 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
              </svg>
              Namespace
            </label>
            <div className="relative">
              <input
                ref={namespaceInputRef}
                type="text"
                value={localNamespace}
                onChange={(e) => handleNamespaceInputChange(e.target.value)}
                onFocus={handleNamespaceInputFocus}
                onKeyPress={(e) => {
                  if (e.key === 'Enter') {
                    if (debounceTimerRef.current) {
                      clearTimeout(debounceTimerRef.current)
                    }
                    onNamespaceChange(localNamespace)
                    setShowNamespaceSuggestions(false)
                  }
                }}
                placeholder="Enter namespace (empty for all)..."
                disabled={isLoading}
                className="w-full pl-3 pr-8 py-2 text-xs border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 transition-colors disabled:bg-gray-100 dark:disabled:bg-gray-700 disabled:cursor-not-allowed"
              />
              {localNamespace ? (
                <button 
                  onClick={() => {
                    setLocalNamespace('')
                    onNamespaceChange('')
                    namespaceInputRef.current?.focus()
                  }}
                  className="absolute inset-y-0 right-0 flex items-center pr-3 text-gray-400 hover:text-gray-600 dark:text-gray-400"
                >
                  <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              ) : (
                <div className="absolute inset-y-0 right-0 flex items-center pr-3 pointer-events-none">
                  <svg className="h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                  </svg>
                </div>
              )}
              
              {/* Namespace Suggestions Dropdown */}
              {showNamespaceSuggestions && (
                <div 
                  ref={suggestionsRef}
                  className="absolute z-10 w-full mt-1 bg-white rounded-md shadow-lg border border-gray-200 dark:border-gray-700 py-1 max-h-48 overflow-y-auto"
                >
                  {suggestedNamespaces.map((ns) => (
                    <div
                      key={ns}
                      onClick={() => selectNamespace(ns)}
                      className="px-3 py-1.5 text-xs text-gray-700 dark:text-gray-300 hover:bg-blue-50 cursor-pointer flex items-center"
                    >
                      {recentNamespaces.includes(ns) ? (
                        <svg className="w-3.5 h-3.5 mr-1.5 text-gray-500 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                      ) : (
                        <svg className="w-3.5 h-3.5 mr-1.5 text-gray-500 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
                        </svg>
                      )}
                      {ns}
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
        
        {/* Clear Filters Button */}
        {(cluster || namespace) && (
          <div className="mt-3 flex justify-end">
            <button
              type="button"
              onClick={handleClearFilters}
              disabled={isLoading}
              className="text-xs text-gray-600 dark:text-gray-400 hover:text-gray-800 focus:outline-none flex items-center disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              <svg className="w-3.5 h-3.5 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
              Clear Filters
            </button>
          </div>
        )}
      </div>
      
      {/* Advanced Filters Panel - Collapsible với scroll */}
      {showAdvancedFilters && (
        <div className="flex-1 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 overflow-y-auto">
          <div className="p-4">
            <div className="grid grid-cols-1 gap-4">
              {/* Node Type Filter - Segmented Toggle */}
              <div className="space-y-3">
                <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300 dark:text-gray-300 mb-1.5 flex items-center">
                  <svg className="w-3.5 h-3.5 mr-1.5 text-gray-600 dark:text-gray-400 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z" />
                  </svg>
                  Node Types
                </label>
                <div className="inline-flex rounded-md border border-gray-300 dark:border-gray-600 dark:border-gray-600 bg-gray-100 dark:bg-gray-700 p-0.5 flex-wrap gap-0.5" role="group">
                  {NODE_TYPE_OPTIONS.map(({ key, label }) => (
                    <button
                      key={key}
                      type="button"
                      onClick={() => onNodeTypeChange(key, !nodeTypeFilters[key])}
                      disabled={isLoading}
                      className={`px-2 py-1 text-xs font-medium rounded transition-all ${
                        nodeTypeFilters[key]
                          ? 'bg-white dark:bg-gray-800 text-blue-700 dark:text-blue-400 shadow-sm border border-blue-200 dark:border-blue-600'
                          : 'text-gray-600 dark:text-gray-400 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-50 dark:hover:bg-gray-700 dark:hover:bg-gray-600'
                      } disabled:opacity-50 disabled:cursor-not-allowed`}
                      title={label}
                    >
                      {label}
                    </button>
                  ))}
                </div>
              </div>
              
              {/* Connection Type Filter - Segmented Toggle */}
              <div className="space-y-3">
                <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300 dark:text-gray-300 mb-1.5 flex items-center">
                  <svg className="w-3.5 h-3.5 mr-1.5 text-gray-600 dark:text-gray-400 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
                  </svg>
                  Connection Types
                </label>
                <div className="inline-flex rounded-md border border-gray-300 dark:border-gray-600 dark:border-gray-600 bg-gray-100 dark:bg-gray-700 p-0.5 flex-wrap gap-0.5" role="group">
                  {CONNECTION_TYPE_OPTIONS.map(({ key, label }) => (
                    <button
                      key={key}
                      type="button"
                      onClick={() => onConnectionTypeChange(key, !connectionTypeFilters[key])}
                      disabled={isLoading}
                      className={`px-2 py-1 text-xs font-medium rounded transition-all ${
                        connectionTypeFilters[key]
                          ? 'bg-white dark:bg-gray-800 text-purple-700 dark:text-purple-400 shadow-sm border border-purple-200 dark:border-purple-600'
                          : 'text-gray-600 dark:text-gray-400 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-50 dark:hover:bg-gray-700 dark:hover:bg-gray-600'
                      } disabled:opacity-50 disabled:cursor-not-allowed`}
                      title={label}
                    >
                      {label}
                    </button>
                  ))}
                </div>
              </div>
              
              {/* Layout Options */}
              <div className="space-y-3">
                <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300 dark:text-gray-300 mb-1.5 flex items-center">
                  <svg className="w-3.5 h-3.5 mr-1.5 text-gray-600 dark:text-gray-400 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z" />
                  </svg>
                  Layout Options
                </label>
                <div className="space-y-3">
                  <select 
                    className="block w-full py-1.5 px-2.5 text-xs border border-gray-300 dark:border-gray-600 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-md focus:outline-none focus:ring-blue-500 focus:border-blue-500"
                    value={layout}
                    onChange={(e) => onLayoutChange(e.target.value as LayoutOption)}
                    disabled={isLoading}
                  >
                    <option value="cose">COSE Layout (Default)</option>
                    <option value="fcose">FCOSE Layout (Optimized for Large Graphs)</option>
                    <option value="dagre">Dagre Layout (Directed Flow)</option>
                    <option value="breadthfirst">Breadth-first Layout</option>
                    <option value="circle">Circle Layout</option>
                    <option value="concentric">Concentric Layout</option>
                    <option value="grid">Grid Layout</option>
                  </select>
                  
                  <div className="flex items-center">
                    <input
                      type="checkbox"
                      id="autofit"
                      name="autofit"
                      className="h-3.5 w-3.5 text-blue-600 border-gray-300 dark:border-gray-600 dark:border-gray-600 rounded focus:ring-blue-500"
                      checked={autoFitEnabled}
                      onChange={(e) => onAutoFitChange(e.target.checked)}
                      disabled={isLoading}
                    />
                    <label htmlFor="autofit" className="ml-2 text-xs text-gray-700 dark:text-gray-300 dark:text-gray-300">
                      Auto-fit to view
                    </label>
                  </div>
                </div>
              </div>
              
              <div className="mt-3 pt-3 border-t border-gray-200 dark:border-gray-700 dark:border-gray-700">
                <div className="flex items-center justify-between">
                  <p className="text-xs text-gray-500 dark:text-gray-400 dark:text-gray-400 flex items-center">
                    <svg className="w-3.5 h-3.5 mr-1 text-gray-400 dark:text-gray-500 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    Advanced filters are now active. Adjust visibility and layout to refine the graph.
                  </p>
                  {onResetAdvancedFilters && (
                    <button
                      type="button"
                      onClick={onResetAdvancedFilters}
                      className="text-xs text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 focus:outline-none"
                    >
                      Reset advanced filters
                    </button>
                  )}
                </div>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default GraphFilters