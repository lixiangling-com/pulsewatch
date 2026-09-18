import { AlertCircle } from 'lucide-react'

export function FormNotice({ children }: { children: string }) {
  return (
    <div className="mb-5 flex gap-2 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-800" role="alert">
      <AlertCircle className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
      <span>{children}</span>
    </div>
  )
}
