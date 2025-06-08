import { useQuery } from '@tanstack/react-query';
import { api } from '../lib/api';

export function SpotifyPlaylists() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['spotify-playlists'],
    queryFn: () => api.getSpotifyPlaylists(),
  });

  if (isLoading) {
    return (
      <div className="animate-pulse">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {[...Array(6)].map((_, i) => (
            <div key={i} className="bg-gray-200 rounded-lg h-48"></div>
          ))}
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center py-12">
        <p className="text-red-600">Failed to load playlists</p>
        <p className="text-gray-500 mt-2">Please try again later</p>
      </div>
    );
  }

  if (!data || data.items.length === 0) {
    return (
      <div className="text-center py-12">
        <p className="text-gray-500">No playlists found</p>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {data.items.map((playlist) => (
        <div
          key={playlist.id}
          className="bg-white overflow-hidden shadow rounded-lg hover:shadow-lg transition-shadow"
        >
          <div className="p-5">
            {playlist.images[0] && (
              <img
                src={playlist.images[0].url}
                alt={playlist.name}
                className="w-full h-40 object-cover rounded-md mb-4"
              />
            )}
            <h3 className="text-lg font-medium text-gray-900 truncate">
              {playlist.name}
            </h3>
            {playlist.description && (
              <p className="mt-1 text-sm text-gray-500 line-clamp-2">
                {playlist.description}
              </p>
            )}
            <div className="mt-3 flex items-center justify-between text-sm">
              <span className="text-gray-500">
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
          </div>
        </div>
      ))}
    </div>
  );
} 