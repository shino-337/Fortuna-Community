import { useQuery } from '@tanstack/react-query'
import { serviceAccountsApi } from '../services/api'

export const useServiceAccountPermissions = (id: string | null) => {
  return useQuery({
    queryKey: ['serviceAccountPermissions', id],
    queryFn: () => {
      if (!id) throw new Error('ServiceAccount ID is required')
      return serviceAccountsApi.getPermissions(id).then((res) => res.data)
    },
    enabled: !!id,
  })
}

