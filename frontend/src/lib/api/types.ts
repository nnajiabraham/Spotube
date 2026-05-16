// Type definitions for the Spotube backend API

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
    width: number;
    height: number;
  }>;
}

export interface YouTubePlaylist {
  id: string;
  name: string;
  title: string;
  description: string;
  itemCount: number;
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
  last_analysis_at: number | null;
  tracks_count: number;
  created: number;
  updated: number;
}

export interface MappingsResponse {
  items: Mapping[];
  page: number;
  perPage: number;
  totalItems: number;
  totalPages: number;
}

export interface BlacklistEntry {
  id: string;
  mapping_id: string;
  service: 'spotify' | 'youtube';
  track_id: string;
  reason: string;
  skip_counter: number;
  last_skipped_at: number | null;
  created: number;
  updated: number;
}

export interface BlacklistResponse {
  items: BlacklistEntry[];
  page: number;
  perPage: number;
  totalItems: number;
  totalPages: number;
}

export interface ActivityLog {
  id: string;
  level: 'info' | 'warn' | 'error';
  message: string;
  mapping_id?: string;
  job_type: 'analysis' | 'executor' | 'system';
  created: number;
  sync_item_id?: string | null;
}

export interface ActivityLogsResponse {
  items: ActivityLog[];
  page: number;
  perPage: number;
  totalItems: number;
  totalPages: number;
}

export interface DashboardStats {
  mappings: {
    total: number;
  };
  queue: {
    pending: number;
    running: number;
    done: number;
    error: number;
    skipped: number;
  };
  recent_runs: Array<{
    timestamp: number;
    job_type: string;
    level: string;
    message: string;
    mapping_id?: string;
  }>;
  youtube_quota: {
    used: number;
    limit: number;
  };
}

export interface SaveSettingsRequest {
  spotify_client_id: string;
  spotify_client_secret: string;
  google_client_id: string;
  google_client_secret: string;
}

export interface CreateMappingRequest {
  spotify_playlist_id: string;
  youtube_playlist_id: string;
  spotify_playlist_name?: string;
  youtube_playlist_name?: string;
  sync_name?: boolean;
  sync_tracks?: boolean;
  interval_minutes?: number;
}

export interface UpdateMappingRequest {
  spotify_playlist_name?: string;
  youtube_playlist_name?: string;
  sync_name?: boolean;
  sync_tracks?: boolean;
  interval_minutes?: number;
}

export interface CreateBlacklistRequest {
  mapping_id: string;
  service: 'spotify' | 'youtube';
  track_id: string;
  reason?: string;
}
