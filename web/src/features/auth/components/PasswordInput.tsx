import { Eye, EyeOff } from 'lucide-react'
import { useState } from 'react'
import type { InputHTMLAttributes } from 'react'
import { cn } from '../../../shared/lib/utils'

type PasswordInputProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'type'>

export function PasswordInput({ className, ...props }: PasswordInputProps) {
  const [visible, setVisible] = useState(false)
  return (
    <div className="relative">
      <input
        {...props}
        type={visible ? 'text' : 'password'}
        className={cn('h-10 w-full rounded-md border border-slate-300 bg-white px-3 pr-11 text-sm outline-none transition focus:border-teal-600 focus:ring-2 focus:ring-teal-100 disabled:cursor-not-allowed disabled:bg-slate-100', className)}
      />
      <button
        type="button"
        className="absolute inset-y-0 right-0 grid w-10 place-items-center text-slate-500 hover:text-slate-900"
        onClick={() => setVisible((current) => !current)}
        title={visible ? '隐藏密码' : '显示密码'}
        aria-label={visible ? '隐藏密码' : '显示密码'}
      >
        {visible ? <EyeOff className="size-4" aria-hidden="true" /> : <Eye className="size-4" aria-hidden="true" />}
      </button>
    </div>
  )
}
