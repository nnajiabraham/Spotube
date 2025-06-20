import { createLazyFileRoute } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../lib/api';

function SpotifyPlaylistsComponent() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['spotify-playlists'],
    queryFn: () => api.getSpotifyPlaylists({ limit: 50 }),
  });

  if (isLoading) {
    return (
      <div className="min-h-screen bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
        <div className="max-w-7xl mx-auto">
          <h1 className="text-2xl font-semibold text-gray-900 mb-8">Your Spotify Playlists</h1>
          <div className="text-center py-12">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-gray-900 mx-auto"></div>
            <p className="mt-4 text-gray-600">Loading playlists...</p>
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="min-h-screen bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
        <div className="max-w-7xl mx-auto">
          <h1 className="text-2xl font-semibold text-gray-900 mb-8">Your Spotify Playlists</h1>
          <div className="text-center py-12">
            <div className="text-red-600 mb-4">
              <svg className="mx-auto h-12 w-12" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <h3 className="text-lg font-medium text-gray-900 mb-2">Error loading playlists</h3>
            <p className="text-gray-600">{error.message}</p>
          </div>
        </div>
      </div>
    );
  }

  const playlists = data?.items || [];

  return (
    <div className="min-h-screen bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-7xl mx-auto">
        <div className="sm:flex sm:items-center">
          <div className="sm:flex-auto">
            <h1 className="text-2xl font-semibold text-gray-900">Your Spotify Playlists</h1>
            <p className="mt-2 text-sm text-gray-700">
              Manage and view your Spotify playlists ({data?.total || 0} total)
            </p>
          </div>
        </div>

        {playlists.length === 0 ? (
          <div className="mt-8 text-center py-12">
            <div className="text-gray-400 mb-4">
              <svg className="mx-auto h-12 w-12" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 10l12-3" />
              </svg>
            </div>
            <h3 className="text-lg font-medium text-gray-900 mb-2">No playlists found</h3>
            <p className="text-gray-600">Create some playlists in Spotify to see them here.</p>
          </div>
        ) : (
          <div className="mt-8 grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {playlists.map((playlist) => (
              <div
                key={playlist.id}
                className="bg-white overflow-hidden shadow rounded-lg hover:shadow-lg transition-shadow"
              >
                <div className="p-6">
                  {playlist.images[0] && (
                    <img
                      src={playlist.images[0].url}
                      alt={playlist.name}
                      className="w-full h-48 object-cover rounded-md mb-4"
                    />
                  )}
                  <h3 className="text-lg font-medium text-gray-900 truncate mb-2">
                    {playlist.name}
                  </h3>
                  {playlist.description && (
                    <p className="text-sm text-gray-500 line-clamp-2 mb-3">
                      {playlist.description}
                    </p>
                  )}
                  <div className="flex items-center justify-between text-sm">
                    <span className="text-gray-600">
                      {playlist.track_count} tracks
                    </span>
                    <span className={`px-2 py-1 rounded-full text-xs font-medium ${
                      playlist.public 
                        ? 'bg-green-100 text-green-800' 
                        : 'bg-gray-100 text-gray-800'
                    }`}>
                      {playlist.public ? 'Public' : 'Private'}
                    </span>
                  </div>
                  <div className="mt-3 text-xs text-gray-500">
                    by {playlist.owner.display_name}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

export const Route = createLazyFileRoute('/settings/spotify')({
  component: SpotifyPlaylistsComponent,
}); 