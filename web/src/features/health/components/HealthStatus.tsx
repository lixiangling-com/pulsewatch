import type { HealthResponse } from '../api'
import { ApiError } from '../../../shared/api/client'

function dependencyLabel(status: HealthResponse['dependencies'][string] | undefined) {
  if (!status) return '未知'
  if (status.status === 'up') return `正常${status.latency_ms !== undefined ? ` · ${status.latency_ms} ms` : ''}`
  return `不可用${status.error_code ? ` · ${status.error_code}` : ''}`
}

type HealthStatusProps = {
  data?: HealthResponse
  error: Error | null
  isPending: boolean
}

export function HealthStatus({ data, error, isPending }: HealthStatusProps) {
  const errorData = error instanceof ApiError && error.body && typeof error.body === 'object'
    ? error.body as Partial<HealthResponse>
    : undefined
  const visibleData = errorData?.dependencies
    ? { ...data, ...errorData } as HealthResponse
    : data
  const state = isPending
    ? <p className="state loading">正在检查 API…</p>
    : error
      ? <p className="state unavailable">API 不可用</p>
      : data?.status === 'ready'
      ? <p className="state ready">API 就绪</p>
      : <p className="state unavailable">API 不可用</p>

  return (
    <>
      {state}
      {visibleData && (
        <dl className="dependencies">
          <div><dt>PostgreSQL</dt><dd>{dependencyLabel(visibleData.dependencies.postgres)}</dd></div>
          <div><dt>Redis</dt><dd>{dependencyLabel(visibleData.dependencies.redis)}</dd></div>
        </dl>
      )}
      {error && <p className="error">API 当前不可用，请稍后重试。</p>}
      {(errorData?.request_id ?? data?.request_id ?? (error instanceof ApiError ? error.requestID : null)) && (
        <p className="request-id">request_id：{errorData?.request_id ?? data?.request_id ?? (error instanceof ApiError ? error.requestID : '')}</p>
      )}
    </>
  )
}
