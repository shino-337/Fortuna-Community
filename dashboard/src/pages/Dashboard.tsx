import { useClusters } from '../hooks/useClusters'
import { useServiceAccounts } from '../hooks/useServiceAccounts'
import { useAuditLogs } from '../hooks/useAuditLogs'

const Dashboard = () => {
  const { data: clusters, isLoading: clustersLoading } = useClusters()
  const { data: serviceAccounts, isLoading: sasLoading } = useServiceAccounts()
  const { data: auditLogs, isLoading: logsLoading } = useAuditLogs({ page: 1, pageSize: 10 })

  const totalClusters = clusters?.length || 0
  const totalServiceAccounts = serviceAccounts?.serviceAccounts?.length || 0
  const recentLogs = auditLogs?.logs?.length || 0

  return (
    <div className="min-h-screen bg-gray-50">
      <div className="container mx-auto px-4 py-6 max-w-7xl">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-gray-900 mb-2">KSAM Dashboard</h1>
          <p className="text-sm text-gray-600">Overview of your Kubernetes ServiceAccount management</p>
        </div>
        
        {clustersLoading || sasLoading || logsLoading ? (
          <div className="text-center py-8">
            <div className="text-gray-500">Loading...</div>
          </div>
        ) : (
          <>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
              <div className="bg-white p-6 rounded-lg shadow-sm border border-gray-200">
                <h2 className="text-lg font-semibold mb-2 text-gray-700">Clusters</h2>
                <p className="text-3xl font-bold text-blue-600">{totalClusters}</p>
                <p className="text-sm text-gray-500 mt-2">Total connected clusters</p>
              </div>
              
              <div className="bg-white p-6 rounded-lg shadow-sm border border-gray-200">
                <h2 className="text-lg font-semibold mb-2 text-gray-700">ServiceAccounts</h2>
                <p className="text-3xl font-bold text-green-600">{totalServiceAccounts}</p>
                <p className="text-sm text-gray-500 mt-2">Total service accounts</p>
              </div>
              
              <div className="bg-white p-6 rounded-lg shadow-sm border border-gray-200">
                <h2 className="text-lg font-semibold mb-2 text-gray-700">Recent Activity</h2>
                <p className="text-3xl font-bold text-purple-600">{recentLogs}</p>
                <p className="text-sm text-gray-500 mt-2">Recent audit logs</p>
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div className="bg-white p-6 rounded-lg shadow-sm border border-gray-200">
                <h2 className="text-lg font-semibold mb-4 text-gray-800">Clusters Status</h2>
                {clusters && Array.isArray(clusters) && clusters.length > 0 ? (
                  <div className="space-y-2">
                    {clusters.map((cluster) => (
                      <div key={cluster.id} className="flex items-center justify-between p-2 bg-gray-50 rounded">
                        <span className="font-medium">{cluster.name}</span>
                        <span
                          className={`px-2 py-1 rounded text-xs ${
                            cluster.status === 'active'
                              ? 'bg-green-100 text-green-800'
                              : 'bg-red-100 text-red-800'
                          }`}
                        >
                          {cluster.status}
                        </span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-gray-500">No clusters connected</p>
                )}
              </div>

              <div className="bg-white p-6 rounded-lg shadow-sm border border-gray-200">
                <h2 className="text-lg font-semibold mb-4 text-gray-800">Recent Audit Logs</h2>
                {auditLogs && auditLogs.logs && auditLogs.logs.length > 0 ? (
                  <div className="space-y-2">
                    {auditLogs.logs.slice(0, 5).map((log: any) => (
                      <div key={log.id} className="p-2 bg-gray-50 rounded text-sm">
                        <div className="flex justify-between">
                          <span className="font-medium">{log.action}</span>
                          <span className="text-gray-500">{log.resource}</span>
                        </div>
                        <div className="text-gray-500 text-xs mt-1">
                          {new Date(log.createdAt).toLocaleString()}
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-gray-500">No recent activity</p>
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
