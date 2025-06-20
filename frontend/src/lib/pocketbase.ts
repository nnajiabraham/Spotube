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

export interface Mapping {
  id: string;
  spotify_playlist_id: string;
  youtube_playlist_id: string;
  spotify_playlist_name: string;
  youtube_playlist_name: string;
  sync_name: boolean;
  sync_tracks: boolean;
  interval_minutes: number;
  created: string;
  updated: string;
}

export interface MappingsResponse {
  page: number;
  perPage: number;
  totalItems: number;
  totalPages: number;
  items: Mapping[];
}

export interface BlacklistEntry {
  id: string;
  mapping_id: string;
  service: 'spotify' | 'youtube';
  track_id: string;
  reason: string;
  skip_counter: number;
  last_skipped_at: string;
  created: string;
  updated: string;
}

export interface BlacklistResponse {
  page: number;
  perPage: number;
  totalItems: number;
  totalPages: number;
  items: BlacklistEntry[];
} 