import { apiFetch } from '../../shared/api/client'
import type { components } from '../../generated/api-types'

export type Monitor = components['schemas']['Monitor']
export type MonitorListResponse = components['schemas']['MonitorListResponse']
export type CreateMonitorInput = components['schemas']['CreateMonitorRequest']
export type UpdateMonitorInput = components['schemas']['UpdateMonitorRequest']

export const monitorKeys = {
  all: ['monitors'] as const,
  list: (page: number, pageSize: number) => ['monitors', 'list', { page, pageSize }] as const,
  detail: (id: string) => ['monitors', 'detail', id] as const,
}

export function listMonitors(page = 1, pageSize = 100) {
  return apiFetch<MonitorListResponse>(`/api/v1/monitors?page=${page}&page_size=${pageSize}`)
}

export function createMonitor(input: CreateMonitorInput) {
  return apiFetch<Monitor>('/api/v1/monitors', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export function updateMonitor(id: string, input: UpdateMonitorInput) {
  return apiFetch<Monitor>(`/api/v1/monitors/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export function pauseMonitor(id: string) {
  return apiFetch<Monitor>(`/api/v1/monitors/${id}/pause`, { method: 'POST' })
}

export function resumeMonitor(id: string) {
  return apiFetch<Monitor>(`/api/v1/monitors/${id}/resume`, { method: 'POST' })
}

export function deleteMonitor(id: string) {
  return apiFetch<void>(`/api/v1/monitors/${id}`, { method: 'DELETE' })
}
