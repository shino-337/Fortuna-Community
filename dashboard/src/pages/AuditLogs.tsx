import { useState, useCallback, useMemo } from 'react'
import { useAuditLogs } from '../hooks/useAuditLogs'
import { useClusters } from '../hooks/useClusters'
import type { AuditLog } from '../services/api'

const AuditLogs = () => {
  const [cluster, setCluster] = useState<string>('')
  const [resource, setResource] = useState<string>('')
  const [action, setAction] = useState<string>('')
  const [page, setPage] = useState(1)
  const pageSize = 20

  // Separate filter state from applied filters
  const [clusterFilter, setClusterFilter] = useState<string>('')
  const [resourceFilter, setResourceFilter] = useState<string>('')
  const [actionFilter, setActionFilter] = useState<string>('')

  // Memoize query params to prevent unnecessary refetches
  const queryParams = useMemo(
    () => ({
      cluster: clusterFilter || undefined,
      resource: resourceFilter || undefined,
      action: actionFilter || undefined,
      page,
      pageSize,
    }),
    [clusterFilter, resourceFilter, actionFilter, page, pageSize]
  )

  const { data, isLoading, error } = useAuditLogs(queryParams)
  const { data: clusters } = useClusters()

  const isFilterDirty = 
    cluster !== clusterFilter || 
    resource !== resourceFilter || 
    action !== actionFilter

  const applyFilters = useCallback(() => {
    setClusterFilter(cluster)
    setResourceFilter(resource)
    setActionFilter(action)
    setPage(1)
  }, [cluster, resource, action])

  const handleClearFilters = useCallback(() => {
    setCluster('')
    setResource('')
    setAction('')
    setClusterFilter('')
    setResourceFilter('')
    setActionFilter('')
    setPage(1)
  }, [])

  if (isLoading) {
    return (
      <div className="min-h-screen bg-gray-50">
        <div className="container mx-auto px-4 py-6 max-w-7xl">
          <div className="text-center py-8">
            <div className="text-gray-500">Loading...</div>
          </div>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="min-h-screen bg-gray-50">
        <div className="container mx-auto px-4 py-6 max-w-7xl">
          <div className="text-center py-8">
            <div className="text-red-500">Error loading audit logs: {String(error)}</div>
          </div>
        </div>
      </div>
    )
  }

  const logs = data?.logs || []
  const total = data?.total || 0
  const totalPages = Math.ceil(total / pageSize)

  return (
    <div className="min-h-screen bg-gray-50">
      <div className="container mx-auto px-4 py-6 max-w-7xl">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-gray-900 mb-2">Audit Logs</h1>
          <p className="text-sm text-gray-600">View and filter audit logs for ServiceAccount operations</p>
        </div>

        {/* Filters */}
        <div className="bg-white p-5 rounded-lg shadow-sm border border-gray-200 mb-6">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <label className="block text-sm font-semibold text-gray-700 mb-2">
                Cluster
              </label>
              <select
                value={cluster}
                onChange={(e) => setCluster(e.target.value)}
                className="w-full px-4 py-2.5 text-sm border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 transition-colors"
              >
                <option value="">All Clusters</option>
                {clusters && Array.isArray(clusters) && clusters.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-sm font-semibold text-gray-700 mb-2">
                Resource
              </label>
              <select
                value={resource}
                onChange={(e) => setResource(e.target.value)}
                className="w-full px-4 py-2.5 text-sm border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 transition-colors"
              >
                <option value="">All Resources</option>
                <option value="serviceaccount">ServiceAccount</option>
                <option value="rolebinding">RoleBinding</option>
                <option value="clusterrolebinding">ClusterRoleBinding</option>
                <option value="role">Role</option>
                <option value="clusterrole">ClusterRole</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-semibold text-gray-700 mb-2">
                Action
              </label>
              <select
                value={action}
                onChange={(e) => setAction(e.target.value)}
                className="w-full px-4 py-2.5 text-sm border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 transition-colors"
              >
                <option value="">All Actions</option>
                <option value="create">Create</option>
                <option value="update">Update</option>
                <option value="delete">Delete</option>
              </select>
            </div>
          </div>

          <div className="mt-4 flex flex-col md:flex-row md:items-center md:justify-between gap-3">
            <div className="text-xs text-gray-500">
              Apply to update the results. Filters will not trigger search until you click apply.
            </div>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={handleClearFilters}
                className="px-4 py-2 text-sm border border-gray-300 rounded-md text-gray-600 hover:text-gray-800 hover:bg-gray-50 transition-colors"
              >
                Clear Filters
              </button>
              <button
                type="button"
                onClick={applyFilters}
                disabled={!isFilterDirty}
                className="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md shadow-sm hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                Apply Filters
              </button>
            </div>
          </div>
        </div>

        {/* Audit Logs Table */}
        <div className="bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden">
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Timestamp
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Action
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Resource
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Cluster
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    User
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    IP Address
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Details
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {logs.length === 0 ? (
                  <tr>
                    <td colSpan={7} className="px-6 py-4 text-center text-gray-500">
                      No audit logs found
                    </td>
                  </tr>
                ) : (
                  logs.map((log: AuditLog) => {
                    const date = new Date(log.createdAt)
                    const formattedDate = date.toLocaleDateString('en-US', {
                      year: 'numeric',
                      month: 'short',
                      day: 'numeric',
                    })
                    const formattedTime = date.toLocaleTimeString('en-US', {
                      hour: '2-digit',
                      minute: '2-digit',
                      second: '2-digit',
                    })

                    return (
                      <tr key={log.id} className="hover:bg-gray-50">
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="text-sm text-gray-900">
                            <div className="font-medium">{formattedDate}</div>
                            <div className="text-xs text-gray-500">{formattedTime}</div>
                          </div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <span
                            className={`px-2 py-1 inline-flex text-xs leading-5 font-semibold rounded-full capitalize ${
                              log.action === 'create'
                                ? 'bg-green-100 text-green-800'
                                : log.action === 'update'
                                ? 'bg-yellow-100 text-yellow-800'
                                : 'bg-red-100 text-red-800'
                            }`}
                          >
                            {log.action}
                          </span>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="text-sm text-gray-900 capitalize">{log.resource}</div>
                          {log.resourceId && (
                            <div className="text-xs text-gray-500 font-mono">{log.resourceId}</div>
                          )}
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="text-sm text-gray-500">{log.clusterId || '-'}</div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="text-sm text-gray-500">{log.user || '-'}</div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="text-sm text-gray-500 font-mono">{log.ip || '-'}</div>
                        </td>
                        <td className="px-6 py-4">
                          {log.details ? (
                            <div className="text-sm text-gray-600 max-w-xs truncate" title={log.details}>
                              {log.details}
                            </div>
                          ) : (
                            <div className="text-sm text-gray-400">-</div>
                          )}
                        </td>
                      </tr>
                    )
                  })
                )}
              </tbody>
            </table>
          </div>

          {/* Pagination */}
          {totalPages > 1 && (
            <div className="bg-gray-50 px-6 py-3 flex items-center justify-between border-t border-gray-200">
              <div className="text-sm text-gray-700">
                Showing {((page - 1) * pageSize) + 1} to {Math.min(page * pageSize, total)} of {total} results
              </div>
              <div className="flex space-x-2">
                <button
                  onClick={() => setPage(page - 1)}
                  disabled={page === 1}
                  className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Previous
                </button>
                <button
                  onClick={() => setPage(page + 1)}
                  disabled={page >= totalPages}
                  className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Next
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

export default AuditLogs
