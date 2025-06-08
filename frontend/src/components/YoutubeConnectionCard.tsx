import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { api, ApiError } from '../lib/api';
import { YoutubeLogo } from './YoutubeLogo';

// YouTube Connection Card Component
export function YoutubeConnectionCard() {
  const { isLoading, error } = useQuery({
    queryKey: ['youtube-connection'],
    queryFn: () => api.getYouTubePlaylists(),
    retry: (failureCount, error) => {
      if (error instanceof ApiError && error.status === 401) {
        return false;
      }
      return failureCount < 3;
    },
  });

  const isConnected = !error;

  const renderContent = () => {
    if (isLoading) {
      return (
        <div className="animate-pulse" role="status">
          <div className="h-4 bg-gray-200 rounded w-3/4"></div>
          <div className="mt-2 h-4 bg-gray-200 rounded w-1/2"></div>
        </div>
      );
    }

    if (isConnected) {
      return (
        <>
          <h3 className="text-lg leading-6 font-medium text-gray-900 flex items-center">
            <YoutubeLogo className="h-6 w-6 mr-2" />
            YouTube Connected
          </h3>
          <div className="mt-2 max-w-xl text-sm text-gray-500">
            <p>Your YouTube account is connected and ready to sync.</p>
          </div>
          <div className="mt-5">
            <Link
              to="/settings/youtube"
              className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-red-600 hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500"
            >
              View Playlists
            </Link>
          </div>
        </>
      );
    }

    return (
      <>
        <h3 className="text-lg leading-6 font-medium text-gray-900 flex items-center">
          <YoutubeLogo className="h-6 w-6 mr-2" />
          Connect YouTube
        </h3>
        <div className="mt-2 max-w-xl text-sm text-gray-500">
          <p>Connect your YouTube account to start syncing your playlists.</p>
        </div>
        <div className="mt-5">
          <a
            href="/api/auth/google/login"
            className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-red-600 hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500"
          >
            Connect YouTube
          </a>
        </div>
      </>
    );
  };

  return (
    <div className="bg-white overflow-hidden shadow rounded-lg">
      <div className="px-4 py-5 sm:p-6">{renderContent()}</div>
    </div>
  );
} 