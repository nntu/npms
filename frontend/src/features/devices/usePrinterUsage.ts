import { useQuery } from '@tanstack/react-query'
import { listPrinterUsage } from '../../api/client'

export function usePrinterUsage(id: string) {
  return useQuery({ queryKey: ['printer-usage', id], queryFn: () => listPrinterUsage(id), staleTime: 30_000 })
}
