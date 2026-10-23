import { Link, createRootRoute, Outlet, redirect, useRouterState } from '@tanstack/react-router'
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools'
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import { setupAPI } from '../lib/api/setup'
import { syncItemsAPI } from '../lib/api/sync-items'
import { dashboardAPI } from '../lib/api/dashboard'
import { backupsAPI } from '../lib/api/backups'
import { CountBadge, SpotubeMark } from '../components/ui'

// Check if setup is required by calling the backend
async function checkSetupStatus() {
  try {
    const { required } = await setupAPI.isRequired()
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
  { to: '/dashboard', label: 'dashboard' },
  { to: '/mappings', label: 'mappings' },
  { to: '/review', label: 'review' },
  { to: '/sync-queue', label: 'queue' },
  { to: '/matches', label: 'matches' },
  { to: '/backups', label: 'backups' },
  { to: '/logs', label: 'activity' },
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
    `inline-flex items-center rounded-md px-2.5 py-1 text-[13px] transition-colors ${
      pathname === target || pathname.startsWith(`${target}/`)
        ? 'bg-ink text-white'
        : 'text-gray-500 hover:bg-gray-100 hover:text-ink'
    }`

  return (
    <header className="sticky top-0 z-30 border-b border-gray-200 bg-canvas/85 backdrop-blur">
      {/* Brand rule: Spotify green into YouTube red. */}
      <div className="h-[3px] bg-gradient-to-r from-spotify via-amber-200 to-youtube" aria-hidden="true" />
      <div className="mx-auto flex h-12 max-w-7xl items-center justify-between gap-4 px-4 sm:px-6 lg:px-8">
        <Link to="/dashboard" className="flex shrink-0 items-center gap-2 text-[15px] font-bold tracking-tight text-ink">
          <SpotubeMark className="h-6 w-6" />
          spotube
          <span className="hidden rounded border border-gray-300 px-1 text-[10px] font-medium text-gray-500 sm:inline">
            sp⇄yt
          </span>
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
    </header>
  )
}

/** One-line status at the bottom of every page: queue, quota and backups at a glance. */
function StatusBar() {
  const stats = useQuery({ queryKey: ['dashboard-stats'], queryFn: dashboardAPI.getStats, refetchInterval: 60_000 })
  const backups = useQuery({ queryKey: ['backups'], queryFn: () => backupsAPI.list().then((r) => r.items) })
  const q = stats.data?.queue
  const quota = stats.data?.youtube_quota
  const pct = quota && quota.limit ? Math.min(100, (quota.used / quota.limit) * 100) : 0
  const failing = backups.data?.filter((b) => b.last_error).length ?? 0
  const sep = <span className="text-gray-300">│</span>

  return (
    <footer className="sticky bottom-0 z-20 border-t border-gray-200 bg-canvas/90 backdrop-blur">
      <div className="mx-auto flex h-7 max-w-7xl items-center gap-3 overflow-x-auto whitespace-nowrap px-4 text-[11px] text-gray-500 sm:px-6 lg:px-8">
        <span className="inline-flex items-center gap-1.5">
          <span className={`h-1.5 w-1.5 rounded-full ${stats.error ? 'bg-youtube' : 'bg-spotify'}`} />
          {stats.error ? 'api offline' : 'api ok'}
        </span>
        {sep}
        <Link to="/review" className="hover:text-ink">
          review <span className={q?.review ? 'font-semibold text-amber-700' : 'text-ink'}>{q?.review ?? '–'}</span>
        </Link>
        <Link to="/sync-queue" search={{ status: 'pending' }} className="hover:text-ink">
          pending <span className="text-ink">{q?.pending ?? '–'}</span>
        </Link>
        <Link to="/sync-queue" search={{ status: 'error' }} className="hover:text-ink">
          failed <span className={q?.error ? 'font-semibold text-youtube-700' : 'text-ink'}>{q?.error ?? '–'}</span>
        </Link>
        {sep}
        <span className="inline-flex items-center gap-1.5" title="YouTube quota used today (resets midnight Pacific)">
          yt quota
          <span className="inline-block h-1 w-16 overflow-hidden rounded-full bg-gray-200">
            <span
              className={`block h-full ${pct > 80 ? 'bg-youtube' : pct > 50 ? 'bg-amber-400' : 'bg-spotify'}`}
              style={{ width: `${pct}%` }}
            />
          </span>
          <span className="tabular-nums text-ink">{quota ? quota.used.toLocaleString() : '–'}</span>/
          {(quota?.limit ?? 10000).toLocaleString()}
        </span>
        {sep}
        <Link to="/backups" className="hover:text-ink">
          backups <span className="text-ink">{backups.data?.length ?? '–'}</span>
          {failing > 0 && <span className="text-youtube-700"> ({failing} failing)</span>}
        </Link>
      </div>
    </footer>
  )
}

function AppShell() {
  const pathname = useRouterState({ select: (state) => state.location.pathname })
  const showNavigation = !pathname.startsWith('/setup')

  return (
    <QueryClientProvider client={queryClient}>
      <div className="flex min-h-screen flex-col font-sans text-ink antialiased">
        {showNavigation && <Header pathname={pathname} />}
        <div className="flex-1">
          <Outlet />
        </div>
        {showNavigation && <StatusBar />}
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