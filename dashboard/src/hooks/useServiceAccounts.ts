import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { serviceAccountsApi, type ServiceAccount } from '../services/api'

interface ServiceAccountsParams {
  cluster?: string
  namespace?: string
  page?: number
  pageSize?: number
}

export interface UseServiceAccountsOptions {
  refetchInterval?: number | false
  refetchOnWindowFocus?: boolean
  refetchOnMount?: boolean
  refetchOnReconnect?: boolean
  enabled?: boolean
}

export const useServiceAccounts = (
  params?: ServiceAccountsParams,
  options?: UseServiceAccountsOptions
) => {
  return useQuery({
    queryKey: ['serviceAccounts', params],
    queryFn: async () => {
      const response = await serviceAccountsApi.getAll(params)
      return response.data
    },
    // Auto-refresh every 5 seconds to get real-time updates (configurable)
    refetchInterval: options?.refetchInterval ?? 5000,
    // Also refetch when window regains focus (configurable)
    refetchOnWindowFocus: options?.refetchOnWindowFocus ?? true,
    // Refetch on mount (configurable)
    refetchOnMount: options?.refetchOnMount ?? true,
    // Refetch on reconnect (configurable)
    refetchOnReconnect: options?.refetchOnReconnect ?? true,
    enabled: options?.enabled ?? true,
  })
}

export const useServiceAccount = (id: string) => {
  return useQuery({
    queryKey: ['serviceAccount', id],
    queryFn: async () => {
      const response = await serviceAccountsApi.getById(id)
      return response.data
    },
    enabled: !!id,
  })
}

export const useUpdateServiceAccount = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<ServiceAccount> }) =>
      serviceAccountsApi.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['serviceAccounts'] })
    },
  })
}

export const useDeleteServiceAccount = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => serviceAccountsApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['serviceAccounts'] })
    },
  })
}

