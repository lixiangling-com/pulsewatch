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
    ? error.body as unknown as Partial<HealthResponse>
    : undefined
  const visibleData = errorData?.dependencies
    ? { ...data, ...errorData } as HealthResponse
    : data
  const state = isPending
    ? <p className="mt-7 text-lg font-semibold text-slate-600">正在检查 API…</p>
    : error
      ? <p className="mt-7 text-lg font-semibold text-red-700">API 不可用</p>
    : data?.status === 'ready'
      ? <p className="mt-7 text-lg font-semibold text-emerald-700">API 就绪</p>
      : <p className="mt-7 text-lg font-semibold text-red-700">API 不可用</p>

  return (
    <>
      {state}
      {visibleData && (
        <dl className="mt-5 divide-y divide-slate-200 border-y border-slate-200">
          <div className="flex items-center justify-between gap-5 py-3"><dt className="text-sm text-slate-600">PostgreSQL</dt><dd className="text-sm font-medium text-slate-900">{dependencyLabel(visibleData.dependencies.postgres)}</dd></div>
          <div className="flex items-center justify-between gap-5 py-3"><dt className="text-sm text-slate-600">Redis</dt><dd className="text-sm font-medium text-slate-900">{dependencyLabel(visibleData.dependencies.redis)}</dd></div>
        </dl>
      )}
      {error && <p className="mt-4 text-sm text-red-700">API 当前不可用，请稍后重试。</p>}
      {(errorData?.request_id ?? data?.request_id ?? (error instanceof ApiError ? error.requestID : null)) && (
        <p className="mt-3 break-all text-xs text-slate-500">request_id：{errorData?.request_id ?? data?.request_id ?? (error instanceof ApiError ? error.requestID : '')}</p>
      )}
    </>
  )
}
