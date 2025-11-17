import { useServiceAccount } from '../../hooks/useServiceAccounts'

interface NodeDetailsPanelProps {
  nodeId: string | null
  nodeType: string | null
  nodeData: Record<string, any> | null
  onClose: () => void
}

const NodeDetailsPanel = ({ nodeId, nodeType, nodeData, onClose }: NodeDetailsPanelProps) => {
  // Get ServiceAccount details when ServiceAccount node is selected
  const { data: serviceAccount, isLoading: isLoadingDetails } = useServiceAccount(
    (nodeType === 'serviceaccount' && nodeData?.id) ? String(nodeData.id) : ''
  )

  if (!nodeId || !nodeType || !nodeData) {
    return null
  }

  return (
    <div className="w-96 bg-white border-l border-gray-200 flex-shrink-0 overflow-y-auto h-full">
      <div className="p-4">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-gray-900 flex items-center">
            {nodeType === 'serviceaccount' && (
              <svg className="w-5 h-5 mr-1.5 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
              </svg>
            )}
            {nodeType === 'role' && (
              <svg className="w-5 h-5 mr-1.5 text-purple-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
              </svg>
            )}
            {nodeType === 'clusterrole' && (
              <svg className="w-5 h-5 mr-1.5 text-purple-800" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
              </svg>
            )}
            {nodeType === 'namespace' && (
              <svg className="w-5 h-5 mr-1.5 text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
              </svg>
            )}
            {nodeType === 'cluster' && (
              <svg className="w-5 h-5 mr-1.5 text-gray-800" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
              </svg>
            )}
            {nodeType === 'serviceaccount' ? 'ServiceAccount' : 
             nodeType === 'role' ? 'Role' :
             nodeType === 'clusterrole' ? 'ClusterRole' :
             nodeType === 'namespace' ? 'Namespace' :
             nodeType === 'cluster' ? 'Cluster' :
             'Node'} Details
          </h2>
          <button
            onClick={onClose}
            className="p-1.5 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded transition-colors"
            title="Close details"
          >
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {nodeType === 'serviceaccount' && isLoadingDetails ? (
          <div className="text-gray-500 text-sm flex items-center justify-center p-4">
            <svg className="animate-spin -ml-1 mr-3 h-5 w-5 text-blue-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            <span>Loading details...</span>
          </div>
        ) : nodeType === 'serviceaccount' && serviceAccount ? (
          <div className="space-y-2 text-sm">
            <div className="grid grid-cols-2 gap-2">
              <div className="text-gray-600 flex items-center">
                <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                </svg>
                <span>Name:</span>
              </div>
              <div className="font-medium text-gray-900">{serviceAccount.name}</div>
              
              <div className="text-gray-600 flex items-center">
                <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
                </svg>
                <span>Namespace:</span>
              </div>
              <div className="font-medium text-gray-900">{serviceAccount.namespace}</div>
              
              <div className="text-gray-600 flex items-center">
                <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
                </svg>
                <span>Cluster:</span>
              </div>
              <div className="font-medium text-gray-900">{serviceAccount.clusterId}</div>
              
              <div className="text-gray-600 flex items-center">
                <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V8a2 2 0 00-2-2h-5m-4 0V5a2 2 0 114 0v1m-4 0a2 2 0 104 0m-5 8a2 2 0 100-4 2 2 0 000 4zm0 0c1.306 0 2.417.835 2.83 2M9 14a3.001 3.001 0 00-2.83 2M15 11h3m-3 4h2" />
                </svg>
                <span>UID:</span>
              </div>
              <div className="font-mono text-xs text-gray-700 break-all">{serviceAccount.uid}</div>
              
              <div className="text-gray-600 flex items-center">
                <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
                </svg>
                <span>Created:</span>
              </div>
              <div className="text-gray-700">
                {new Date(serviceAccount.createdAt).toLocaleString()}
              </div>
              
              <div className="text-gray-600 flex items-center">
                <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                </svg>
                <span>Updated:</span>
              </div>
              <div className="text-gray-700">
                {new Date(serviceAccount.updatedAt).toLocaleString()}
              </div>
            </div>

            {serviceAccount.labels && serviceAccount.labels !== '' && (
              <div className="mt-3 pt-3 border-t border-gray-200">
                <div className="text-gray-600 mb-1 flex items-center">
                  <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z" />
                  </svg>
                  <span>Labels:</span>
                </div>
                <div className="font-mono text-xs text-gray-700 break-all bg-gray-50 p-2 rounded">
                  {serviceAccount.labels}
                </div>
              </div>
            )}

            {serviceAccount.secrets && serviceAccount.secrets !== '' && (
              <div className="mt-3 pt-3 border-t border-gray-200">
                <div className="text-gray-600 mb-1 flex items-center">
                  <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                  </svg>
                  <span>Secrets:</span>
                </div>
                <div className="font-mono text-xs text-gray-700 break-all bg-gray-50 p-2 rounded">
                  {serviceAccount.secrets}
                </div>
              </div>
            )}
          </div>
        ) : (
          <div className="space-y-2 text-sm">
            <div className="grid grid-cols-2 gap-2">
              <div className="text-gray-600 flex items-center">
                <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z" />
                </svg>
                <span>Name:</span>
              </div>
              <div className="font-medium text-gray-900">{nodeData.name || nodeData.label || 'N/A'}</div>
              
              {nodeData.cluster && (
                <>
                  <div className="text-gray-600 flex items-center">
                    <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
                    </svg>
                    <span>Cluster:</span>
                  </div>
                  <div className="font-medium text-gray-900">{nodeData.cluster}</div>
                </>
              )}
              
              {nodeData.namespace && (
                <>
                  <div className="text-gray-600 flex items-center">
                    <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
                    </svg>
                    <span>Namespace:</span>
                  </div>
                  <div className="font-medium text-gray-900">{nodeData.namespace}</div>
                </>
              )}
              
              {nodeData.uid && (
                <>
                  <div className="text-gray-600 flex items-center">
                    <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V8a2 2 0 00-2-2h-5m-4 0V5a2 2 0 114 0v1m-4 0a2 2 0 104 0m-5 8a2 2 0 100-4 2 2 0 000 4zm0 0c1.306 0 2.417.835 2.83 2M9 14a3.001 3.001 0 00-2.83 2M15 11h3m-3 4h2" />
                    </svg>
                    <span>UID:</span>
                  </div>
                  <div className="font-mono text-xs text-gray-700 break-all">{nodeData.uid}</div>
                </>
              )}
            </div>
            
            {nodeType === 'role' && (
              <div className="mt-3 pt-3 border-t border-gray-200">
                <div className="text-gray-600 mb-1 text-xs">
                  Role is scoped to namespace: <span className="font-medium">{nodeData.namespace || 'N/A'}</span>
                </div>
              </div>
            )}
            
            {nodeType === 'clusterrole' && (
              <div className="mt-3 pt-3 border-t border-gray-200">
                <div className="text-gray-600 mb-1 text-xs">
                  ClusterRole is cluster-scoped and applies to all namespaces.
                </div>
              </div>
            )}
            
            {nodeType === 'namespace' && (
              <div className="mt-3 pt-3 border-t border-gray-200">
                <div className="text-gray-600 mb-1 text-xs">
                  Namespace contains ServiceAccounts, Roles, and RoleBindings.
                </div>
              </div>
            )}
            
            {nodeType === 'cluster' && (
              <div className="mt-3 pt-3 border-t border-gray-200">
                <div className="text-gray-600 mb-1 text-xs">
                  Cluster contains all namespaces and cluster-scoped resources.
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

export default NodeDetailsPanel

