/**
 * ClusterStatusCard Component
 * Displays detailed cluster connection status and resource counts
 * K8s Fortuna Platform - Phase 2
 */

import React from 'react'
import { type ClusterStats } from '../../services/api'
import { Activity, AlertCircle, CheckCircle, WifiOff } from 'lucide-react'

interface ClusterStatusCardProps {
  cluster: ClusterStats
}

const ClusterStatusCard: React.FC<ClusterStatusCardProps> = ({ cluster }) => {
  const getStatusIcon = () => {
    switch (cluster.connectionStatus) {
      case 'connected':
        return <CheckCircle className="w-5 h-5 text-green-500" />
      case 'degraded':
        return <AlertCircle className="w-5 h-5 text-yellow-500" />
      case 'disconnected':
        return <WifiOff className="w-5 h-5 text-red-500" />
      default:
        return <Activity className="w-5 h-5 text-gray-400" />
    }
  }

  const getStatusBadge = () => {
    const statusMap = {
      connected: {
        bg: 'bg-green-100 dark:bg-green-900',
        text: 'text-green-800 dark:text-green-200',
        label: 'Connected',
      },
      degraded: {
        bg: 'bg-yellow-100 dark:bg-yellow-900',
        text: 'text-yellow-800 dark:text-yellow-200',
        label: 'Degraded',
      },
      disconnected: {
        bg: 'bg-red-100 dark:bg-red-900',
        text: 'text-red-800 dark:text-red-200',
        label: 'Disconnected',
      },
      unknown: {
        bg: 'bg-gray-100 dark:bg-gray-700',
        text: 'text-gray-800 dark:text-gray-200',
        label: 'Unknown',
      },
    }

    const status = statusMap[cluster.connectionStatus] || statusMap.unknown

    return (
      <span className={`px-2 py-1 rounded text-xs font-medium ${status.bg} ${status.text}`}>
        {status.label}
      </span>
    )
  }

  const formatTimeSince = (dateString: string | undefined) => {
    if (!dateString) return 'Never'
    const date = new Date(dateString)
    const now = new Date()
    const diffMs = now.getTime() - date.getTime()
    const diffMins = Math.floor(diffMs / 60000)

    if (diffMins < 1) return 'Just now'
    if (diffMins < 60) return `${diffMins}m ago`
    const diffHours = Math.floor(diffMins / 60)
    if (diffHours < 24) return `${diffHours}h ago`
    const diffDays = Math.floor(diffHours / 24)
    return `${diffDays}d ago`
  }

  return (
    <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4 hover:shadow-md transition-shadow">
      {/* Header */}
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-2">
          {getStatusIcon()}
          <div>
            <h3 className="font-semibold text-gray-900 dark:text-white">{cluster.name}</h3>
            <p className="text-xs text-gray-500 dark:text-gray-400">{cluster.id}</p>
          </div>
        </div>
        {getStatusBadge()}
      </div>

      {/* Metrics Grid */}
      <div className="grid grid-cols-3 gap-2 mb-3">
        <div className="bg-gray-50 dark:bg-gray-700 rounded p-2">
          <div className="text-xs text-gray-500 dark:text-gray-400">SAs</div>
          <div className="text-lg font-semibold text-gray-900 dark:text-white">
            {cluster.serviceAccountCount}
          </div>
        </div>
        <div className="bg-gray-50 dark:bg-gray-700 rounded p-2">
          <div className="text-xs text-gray-500 dark:text-gray-400">Roles</div>
          <div className="text-lg font-semibold text-gray-900 dark:text-white">
            {cluster.roleCount + cluster.clusterRoleCount}
          </div>
        </div>
        <div className="bg-gray-50 dark:bg-gray-700 rounded p-2">
          <div className="text-xs text-gray-500 dark:text-gray-400">Pods</div>
          <div className="text-lg font-semibold text-gray-900 dark:text-white">
            {cluster.podCount}
          </div>
        </div>
      </div>

      {/* Footer */}
      <div className="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
        <span>Last sync: {formatTimeSince(cluster.lastSync)}</span>
        {cluster.agentVersion && (
          <span className="font-mono">{cluster.agentVersion}</span>
        )}
      </div>
    </div>
  )
}

export default ClusterStatusCard

