import React from 'react'

interface NodeDetailsPanelProps {
  node: any
  onClose: () => void
}

/**
 * Node Details Panel - Right sidebar
 * Shows detailed information about selected node
 */
const NodeDetailsPanel: React.FC<NodeDetailsPanelProps> = ({ node, onClose }) => {
  if (!node) {
    return (
      <div className="flex items-center justify-center h-full p-4 text-gray-500 dark:text-gray-400">
        <div className="text-center">
          <svg className="w-12 h-12 mx-auto mb-2 text-gray-300 dark:text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <p className="text-sm">Select a node to view details</p>
        </div>
      </div>
    )
  }
  
  const nodeType = node.type || 'unknown'
  const nodeData = node.data || {}

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-700">
        <h2 className="text-base font-semibold text-gray-900 dark:text-white">
          Node Details
          </h2>
          <button
            onClick={onClose}
          className="p-1.5 rounded hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-600 dark:text-gray-400 transition-colors"
          title="Close panel"
          >
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-4">
        <div className="space-y-4">
        {/* Node Type Badge */}
        <div>
          <span className={`
            inline-flex items-center px-3 py-1 rounded-full text-xs font-medium
            ${nodeType === 'serviceaccount' ? 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200' : ''}
            ${nodeType === 'role' ? 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200' : ''}
            ${nodeType === 'clusterrole' ? 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200' : ''}
            ${nodeType === 'namespace' ? 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200' : ''}
            ${nodeType === 'cluster' ? 'bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200' : ''}
          `}>
            {nodeType.toUpperCase()}
          </span>
              </div>
              
        {/* Node Name */}
        <div>
          <h3 className="text-lg font-bold text-gray-900 dark:text-white break-words leading-tight">
            {String(nodeData.name || nodeData.label || node.id || 'Unknown')}
          </h3>
            </div>

        {/* Properties */}
        <div className="space-y-3">
          <h4 className="text-sm font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wider">
            Properties
          </h4>
          
          <div className="space-y-2">
            {nodeData.namespace && (
              <div className="flex justify-between items-start">
                <span className="text-sm text-gray-600 dark:text-gray-400">Namespace:</span>
                <span className="text-sm font-medium text-gray-900 dark:text-white text-right">
                  {nodeData.namespace}
                </span>
              </div>
            )}

            {nodeData.cluster && (
              <div className="flex justify-between items-start">
                <span className="text-sm text-gray-600 dark:text-gray-400">Cluster:</span>
                <span className="text-sm font-medium text-gray-900 dark:text-white text-right">
                  {nodeData.cluster}
                </span>
              </div>
            )}
            
            {nodeData.uid && (
              <div className="flex justify-between items-start">
                <span className="text-sm text-gray-600 dark:text-gray-400">UID:</span>
                <span className="text-xs font-mono text-gray-700 dark:text-gray-300 text-right break-all">
                  {nodeData.uid}
                </span>
              </div>
            )}
            
            {nodeData.createdAt && (
              <div className="flex justify-between items-start">
                <span className="text-sm text-gray-600 dark:text-gray-400">Created:</span>
                <span className="text-sm text-gray-900 dark:text-white text-right">
                  {(() => {
                    try {
                      return new Date(nodeData.createdAt).toLocaleDateString()
                    } catch {
                      return String(nodeData.createdAt)
                    }
                  })()}
                </span>
              </div>
            )}
          </div>
        </div>
        
        {/* Labels */}
        {nodeData.labels && (
          <div className="space-y-2">
            <h4 className="text-sm font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wider">
              Labels
            </h4>
            <div className="bg-gray-50 dark:bg-gray-900 rounded p-3">
              <pre className="text-xs font-mono text-gray-700 dark:text-gray-300 whitespace-pre-wrap break-all">
                {typeof nodeData.labels === 'object' 
                  ? JSON.stringify(nodeData.labels, null, 2) 
                  : String(nodeData.labels)}
              </pre>
                </div>
              </div>
            )}
            
        {/* Secrets */}
        {nodeData.secrets && (
          <div className="space-y-2">
            <h4 className="text-sm font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wider">
              Secrets
            </h4>
            <div className="bg-gray-50 dark:bg-gray-900 rounded p-3">
              <pre className="text-xs font-mono text-gray-700 dark:text-gray-300 whitespace-pre-wrap break-all">
                {typeof nodeData.secrets === 'object' 
                  ? JSON.stringify(nodeData.secrets, null, 2) 
                  : String(nodeData.secrets)}
              </pre>
                </div>
              </div>
            )}
        
        {/* Connections */}
        <div className="space-y-2">
          <h4 className="text-sm font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wider">
            Connections
          </h4>
          <div className="text-sm text-gray-600 dark:text-gray-400">
            <p>Select a node in the graph to see its connections</p>
          </div>
        </div>
        </div>
      </div>
      
      {/* Footer Actions */}
      <div className="px-4 py-3 border-t border-gray-200 dark:border-gray-700">
        <div className="space-y-2">
        <button className="w-full px-3 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 dark:bg-blue-700 dark:hover:bg-blue-600 rounded-lg transition-colors shadow-sm">
          View Full Details
        </button>
        <button className="w-full px-3 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 rounded-lg transition-colors">
          Export Info
        </button>
        </div>
      </div>
    </div>
  )
}

export default NodeDetailsPanel
