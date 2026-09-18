import { useQuery } from '@tanstack/react-query'
import { Activity } from 'lucide-react'
import { Link } from 'react-router-dom'
import { getReadyHealth } from '../api'
import { HealthStatus } from '../components/HealthStatus'

export function HealthPage() {
  const health = useQuery({
    queryKey: ['health', 'ready'],
    queryFn: getReadyHealth,
    refetchInterval: 5000,
    retry: 1,
  })

  return (
    <main className="grid min-h-screen place-items-center bg-slate-50 px-5 py-10 text-slate-950">
      <section className="w-full max-w-xl rounded-md border border-slate-200 bg-white p-7 shadow-sm sm:p-9" aria-live="polite">
        <Link to="/login" className="flex w-fit items-center gap-2 font-semibold text-slate-950">
          <span className="grid size-8 place-items-center rounded-md bg-teal-600 text-white"><Activity className="size-4" aria-hidden="true" /></span>
          PulseWatch
        </Link>
        <div className="mt-8">
          <p className="text-xs font-medium uppercase tracking-[0.12em] text-teal-700">System diagnostics</p>
          <h1 className="mt-2 text-2xl font-semibold">运行时状态</h1>
        </div>
        <HealthStatus data={health.data} error={health.error instanceof Error ? health.error : null} isPending={health.isPending} />
        <p className="mt-6 text-xs text-slate-500">页面每 5 秒重新检查一次，不保存任何令牌。</p>
      </section>
    </main>
  )
}
