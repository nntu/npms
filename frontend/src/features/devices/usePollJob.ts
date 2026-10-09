import { useQuery } from '@tanstack/react-query'
import { getJob } from '../../api/client'

export function usePollJob(id: string | null) {
  return useQuery({
    queryKey: ['job', id],
    queryFn: () => getJob(id as string),
    enabled: id !== null,
    refetchInterval: (query) =>
      query.state.data?.status === 'success' || query.state.data?.status === 'failed' ? false : 1000,
  })
}
