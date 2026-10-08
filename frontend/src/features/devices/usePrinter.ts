import { useQuery } from '@tanstack/react-query'
import { getPrinter } from '../../api/client'

export function usePrinter(id: string | null) {
  return useQuery({
    queryKey: ['printer', id],
    queryFn: () => getPrinter(id as string),
    enabled: id !== null,
    staleTime: 30_000,
  })
}
