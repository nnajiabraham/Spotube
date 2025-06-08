import {
  createLazyFileRoute
} from '@tanstack/react-router';
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
      <div className="p-4">
        <h2 className="text-xl font-bold mb-4">Your YouTube Playlists</h2>
        <div>Loading...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-4">
        <h2 className="text-xl font-bold mb-4">Your YouTube Playlists</h2>
        <div className="text-red-500">Error fetching playlists: {error.message}</div>
      </div>
    );
  }

  return (
    <div className="p-4">
      <h2 className="text-xl font-bold mb-4">Your YouTube Playlists</h2>
      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
        {data?.items.map((playlist) => (
          <div key={playlist.id} className="bg-white p-4 rounded-lg shadow">
            <h3 className="font-bold text-lg">{playlist.title}</h3>
            <p className="text-sm text-gray-600 truncate">{playlist.description}</p>
            <p className="text-xs text-gray-500 mt-2">{playlist.itemCount} tracks</p>
          </div>
        ))}
      </div>
    </div>
  );
} 