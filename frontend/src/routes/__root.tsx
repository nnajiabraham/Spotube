import { Link, createRootRoute, Outlet, redirect, useRouterState } from '@tanstack/react-router'
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools'
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import { api } from '../lib/api'
import { syncItemsAPI } from '../lib/api/sync-items'
import { CountBadge, SpotubeMark } from '../components/ui'

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
  { to: '/review', label: 'Review' },
  { to: '/sync-queue', label: 'Sync Queue' },
  { to: '/matches', label: 'Matches' },
  { to: '/logs', label: 'Activity' },
] as const

/** Number of tracks waiting in Review, shown as a badge in the nav. */
function useReviewCount() {
  const { data } = useQuery({
    queryKey: ['review-count'],
    queryFn: () => syncItemsAPI.getList({ status: 'review', per_page: 1 }).then((r) => r.totalItems),
    refetchInterval: 60_000,
  })
  return data
}

function Header({ pathname }: { pathname: string }) {
  const reviewCount = useReviewCount()
  const navLinkClass = (target: string) =>
    `inline-flex items-center rounded-lg px-3 py-1.5 text-sm font-medium transition-colors ${
      pathname === target || pathname.startsWith(`${target}/`)
        ? 'bg-white/15 text-white'
        : 'text-gray-400 hover:bg-white/10 hover:text-white'
    }`

  return (
    <header className="sticky top-0 z-30 bg-ink text-white">
      <div className="mx-auto flex h-14 max-w-7xl items-center justify-between gap-4 px-4 sm:px-6 lg:px-8">
        <Link to="/dashboard" className="flex shrink-0 items-center gap-2 font-semibold tracking-tight">
          <SpotubeMark className="h-7 w-7 rounded-lg ring-1 ring-white/15" />
          Spotube
        </Link>
        <nav className="-mr-2 flex items-center gap-0.5 overflow-x-auto" aria-label="Main">
          {navItems.map((item) => (
            <Link key={item.to} to={item.to} className={navLinkClass(item.to)}>
              {item.label}
              {item.to === '/review' && <CountBadge count={reviewCount} />}
            </Link>
          ))}
        </nav>
      </div>
      {/* Brand rule: Spotify green into YouTube red. */}
      <div className="h-0.5 bg-gradient-to-r from-spotify via-ink-500 to-youtube" aria-hidden="true" />
    </header>
  )
}

function AppShell() {
  const pathname = useRouterState({ select: (state) => state.location.pathname })
  const showNavigation = !pathname.startsWith('/setup')

  return (
    <QueryClientProvider client={queryClient}>
      <div className="min-h-screen bg-gray-50 font-sans text-ink antialiased">
        {showNavigation && <Header pathname={pathname} />}
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