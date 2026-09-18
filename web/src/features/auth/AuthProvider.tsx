import { createContext, type ReactNode, useContext, useEffect, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { refreshOnce } from '../../shared/api/client'
import { session } from '../../shared/api/session'
import { getCurrentUser, logout as logoutRequest, type AuthSession, type AuthUser } from './api'

type AuthStatus = 'restoring' | 'authenticated' | 'anonymous'

type AuthContextValue = {
  status: AuthStatus
  user: AuthUser | null
  establishSession: (value: AuthSession) => void
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const [status, setStatus] = useState<AuthStatus>('restoring')
  const [user, setUser] = useState<AuthUser | null>(null)
  useEffect(() => {
    let active = true

    const restore = async () => {
      try {
        await refreshOnce()
        const currentUser = await getCurrentUser()
        if (!active) return
        setUser(currentUser)
        setStatus('authenticated')
      } catch {
        session.clear()
        if (!active) return
        setUser(null)
        setStatus('anonymous')
      }
    }

    void restore()
    const unsubscribe = session.subscribe(() => {
      if (session.getAccessToken() === null && active) {
        setUser(null)
        setStatus('anonymous')
      }
    })
    return () => {
      active = false
      unsubscribe()
    }
  }, [])

  const establishSession = (value: AuthSession) => {
    session.setAccessToken(value.access_token)
    setUser(value.user)
    setStatus('authenticated')
  }

  const logout = async () => {
    try {
      await logoutRequest()
    } finally {
      session.clear()
      queryClient.clear()
      setUser(null)
      setStatus('anonymous')
    }
  }

  return <AuthContext.Provider value={{ status, user, establishSession, logout }}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) throw new Error('useAuth must be used within AuthProvider')
  return context
}
