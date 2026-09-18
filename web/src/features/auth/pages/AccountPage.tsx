import { Activity, LogOut, ShieldCheck } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../AuthProvider'

export function AccountPage() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  const handleLogout = async () => {
    await logout()
    navigate('/login', { replace: true })
  }

  return (
    <main className="min-h-screen bg-slate-50 text-slate-950">
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex h-[60px] max-w-6xl items-center justify-between px-5">
          <div className="flex items-center gap-2 font-semibold">
            <span className="grid size-8 place-items-center rounded-md bg-teal-600 text-white"><Activity className="size-4" aria-hidden="true" /></span>
            PulseWatch
          </div>
          <div className="flex items-center gap-3">
            <span className="hidden text-sm text-slate-600 sm:block">{user?.email}</span>
            <button type="button" onClick={() => void handleLogout()} className="inline-flex h-9 items-center gap-2 rounded-md border border-slate-300 bg-white px-3 text-sm font-medium text-slate-800 hover:bg-slate-100" title="退出登录">
              <LogOut className="size-4" aria-hidden="true" />
              退出
            </button>
          </div>
        </div>
      </header>
      <section className="mx-auto max-w-6xl px-5 py-12">
        <p className="text-xs font-medium uppercase tracking-[0.12em] text-teal-700">Account</p>
        <h1 className="mt-2 text-2xl font-semibold">账户已连接</h1>
        <p className="mt-2 text-sm text-slate-600">Day06 将在这里加入你的监控总览与管理操作。</p>
        <div className="mt-8 max-w-2xl rounded-md border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex items-start gap-3">
            <span className="grid size-9 place-items-center rounded-md bg-teal-50 text-teal-700"><ShieldCheck className="size-5" aria-hidden="true" /></span>
            <div>
              <h2 className="text-base font-semibold">当前会话</h2>
              <dl className="mt-4 grid gap-4 text-sm sm:grid-cols-2">
                <div><dt className="text-slate-500">登录账户</dt><dd className="mt-1 font-medium text-slate-900">{user?.email}</dd></div>
                <div><dt className="text-slate-500">Access Token</dt><dd className="mt-1 font-medium text-slate-900">仅保存在浏览器内存</dd></div>
              </dl>
            </div>
          </div>
        </div>
      </section>
    </main>
  )
}
