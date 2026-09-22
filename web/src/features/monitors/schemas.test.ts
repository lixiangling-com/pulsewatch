import { describe, expect, it } from 'vitest'
import { monitorSchema } from './schemas'

describe('monitorSchema', () => {
  it('accepts the backend contract values', () => {
    expect(monitorSchema.parse({
      name: '  API  ',
      url: 'https://example.com/health',
      interval_minutes: 5,
      expected_status: 200,
    })).toMatchObject({ name: 'API', interval_minutes: 5 })
  })

  it('rejects unsupported URLs, intervals, status codes and names', () => {
    const result = monitorSchema.safeParse({
      name: ' ',
      url: 'ftp://localhost',
      interval_minutes: 2,
      expected_status: 700,
    })
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.flatten().fieldErrors.name).toBeTruthy()
      expect(result.error.flatten().fieldErrors.url).toBeTruthy()
      expect(result.error.flatten().fieldErrors.interval_minutes).toBeTruthy()
      expect(result.error.flatten().fieldErrors.expected_status).toBeTruthy()
    }
  })
})
