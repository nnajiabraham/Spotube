import '@testing-library/jest-dom'
import { cleanup } from '@testing-library/react'
import { afterEach, beforeAll, afterAll } from 'vitest'

// Cleanup after each test
afterEach(() => {
  cleanup()
})

// Declare global type for MSW server
declare global {
  // eslint-disable-next-line no-var
  var mswServer: import('msw/node').SetupServerApi | undefined
}

// Only set up MSW in test environment
if (import.meta.env.MODE === 'test') {
  // Dynamic import to avoid Vite processing in dev mode
  ;(async () => {
    const { setupServer } = await import('msw/node')
    const { handlers } = await import('./mocks/handlers')
    
    const server = setupServer(...handlers)
    
    // Make server available globally for tests
    globalThis.mswServer = server
    
    beforeAll(() => server.listen({ onUnhandledRequest: 'error' }))
    afterEach(() => server.resetHandlers())
    afterAll(() => server.close())
  })()
} 