const apiBaseURL = (import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080').replace(/\/$/, '')

export class ApiError extends Error {
  readonly status: number
  readonly requestID: string | null
  readonly body: unknown

  constructor(message: string, status: number, requestID: string | null, body: unknown = null) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.requestID = requestID
    this.body = body
  }
}

export async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  let response: Response
  try {
    response = await fetch(`${apiBaseURL}${path.startsWith('/') ? path : `/${path}`}`, {
      credentials: 'include',
      ...init,
      headers: {
        Accept: 'application/json',
        ...init.headers,
      },
    })
  } catch {
    throw new ApiError('API 不可用', 0, null)
  }

  const body = await response.json().catch(() => null) as T | { error?: string; request_id?: string } | null
  if (!response.ok) {
    const requestID = body && typeof body === 'object' && 'request_id' in body
      ? (body.request_id ?? null)
      : response.headers.get('X-Request-ID')
    throw new ApiError(response.status === 503 ? 'API 不可用' : '请求失败', response.status, requestID, body)
  }
  return body as T
}
