import { Activity, CheckCircle2, CircleAlert, Clock3, LoaderCircle, LogOut, PauseCircle, Pencil, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ApiError } from '../../../shared/api/client'
import { useAuth } from '../../auth/AuthProvider'
import type { Monitor } from '../api'
import { usePauseMonitor, useMonitors, useResumeMonitor } from '../hooks'
import { DeleteMonitorDialog } from '../components/DeleteMonitorDialog'
import { MonitorFormDialog } from '../components/MonitorFormDialog'
import { StatusBadge } from '../components/StatusBadge'

const formatDate = (value?: string | null) => value ? new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) : '—'
const formatURL = (value: string) => value.replace(/^https?:\/\//, '')

export function DashboardPage() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const monitors = useMonitors()
  const pause = usePauseMonitor()
  const resume = useResumeMonitor()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Monitor | null>(null)
  const [deleting, setDeleting] = useState<Monitor | null>(null)
  const [actionNotice, setActionNotice] = useState<string | null>(null)
  const items = monitors.data?.items ?? []
  const stats = useMemo(() => ({ total: monitors.data?.meta.total ?? 0, healthy: items.filter((item) => item.status === 'up').length, attention: items.filter((item) => ['down', 'confirming_down'].includes(item.status)).length, paused: items.filter((item) => item.status === 'paused').length }), [items, monitors.data?.meta.total])

  const logoutAndLeave = async () => { await logout(); navigate('/login', { replace: true }) }
  const togglePause = async (monitor: Monitor) => {
    setActionNotice(null)
    try { if (monitor.status === 'paused') await resume.mutateAsync(monitor.id); else await pause.mutateAsync(monitor.id) }
    catch (error) { setActionNotice(error instanceof ApiError ? `${error.message}${error.requestID ? `（请求 ID：${error.requestID}）` : ''}` : '操作失败，请稍后重试') }
  }
  const actionBusy = pause.isPending || resume.isPending

  return <main className="min-h-screen bg-slate-50 text-slate-950">
    <header className="border-b border-slate-200 bg-white"><div className="mx-auto flex h-[60px] max-w-[1380px] items-center justify-between px-6"><div className="flex items-center gap-2 font-semibold"><span className="grid size-8 place-items-center rounded-md bg-teal-600 text-white"><Activity className="size-4" aria-hidden="true" /></span>PulseWatch</div><div className="flex items-center gap-4"><span className="text-sm text-slate-600">{user?.email}</span><button type="button" onClick={() => void logoutAndLeave()} className="inline-flex h-9 items-center gap-2 rounded-md border border-slate-300 px-3 text-sm font-medium hover:bg-slate-50" title="退出登录"><LogOut className="size-4" aria-hidden="true" />退出</button></div></div></header>
    <section className="mx-auto max-w-[1380px] px-6 py-8"><div className="flex items-start justify-between gap-6"><div><p className="text-xs font-semibold uppercase tracking-[0.14em] text-teal-700">Monitor workspace</p><h1 className="mt-2 text-2xl font-semibold tracking-tight">监控总览</h1><p className="mt-2 text-sm text-slate-600">在这里创建、编辑和管理你的 HTTP/HTTPS 监控。</p></div><button type="button" onClick={() => { setEditing(null); setFormOpen(true) }} className="inline-flex h-10 items-center gap-2 rounded-md bg-teal-600 px-4 text-sm font-medium text-white shadow-sm hover:bg-teal-700"><Plus className="size-4" aria-hidden="true" />添加监控</button></div>
      <div className="mt-7 grid grid-cols-4 gap-4"><StatCard label="全部监控" value={stats.total} icon={<Activity className="size-4" />} /><StatCard label="当前正常" value={stats.healthy} icon={<CheckCircle2 className="size-4" />} tone="green" /><StatCard label="需要关注" value={stats.attention} icon={<CircleAlert className="size-4" />} tone="red" /><StatCard label="已暂停" value={stats.paused} icon={<PauseCircle className="size-4" />} /></div>
      {actionNotice && <div className="mt-5 flex items-center justify-between rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800" role="alert"><span>{actionNotice}</span><button type="button" className="font-medium underline" onClick={() => setActionNotice(null)}>关闭</button></div>}
      <section className="mt-7 overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm"><div className="flex items-center justify-between border-b border-slate-200 px-5 py-4"><div><h2 className="font-semibold">监控列表</h2><p className="mt-1 text-xs text-slate-500">按最近创建时间排序 · 配置版本用于保证旧任务不会覆盖新配置</p></div>{monitors.isFetching && <LoaderCircle className="size-4 animate-spin text-teal-600" aria-label="正在刷新" />}</div>
        {monitors.isPending ? <SkeletonTable /> : monitors.isError ? <FailureState error={monitors.error} onRetry={() => void monitors.refetch()} /> : items.length === 0 ? <EmptyState onCreate={() => { setEditing(null); setFormOpen(true) }} /> : <MonitorTable items={items} actionBusy={actionBusy} onEdit={(monitor) => { setEditing(monitor); setFormOpen(true) }} onToggle={(monitor) => void togglePause(monitor)} onDelete={setDeleting} />}
      </section>
    </section>
    <MonitorFormDialog open={formOpen} monitor={editing} onClose={() => setFormOpen(false)} />
    <DeleteMonitorDialog monitor={deleting} onClose={() => setDeleting(null)} />
  </main>
}

function StatCard({ label, value, icon, tone = 'teal' }: { label: string; value: number; icon: React.ReactNode; tone?: 'teal' | 'green' | 'red' }) { const colors = { teal: 'bg-teal-50 text-teal-700', green: 'bg-emerald-50 text-emerald-700', red: 'bg-red-50 text-red-700' }; return <div className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm"><div className="flex items-center gap-2 text-sm text-slate-500"><span className={`grid size-7 place-items-center rounded-md ${colors[tone]}`}>{icon}</span>{label}</div><p className="mt-3 text-2xl font-semibold">{value}</p></div> }
function SkeletonTable() { return <div className="space-y-3 p-5" aria-label="正在加载监控"><div className="h-8 animate-pulse rounded bg-slate-100" />{[1, 2, 3, 4].map((row) => <div key={row} className="h-14 animate-pulse rounded bg-slate-100" />)}</div> }
function EmptyState({ onCreate }: { onCreate: () => void }) { return <div className="grid min-h-[320px] place-items-center p-10 text-center"><div><span className="mx-auto grid size-12 place-items-center rounded-full bg-teal-50 text-teal-700"><Activity className="size-6" aria-hidden="true" /></span><h3 className="mt-4 font-semibold">还没有监控</h3><p className="mt-2 text-sm text-slate-600">添加第一个监控后，它会出现在这里。</p><button type="button" onClick={onCreate} className="mt-5 inline-flex h-10 items-center gap-2 rounded-md bg-teal-600 px-4 text-sm font-medium text-white hover:bg-teal-700"><Plus className="size-4" aria-hidden="true" />添加监控</button></div></div> }
function FailureState({ error, onRetry }: { error: unknown; onRetry: () => void }) { const apiError = error instanceof ApiError; return <div className="grid min-h-[320px] place-items-center p-10 text-center"><div><span className="mx-auto grid size-12 place-items-center rounded-full bg-red-50 text-red-700"><CircleAlert className="size-6" aria-hidden="true" /></span><h3 className="mt-4 font-semibold">暂时无法加载监控</h3><p className="mt-2 text-sm text-slate-600">{apiError ? error.message : '请稍后重试。'}</p>{apiError && error.requestID && <p className="mt-1 text-xs text-slate-500">请求 ID：{error.requestID}</p>}<button type="button" onClick={onRetry} className="mt-5 inline-flex h-10 items-center gap-2 rounded-md border border-slate-300 px-4 text-sm font-medium hover:bg-slate-50"><RefreshCw className="size-4" aria-hidden="true" />重试</button></div></div> }
function MonitorTable({ items, actionBusy, onEdit, onToggle, onDelete }: { items: Monitor[]; actionBusy: boolean; onEdit: (monitor: Monitor) => void; onToggle: (monitor: Monitor) => void; onDelete: (monitor: Monitor) => void }) { return <div className="overflow-x-auto"><table className="w-full min-w-[1120px] table-fixed text-left text-sm"><thead className="bg-slate-50 text-xs font-medium text-slate-500"><tr><th className="w-[27%] px-5 py-3">名称 / URL</th><th className="w-[12%] px-3 py-3">状态</th><th className="w-[11%] px-3 py-3">最近耗时</th><th className="w-[16%] px-3 py-3">上次检查</th><th className="w-[16%] px-3 py-3">下次检查</th><th className="w-[8%] px-3 py-3">间隔</th><th className="w-[10%] px-3 py-3">操作</th></tr></thead><tbody className="divide-y divide-slate-100">{items.map((monitor) => <tr key={monitor.id} className="align-middle"><td className="px-5 py-4"><div className="truncate font-medium" title={monitor.name}>{monitor.name}</div><div className="mt-1 truncate text-xs text-slate-500" title={monitor.url}>{formatURL(monitor.url)}</div></td><td className="px-3 py-4"><StatusBadge status={monitor.status} /></td><td className="px-3 py-4 text-slate-600">{monitor.last_latency_ms == null ? '—' : `${monitor.last_latency_ms} ms`}</td><td className="px-3 py-4 text-xs text-slate-600">{formatDate(monitor.last_checked_at)}</td><td className="px-3 py-4 text-xs text-slate-600">{formatDate(monitor.next_check_at)}</td><td className="px-3 py-4 text-slate-600">{monitor.interval_minutes} 分钟</td><td className="px-3 py-4"><div className="flex items-center gap-1"><button type="button" onClick={() => onEdit(monitor)} className="grid size-8 place-items-center rounded-md text-slate-600 hover:bg-slate-100 hover:text-slate-950" title="编辑" aria-label={`编辑 ${monitor.name}`}><Pencil className="size-4" aria-hidden="true" /></button><button type="button" disabled={actionBusy} onClick={() => onToggle(monitor)} className="grid size-8 place-items-center rounded-md text-slate-600 hover:bg-slate-100 hover:text-slate-950 disabled:cursor-not-allowed disabled:opacity-50" title={monitor.status === 'paused' ? '恢复' : '暂停'} aria-label={`${monitor.status === 'paused' ? '恢复' : '暂停'} ${monitor.name}`}>{monitor.status === 'paused' ? <Clock3 className="size-4" aria-hidden="true" /> : <PauseCircle className="size-4" aria-hidden="true" />}</button><button type="button" onClick={() => onDelete(monitor)} className="grid size-8 place-items-center rounded-md text-slate-600 hover:bg-red-50 hover:text-red-700" title="删除" aria-label={`删除 ${monitor.name}`}><Trash2 className="size-4" aria-hidden="true" /></button></div></td></tr>)}</tbody></table></div> }
