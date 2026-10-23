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

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
    },
  },
})

// Floating devtools cover page content, so they're opt-in: VITE_DEVTOOLS=true in frontend/.env.
const showDevtools = import.meta.env.DEV && import.meta.env.VITE_DEVTOOLS === 'true'

const navItems = [
  { to: '/dashboard', label: 'Dashboard' },
  { to: '/mappings', label: 'Mappings' },
  { to: '/sync-queue', label: 'Sync Queue' },
  { to: '/logs', label: 'Activity' },
] as const

function AppShell() {
  const pathname = useRouterState({ select: (state) => state.location.pathname })
  const showNavigation = !pathname.startsWith('/setup')

  const navLinkClass = (target: string) =>
    `rounded-lg px-3 py-1.5 text-sm font-medium transition-colors ${
      pathname === target || pathname.startsWith(`${target}/`)
        ? 'bg-gray-900 text-white'
        : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900'
    }`

  return (
    <QueryClientProvider client={queryClient}>
      <div className="min-h-screen bg-gray-50 font-sans text-gray-900 antialiased">
        {showNavigation && (
          <header className="sticky top-0 z-30 border-b border-gray-200 bg-white/90 backdrop-blur">
            <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-4 sm:px-6 lg:px-8">
              <Link to="/dashboard" className="flex items-center gap-2 font-semibold tracking-tight">
                <img src="/favicon.svg" alt="" className="h-7 w-7" />
                Spotube
              </Link>
              <nav className="flex items-center gap-1">
                {navItems.map((item) => (
                  <Link key={item.to} to={item.to} className={navLinkClass(item.to)}>
                    {item.label}
                  </Link>
                ))}
              </nav>
            </div>
          </header>
        )}
        <Outlet />
        {showDevtools && (
          <>
            <TanStackRouterDevtools />
            <ReactQueryDevtools />
          </>
        )}
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