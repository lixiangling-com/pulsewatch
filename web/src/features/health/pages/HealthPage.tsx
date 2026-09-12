import { useQuery } from '@tanstack/react-query'
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
    <main className="shell">
      <section className="card" aria-live="polite">
        <div className="eyebrow">PULSEWATCH · DAY02</div>
        <h1>运行时状态</h1>
        <HealthStatus data={health.data} error={health.error instanceof Error ? health.error : null} isPending={health.isPending} />
        <p className="hint">页面每 5 秒重新检查一次，不保存任何令牌。</p>
      </section>
    </main>
  )
}
