import { apiFetch } from '../../shared/api/client'

export type DependencyStatus = {
  status: 'up' | 'down'
  latency_ms?: number
  error_code?: string
}

export type HealthResponse = {
  status: 'ready' | 'not_ready' | 'ok'
  service: string
  timestamp: string
  request_id: string
  dependencies: Record<string, DependencyStatus>
}

export async function getReadyHealth(): Promise<HealthResponse> {
  return apiFetch<HealthResponse>('/health/ready')
}
