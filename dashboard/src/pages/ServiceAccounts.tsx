import { useState, useEffect, useCallback, useMemo } from 'react'
import { useServiceAccounts, useDeleteServiceAccount, type UseServiceAccountsOptions } from '../hooks/useServiceAccounts'
import { useClusters } from '../hooks/useClusters'
import { useServiceAccountPermissions } from '../hooks/useServiceAccountPermissions'
import type { ServiceAccount as ServiceAccountDto } from '../services/api'
import { RBACGuard } from '../components/RBACGuard'

const ServiceAccounts = () => {
  const [cluster, setCluster] = useState<string>('')
  const [namespaceFilter, setNamespaceFilter] = useState<string>('')
  const [namespaceInput, setNamespaceInput] = useState<string>('')
  const [page, setPage] = useState(1)
  const [selectedSAId, setSelectedSAId] = useState<string | null>(null)
  const pageSize = 20
  const trimmedNamespaceInput = namespaceInput.trim()
  const isNamespaceDirty = trimmedNamespaceInput !== namespaceFilter

  const queryParams = useMemo(
    () => ({
      cluster: cluster || undefined,
      namespace: namespaceFilter || undefined,
      page,
      pageSize,
    }),
    [cluster, namespaceFilter, page, pageSize]
  )

  const queryOptions = useMemo<UseServiceAccountsOptions>(
    () => ({
      refetchInterval: 5000,
      refetchOnWindowFocus: true,
      refetchOnMount: true,
      refetchOnReconnect: true,
      enabled: true,
    }),
    []
  )

  const { data, isLoading, error } = useServiceAccounts(queryParams, queryOptions)
  const { data: clusters } = useClusters()
  const deleteMutation = useDeleteServiceAccount()
  const { data: permissions, isLoading: isLoadingPermissions } = useServiceAccountPermissions(selectedSAId)

  const handleDelete = async (id: string) => {
    if (confirm('Are you sure you want to delete this ServiceAccount?')) {
      await deleteMutation.mutateAsync(id)
    }
  }

  const handleViewPermissions = (id: string) => {
    setSelectedSAId(id)
  }

  const applyNamespaceFilter = useCallback(() => {
    if (namespaceFilter === trimmedNamespaceInput) {
      return
    }
    setNamespaceFilter(trimmedNamespaceInput)
    setPage(1)
  }, [namespaceFilter, trimmedNamespaceInput])

  const handleClearFilters = useCallback(() => {
    setCluster('')
    setNamespaceFilter('')
    setNamespaceInput('')
    setPage(1)
  }, [])

  const closePermissionsModal = () => {
    setSelectedSAId(null)
  }

  useEffect(() => {
    setNamespaceInput(namespaceFilter)
  }, [namespaceFilter])

  if (isLoading) {
    return (
      <div className="h-full overflow-y-auto bg-gray-50 dark:bg-gray-900">
        <div className="container mx-auto px-6 py-6 max-w-7xl">
          <div className="text-center py-8">
            <div className="text-gray-500 dark:text-gray-400">Loading...</div>
          </div>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="h-full overflow-y-auto bg-gray-50 dark:bg-gray-900">
        <div className="container mx-auto px-6 py-6 max-w-7xl">
          <div className="text-center py-8">
            <div className="text-red-500 dark:text-red-400">Error loading service accounts: {String(error)}</div>
          </div>
        </div>
      </div>
    )
  }

  const serviceAccounts = data?.serviceAccounts || []
  const total = data?.total || 0
  const totalPages = Math.ceil(total / pageSize)

  return (
    <div className="h-full overflow-y-auto bg-gray-50 dark:bg-gray-900">
      <div className="container mx-auto px-6 py-6 max-w-7xl">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white mb-2">ServiceAccounts</h1>
          <p className="text-sm text-gray-600 dark:text-gray-400">Manage and view all ServiceAccounts across clusters</p>
        </div>

        {/* Filters */}
        <div className="bg-white dark:bg-gray-800 p-5 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 mb-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">
                Cluster
              </label>
              <select
                value={cluster}
                onChange={(e) => {
                  setCluster(e.target.value)
                  setPage(1)
                }}
                className="w-full px-4 py-2.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 transition-colors"
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
              <label className="block text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">
                Namespace
              </label>
              <input
                type="text"
                value={namespaceInput}
                onChange={(e) => {
                  setNamespaceInput(e.target.value)
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    applyNamespaceFilter()
                  }
                }}
                placeholder="Enter namespace (empty for all)..."
                className="w-full px-4 py-2.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 transition-colors"
              />
            </div>
          </div>

          <div className="mt-4 flex flex-col md:flex-row md:items-center md:justify-between gap-3">
            <div className="text-xs text-gray-500 dark:text-gray-400">
              Apply to update the results. Filters will not trigger search until you click apply.
            </div>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={handleClearFilters}
                className="px-4 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md text-gray-600 dark:text-gray-300 hover:text-gray-800 dark:hover:text-white hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
              >
                Clear Filters
              </button>
              <button
                type="button"
                onClick={applyNamespaceFilter}
                disabled={!isNamespaceDirty}
                className="px-4 py-2 text-sm font-medium text-white bg-blue-600 dark:bg-blue-500 rounded-md shadow-sm hover:bg-blue-700 dark:hover:bg-blue-600 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                Apply Filters
              </button>
            </div>
          </div>
        </div>

        {/* ServiceAccounts Table */}
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 overflow-hidden">
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
              <thead className="bg-gray-50 dark:bg-gray-700">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                    Name
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                    Namespace
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                    Cluster
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                    UID
                  </th>
                  <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                {serviceAccounts.length === 0 ? (
                  <tr>
                    <td colSpan={5} className="px-6 py-4 text-center text-gray-500 dark:text-gray-400">
                      No service accounts found
                    </td>
                  </tr>
                ) : (
                  serviceAccounts.map((sa: ServiceAccountDto) => (
                    <tr key={sa.id} className="hover:bg-gray-50 dark:hover:bg-gray-700">
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="text-sm font-medium text-gray-900 dark:text-white">{sa.name}</div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="text-sm text-gray-500 dark:text-gray-400">{sa.namespace}</div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="text-sm text-gray-500 dark:text-gray-400">{sa.clusterId}</div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="text-sm text-gray-500 dark:text-gray-400 font-mono text-xs">{sa.uid}</div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium space-x-2">
                        <button
                          onClick={() => handleViewPermissions(String(sa.id))}
                          className="text-blue-600 dark:text-blue-400 hover:text-blue-900 dark:hover:text-blue-300"
                        >
                          Permissions
                        </button>
                        <RBACGuard resource="serviceaccounts" action="delete">
                        <button
                          onClick={() => handleDelete(String(sa.id))}
                            className="text-red-600 dark:text-red-400 hover:text-red-900 dark:hover:text-red-300"
                        >
                          Delete
                        </button>
                        </RBACGuard>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>

          {/* Pagination */}
          {totalPages > 1 && (
            <div className="bg-gray-50 dark:bg-gray-700 px-6 py-3 flex items-center justify-between border-t border-gray-200 dark:border-gray-600">
              <div className="text-sm text-gray-700 dark:text-gray-300">
                Showing {((page - 1) * pageSize) + 1} to {Math.min(page * pageSize, total)} of {total} results
              </div>
              <div className="flex space-x-2">
                <button
                  onClick={() => setPage(page - 1)}
                  disabled={page === 1}
                  className="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Previous
                </button>
                <button
                  onClick={() => setPage(page + 1)}
                  disabled={page >= totalPages}
                  className="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Next
                </button>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Permissions Modal */}
      {selectedSAId && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50" onClick={closePermissionsModal}>
          <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-4xl w-full mx-4 max-h-[90vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
            <div className="p-6">
              <div className="flex justify-between items-center mb-4">
                <h2 className="text-2xl font-bold text-gray-900 dark:text-white">ServiceAccount Permissions</h2>
                <button
                  onClick={closePermissionsModal}
                  className="text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300"
                >
                  <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>

              {isLoadingPermissions ? (
                <div className="text-center py-8">
                  <div className="text-gray-500 dark:text-gray-400">Loading permissions...</div>
                </div>
              ) : permissions ? (
                <div className="space-y-6">
                  {/* RoleBindings */}
                  {permissions.roleBindings.length > 0 && (
                    <div>
                      <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-3">RoleBindings</h3>
                      <div className="space-y-4">
                        {permissions.roleBindings.map((rb, idx) => (
                          <div key={idx} className="border border-gray-200 dark:border-gray-700 rounded-lg p-4 bg-gray-50 dark:bg-gray-700">
                            <div className="mb-2">
                              <span className="font-medium text-gray-900 dark:text-white">RoleBinding: </span>
                              <span className="text-gray-700 dark:text-gray-300">{rb.roleBinding.name}</span>
                              <span className="text-gray-500 dark:text-gray-400 text-sm ml-2">({rb.roleBinding.namespace})</span>
                            </div>
                            <div className="mb-2">
                              <span className="font-medium text-gray-900 dark:text-white">Role: </span>
                              <span className="text-gray-700 dark:text-gray-300">{rb.role.name}</span>
                            </div>
                            {rb.role.rules && (
                              <div className="mt-2">
                                <div className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Rules:</div>
                                <pre className="bg-gray-100 dark:bg-gray-800 p-2 rounded text-xs overflow-x-auto text-gray-900 dark:text-gray-100">
                                  {rb.role.rules}
                                </pre>
                              </div>
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* ClusterRoleBindings */}
                  {permissions.clusterRoleBindings.length > 0 && (
                    <div>
                      <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-3">ClusterRoleBindings</h3>
                      <div className="space-y-4">
                        {permissions.clusterRoleBindings.map((crb, idx) => (
                          <div key={idx} className="border border-gray-200 dark:border-gray-700 rounded-lg p-4 bg-gray-50 dark:bg-gray-700">
                            <div className="mb-2">
                              <span className="font-medium text-gray-900 dark:text-white">ClusterRoleBinding: </span>
                              <span className="text-gray-700 dark:text-gray-300">{crb.clusterRoleBinding.name}</span>
                            </div>
                            <div className="mb-2">
                              <span className="font-medium text-gray-900 dark:text-white">ClusterRole: </span>
                              <span className="text-gray-700 dark:text-gray-300">{crb.clusterRole.name}</span>
                            </div>
                            {crb.clusterRole.rules && (
                              <div className="mt-2">
                                <div className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Rules:</div>
                                <pre className="bg-gray-100 dark:bg-gray-800 p-2 rounded text-xs overflow-x-auto text-gray-900 dark:text-gray-100">
                                  {crb.clusterRole.rules}
                                </pre>
                              </div>
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Effective Rules */}
                  {permissions.effectiveRules.length > 0 && (
                    <div>
                      <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-3">Effective Rules</h3>
                      <div className="space-y-4">
                        {permissions.effectiveRules.map((rule, idx) => (
                          <div key={idx} className="border border-gray-200 dark:border-gray-700 rounded-lg p-4 bg-gray-50 dark:bg-gray-700">
                            <div className="grid grid-cols-1 gap-2 text-sm">
                              {rule.verbs && rule.verbs.length > 0 && (
                                <div>
                                  <span className="font-medium text-gray-700 dark:text-gray-300">Verbs: </span>
                                  <span className="text-gray-900 dark:text-white">{rule.verbs.join(', ')}</span>
                                </div>
                              )}
                              {rule.apiGroups && rule.apiGroups.length > 0 && (
                                <div>
                                  <span className="font-medium text-gray-700 dark:text-gray-300">API Groups: </span>
                                  <span className="text-gray-900 dark:text-white">{rule.apiGroups.join(', ')}</span>
                                </div>
                              )}
                              {rule.resources && rule.resources.length > 0 && (
                                <div>
                                  <span className="font-medium text-gray-700 dark:text-gray-300">Resources: </span>
                                  <span className="text-gray-900 dark:text-white">{rule.resources.join(', ')}</span>
                                </div>
                              )}
                              {rule.resourceNames && rule.resourceNames.length > 0 && (
                                <div>
                                  <span className="font-medium text-gray-700 dark:text-gray-300">Resource Names: </span>
                                  <span className="text-gray-900 dark:text-white">{rule.resourceNames.join(', ')}</span>
                                </div>
                              )}
                              {rule.nonResourceURLs && rule.nonResourceURLs.length > 0 && (
                                <div>
                                  <span className="font-medium text-gray-700 dark:text-gray-300">Non-Resource URLs: </span>
                                  <span className="text-gray-900 dark:text-white">{rule.nonResourceURLs.join(', ')}</span>
                                </div>
                              )}
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {permissions.roleBindings.length === 0 && 
                   permissions.clusterRoleBindings.length === 0 && 
                   permissions.effectiveRules.length === 0 && (
                    <div className="text-center py-8 text-gray-500 dark:text-gray-400">
                      No permissions found for this ServiceAccount
                    </div>
                  )}
                </div>
              ) : (
                <div className="text-center py-8 text-red-500 dark:text-red-400">
                  Failed to load permissions
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default ServiceAccounts

