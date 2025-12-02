import { useClusterStats } from '../hooks/useClusters'
import { useServiceAccounts } from '../hooks/useServiceAccounts'
import { useAuditLogs } from '../hooks/useAuditLogs'
import ClusterStatusCard from '../components/Dashboard/ClusterStatusCard'

const Dashboard = () => {
  const { data: clusterStats, isLoading: clustersLoading } = useClusterStats()
  const { data: serviceAccounts, isLoading: sasLoading } = useServiceAccounts()
  const { data: auditLogs, isLoading: logsLoading } = useAuditLogs({ page: 1, pageSize: 10 })

  const totalClusters = clusterStats?.length || 0
  const connectedClusters = clusterStats?.filter(c => c.connectionStatus === 'connected').length || 0
  const totalServiceAccounts = serviceAccounts?.serviceAccounts?.length || 0
  const recentLogs = auditLogs?.logs?.length || 0

  return (
    <div className="h-full overflow-y-auto bg-gray-50 dark:bg-gray-900">
      <div className="container mx-auto px-6 py-6 max-w-7xl">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white mb-2">Dashboard</h1>
          <p className="text-sm text-gray-600 dark:text-gray-400">Overview of your Kubernetes ServiceAccount management</p>
        </div>
        
        {clustersLoading || sasLoading || logsLoading ? (
          <div className="text-center py-8">
            <div className="text-gray-500 dark:text-gray-400">Loading...</div>
          </div>
        ) : (
          <>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
              <div className="bg-white dark:bg-gray-800 p-6 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700">
                <h2 className="text-lg font-semibold mb-2 text-gray-700 dark:text-gray-300">Clusters</h2>
                <p className="text-3xl font-bold text-pink-600 dark:text-pink-400">{totalClusters}</p>
                <p className="text-sm text-gray-500 dark:text-gray-400 mt-2">
                  <span className="text-green-600 dark:text-green-400 font-semibold">{connectedClusters} connected</span>
                  {' '}• {totalClusters - connectedClusters} offline
                </p>
              </div>
              
              <div className="bg-white dark:bg-gray-800 p-6 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700">
                <h2 className="text-lg font-semibold mb-2 text-gray-700 dark:text-gray-300">ServiceAccounts</h2>
                <p className="text-3xl font-bold text-green-600 dark:text-green-400">{totalServiceAccounts}</p>
                <p className="text-sm text-gray-500 dark:text-gray-400 mt-2">Total service accounts</p>
              </div>
              
              <div className="bg-white dark:bg-gray-800 p-6 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700">
                <h2 className="text-lg font-semibold mb-2 text-gray-700 dark:text-gray-300">Recent Activity</h2>
                <p className="text-3xl font-bold text-purple-600 dark:text-purple-400">{recentLogs}</p>
                <p className="text-sm text-gray-500 dark:text-gray-400 mt-2">Recent audit logs</p>
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div className="bg-white dark:bg-gray-800 p-6 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700">
                <h2 className="text-lg font-semibold mb-4 text-gray-800 dark:text-gray-200">Clusters Status</h2>
                {clusterStats && Array.isArray(clusterStats) && clusterStats.length > 0 ? (
                  <div className="space-y-3 max-h-96 overflow-y-auto">
                    {clusterStats.map((cluster) => (
                      <ClusterStatusCard key={cluster.id} cluster={cluster} />
                    ))}
                  </div>
                ) : (
                  <div className="text-center py-8">
                    <p className="text-gray-500 dark:text-gray-400 mb-2">No clusters connected</p>
                    <p className="text-sm text-gray-400 dark:text-gray-500">
                      Deploy the KSAM agent to your clusters to start monitoring
                    </p>
                  </div>
                )}
              </div>

              <div className="bg-white dark:bg-gray-800 p-6 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700">
                <h2 className="text-lg font-semibold mb-4 text-gray-800 dark:text-gray-200">Recent Audit Logs</h2>
                {auditLogs && auditLogs.logs && auditLogs.logs.length > 0 ? (
                  <div className="space-y-2">
                    {auditLogs.logs.slice(0, 5).map((log: any) => (
                      <div key={log.id} className="p-2 bg-gray-50 dark:bg-gray-700 rounded text-sm">
                        <div className="flex justify-between">
                          <span className="font-medium text-gray-900 dark:text-gray-100">{log.action}</span>
                          <span className="text-gray-500 dark:text-gray-400">{log.resource}</span>
                        </div>
                        <div className="text-gray-500 dark:text-gray-400 text-xs mt-1">
                          {new Date(log.createdAt).toLocaleString()}
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-gray-500 dark:text-gray-400">No recent activity</p>
                )}
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  )
}

export default Dashboard
