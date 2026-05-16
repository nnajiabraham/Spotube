import { createLazyFileRoute, useSearch } from '@tanstack/react-router'
import { SpotifyConnectionCard } from '../components/SpotifyConnectionCard'
import { YoutubeConnectionCard } from '../components/YoutubeConnectionCard'
import { DashboardStatsCards } from '../components/DashboardStatsCards'
import { useEffect, useState } from 'react'

function Dashboard() {
  const search = useSearch({ from: '/dashboard' })
  const [isPaused, setIsPaused] = useState(false)
  const [toast, setToast] = useState<{ type: 'success' | 'error'; message: string } | null>(null)
  
  useEffect(() => {
    if (search && typeof search === 'object') {
      if ('spotify' in search) {
        const status = (search as { spotify?: string }).spotify
        if (status === 'connected') {
          setToast({ type: 'success', message: 'Spotify connected successfully!' })
        } else if (status === 'error') {
          setToast({ type: 'error', message: (search as { message?: string }).message || 'Spotify connection failed' })
        }
      }
      if ('youtube' in search) {
        const status = (search as { youtube?: string }).youtube
        if (status === 'connected') {
          setToast({ type: 'success', message: 'YouTube connected successfully!' })
        } else if (status === 'error') {
          setToast({ type: 'error', message: (search as { message?: string }).message || 'YouTube connection failed' })
        }
      }
    }
  }, [search])

  useEffect(() => {
    if (toast) {
      const timer = setTimeout(() => setToast(null), 5000)
      return () => clearTimeout(timer)
    }
  }, [toast])

  return (
    <div className="py-8 px-4 sm:px-6 lg:px-8">
      <div className="max-w-7xl mx-auto">
        {toast && (
          <div className={`mb-6 rounded-md p-4 ${
            toast.type === 'success' ? 'bg-green-50 text-green-800' : 'bg-red-50 text-red-800'
          }`}>
            {toast.message}
          </div>
        )}

        <div className="mb-10">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">System Status</h2>
          <DashboardStatsCards 
            isPaused={isPaused}
            onTogglePause={() => setIsPaused(prev => !prev)}
            onRefresh={() => {}}
          />
        </div>

        <div>
          <h2 className="text-lg font-semibold text-gray-900 mb-4">Service Connections</h2>
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
            <SpotifyConnectionCard />
            <YoutubeConnectionCard />
          </div>
        </div>
      </div>
    </div>
  )
}

export const Route = createLazyFileRoute('/dashboard')({
  component: Dashboard,
})
