import { Activity } from 'lucide-react'
import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from './AuthProvider'

export function ProtectedRoute() {
  const { status } = useAuth()
  const location = useLocation()

  if (status === 'restoring') {
    return (
      <main className="grid min-h-screen place-items-center bg-slate-50 px-6 text-slate-700">
        <div className="flex items-center gap-3 text-sm" role="status">
          <Activity className="size-5 animate-pulse text-teal-600" aria-hidden="true" />
          正在恢复会话…
        </div>
      </main>
    )
  }
  if (status === 'anonymous') return <Navigate to="/login" replace state={{ from: location }} />
  return <Outlet />
}
