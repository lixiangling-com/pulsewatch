import { http, HttpResponse } from 'msw'
import { afterEach, describe, expect, it } from 'vitest'
import { apiFetch, AuthExpiredError } from './client'
import { session } from './session'
import { server } from '../../test/server'

const api = 'http://localhost:8080'

afterEach(() => session.clear())

describe('apiFetch', () => {
  it('shares one refresh request across concurrent 401 responses and retries each request once', async () => {
    let protectedCalls = 0
    let refreshCalls = 0
    server.use(
      http.get(`${api}/protected`, () => {
        protectedCalls++
        if (protectedCalls <= 3) return HttpResponse.json({ error: { code: 'UNAUTHORIZED' } }, { status: 401 })
        return HttpResponse.json({ ok: true })
      }),
      http.post(`${api}/api/v1/auth/refresh`, () => {
        refreshCalls++
        return HttpResponse.json({ access_token: 'new-access-token' })
      }),
    )

    await expect(Promise.all([
      apiFetch<{ ok: boolean }>('/protected'),
      apiFetch<{ ok: boolean }>('/protected'),
      apiFetch<{ ok: boolean }>('/protected'),
    ])).resolves.toEqual([{ ok: true }, { ok: true }, { ok: true }])

    expect(refreshCalls).toBe(1)
    expect(protectedCalls).toBe(6)
    expect(session.getAccessToken()).toBe('new-access-token')
  })

  it('clears the in-memory access token when refresh fails', async () => {
    session.setAccessToken('expired-access-token')
    server.use(
      http.get(`${api}/protected`, () => HttpResponse.json({ error: { code: 'UNAUTHORIZED' } }, { status: 401 })),
      http.post(`${api}/api/v1/auth/refresh`, () => HttpResponse.json({ error: { code: 'UNAUTHORIZED' } }, { status: 401 })),
    )

    await expect(apiFetch('/protected')).rejects.toBeInstanceOf(AuthExpiredError)
    expect(session.getAccessToken()).toBeNull()
  })

  it('never retries an authentication endpoint after a 401 response', async () => {
    let calls = 0
    server.use(http.post(`${api}/api/v1/auth/login`, () => {
      calls++
      return HttpResponse.json({ error: { code: 'INVALID_CREDENTIALS' } }, { status: 401 })
    }))

    await expect(apiFetch('/api/v1/auth/login', { method: 'POST' })).rejects.toMatchObject({ status: 401 })
    expect(calls).toBe(1)
  })
})
