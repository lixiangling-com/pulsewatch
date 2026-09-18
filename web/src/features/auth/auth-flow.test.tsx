import { fireEvent, screen, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { afterEach, describe, expect, it } from 'vitest'
import App from '../../app/App'
import { session } from '../../shared/api/session'
import { server } from '../../test/server'
import { renderWithProviders } from '../../test/render'

const api = 'http://localhost:8080'
const currentUser = {
  id: '11111111-1111-4111-8111-111111111111',
  email: 'demo@example.com',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

function refreshReturns(token: string) {
  return http.post(`${api}/api/v1/auth/refresh`, () => HttpResponse.json({
    access_token: token,
    expires_in: 900,
    user: currentUser,
  }))
}

function meReturns() {
  return http.get(`${api}/api/v1/auth/me`, () => HttpResponse.json(currentUser))
}

function emailField(container: HTMLElement) {
  return container.querySelector<HTMLInputElement>('input[type="email"]')!
}

function passwordFields(container: HTMLElement) {
  return Array.from(container.querySelectorAll<HTMLInputElement>('input[type="password"]'))
}

afterEach(() => session.clear())

describe('session restore on reload', () => {
  it('restores the session from the HttpOnly cookie without persisting the access token', async () => {
    let refreshCalls = 0
    server.use(
      http.post(`${api}/api/v1/auth/refresh`, () => {
        refreshCalls++
        return HttpResponse.json({ access_token: 'restored-access-token', expires_in: 900, user: currentUser })
      }),
      meReturns(),
    )

    renderWithProviders(<App />, '/')

    expect(await screen.findByRole('heading', { name: '账户已连接' })).toBeInTheDocument()
    expect(screen.getAllByText(currentUser.email).length).toBeGreaterThan(0)
    expect(refreshCalls).toBe(1)

    // The access token lives in memory only: no web storage, no readable cookie.
    expect(window.localStorage.length).toBe(0)
    expect(window.sessionStorage.length).toBe(0)
    expect(document.cookie).not.toContain('restored-access-token')
  })

  it('shows the fixed loading screen instead of protected content while restoring', async () => {
    let releaseRefresh: (() => void) | undefined
    const gate = new Promise<void>((resolve) => {
      releaseRefresh = resolve
    })
    server.use(
      http.post(`${api}/api/v1/auth/refresh`, async () => {
        await gate
        return HttpResponse.json({ access_token: 'restored-access-token', expires_in: 900, user: currentUser })
      }),
      meReturns(),
    )

    renderWithProviders(<App />, '/')

    expect(await screen.findByText('正在恢复会话…')).toBeInTheDocument()
    expect(screen.queryByText('账户已连接')).not.toBeInTheDocument()

    releaseRefresh?.()
    expect(await screen.findByRole('heading', { name: '账户已连接' })).toBeInTheDocument()
  })

  it('never renders the account page when refresh fails', async () => {
    server.use(
      http.post(`${api}/api/v1/auth/refresh`, () => HttpResponse.json(
        { error: { code: 'UNAUTHORIZED', message: '认证已失效，请重新登录', request_id: 'req-restore' } },
        { status: 401 },
      )),
    )

    renderWithProviders(<App />, '/')

    expect(await screen.findByRole('heading', { name: '登录控制台' })).toBeInTheDocument()
    expect(screen.queryByText('账户已连接')).not.toBeInTheDocument()
    expect(session.getAccessToken()).toBeNull()
  })
})

describe('credentials form feedback', () => {
  it('blocks submission on client-side validation errors', async () => {
    let registerCalls = 0
    server.use(
      http.post(`${api}/api/v1/auth/refresh`, () => HttpResponse.json({}, { status: 401 })),
      http.post(`${api}/api/v1/auth/register`, () => {
        registerCalls++
        return HttpResponse.json({}, { status: 201 })
      }),
    )

    const { container } = renderWithProviders(<App />, '/register')
    const submit = await screen.findByRole('button', { name: '创建并登录' })

    fireEvent.change(emailField(container), { target: { value: 'not-an-email' } })
    const [password, confirm] = passwordFields(container)
    fireEvent.change(password, { target: { value: 'short' } })
    fireEvent.change(confirm, { target: { value: 'different' } })
    fireEvent.click(submit)

    expect(await screen.findByText('请输入有效邮箱')).toBeInTheDocument()
    expect(screen.getByText('密码至少需要 8 个字符')).toBeInTheDocument()
    expect(screen.getByText('两次输入的密码不一致')).toBeInTheDocument()
    expect(registerCalls).toBe(0)
  })

  it('maps a 422 field_errors response onto the matching input', async () => {
    server.use(
      http.post(`${api}/api/v1/auth/refresh`, () => HttpResponse.json({}, { status: 401 })),
      http.post(`${api}/api/v1/auth/register`, () => HttpResponse.json({
        error: {
          code: 'VALIDATION_ERROR',
          message: '请检查输入',
          field_errors: { password: ['密码长度应为 8 到 72 个字节'] },
          request_id: 'req-422',
        },
      }, { status: 422 })),
    )

    const { container } = renderWithProviders(<App />, '/register')
    const submit = await screen.findByRole('button', { name: '创建并登录' })

    fireEvent.change(emailField(container), { target: { value: 'demo@example.com' } })
    const [password, confirm] = passwordFields(container)
    fireEvent.change(password, { target: { value: 'correct-horse-battery-staple' } })
    fireEvent.change(confirm, { target: { value: 'correct-horse-battery-staple' } })
    fireEvent.click(submit)

    expect(await screen.findByText('密码长度应为 8 到 72 个字节')).toBeInTheDocument()
    expect(password).toHaveAttribute('aria-invalid', 'true')
    // Field problems stay under the input; they must not also raise a top notice.
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('surfaces a 409 duplicate email as a distinct field error', async () => {
    server.use(
      http.post(`${api}/api/v1/auth/refresh`, () => HttpResponse.json({}, { status: 401 })),
      http.post(`${api}/api/v1/auth/register`, () => HttpResponse.json({
        error: {
          code: 'EMAIL_ALREADY_REGISTERED',
          message: '该邮箱已注册',
          field_errors: { email: ['该邮箱已注册'] },
          request_id: 'req-409',
        },
      }, { status: 409 })),
    )

    const { container } = renderWithProviders(<App />, '/register')
    const submit = await screen.findByRole('button', { name: '创建并登录' })

    fireEvent.change(emailField(container), { target: { value: 'demo@example.com' } })
    const [password, confirm] = passwordFields(container)
    fireEvent.change(password, { target: { value: 'correct-horse-battery-staple' } })
    fireEvent.change(confirm, { target: { value: 'correct-horse-battery-staple' } })
    fireEvent.click(submit)

    expect(await screen.findByText('该邮箱已注册')).toBeInTheDocument()
    expect(emailField(container)).toHaveAttribute('aria-invalid', 'true')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('surfaces a 401 login failure as a top notice', async () => {
    server.use(
      http.post(`${api}/api/v1/auth/refresh`, () => HttpResponse.json({}, { status: 401 })),
      http.post(`${api}/api/v1/auth/login`, () => HttpResponse.json({
        error: { code: 'INVALID_CREDENTIALS', message: '邮箱或密码不正确', request_id: 'req-401' },
      }, { status: 401 })),
    )

    const { container } = renderWithProviders(<App />, '/login')
    const submit = await screen.findByRole('button', { name: '登录' })

    fireEvent.change(emailField(container), { target: { value: 'demo@example.com' } })
    fireEvent.change(passwordFields(container)[0], { target: { value: 'correct-horse-battery-staple' } })
    fireEvent.click(submit)

    expect(await screen.findByRole('alert')).toHaveTextContent('邮箱或密码不正确')
  })

  it('surfaces a network failure as a distinct top notice', async () => {
    server.use(
      http.post(`${api}/api/v1/auth/refresh`, () => HttpResponse.json({}, { status: 401 })),
      http.post(`${api}/api/v1/auth/login`, () => HttpResponse.error()),
    )

    const { container } = renderWithProviders(<App />, '/login')
    const submit = await screen.findByRole('button', { name: '登录' })

    fireEvent.change(emailField(container), { target: { value: 'demo@example.com' } })
    fireEvent.change(passwordFields(container)[0], { target: { value: 'correct-horse-battery-staple' } })
    fireEvent.click(submit)

    expect(await screen.findByRole('alert')).toHaveTextContent('网络连接失败')
  })
})

describe('logout', () => {
  it('clears the session and blocks the protected route afterwards', async () => {
    server.use(
      refreshReturns('restored-access-token'),
      meReturns(),
      http.post(`${api}/api/v1/auth/logout`, () => new HttpResponse(null, { status: 204 })),
    )

    renderWithProviders(<App />, '/')
    expect(await screen.findByRole('heading', { name: '账户已连接' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /退出/ }))

    expect(await screen.findByRole('heading', { name: '登录控制台' })).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByText('账户已连接')).not.toBeInTheDocument())
    expect(session.getAccessToken()).toBeNull()
    expect(window.localStorage.length).toBe(0)
    expect(window.sessionStorage.length).toBe(0)
  })
})
