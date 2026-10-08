import { useQuery } from '@tanstack/react-query'
import { listProfiles } from '../../api/client'

export function useProfiles() {
  return useQuery({ queryKey: ['profiles'], queryFn: listProfiles, staleTime: 60_000 })
}
