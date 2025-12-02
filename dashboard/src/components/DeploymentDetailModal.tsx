import { useQuery } from '@tanstack/react-query'
import { deploymentsApi, type Deployment } from '../services/api'
import { formatDistanceToNow } from 'date-fns'

interface DeploymentDetailModalProps {
  deploymentId: string
  onClose: () => void
}

const DeploymentDetailModal: React.FC<DeploymentDetailModalProps> = ({ deploymentId, onClose }) => {
  const { data: deploymentResponse, isLoading, error } = useQuery({
    queryKey: ['deployment', deploymentId],
    queryFn: () => deploymentsApi.getById(deploymentId),
    enabled: !!deploymentId,
  })

  const deployment: Deployment | undefined = deploymentResponse?.data

  const getReplicaStatusColor = (ready: number, desired: number) => {
    if (desired === 0) return 'bg-gray-500'
    if (ready === desired) return 'bg-green-500'
    if (ready > 0 && ready < desired) return 'bg-yellow-500'
    return 'bg-red-500'
  }

  const getConditionColor = (status: string) => {
    return status === 'True' ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'
  }

  if (isLoading) {
    return (
      <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50" onClick={onClose}>
        <div className="bg-white dark:bg-gray-800 rounded-lg p-6" onClick={(e) => e.stopPropagation()}>
          <div className="text-center text-gray-500 dark:text-gray-400">Loading...</div>
        </div>
      </div>
    )
  }

  if (error || !deployment) {
    return (
      <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50" onClick={onClose}>
        <div className="bg-white dark:bg-gray-800 rounded-lg p-6" onClick={(e) => e.stopPropagation()}>
          <div className="text-center text-red-500">
            Error loading deployment: {error?.message || 'Not found'}
          </div>
          <button
            onClick={onClose}
            className="mt-4 w-full px-4 py-2 bg-gray-200 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg hover:bg-gray-300 dark:hover:bg-gray-600"
          >
            Close
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4" onClick={onClose}>
      <div
        className="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-5xl w-full max-h-[90vh] overflow-y-auto"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="sticky top-0 bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-6 py-4 flex items-center justify-between z-10">
          <div>
            <h2 className="text-2xl font-bold text-gray-900 dark:text-white">{deployment.name}</h2>
            <p className="text-sm text-gray-600 dark:text-gray-400 mt-1">
              {deployment.namespace} • {deployment.clusterId}
            </p>
          </div>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 transition-colors"
          >
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Content */}
        <div className="px-6 py-4 space-y-6">
          {/* Overview */}
          <div>
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-3">Overview</h3>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
              <div>
                <p className="text-sm font-medium text-gray-500 dark:text-gray-400">Strategy</p>
                <p className="text-base text-gray-900 dark:text-white mt-1">{deployment.strategy}</p>
              </div>
              <div>
                <p className="text-sm font-medium text-gray-500 dark:text-gray-400">Created</p>
                <p className="text-base text-gray-900 dark:text-white mt-1">
                  {formatDistanceToNow(new Date(deployment.createdAt), { addSuffix: true })}
                </p>
              </div>
              <div>
                <p className="text-sm font-medium text-gray-500 dark:text-gray-400">UID</p>
                <p className="text-xs text-gray-900 dark:text-white mt-1 font-mono break-all">
                  {deployment.uid}
                </p>
              </div>
            </div>
          </div>

          {/* Replica Status */}
          <div>
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-3">Replica Status</h3>
            <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
              <div className="text-center p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                <p className="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Desired</p>
                <p className="text-xl font-bold text-gray-900 dark:text-white">{deployment.replicasDesired}</p>
              </div>
              <div className="text-center p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                <p className="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Ready</p>
                <div className="flex items-center justify-center gap-2">
                  <span className={`h-2 w-2 rounded-full ${getReplicaStatusColor(deployment.replicasReady, deployment.replicasDesired)}`}></span>
                  <p className="text-xl font-bold text-gray-900 dark:text-white">{deployment.replicasReady}</p>
                </div>
              </div>
              <div className="text-center p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                <p className="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Available</p>
                <p className="text-xl font-bold text-gray-900 dark:text-white">{deployment.replicasAvailable}</p>
              </div>
              <div className="text-center p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                <p className="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Updated</p>
                <p className="text-xl font-bold text-gray-900 dark:text-white">{deployment.replicasUpdated}</p>
              </div>
              <div className="text-center p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                <p className="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Unavailable</p>
                <p className="text-xl font-bold text-gray-900 dark:text-white">{deployment.replicasUnavailable}</p>
              </div>
            </div>
          </div>

          {/* Containers */}
          {deployment.containers && deployment.containers.length > 0 && (
            <div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-3">
                Containers ({deployment.containers.length})
              </h3>
              <div className="space-y-3">
                {deployment.containers.map((container: any, idx: number) => (
                  <div key={idx} className="border border-gray-200 dark:border-gray-700 rounded-lg p-3">
                    <div className="mb-2">
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">{container.name}</h4>
                      <p className="text-xs text-gray-600 dark:text-gray-400 font-mono mt-1">{container.image}</p>
                    </div>
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-xs">
                      <div>
                        <p className="font-medium text-gray-500 dark:text-gray-400">CPU Request</p>
                        <p className="text-gray-900 dark:text-white mt-1">{container.cpuRequest || 'N/A'}</p>
                      </div>
                      <div>
                        <p className="font-medium text-gray-500 dark:text-gray-400">Memory Request</p>
                        <p className="text-gray-900 dark:text-white mt-1">{container.memoryRequest || 'N/A'}</p>
                      </div>
                      <div>
                        <p className="font-medium text-gray-500 dark:text-gray-400">CPU Limit</p>
                        <p className="text-gray-900 dark:text-white mt-1">{container.cpuLimit || 'N/A'}</p>
                      </div>
                      <div>
                        <p className="font-medium text-gray-500 dark:text-gray-400">Memory Limit</p>
                        <p className="text-gray-900 dark:text-white mt-1">{container.memoryLimit || 'N/A'}</p>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Conditions */}
          {deployment.conditions && deployment.conditions.length > 0 && (
            <div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-3">Conditions</h3>
              <div className="space-y-2">
                {deployment.conditions.map((condition: any, idx: number) => (
                  <div key={idx} className="flex items-start justify-between p-2 bg-gray-50 dark:bg-gray-700 rounded-lg">
                    <div>
                      <div className="flex items-center gap-2">
                        <p className="text-sm font-semibold text-gray-900 dark:text-white">{condition.type}</p>
                        <span className={`text-xs font-medium ${getConditionColor(condition.status)}`}>
                          {condition.status}
                        </span>
                      </div>
                      {condition.reason && (
                        <p className="text-xs text-gray-600 dark:text-gray-400 mt-1">Reason: {condition.reason}</p>
                      )}
                      {condition.message && (
                        <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">{condition.message}</p>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Selector */}
          {deployment.selector && Object.keys(deployment.selector).length > 0 && (
            <div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-3">Selector</h3>
              <div className="flex flex-wrap gap-2">
                {Object.entries(deployment.selector).map(([key, value]) => (
                  <span
                    key={key}
                    className="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200"
                  >
                    {key}: {String(value)}
                  </span>
                ))}
              </div>
            </div>
          )}

          {/* Labels */}
          {deployment.labels && Object.keys(deployment.labels).length > 0 && (
            <div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-3">Labels</h3>
              <div className="flex flex-wrap gap-2">
                {Object.entries(deployment.labels).map(([key, value]) => (
                  <span
                    key={key}
                    className="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200"
                  >
                    {key}: {String(value)}
                  </span>
                ))}
              </div>
            </div>
          )}

          {/* Annotations */}
          {deployment.annotations && Object.keys(deployment.annotations).length > 0 && (
            <div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-3">Annotations</h3>
              <div className="space-y-2 max-h-48 overflow-y-auto">
                {Object.entries(deployment.annotations).map(([key, value]) => (
                  <div key={key} className="flex flex-col gap-1 p-2 bg-gray-50 dark:bg-gray-700 rounded text-xs">
                    <span className="font-medium text-gray-700 dark:text-gray-300">{key}</span>
                    <span className="text-gray-600 dark:text-gray-400 break-all">{String(value)}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="sticky bottom-0 bg-white dark:bg-gray-800 border-t border-gray-200 dark:border-gray-700 px-6 py-4">
          <button
            onClick={onClose}
            className="w-full px-4 py-2 bg-pink-600 text-white rounded-lg hover:bg-pink-700 transition-colors font-medium"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  )
}

export default DeploymentDetailModal

