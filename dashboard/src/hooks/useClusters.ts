import { useQuery } from '@tanstack/react-query'
import { clustersApi, type Cluster, type ClusterStats } from '../services/api'

export const useClusters = () => {
  return useQuery<Cluster[]>({
    queryKey: ['clusters'],
    queryFn: async () => {
      const response = await clustersApi.getAll()
      // Ensure response is always an array
      const data = response.data
      if (data && 'clusters' in data) {
        return data.clusters || []
      }
      return Array.isArray(data) ? data : []
    },
  })
}

export const useClusterStats = () => {
  return useQuery<ClusterStats[]>({
    queryKey: ['clusters', 'stats'],
    queryFn: async () => {
      const response = await clustersApi.getStats()
      return response.data.clusters || []
    },
    refetchInterval: 30000, // Refetch every 30 seconds for real-time status
  })
}

export const useCluster = (id: string) => {
  return useQuery<Cluster>({
    queryKey: ['cluster', id],
    queryFn: async () => {
      const response = await clustersApi.getById(id)
      return response.data
    },
    enabled: !!id,
  })
}

