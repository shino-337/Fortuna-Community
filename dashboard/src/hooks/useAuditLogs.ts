import { useQuery } from '@tanstack/react-query'
import { auditApi, type AuditLog } from '../services/api'

interface AuditLogsParams {
  cluster?: string
  resource?: string
  action?: string
  page?: number
  pageSize?: number
}

interface AuditLogsResponse {
  logs: AuditLog[]
  total: number
  page: number
  pageSize: number
}

export const useAuditLogs = (
  params?: AuditLogsParams,
  options?: { refetchInterval?: number }
) => {
  return useQuery<AuditLogsResponse>({
    queryKey: ['auditLogs', params],
    queryFn: async () => {
      const response = await auditApi.getLogs(params)
      return response.data
    },
    refetchInterval: options?.refetchInterval ?? false,
  })
}

export const useAuditReports = () => {
  return useQuery({
    queryKey: ['auditReports'],
    queryFn: async () => {
      const response = await auditApi.getReports()
      return response.data
    },
  })
}

