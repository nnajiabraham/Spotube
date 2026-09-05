import { Link, createLazyFileRoute, useSearch } from '@tanstack/react-router'
import { SpotifyConnectionCard } from '../components/SpotifyConnectionCard'
import { YoutubeConnectionCard } from '../components/YoutubeConnectionCard'
import { DashboardStatsCards } from '../components/DashboardStatsCards'
import { useEffect, useState } from 'react'

function Dashboard() {
  const search = useSearch({ from: '/dashboard' })
  const [isPaused, setIsPaused] = useState(true)
  
  const [authNotice, setAuthNotice] = useState<{ type: 'success' | 'error'; service: string; message: string } | null>(null)

  useEffect(() => {
    if (!search || typeof search !== 'object') {
      return
    }

    const params = search as { spotify?: string; youtube?: string; message?: string }

    if (params.spotify === 'connected') {
      setAuthNotice({ type: 'success', service: 'Spotify', message: 'Connected successfully.' })
    } else if (params.spotify === 'error') {
      setAuthNotice({
        type: 'error',
        service: 'Spotify',
        message: params.message || 'Connection failed. Try again.',
      })
    }

    if (params.youtube === 'connected') {
      setAuthNotice({ type: 'success', service: 'YouTube', message: 'Connected successfully.' })
    } else if (params.youtube === 'error') {
      setAuthNotice({
        type: 'error',
        service: 'YouTube',
        message: params.message || 'Connection failed. Try again.',
      })
    }
  }, [search]);

  const handleTogglePause = () => {
    setIsPaused(prev => !prev)
  }

  const handleRefresh = () => {
    // The actual refresh logic is handled in DashboardStatsCards
    console.log('Dashboard refreshed')
  }

  return (
    <div className="min-h-screen bg-gray-50 py-8 px-4 sm:px-6 lg:px-8">
      <div className="max-w-7xl mx-auto">
        {authNotice && (
          <div
            className={`mb-6 rounded-md p-4 ${
              authNotice.type === 'success'
                ? 'bg-green-50 text-green-800 border border-green-200'
                : 'bg-red-50 text-red-800 border border-red-200'
            }`}
            role="status"
          >
            <p className="font-medium">
              {authNotice.service}: {authNotice.message}
            </p>
            {authNotice.type === 'error' && authNotice.message.includes('state mismatch') && (
              <p className="mt-2 text-sm">
                Use the same host for frontend API calls and OAuth (e.g. both localhost or both 127.0.0.1).
              </p>
            )}
          </div>
        )}

        <div className="text-center mb-8">
          <h1 className="text-4xl font-extrabold text-gray-900">
            Spotube Dashboard
          </h1>
          <p className="mt-4 text-xl text-gray-600">
            Monitor your playlist synchronization system
          </p>
        </div>
        
        {/* Dashboard Stats Cards */}
        <div className="mb-10">
          <h2 className="text-2xl font-semibold text-gray-900 mb-6">System Status</h2>
          <DashboardStatsCards 
            isPaused={isPaused}
            onTogglePause={handleTogglePause}
            onRefresh={handleRefresh}
          />
        </div>

        {/* Service Connection Cards */}
        <div className="mb-10">
          <h2 className="text-2xl font-semibold text-gray-900 mb-6">Service Connections</h2>
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
            <SpotifyConnectionCard />
            <YoutubeConnectionCard />
            
            {/* Mappings Card */}
            <div className="bg-white overflow-hidden shadow rounded-lg">
              <div className="p-5">
                <div className="flex items-center">
                  <div className="flex-shrink-0">
                    <svg className="h-6 w-6 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
                    </svg>
                  </div>
                  <div className="ml-5 w-0 flex-1">
                    <dt className="text-sm font-medium text-gray-500 truncate">
                      Playlist Mappings
                    </dt>
                    <dd className="flex items-baseline">
                      <div className="text-2xl font-semibold text-gray-900">
                        Manage Sync
                      </div>
                    </dd>
                  </div>
                </div>
                <div className="mt-5">
                  <Link
                    to="/mappings"
                    className="w-full flex justify-center py-2 px-4 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
                  >
                    View Mappings
                  </Link>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export const Route = createLazyFileRoute('/dashboard')({
  component: Dashboard,
}) 