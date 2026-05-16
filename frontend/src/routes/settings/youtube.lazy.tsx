import { createLazyFileRoute } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../lib/api';

export const Route = createLazyFileRoute('/settings/youtube')({
  component: YouTubePlaylistsComponent,
});

export function YouTubePlaylistsComponent() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['youtube-playlists'],
    queryFn: api.getYouTubePlaylists,
  });

  if (isLoading) {
    return (
      <div className="py-8 px-4 sm:px-6 lg:px-8">
        <div className="max-w-7xl mx-auto">
          <h2 className="text-2xl font-semibold text-gray-900 mb-4">Your YouTube Playlists</h2>
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
      <div className="py-8 px-4 sm:px-6 lg:px-8">
        <div className="max-w-7xl mx-auto">
          <h2 className="text-2xl font-semibold text-gray-900 mb-4">Your YouTube Playlists</h2>
          <div className="text-red-500">Error fetching playlists: {error.message}</div>
        </div>
      </div>
    );
  }

  const playlists = data || [];

  return (
    <div className="py-8 px-4 sm:px-6 lg:px-8">
      <div className="max-w-7xl mx-auto">
        <h2 className="text-2xl font-semibold text-gray-900 mb-4">Your YouTube Playlists</h2>
        <p className="text-sm text-gray-700 mb-6">{playlists.length} playlists</p>
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
          {playlists.map((playlist) => (
            <div key={playlist.id} className="bg-white p-4 rounded-lg shadow hover:shadow-lg transition-shadow">
              <h3 className="font-bold text-lg">{playlist.title || playlist.name}</h3>
              {playlist.description && (
                <p className="text-sm text-gray-600 truncate">{playlist.description}</p>
              )}
              <p className="text-xs text-gray-500 mt-2">{playlist.itemCount ?? 0} tracks</p>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
