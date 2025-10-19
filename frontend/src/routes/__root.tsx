import { createRootRoute, Outlet, redirect } from '@tanstack/react-router'
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import { api } from '../lib/api'

// Check if setup is required by calling the backend
async function checkSetupStatus() {
  try {
    const { required } = await api.getSetupStatus()
    return required
  } catch (error) {
    console.error('Error checking setup status:', error)
    // Default to requiring setup if we can't check
    return true
  }
}

const queryClient = new QueryClient()

export const Route = createRootRoute({
  component: () => (
    <QueryClientProvider client={queryClient}>
      <div className="min-h-screen bg-gray-50 font-sans antialiased">
        <Outlet />
        <TanStackRouterDevtools />
        <ReactQueryDevtools />
      </div>
    </QueryClientProvider>
  ),
  beforeLoad: async ({ location }) => {
    const setupRequired = await checkSetupStatus()

    const isSetupRoute = location.pathname.startsWith('/setup')
    if (setupRequired && !isSetupRoute) {
      throw redirect({ to: '/setup' })
    }
    if (!setupRequired && isSetupRoute) {
      throw redirect({ to: '/dashboard' })
    }
  },
}) 