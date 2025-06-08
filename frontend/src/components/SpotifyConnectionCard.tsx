import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { api, ApiError } from '../lib/api';

export function SpotifyConnectionCard() {
  // Check if user is connected by trying to fetch playlists
  const { isLoading, error } = useQuery({
    queryKey: ['spotify-connection'],
    queryFn: () => api.getSpotifyPlaylists({ limit: 1 }),
    retry: false,
  });

  const isConnected = !error || (error instanceof ApiError && error.status !== 401);

  if (isLoading) {
    return (
      <div className="bg-white overflow-hidden shadow rounded-lg">
        <div className="px-4 py-5 sm:p-6">
          <div className="animate-pulse" role="status">
            <div className="h-4 bg-gray-200 rounded w-3/4"></div>
            <div className="mt-2 h-4 bg-gray-200 rounded w-1/2"></div>
          </div>
        </div>
      </div>
    );
  }

  if (isConnected) {
    return (
      <div className="bg-white overflow-hidden shadow rounded-lg">
        <div className="px-4 py-5 sm:p-6">
          <h3 className="text-lg leading-6 font-medium text-gray-900">
            Spotify Connected
          </h3>
          <div className="mt-2 max-w-xl text-sm text-gray-500">
            <p>Your Spotify account is connected and ready to sync.</p>
          </div>
          <div className="mt-5">
            <Link
              to="/dashboard"
              className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-green-600 hover:bg-green-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-green-500"
            >
              View Playlists
            </Link>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-white overflow-hidden shadow rounded-lg">
      <div className="px-4 py-5 sm:p-6">
        <h3 className="text-lg leading-6 font-medium text-gray-900">
          Connect Spotify
        </h3>
        <div className="mt-2 max-w-xl text-sm text-gray-500">
          <p>Connect your Spotify account to start syncing your playlists.</p>
        </div>
        <div className="mt-5">
          <a
            href="/api/auth/spotify/login"
            className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-green-600 hover:bg-green-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-green-500"
          >
            Connect Spotify
          </a>
        </div>
      </div>
    </div>
  );
} 