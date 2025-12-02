import { useParams, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { deploymentsApi, type Deployment } from '../services/api'
import { formatDistanceToNow } from 'date-fns'

const DeploymentDetail = () => {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const { data: deploymentResponse, isLoading, error } = useQuery({
    queryKey: ['deployment', id],
    queryFn: () => deploymentsApi.getById(id!),
    enabled: !!id,
  })

  const deployment: Deployment | undefined = deploymentResponse?.data

  if (isLoading) {
    return (
      <div className="h-full overflow-y-auto bg-gray-50 dark:bg-gray-900">
        <div className="container mx-auto px-6 py-6 max-w-7xl">
          <div className="text-center py-8 text-gray-500 dark:text-gray-400">Loading...</div>
        </div>
      </div>
    )
  }

  if (error || !deployment) {
    return (
      <div className="h-full overflow-y-auto bg-gray-50 dark:bg-gray-900">
        <div className="container mx-auto px-6 py-6 max-w-7xl">
          <div className="text-center py-8 text-red-500">
            Error loading deployment: {error?.message || 'Not found'}
          </div>
        </div>
      </div>
    )
  }

  const getReplicaStatusColor = (ready: number, desired: number) => {
    if (desired === 0) return 'bg-gray-500'
    if (ready === desired) return 'bg-green-500'
    if (ready > 0 && ready < desired) return 'bg-yellow-500'
    return 'bg-red-500'
  }

  const getConditionColor = (status: string) => {
    return status === 'True' ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'
  }

  return (
    <div className="h-full overflow-y-auto bg-gray-50 dark:bg-gray-900">
      <div className="container mx-auto px-6 py-6 max-w-7xl">
        {/* Header */}
        <div className="mb-6">
          <button
            onClick={() => navigate('/deployments')}
            className="mb-4 text-sm text-blue-600 dark:text-blue-400 hover:underline"
          >
            ← Back to Deployments
          </button>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white mb-2">
            Deployment: {deployment.name}
          </h1>
          <p className="text-sm text-gray-600 dark:text-gray-400">
            Namespace: {deployment.namespace} • Cluster: {deployment.clusterId}
          </p>
        </div>

        {/* Overview Section */}
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 mb-6">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Overview</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            <div>
              <p className="text-sm font-medium text-gray-500 dark:text-gray-400">Name</p>
              <p className="text-base text-gray-900 dark:text-white mt-1">{deployment.name}</p>
            </div>
            <div>
              <p className="text-sm font-medium text-gray-500 dark:text-gray-400">Namespace</p>
              <p className="text-base text-gray-900 dark:text-white mt-1">{deployment.namespace}</p>
            </div>
            <div>
              <p className="text-sm font-medium text-gray-500 dark:text-gray-400">Cluster</p>
              <p className="text-base text-gray-900 dark:text-white mt-1">{deployment.clusterId}</p>
            </div>
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
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 mb-6">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Replica Status</h2>
          <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
            <div className="text-center p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
              <p className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">Desired</p>
              <p className="text-2xl font-bold text-gray-900 dark:text-white">{deployment.replicasDesired}</p>
            </div>
            <div className="text-center p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
              <p className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">Ready</p>
              <div className="flex items-center justify-center gap-2">
                <span className={`h-3 w-3 rounded-full ${getReplicaStatusColor(deployment.replicasReady, deployment.replicasDesired)}`}></span>
                <p className="text-2xl font-bold text-gray-900 dark:text-white">{deployment.replicasReady}</p>
              </div>
            </div>
            <div className="text-center p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
              <p className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">Available</p>
              <p className="text-2xl font-bold text-gray-900 dark:text-white">{deployment.replicasAvailable}</p>
            </div>
            <div className="text-center p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
              <p className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">Updated</p>
              <p className="text-2xl font-bold text-gray-900 dark:text-white">{deployment.replicasUpdated}</p>
            </div>
            <div className="text-center p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
              <p className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">Unavailable</p>
              <p className="text-2xl font-bold text-gray-900 dark:text-white">{deployment.replicasUnavailable}</p>
            </div>
          </div>
        </div>

        {/* Containers */}
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 mb-6">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            Containers ({deployment.containers?.length || 0})
          </h2>
          {deployment.containers && deployment.containers.length > 0 ? (
            <div className="space-y-4">
              {deployment.containers.map((container, idx) => (
                <div key={idx} className="border border-gray-200 dark:border-gray-700 rounded-lg p-4">
                  <div className="flex items-start justify-between mb-3">
                    <div>
                      <h3 className="text-base font-semibold text-gray-900 dark:text-white">{container.name}</h3>
                      <p className="text-sm text-gray-600 dark:text-gray-400 font-mono mt-1">{container.image}</p>
                    </div>
                  </div>
                  <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
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
          ) : (
            <p className="text-sm text-gray-500 dark:text-gray-400">No containers information available</p>
          )}
        </div>

        {/* Conditions */}
        {deployment.conditions && deployment.conditions.length > 0 && (
          <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 mb-6">
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Conditions</h2>
            <div className="space-y-3">
              {deployment.conditions.map((condition, idx) => (
                <div key={idx} className="flex items-start justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                  <div className="flex-1">
                    <div className="flex items-center gap-2">
                      <p className="font-semibold text-gray-900 dark:text-white">{condition.type}</p>
                      <span className={`text-sm font-medium ${getConditionColor(condition.status)}`}>
                        {condition.status}
                      </span>
                    </div>
                    {condition.reason && (
                      <p className="text-sm text-gray-600 dark:text-gray-400 mt-1">Reason: {condition.reason}</p>
                    )}
                    {condition.message && (
                      <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">{condition.message}</p>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Selector */}
        {deployment.selector && Object.keys(deployment.selector).length > 0 && (
          <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 mb-6">
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Selector</h2>
            <div className="flex flex-wrap gap-2">
              {Object.entries(deployment.selector).map(([key, value]) => (
                <span
                  key={key}
                  className="inline-flex items-center px-3 py-1 rounded-full text-sm font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200"
                >
                  {key}: {value}
                </span>
              ))}
            </div>
          </div>
        )}

        {/* Labels */}
        {deployment.labels && Object.keys(deployment.labels).length > 0 && (
          <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 mb-6">
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Labels</h2>
            <div className="flex flex-wrap gap-2">
              {Object.entries(deployment.labels).map(([key, value]) => (
                <span
                  key={key}
                  className="inline-flex items-center px-3 py-1 rounded-full text-sm font-medium bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200"
                >
                  {key}: {value}
                </span>
              ))}
            </div>
          </div>
        )}

        {/* Annotations */}
        {deployment.annotations && Object.keys(deployment.annotations).length > 0 && (
          <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6">
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Annotations</h2>
            <div className="space-y-2">
              {Object.entries(deployment.annotations).map(([key, value]) => (
                <div key={key} className="flex flex-col gap-1 p-2 bg-gray-50 dark:bg-gray-700 rounded">
                  <span className="text-sm font-medium text-gray-700 dark:text-gray-300">{key}</span>
                  <span className="text-sm text-gray-600 dark:text-gray-400 break-all">{value}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

export default DeploymentDetail

