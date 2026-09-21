import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  createMonitor,
  deleteMonitor,
  listMonitors,
  monitorKeys,
  pauseMonitor,
  resumeMonitor,
  updateMonitor,
  type CreateMonitorInput,
  type UpdateMonitorInput,
} from './api'

export function useMonitors() {
  return useQuery({
    queryKey: monitorKeys.list(1, 100),
    queryFn: () => listMonitors(1, 100),
  })
}

export function useCreateMonitor() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateMonitorInput) => createMonitor(input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: monitorKeys.all }),
  })
}

export function useUpdateMonitor() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateMonitorInput }) => updateMonitor(id, input),
    onSuccess: (_monitor, variables) => {
      queryClient.invalidateQueries({ queryKey: monitorKeys.all })
      queryClient.invalidateQueries({ queryKey: monitorKeys.detail(variables.id) })
    },
  })
}

export function usePauseMonitor() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: pauseMonitor,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: monitorKeys.all }),
  })
}

export function useResumeMonitor() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: resumeMonitor,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: monitorKeys.all }),
  })
}

export function useDeleteMonitor() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: deleteMonitor,
    onSuccess: (_value, id) => {
      queryClient.invalidateQueries({ queryKey: monitorKeys.all })
      queryClient.removeQueries({ queryKey: monitorKeys.detail(id) })
    },
  })
}
