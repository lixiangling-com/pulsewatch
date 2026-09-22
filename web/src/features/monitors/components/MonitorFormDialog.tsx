import { zodResolver } from '@hookform/resolvers/zod'
import { LoaderCircle } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useForm, type UseFormSetError } from 'react-hook-form'
import { ApiError } from '../../../shared/api/client'
import type { Monitor } from '../api'
import { useCreateMonitor, useUpdateMonitor } from '../hooks'
import { monitorSchema, type MonitorFormValues } from '../schemas'
import { Modal } from './Modal'

type Props = { open: boolean; monitor?: Monitor | null; onClose: () => void }

function mapServerErrors(error: unknown, setError: UseFormSetError<MonitorFormValues>) {
  if (!(error instanceof ApiError)) return false
  const fields = error.body?.error?.field_errors
  if (!fields) return false
  let mapped = false
  for (const [field, messages] of Object.entries(fields)) {
    if (field === 'name' || field === 'url' || field === 'interval_minutes' || field === 'expected_status') {
      setError(field, { type: 'server', message: messages[0] })
      mapped = true
    }
  }
  return mapped
}

export function MonitorFormDialog({ open, monitor, onClose }: Props) {
  const create = useCreateMonitor()
  const update = useUpdateMonitor()
  const [notice, setNotice] = useState<string | null>(null)
  const form = useForm<MonitorFormValues>({
    resolver: zodResolver(monitorSchema),
    defaultValues: { name: '', url: 'https://', interval_minutes: 5, expected_status: 200 },
  })

  useEffect(() => {
    if (!open) return
    setNotice(null)
    form.reset(monitor ? {
      name: monitor.name,
      url: monitor.url,
      interval_minutes: monitor.interval_minutes,
      expected_status: monitor.expected_status,
    } : { name: '', url: 'https://', interval_minutes: 5, expected_status: 200 })
  }, [form, monitor, open])

  const submit = async (values: MonitorFormValues) => {
    setNotice(null)
    const input = { ...values, interval_minutes: values.interval_minutes as 1 | 5 | 10 }
    try {
      if (monitor) await update.mutateAsync({ id: monitor.id, input })
      else await create.mutateAsync(input)
      onClose()
    } catch (error) {
      if (!mapServerErrors(error, form.setError)) {
        setNotice(error instanceof ApiError ? `${error.message}${error.requestID ? `（请求 ID：${error.requestID}）` : ''}` : '暂时无法保存，请稍后重试')
      }
    }
  }

  const busy = create.isPending || update.isPending
  const nameError = form.formState.errors.name?.message
  const urlError = form.formState.errors.url?.message
  const intervalError = form.formState.errors.interval_minutes?.message
  const statusError = form.formState.errors.expected_status?.message
  const inputClass = 'mt-1.5 h-10 w-full rounded-md border border-slate-300 bg-white px-3 text-sm outline-none focus:border-teal-600 focus:ring-2 focus:ring-teal-100'
  const errorClass = 'mt-1 block text-xs text-red-700'

  return <Modal open={open} onClose={onClose} title={monitor ? '编辑监控' : '添加监控'} description="保存后会立即出现在监控列表中。">
    <form onSubmit={form.handleSubmit(submit)} noValidate className="space-y-4">
      {notice && <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-800" role="alert">{notice}</div>}
      <label className="block text-sm font-medium text-slate-800">名称<input className={inputClass} aria-invalid={Boolean(nameError)} {...form.register('name')} />{nameError && <span className={errorClass}>{nameError}</span>}</label>
      <label className="block text-sm font-medium text-slate-800">URL<input className={inputClass} aria-invalid={Boolean(urlError)} {...form.register('url')} />{urlError && <span className={errorClass}>{urlError}</span>}</label>
      <div className="grid grid-cols-2 gap-4">
        <label className="block text-sm font-medium text-slate-800">检查间隔<select className={inputClass} {...form.register('interval_minutes', { valueAsNumber: true })}><option value={1}>每 1 分钟</option><option value={5}>每 5 分钟</option><option value={10}>每 10 分钟</option></select>{intervalError && <span className={errorClass}>{intervalError}</span>}</label>
        <label className="block text-sm font-medium text-slate-800">预期状态码<input type="number" min={100} max={599} className={inputClass} {...form.register('expected_status', { valueAsNumber: true })} />{statusError && <span className={errorClass}>{statusError}</span>}</label>
      </div>
      <div className="flex justify-end gap-3 border-t border-slate-200 pt-5"><button type="button" onClick={onClose} className="h-10 rounded-md border border-slate-300 px-4 text-sm font-medium text-slate-700 hover:bg-slate-50">取消</button><button type="submit" disabled={busy} className="inline-flex h-10 items-center gap-2 rounded-md bg-teal-600 px-4 text-sm font-medium text-white hover:bg-teal-700 disabled:cursor-not-allowed disabled:bg-teal-300">{busy && <LoaderCircle className="size-4 animate-spin" aria-hidden="true" />}保存</button></div>
    </form>
  </Modal>
}
