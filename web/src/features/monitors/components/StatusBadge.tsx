import { Activity, AlertTriangle, CheckCircle2, Clock3, PauseCircle, XCircle } from 'lucide-react'
import type { Monitor } from '../api'

const statusConfig: Record<Monitor['status'], { label: string; className: string; Icon: typeof Activity }> = {
  pending: { label: '待检查', className: 'bg-amber-50 text-amber-800 ring-amber-200', Icon: Clock3 },
  up: { label: '正常', className: 'bg-emerald-50 text-emerald-800 ring-emerald-200', Icon: CheckCircle2 },
  confirming_down: { label: '确认异常', className: 'bg-orange-50 text-orange-800 ring-orange-200', Icon: AlertTriangle },
  down: { label: '异常', className: 'bg-red-50 text-red-800 ring-red-200', Icon: XCircle },
  confirming_up: { label: '恢复中', className: 'bg-blue-50 text-blue-800 ring-blue-200', Icon: Activity },
  paused: { label: '已暂停', className: 'bg-slate-100 text-slate-700 ring-slate-300', Icon: PauseCircle },
}

export function StatusBadge({ status }: { status: Monitor['status'] }) {
  const config = statusConfig[status]
  const Icon = config.Icon
  return <span className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium ring-1 ${config.className}`}><Icon className="size-3.5" aria-hidden="true" />{config.label}</span>
}
