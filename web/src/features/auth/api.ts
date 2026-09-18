import { apiFetch } from '../../shared/api/client'

export type AuthUser = {
  id: string
  email: string
  created_at: string
  updated_at: string
}

export type AuthSession = {
  access_token: string
  expires_in: number
  user: AuthUser
}

type Credentials = {
  email: string
  password: string
}

export function register(input: Credentials) {
  return apiFetch<AuthSession>('/api/v1/auth/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
    retryOnUnauthorized: false,
  })
}

export function login(input: Credentials) {
  return apiFetch<AuthSession>('/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
    retryOnUnauthorized: false,
  })
}

export function getCurrentUser() {
  return apiFetch<AuthUser>('/api/v1/auth/me')
}

export function logout() {
  return apiFetch<void>('/api/v1/auth/logout', { method: 'POST', retryOnUnauthorized: false })
}
