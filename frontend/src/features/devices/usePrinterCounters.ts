import { useQuery } from '@tanstack/react-query'
import { listPrinterCounters } from '../../api/client'

export function usePrinterCounters(id: string) {
  return useQuery({
    queryKey: ['printer-counters', id],
    queryFn: () => listPrinterCounters(id),
    staleTime: 30_000,
  })
}
