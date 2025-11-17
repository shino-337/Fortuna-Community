import { useQuery } from '@tanstack/react-query'
import { auditApi } from '../services/api'

interface AuditLogsParams {
  cluster?: string
  resource?: string
  action?: string
  page?: number
  pageSize?: number
}

export const useAuditLogs = (params?: AuditLogsParams) => {
  return useQuery({
    queryKey: ['auditLogs', params],
    queryFn: async () => {
      const response = await auditApi.getLogs(params)
      return response.data
    },
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

