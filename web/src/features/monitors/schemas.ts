import { z } from 'zod'

export const monitorSchema = z.object({
  name: z.string().trim().min(1, '名称不能为空').max(80, '名称最多 80 个字'),
  url: z.string().trim().max(2048, 'URL 最多 2048 个字符').refine((value) => {
    try {
      const parsed = new URL(value)
      return (parsed.protocol === 'http:' || parsed.protocol === 'https:') && Boolean(parsed.hostname)
    } catch {
      return false
    }
  }, '请输入有效的 HTTP/HTTPS URL'),
  interval_minutes: z.coerce.number().refine((value) => [1, 5, 10].includes(value), '间隔只能是 1、5 或 10 分钟'),
  expected_status: z.coerce.number().int('状态码必须是整数').min(100, '状态码不能小于 100').max(599, '状态码不能大于 599'),
})

export type MonitorFormValues = z.infer<typeof monitorSchema>
