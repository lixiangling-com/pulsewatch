import { LoaderCircle, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { ApiError } from '../../../shared/api/client'
import type { Monitor } from '../api'
import { useDeleteMonitor } from '../hooks'
import { Modal } from './Modal'

export function DeleteMonitorDialog({ monitor, onClose }: { monitor: Monitor | null; onClose: () => void }) {
  const mutation = useDeleteMonitor()
  const [notice, setNotice] = useState<string | null>(null)
  if (!monitor) return null
  const remove = async () => {
    setNotice(null)
    try {
      await mutation.mutateAsync(monitor.id)
      onClose()
    } catch (error) {
      setNotice(error instanceof ApiError ? `${error.message}${error.requestID ? `（请求 ID：${error.requestID}）` : ''}` : '暂时无法删除，请稍后重试')
    }
  }
  return <Modal open={Boolean(monitor)} onClose={onClose} title="删除监控" description="删除后它会从当前列表消失，但不会立即清除数据库记录。" width="max-w-md" role="alertdialog">
    {notice && <div className="mb-4 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-800" role="alert">{notice}</div>}
    <p className="text-sm leading-6 text-slate-700">确定要删除 <strong className="text-slate-950">{monitor.name}</strong> 吗？这个操作不能通过当前页面撤销。</p>
    <div className="mt-6 flex justify-end gap-3"><button type="button" onClick={onClose} className="h-10 rounded-md border border-slate-300 px-4 text-sm font-medium text-slate-700 hover:bg-slate-50">取消</button><button type="button" onClick={() => void remove()} disabled={mutation.isPending} className="inline-flex h-10 items-center gap-2 rounded-md bg-red-600 px-4 text-sm font-medium text-white hover:bg-red-700 disabled:cursor-not-allowed disabled:bg-red-300"><Trash2 className="size-4" aria-hidden="true" />{mutation.isPending && <LoaderCircle className="size-4 animate-spin" aria-hidden="true" />}确认删除</button></div>
  </Modal>
}
