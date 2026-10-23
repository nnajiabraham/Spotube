// Type definitions for the HTTP API

export interface SetupStatus {
  required: boolean;
}

export interface SpotifyPlaylist {
  id: string;
  name: string;
  images?: Array<{
    url: string;
    width?: number;
    height?: number;
  }>;
  description?: string | null;
  track_count?: number;
  public?: boolean;
  owner?: {
    id?: string;
    display_name?: string;
  };
}

export interface YouTubePlaylist {
  id: string;
  name: string;
  title?: string;
  description?: string | null;
  itemCount?: number;
}

/** When a mapping syncs: every interval_minutes from schedule_start (HH:MM in schedule_tz) on
 * schedule_days (bitmask, Sunday = 1 ... Saturday = 64), starting again each day. */
export interface MappingSchedule {
  interval_minutes: number;
  schedule_start: string;
  schedule_days: number;
  /** IANA zone; empty means the server's. */
  schedule_tz?: string;
}

export interface QueueCounts {
  pending: number;
  review: number;
  error: number;
  done: number;
  skipped: number;
}

/** ok | error | reconnect_required | waiting_quota | playlist_missing | playlist_full */
export type MappingSyncStatus = string;

export interface Mapping extends MappingSchedule {
  id: string;
  spotify_playlist_id: string;
  youtube_playlist_id: string;
  spotify_playlist_name: string;
  youtube_playlist_name: string;
  sync_name: boolean;
  sync_tracks: boolean;
  /** Next scheduled run (epoch seconds); null until scheduled. */
  next_run_at?: number | null;
  last_analysis_at: number | null; // epoch seconds
  last_sync_status?: MappingSyncStatus;
  last_sync_error?: string;
  /** The larger side; kept for older clients. Prefer the per-side counts. */
  tracks_count: number;
  spotify_tracks_count?: number;
  youtube_tracks_count?: number;
  queue?: QueueCounts;
  /** A sync for this mapping is running or queued. */
  syncing?: boolean;
  created: number; // epoch seconds
  updated: number; // epoch seconds
}

export interface ScheduleRecommendation {
  schedule: MappingSchedule;
  /** Daily YouTube read units of all mappings before and after adding this one. */
  reads_before: number;
  reads_after: number;
  reason: string;
  warning?: string;
}

export interface DeleteMappingResult {
  deleted: boolean;
  playlists: Array<{ service: 'spotify' | 'youtube'; deleted: boolean; error?: string }>;
}

export interface MirrorRequest {
  source: 'spotify' | 'youtube';
  playlists: Array<{ id: string; name: string; tracks: number }>;
  sync_name?: boolean;
  sync_tracks?: boolean;
  sync_now?: boolean;
  schedule_tz?: string;
}

export interface MirrorResult {
  playlist_id: string;
  name: string;
  status: 'created' | 'already_paired' | 'waiting_quota' | 'failed';
  error?: string;
  mapping?: Mapping;
}

export interface AppSetting {
  key: string;
  env?: string;
  kind: 'bool' | 'int';
  default: string;
  min?: number;
  max?: number;
  label: string;
  description: string;
  group: 'sync' | 'quota' | 'backups' | string;
  value: string;
  /** default | env | database | runtime (changed here, env value returns on restart) */
  source: 'default' | 'env' | 'database' | 'runtime';
  env_pinned: boolean;
  env_value?: string;
}

export interface QuotaBacklog {
  youtube_unlinked: number;
  youtube_linked: number;
  youtube_renames: number;
  spotify: number;
  units: number;
}

export interface QuotaStatus {
  day: string;
  used: number;
  limit: number;
  /** Kept for interactive work (searches, Sync now). */
  reserve: number;
  /** Units scheduled syncs read per day, and the part still to come today. */
  daily_reads: number;
  reads_still_expected: number;
  /** Left for interactive work today. */
  available: number;
  /** Left for applying the queue today. */
  background_available: number;
  exhausted: boolean;
  retry_at?: number;
  reset_at: number;
  backlog: QuotaBacklog;
  daily_capacity: number;
  /** Quota resets until the queue is done: 0 today, -1 never. */
  days_to_drain: number;
}

export interface EstimatePair {
  spotify_playlist_id?: string;
  spotify_tracks: number;
  youtube_videos: number;
  create?: 'spotify' | 'youtube';
}

export interface QuotaEstimate {
  units: number;
  search_units: number;
  insert_units: number;
  read_units: number;
  create_units: number;
  backlog_units: number;
  daily_capacity: number;
  /** 0 today, -1 never. */
  days: number;
  work: {
    to_youtube: number;
    to_youtube_linked: number;
    to_spotify: number;
    youtube_read_videos: number;
    youtube_playlists: number;
  };
  /** Songs already on both sides aren't known yet; the real cost may be lower. */
  upper_bound: boolean;
  /** Spotify playlists were read to count songs already matched. */
  exact: boolean;
  over_youtube_limit: boolean;
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
  /** From the track's queue row, when there is one. */
  track_title?: string;
  track_artist?: string;
  skip_counter: number;
  last_skipped_at: number | null; // epoch seconds
  created: number; // epoch seconds
  updated: number; // epoch seconds
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
  created: number; // epoch seconds
  updated?: number | null;
  sync_item_id?: string | null;
  details_json?: Record<string, unknown> | null;
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
    /** Adds with no confident match, waiting for the user. */
    review: number;
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

// Request types
export interface SaveSettingsRequest {
  spotify_client_id: string;
  spotify_client_secret: string;
  google_client_id: string;
  google_client_secret: string;
}

export interface CreateMappingRequest {
  /** Empty when `create_playlist` is "spotify". */
  spotify_playlist_id: string;
  /** Empty when `create_playlist` is "youtube". */
  youtube_playlist_id: string;
  spotify_playlist_name?: string;
  youtube_playlist_name?: string;
  sync_name?: boolean;
  sync_tracks?: boolean;
  interval_minutes?: number;
  schedule_start?: string;
  schedule_days?: number;
  schedule_tz?: string;
  /** Create this side's playlist (named by its *_playlist_name, else the other side's name). */
  create_playlist?: 'spotify' | 'youtube';
  /** Start the first sync straight away (default: the "Sync new mappings" setting). */
  sync_now?: boolean;
}

export interface UpdateMappingRequest {
  spotify_playlist_name?: string;
  youtube_playlist_name?: string;
  sync_name?: boolean;
  sync_tracks?: boolean;
  interval_minutes?: number;
  schedule_start?: string;
  schedule_days?: number;
  schedule_tz?: string;
}

export interface CreateBlacklistRequest {
  mapping_id: string;
  service: 'spotify' | 'youtube';
  track_id: string;
  reason?: string;
}
