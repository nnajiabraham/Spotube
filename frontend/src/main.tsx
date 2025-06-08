import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider, createRouter } from '@tanstack/react-router'
import './index.css'

// Import the generated route tree
import { routeTree } from './routeTree.gen'

// Create a new router instance
const router = createRouter({ routeTree })

// Register the router instance for type safety
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

// Enable MSW for Vitest tests, but not for development or production
async function enableMocking() {
  // We only want to enable MSW for 'test' mode (Vitest),
  // not for 'development' or 'production'.
  if (import.meta.env.MODE !== 'test') {
    return
  }

  const { worker } = await import('./test/mocks/browser')

  // Start the worker. We can leave the options empty here because
  // onUnhandledRequest is configured in `src/test/setup.ts` for Vitest.
  return worker.start()
}

enableMocking().then(() => {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <RouterProvider router={router} />
    </StrictMode>,
  )
})
