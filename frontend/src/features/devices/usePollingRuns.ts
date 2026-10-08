import { useQuery } from '@tanstack/react-query'
import { listPollingRuns } from '../../api/client'

export function usePollingRuns(id: string) {
  return useQuery({
    queryKey: ['printer-polling-runs', id],
    queryFn: () => listPollingRuns(id),
    staleTime: 15_000,
  })
}
