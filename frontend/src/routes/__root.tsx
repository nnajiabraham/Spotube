import { Link, createRootRoute, Outlet, redirect, useRouterState } from '@tanstack/react-router'
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

function AppShell() {
  const pathname = useRouterState({ select: (state) => state.location.pathname })
  const showNavigation = !pathname.startsWith('/setup')

  const navLinkClass = (target: string) =>
    `px-3 py-2 rounded-md text-sm font-medium ${
      pathname === target || pathname.startsWith(`${target}/`)
        ? 'bg-indigo-100 text-indigo-800'
        : 'text-gray-700 hover:bg-gray-100'
    }`

  return (
    <QueryClientProvider client={queryClient}>
      <div className="min-h-screen bg-gray-50 font-sans antialiased">
        {showNavigation && (
          <header className="border-b border-gray-200 bg-white">
            <div className="mx-auto flex max-w-7xl items-center justify-between px-4 py-3 sm:px-6 lg:px-8">
              <Link to="/dashboard" className="text-lg font-semibold text-gray-900">
                Spotube
              </Link>
              <nav className="flex items-center gap-1">
                <Link to="/dashboard" className={navLinkClass('/dashboard')}>
                  Dashboard
                </Link>
                <Link to="/mappings" className={navLinkClass('/mappings')}>
                  Mappings
                </Link>
                <Link to="/sync-queue" className={navLinkClass('/sync-queue')}>
                  Sync Queue
                </Link>
                <Link to="/settings/spotify" className={navLinkClass('/settings/spotify')}>
                  Spotify
                </Link>
                <Link to="/settings/youtube" className={navLinkClass('/settings/youtube')}>
                  YouTube
                </Link>
                <Link to="/logs" className={`${navLinkClass('/logs')} text-xs`}>
                  Logs (advanced)
                </Link>
              </nav>
            </div>
          </header>
        )}
        <Outlet />
        <TanStackRouterDevtools />
        <ReactQueryDevtools />
      </div>
    </QueryClientProvider>
  )
}

export const Route = createRootRoute({
  component: AppShell,
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