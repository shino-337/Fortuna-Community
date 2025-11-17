import { useQuery } from '@tanstack/react-query'
import { graphApi, type GraphData } from '../services/api'

interface GraphParams {
  cluster?: string
  namespace?: string
}

export const useGraph = (params?: GraphParams) => {
  return useQuery<GraphData>({
    queryKey: ['graph', params],
    queryFn: async () => {
      const response = await graphApi.getGraph(params)
      return response.data
    },
    // Refetch when params change to ensure fresh data
    refetchOnMount: true,
    // Don't cache too long to ensure filter changes are reflected
    staleTime: 0,
  })
}

