import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createDepartment, deleteDepartment, listDepartments, updateDepartment } from '../../api/client'
import type { CreateDepartmentInput, UpdateDepartmentInput } from '../../api/types'

export function useDepartments() {
  return useQuery({
    queryKey: ['departments'],
    queryFn: () => listDepartments(),
    staleTime: 30_000,
  })
}

export function useCreateDepartment() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateDepartmentInput) => createDepartment(input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['departments'] })
      queryClient.invalidateQueries({ queryKey: ['printers'] })
      queryClient.invalidateQueries({ queryKey: ['page-usage-report'] })
    },
  })
}

export function useUpdateDepartment() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateDepartmentInput }) => updateDepartment(id, input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['departments'] })
      queryClient.invalidateQueries({ queryKey: ['printers'] })
      queryClient.invalidateQueries({ queryKey: ['page-usage-report'] })
    },
  })
}

export function useDeleteDepartment() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => deleteDepartment(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['departments'] })
      queryClient.invalidateQueries({ queryKey: ['printers'] })
      queryClient.invalidateQueries({ queryKey: ['page-usage-report'] })
    },
  })
}
