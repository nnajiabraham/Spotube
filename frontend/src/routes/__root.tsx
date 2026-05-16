import { createRootRoute, Outlet, redirect, useRouterState } from '@tanstack/react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { api } from '../lib/api'
import { AppNav } from '../components/AppNav'

async function checkSetupStatus() {
  try {
    const { required } = await api.getSetupStatus()
    return required
  } catch (error) {
    console.error('Error checking setup status:', error)
    return true
  }
}

const queryClient = new QueryClient()

function RootLayout() {
  const routerState = useRouterState()
  const isSetupRoute = routerState.location.pathname.startsWith('/setup')

  return (
    <QueryClientProvider client={queryClient}>
      <div className="min-h-screen bg-gray-50 font-sans antialiased">
        {!isSetupRoute && <AppNav />}
        <Outlet />
      </div>
    </QueryClientProvider>
  )
}

export const Route = createRootRoute({
  component: RootLayout,
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
