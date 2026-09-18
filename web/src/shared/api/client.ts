import { session } from './session'

const apiBaseURL = (import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080').replace(/\/$/, '')

export type ApiErrorBody = {
  error?: {
    code?: string
    message?: string
    field_errors?: Record<string, string[]>
    request_id?: string
  }
}

export class ApiError extends Error {
  readonly status: number
  readonly requestID: string | null
  readonly body: ApiErrorBody | null

  constructor(message: string, status: number, requestID: string | null, body: ApiErrorBody | null = null) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.requestID = requestID
    this.body = body
  }
}

export class AuthExpiredError extends ApiError {
  constructor(requestID: string | null) {
    super('登录已过期，请重新登录', 401, requestID)
    this.name = 'AuthExpiredError'
  }
}

let refreshPromise: Promise<string> | null = null

type FetchOptions = RequestInit & { retryOnUnauthorized?: boolean }

export async function apiFetch<T>(path: string, options: FetchOptions = {}): Promise<T> {
  const { retryOnUnauthorized = true, ...init } = options
  try {
    return await request<T>(path, init)
  } catch (error) {
    if (!(error instanceof ApiError) || error.status !== 401 || !retryOnUnauthorized || isAuthPath(path)) {
      throw error
    }
    try {
      await refreshOnce()
      return await request<T>(path, init)
    } catch (refreshError) {
      session.clear()
      if (refreshError instanceof ApiError) throw new AuthExpiredError(refreshError.requestID)
      throw refreshError
    }
  }
}

export function refreshOnce(): Promise<string> {
  if (!refreshPromise) {
    refreshPromise = request<{ access_token: string }>('/api/v1/auth/refresh', { method: 'POST' })
      .then((response) => {
        session.setAccessToken(response.access_token)
        return response.access_token
      })
      .finally(() => {
        refreshPromise = null
      })
  }
  return refreshPromise
}

async function request<T>(path: string, init: RequestInit): Promise<T> {
  let response: Response
  try {
    const token = session.getAccessToken()
    response = await fetch(`${apiBaseURL}${path.startsWith('/') ? path : `/${path}`}`, {
      credentials: 'include',
      ...init,
      headers: {
        Accept: 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...init.headers,
      },
    })
  } catch {
    throw new ApiError('网络连接失败，请检查 API 是否运行', 0, null)
  }

  if (response.status === 204) return undefined as T
  const body = await response.json().catch(() => null) as ApiErrorBody | T | null
  if (!response.ok) {
    const errorBody = body as ApiErrorBody | null
    const requestID = errorBody?.error?.request_id ?? response.headers.get('X-Request-ID')
    const message = errorBody?.error?.message ?? (response.status === 503 ? 'API 暂时不可用' : '请求失败')
    throw new ApiError(message, response.status, requestID, errorBody)
  }
  return body as T
}

function isAuthPath(path: string) {
  return path.startsWith('/api/v1/auth/')
}
