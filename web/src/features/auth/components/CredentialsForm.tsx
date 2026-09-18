import { zodResolver } from '@hookform/resolvers/zod'
import { LoaderCircle } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { Link, useNavigate } from 'react-router-dom'
import { ApiError } from '../../../shared/api/client'
import { useAuth } from '../AuthProvider'
import { login, register } from '../api'
import { loginSchema, registerSchema } from '../schemas'
import { FormNotice } from './FormNotice'
import { PasswordInput } from './PasswordInput'

type CredentialsFormProps = { mode: 'login' | 'register' }
type FormValues = { email: string; password: string; confirmPassword?: string }

export function CredentialsForm({ mode }: CredentialsFormProps) {
  const navigate = useNavigate()
  const { establishSession } = useAuth()
  const [notice, setNotice] = useState<string | null>(null)
  const isRegister = mode === 'register'
  const form = useForm<FormValues>({
    resolver: zodResolver(isRegister ? registerSchema : loginSchema) as never,
    defaultValues: { email: '', password: '', ...(isRegister ? { confirmPassword: '' } : {}) },
  })

  const submit = async (values: FormValues) => {
    setNotice(null)
    try {
      const result = isRegister
        ? await register({ email: values.email, password: values.password })
        : await login({ email: values.email, password: values.password })
      establishSession(result)
      navigate('/', { replace: true })
    } catch (error) {
      if (error instanceof ApiError) {
        const fields = error.body?.error?.field_errors
        let mapped = false
        if (fields) {
          Object.entries(fields).forEach(([field, messages]) => {
            if (field === 'email' || field === 'password') {
              form.setError(field, { message: messages[0] })
              mapped = true
            }
          })
        }
        // Field problems belong under their input; the top notice is reserved
        // for network, session and service failures.
        if (!mapped) setNotice(error.status === 0 ? '网络连接失败，请检查 API 是否运行。' : error.message)
        return
      }
      setNotice('暂时无法完成操作，请稍后重试。')
    }
  }

  const emailError = form.formState.errors.email?.message
  const passwordError = form.formState.errors.password?.message
  const confirmError = 'confirmPassword' in form.formState.errors ? form.formState.errors.confirmPassword?.message : undefined

  return (
    <form onSubmit={form.handleSubmit(submit)} noValidate>
      <div className="mb-6">
        <p className="text-xs font-medium uppercase tracking-[0.12em] text-teal-700">Account access</p>
        <h1 className="mt-2 text-2xl font-semibold tracking-normal text-slate-950">{isRegister ? '创建账户' : '登录控制台'}</h1>
        <p className="mt-2 text-sm leading-6 text-slate-600">{isRegister ? '创建账户后即可配置并查看你的服务监控。' : '使用你的 PulseWatch 账户继续。'}</p>
      </div>
      {notice && <FormNotice>{notice}</FormNotice>}
      <div className="space-y-4">
        <label className="block text-sm font-medium text-slate-800">
          邮箱
          <input
            type="email"
            autoComplete="email"
            className="mt-1.5 h-10 w-full rounded-md border border-slate-300 bg-white px-3 text-sm outline-none transition placeholder:text-slate-400 focus:border-teal-600 focus:ring-2 focus:ring-teal-100"
            aria-invalid={Boolean(emailError)}
            {...form.register('email')}
          />
          {emailError && <span className="mt-1.5 block text-xs text-red-700">{emailError}</span>}
        </label>
        <label className="block text-sm font-medium text-slate-800">
          密码
          <PasswordInput autoComplete={isRegister ? 'new-password' : 'current-password'} className="mt-1.5" aria-invalid={Boolean(passwordError)} {...form.register('password')} />
          {passwordError && <span className="mt-1.5 block text-xs text-red-700">{passwordError}</span>}
        </label>
        {isRegister && (
          <label className="block text-sm font-medium text-slate-800">
            确认密码
            <PasswordInput autoComplete="new-password" className="mt-1.5" aria-invalid={Boolean(confirmError)} {...form.register('confirmPassword')} />
            {confirmError && <span className="mt-1.5 block text-xs text-red-700">{confirmError}</span>}
          </label>
        )}
      </div>
      <button type="submit" disabled={form.formState.isSubmitting} className="mt-7 flex h-10 w-full items-center justify-center gap-2 rounded-md bg-teal-600 px-4 text-sm font-medium text-white transition hover:bg-teal-700 disabled:cursor-not-allowed disabled:bg-teal-400">
        {form.formState.isSubmitting && <LoaderCircle className="size-4 animate-spin" aria-hidden="true" />}
        {isRegister ? '创建并登录' : '登录'}
      </button>
      <p className="mt-6 text-center text-sm text-slate-600">
        {isRegister ? '已有账户？' : '还没有账户？'}{' '}
        <Link to={isRegister ? '/login' : '/register'} className="font-medium text-teal-700 hover:text-teal-800">
          {isRegister ? '前往登录' : '创建账户'}
        </Link>
      </p>
    </form>
  )
}
