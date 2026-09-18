import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterAll, afterEach, beforeAll } from 'vitest'
import { server } from './server'

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }))
// Vitest runs without `globals`, so testing-library cannot self-register its
// automatic cleanup. Unmount between tests explicitly.
afterEach(() => {
  cleanup()
  server.resetHandlers()
})
afterAll(() => server.close())
