import PocketBase from 'pocketbase';

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8090';

export const pb = new PocketBase(API_BASE_URL);

// Optional: Enable auto-cancellation for React
pb.autoCancellation(false); // We'll use React Query for cancellation

// Type definitions for our collections
export interface SetupStatus {
  required: boolean;
}

export interface SpotifyPlaylist {
  id: string;
  name: string;
  description: string;
  public: boolean;
  track_count: number;
  owner: {
    id: string;
    display_name: string;
  };
  images: Array<{
    url: string;
    height: number;
    width: number;
  }>;
}

export interface PlaylistsResponse {
  items: SpotifyPlaylist[];
  total: number;
  limit: number;
  offset: number;
  next: string;
}

export interface YouTubePlaylist {
  id: string;
  title: string;
  itemCount: number;
  description: string;
}

export interface YouTubePlaylistsResponse {
  items: YouTubePlaylist[];
} 