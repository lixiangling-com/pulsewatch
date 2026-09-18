import { Activity } from 'lucide-react'
import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'

export function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <main className="grid min-h-screen place-items-center bg-slate-50 px-5 py-10 text-slate-950">
      <section className="w-full max-w-[420px] rounded-md border border-slate-200 bg-white p-7 shadow-sm sm:p-9">
        <Link to="/login" className="mb-8 flex w-fit items-center gap-2 text-base font-semibold text-slate-950" aria-label="PulseWatch 登录">
          <span className="grid size-8 place-items-center rounded-md bg-teal-600 text-white"><Activity className="size-4" aria-hidden="true" /></span>
          PulseWatch
        </Link>
        {children}
      </section>
    </main>
  )
}
