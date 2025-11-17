import { useQuery } from '@tanstack/react-query'
import { clustersApi, type Cluster } from '../services/api'

export const useClusters = () => {
  return useQuery<Cluster[]>({
    queryKey: ['clusters'],
    queryFn: async () => {
      const response = await clustersApi.getAll()
      // Ensure response is always an array
      const data = response.data
      return Array.isArray(data) ? data : []
    },
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

