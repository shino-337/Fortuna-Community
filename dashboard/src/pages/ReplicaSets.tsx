import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { replicasetsApi, clustersApi, type ReplicaSet } from '../services/api'
import ReplicaSetDetailModal from '../components/ReplicaSetDetailModal'


export default function ReplicaSets() {
  const [selectedCluster, setSelectedCluster] = useState('')
  const [selectedNamespace, setSelectedNamespace] = useState('')
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [selectedReplicaSetId, setSelectedReplicaSetId] = useState<string | null>(null)

  // Fetch clusters for filter
  const { data: clustersData } = useQuery({
    queryKey: ['clusters'],
    queryFn: async () => {
      const response = await clustersApi.getAll()
      return response.data.clusters
    },
  })

  // Fetch replicasets
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['replicasets', selectedCluster, selectedNamespace, page],
    queryFn: async () => {
      const response = await replicasetsApi.getAll({
        cluster: selectedCluster || undefined,
        namespace: selectedNamespace || undefined,
        page,
        pageSize: 20,
      })
      return response.data
    },
    refetchInterval: 30000, // Refresh every 30 seconds
  })

  // Extract unique namespaces
  const namespaces = Array.from(
    new Set(data?.replicasets.map((rs) => rs.namespace) || [])
  ).sort()

  // Filter replicasets by search
  const filteredReplicaSets = data?.replicasets.filter((rs) => {
    if (!search) return true
    const searchLower = search.toLowerCase()
    return (
      rs.name.toLowerCase().includes(searchLower) ||
      rs.namespace.toLowerCase().includes(searchLower) ||
      (rs.ownerName && rs.ownerName.toLowerCase().includes(searchLower))
    )
  }) || []

  const getImages = (replicaset: ReplicaSet): string[] => {
    if (!replicaset.containers || replicaset.containers.length === 0) return []
    return replicaset.containers.map((c) => c.image)
  }

  return (
    <div className="h-full overflow-y-auto bg-gray-50 dark:bg-gray-900">
      <div className="container mx-auto px-6 py-6 max-w-7xl">
        {/* Header */}
        <div className="flex justify-between items-center mb-6">
          <div>
            <h1 className="text-3xl font-bold text-gray-900 dark:text-white">
              ReplicaSets
            </h1>
            <p className="text-sm text-gray-600 dark:text-gray-400 mt-1">
              Monitor and manage Kubernetes ReplicaSets across clusters
            </p>
          </div>
          <button
            onClick={() => refetch()}
            className="px-4 py-2 bg-pink-600 hover:bg-pink-700 text-white rounded-lg transition-colors"
          >
            🔄 Refresh
          </button>
        </div>

        {/* Filters */}
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm p-4 mb-6">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {/* Cluster Filter */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Cluster
              </label>
              <select
                value={selectedCluster}
                onChange={(e) => {
                  setSelectedCluster(e.target.value)
                  setPage(1)
                }}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-pink-500"
              >
                <option value="">All Clusters</option>
                {clustersData?.map((cluster) => (
                  <option key={cluster.id} value={cluster.id}>
                    {cluster.name}
                  </option>
                ))}
              </select>
            </div>

            {/* Namespace Filter */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Namespace
              </label>
              <select
                value={selectedNamespace}
                onChange={(e) => {
                  setSelectedNamespace(e.target.value)
                  setPage(1)
                }}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-pink-500"
              >
                <option value="">All Namespaces</option>
                {namespaces.map((ns) => (
                  <option key={ns} value={ns}>
                    {ns}
                  </option>
                ))}
              </select>
            </div>

            {/* Search */}
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Search
              </label>
              <input
                type="text"
                placeholder="Search by name..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-pink-500"
              />
            </div>
          </div>
        </div>

        {/* Stats */}
        {data && (
          <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
            <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm p-4">
              <div className="text-sm text-gray-600 dark:text-gray-400">Total</div>
              <div className="text-2xl font-bold text-gray-900 dark:text-white">
                {data.total}
              </div>
            </div>
            <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm p-4">
              <div className="text-sm text-gray-600 dark:text-gray-400">Filtered</div>
              <div className="text-2xl font-bold text-gray-900 dark:text-white">
                {filteredReplicaSets.length}
              </div>
            </div>
            <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm p-4">
              <div className="text-sm text-gray-600 dark:text-gray-400">Clusters</div>
              <div className="text-2xl font-bold text-gray-900 dark:text-white">
                {clustersData?.length || 0}
              </div>
            </div>
            <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm p-4">
              <div className="text-sm text-gray-600 dark:text-gray-400">Namespaces</div>
              <div className="text-2xl font-bold text-gray-900 dark:text-white">
                {namespaces.length}
              </div>
            </div>
          </div>
        )}

        {/* Table */}
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm overflow-hidden">
          {isLoading ? (
            <div className="p-12 text-center">
              <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-pink-600 mx-auto"></div>
              <p className="text-gray-600 dark:text-gray-400 mt-4">Loading replicasets...</p>
            </div>
          ) : error ? (
            <div className="p-12 text-center">
              <p className="text-red-600 dark:text-red-400">Error loading replicasets</p>
            </div>
          ) : filteredReplicaSets.length === 0 ? (
            <div className="p-12 text-center">
              <p className="text-gray-600 dark:text-gray-400">No replicasets found</p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead className="bg-gray-50 dark:bg-gray-700">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                      Name
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                      Namespace
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                      Replicas
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                      Owner
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                      Images
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                      Age
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                  {filteredReplicaSets.map((replicaset) => {
                    const images = getImages(replicaset)
                    const age = Math.floor(
                      (Date.now() - new Date(replicaset.createdAt).getTime()) / (1000 * 60 * 60 * 24)
                    )
                    const replicaStatus = `${replicaset.readyReplicas}/${replicaset.replicas}`
                    const isHealthy = replicaset.readyReplicas === replicaset.replicas

                    return (
                      <tr
                        key={replicaset.id}
                        className="hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
                      >
                        <td className="px-6 py-4 whitespace-nowrap">
                          <button 
                            onClick={() => setSelectedReplicaSetId(String(replicaset.id))}
                            className="group text-left w-full"
                          >
                            <div className="text-sm font-medium text-blue-600 dark:text-blue-400 group-hover:underline cursor-pointer">
                              {replicaset.name}
                            </div>
                            <div className="text-xs text-gray-500 dark:text-gray-400">
                              {replicaset.clusterId}
                            </div>
                          </button>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <span className="px-2 py-1 text-xs font-medium rounded-full bg-purple-100 dark:bg-purple-900 text-purple-800 dark:text-purple-200">
                            {replicaset.namespace}
                          </span>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <span
                            className={`px-2 py-1 text-xs font-medium rounded-full ${
                              isHealthy
                                ? 'bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200'
                                : 'bg-yellow-100 dark:bg-yellow-900 text-yellow-800 dark:text-yellow-200'
                            }`}
                          >
                            {replicaStatus}
                          </span>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-gray-300">
                          {replicaset.ownerKind && replicaset.ownerName ? (
                            <span className="text-xs">
                              {replicaset.ownerKind}: {replicaset.ownerName}
                            </span>
                          ) : (
                            <span className="text-gray-400">-</span>
                          )}
                        </td>
                        <td className="px-6 py-4">
                          <div className="text-xs text-gray-600 dark:text-gray-400 space-y-1 max-w-md">
                            {images.map((img, idx) => (
                              <div key={idx} className="truncate" title={img}>
                                {img}
                              </div>
                            ))}
                          </div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                          {age}d
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Pagination */}
        {data && data.total > data.pageSize && (
          <div className="mt-6 flex justify-center gap-2">
            <button
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page === 1}
              className="px-4 py-2 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-lg text-gray-700 dark:text-gray-300 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Previous
            </button>
            <span className="px-4 py-2 text-gray-700 dark:text-gray-300">
              Page {page} of {Math.ceil(data.total / data.pageSize)}
            </span>
            <button
              onClick={() => setPage((p) => p + 1)}
              disabled={page >= Math.ceil(data.total / data.pageSize)}
              className="px-4 py-2 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-lg text-gray-700 dark:text-gray-300 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Next
            </button>
          </div>
        )}
      </div>

      {/* ReplicaSet Detail Modal */}
      {selectedReplicaSetId && (
        <ReplicaSetDetailModal
          replicaSetId={selectedReplicaSetId}
          onClose={() => setSelectedReplicaSetId(null)}
        />
      )}
    </div>
  )
}

