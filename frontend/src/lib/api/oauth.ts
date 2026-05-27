import { apiClient } from './client';
import type { SpotifyPlaylist, YouTubePlaylist } from './types';

// OAuth login must hit the same host as backend PUBLIC_URL (cookie + provider redirect_uri).
const API_BASE_URL =
  import.meta.env.VITE_PUBLIC_URL ||
  import.meta.env.VITE_API_URL ||
  'http://localhost:8090';

export const oauthAPI = {
  spotify: {
    // Initiate Spotify OAuth login
    getLoginURL(): string {
      return `${API_BASE_URL}/api/auth/spotify/login`;
    },

    // Get user's Spotify playlists
    async getPlaylists(): Promise<SpotifyPlaylist[]> {
      return apiClient.get<SpotifyPlaylist[]>('/api/auth/spotify/playlists');
    },
  },

  youtube: {
    // Initiate YouTube OAuth login  
    getLoginURL(): string {
      return `${API_BASE_URL}/api/auth/youtube/login`;
    },

    // Get user's YouTube playlists
    async getPlaylists(): Promise<YouTubePlaylist[]> {
      return apiClient.get<YouTubePlaylist[]>('/api/auth/youtube/playlists');
    },
  },
};
