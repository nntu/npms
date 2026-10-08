import { useQuery } from '@tanstack/react-query'
import { listPrinters } from '../../api/client'

export function usePrinters() {
  return useQuery({
    queryKey: ['printers'],
    queryFn: () => listPrinters(),
    staleTime: 30_000,
  })
}
