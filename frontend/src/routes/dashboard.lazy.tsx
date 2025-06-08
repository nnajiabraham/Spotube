import { createLazyFileRoute, useSearch } from '@tanstack/react-router'
import { SpotifyConnectionCard } from '../components/SpotifyConnectionCard'
import { YoutubeConnectionCard } from '../components/YoutubeConnectionCard'
import { useEffect } from 'react'

function Dashboard() {
  const search = useSearch({ from: '/dashboard' })
  
  // Show toast notification based on query params
  useEffect(() => {
    if (search && typeof search === 'object' && 'spotify' in search) {
      const spotifyStatus = (search as { spotify?: string }).spotify;
      if (spotifyStatus === 'connected') {
        // In a real app, you'd use a proper toast library
        console.log('Spotify connected successfully!');
      } else if (spotifyStatus === 'error') {
        const message = (search as { message?: string }).message || 'Connection failed';
        console.error('Spotify connection error:', message);
      }
    }
    if (search && typeof search === 'object' && 'youtube' in search) {
      const youtubeStatus = (search as { youtube?: string }).youtube;
      if (youtubeStatus === 'connected') {
        console.log('YouTube connected successfully!');
      } else if (youtubeStatus === 'error') {
        const message = (search as { message?: string }).message || 'Connection failed';
        console.error('YouTube connection error:', message);
      }
    }
  }, [search]);

  return (
    <div className="min-h-screen bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-7xl mx-auto">
        <div className="text-center mb-12">
          <h1 className="text-4xl font-extrabold text-gray-900">
            Welcome to Spotube Dashboard
          </h1>
          <p className="mt-4 text-xl text-gray-600">
            Your music streaming application is ready to use!
          </p>
        </div>
        
        <div className="mt-10 grid grid-cols-1 gap-8 sm:grid-cols-2 lg:grid-cols-3">
          <SpotifyConnectionCard />
          <YoutubeConnectionCard />
        </div>
      </div>
    </div>
  )
}

export const Route = createLazyFileRoute('/dashboard')({
  component: Dashboard,
}) 