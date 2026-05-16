import { Link, useRouterState } from '@tanstack/react-router'
import { LayoutDashboard, ArrowLeftRight, Music, FileText } from 'lucide-react'

const primaryLinks = [
  { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/mappings', label: 'Mappings', icon: ArrowLeftRight },
  { to: '/settings/spotify', label: 'Spotify', icon: Music },
  { to: '/settings/youtube', label: 'YouTube', icon: Music },
] as const

const secondaryLinks = [
  { to: '/logs', label: 'Logs', icon: FileText },
] as const

export function AppNav() {
  const routerState = useRouterState()
  const currentPath = routerState.location.pathname

  const isActive = (to: string) => {
    if (to === '/dashboard') return currentPath === '/dashboard'
    return currentPath.startsWith(to)
  }

  return (
    <nav className="bg-white border-b border-gray-200">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex h-14 items-center justify-between">
          <div className="flex items-center space-x-1">
            <Link
              to="/dashboard"
              className="text-lg font-bold text-gray-900 mr-6 hover:text-indigo-600 transition-colors"
            >
              Spotube
            </Link>

            {primaryLinks.map(({ to, label, icon: Icon }) => (
              <Link
                key={to}
                to={to}
                className={`inline-flex items-center px-3 py-2 text-sm font-medium rounded-md transition-colors ${
                  isActive(to)
                    ? 'bg-indigo-50 text-indigo-700'
                    : 'text-gray-600 hover:text-gray-900 hover:bg-gray-50'
                }`}
              >
                <Icon className="h-4 w-4 mr-1.5" />
                {label}
              </Link>
            ))}
          </div>

          <div className="flex items-center space-x-1">
            {secondaryLinks.map(({ to, label, icon: Icon }) => (
              <Link
                key={to}
                to={to}
                className={`inline-flex items-center px-3 py-2 text-sm font-medium rounded-md transition-colors ${
                  isActive(to)
                    ? 'bg-gray-100 text-gray-900'
                    : 'text-gray-400 hover:text-gray-600 hover:bg-gray-50'
                }`}
              >
                <Icon className="h-4 w-4 mr-1.5" />
                {label}
              </Link>
            ))}
          </div>
        </div>
      </div>
    </nav>
  )
}
