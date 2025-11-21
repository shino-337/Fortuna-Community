import { useQuery } from '@tanstack/react-query'
import { graphApi, type GraphData } from '../services/api'

interface GraphParams {
  cluster?: string
  namespace?: string
}

export const useGraph = (params?: GraphParams) => {
  const queryKey = ['graph', params?.cluster || '', params?.namespace || '']
  
  console.log('🔍 [useGraph] Hook called with params:', {
    cluster: params?.cluster,
    namespace: params?.namespace,
    queryKey,
    timestamp: new Date().toISOString()
  })
  
  return useQuery<GraphData>({
    queryKey, // Explicit keys - use empty string instead of undefined for consistent cache keys
    queryFn: async () => {
      console.log('📡 [useGraph] Fetching graph data with params:', params)
      const response = await graphApi.getGraph(params)
      console.log('✅ [useGraph] Graph data received:', {
        nodesCount: response.data?.nodes?.length || 0,
        edgesCount: response.data?.edges?.length || 0,
        timestamp: new Date().toISOString()
      })
      return response.data
    },
    // Refetch when params change to ensure fresh data
    refetchOnMount: true,
    refetchOnWindowFocus: false, // Don't refetch on window focus to avoid unnecessary requests
    // Don't cache too long to ensure filter changes are reflected
    staleTime: 0,
    // Ensure we don't keep previous data when filters change
    placeholderData: undefined,
    // Force refetch when query key changes
    enabled: true,
  })
}

