import { z } from 'zod'

const email = z.string().trim().email('请输入有效邮箱').max(320, '邮箱不能超过 320 个字符')
const password = z.string().min(8, '密码至少需要 8 个字符').max(72, '密码不能超过 72 个字符')

export const loginSchema = z.object({ email, password })
export const registerSchema = loginSchema.extend({
  confirmPassword: z.string(),
}).refine((value) => value.password === value.confirmPassword, {
  path: ['confirmPassword'],
  message: '两次输入的密码不一致',
})

export type LoginValues = z.infer<typeof loginSchema>
export type RegisterValues = z.infer<typeof registerSchema>
