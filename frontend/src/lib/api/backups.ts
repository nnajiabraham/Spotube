import { apiClient } from './client';
import type { Service } from './tracks';

export interface SnapshotSummary {
  id: string;
  backup_id: string;
  playlist_name: string;
  track_count: number;
  /** Change against the previous snapshot; absent on the first one. */
  added?: number;
  removed?: number;
  source: 'initial' | 'scheduled' | 'manual' | 'sync';
  /** When the playlist first looked like this. */
  created: number;
  /** The last check that found the playlist unchanged. */
  confirmed_at: number;
}

export interface Backup {
  id: string;
  service: Service;
  playlist_id: string;
  playlist_name: string;
  enabled: boolean;
  interval_hours: number;
  keep_snapshots: number;
  last_checked_at?: number;
  last_error?: string;
  created: number;
  updated: number;
  snapshot_count: number;
  latest_snapshot?: SnapshotSummary;
}

export interface SnapshotTrack {
  position: number;
  id: string;
  title: string;
  artist?: string;
  raw_title?: string;
  channel?: string;
  duration_ms?: number;
  spotify_id?: string;
  youtube_id?: string;
}

export interface Snapshot extends SnapshotSummary {
  tracks: SnapshotTrack[];
  added_tracks: SnapshotTrack[];
  removed_tracks: SnapshotTrack[];
  has_previous: boolean;
}

export type RestoreMode = 'new' | 'missing';

export interface RestoreResult {
  mode: RestoreMode;
  service: Service;
  playlist_id: string;
  playlist_name: string;
  playlist_url: string;
  added: number;
  already_present: number;
  no_counterpart: SnapshotTrack[];
  error?: string;
}

/** YouTube quota a restore would use: 50 per insert, plus the playlist itself or a read. */
export function estimateRestoreCost(service: Service, mode: RestoreMode, tracks: number): number {
  if (service !== 'youtube') return 0;
  if (mode === 'missing') return 2 * (Math.floor(tracks / 50) + 1) + 50 * tracks;
  return 50 * (tracks + 1);
}

export function playlistURL(service: Service, id: string): string {
  return service === 'spotify' ? `https://open.spotify.com/playlist/${id}` : `https://www.youtube.com/playlist?list=${id}`;
}

export const backupsAPI = {
  async list(): Promise<{ items: Backup[] }> {
    return apiClient.get('/api/backups');
  },

  async get(id: string): Promise<Backup> {
    return apiClient.get(`/api/backups/${id}`);
  },

  /** Starts backing up a playlist and takes its first snapshot. */
  async create(data: { service: Service; playlist_id: string; playlist_name?: string; interval_hours?: number }): Promise<Backup> {
    return apiClient.post('/api/backups', data);
  },

  async update(id: string, data: { enabled?: boolean; interval_hours?: number; keep_snapshots?: number }): Promise<Backup> {
    return apiClient.patch(`/api/backups/${id}`, data);
  },

  async remove(id: string): Promise<void> {
    return apiClient.delete(`/api/backups/${id}`);
  },

  /** Backs up now; changed is false when the playlist matched the latest snapshot. */
  async run(id: string): Promise<{ snapshot: SnapshotSummary; changed: boolean }> {
    return apiClient.post(`/api/backups/${id}/run`);
  },

  async snapshots(id: string): Promise<{ items: SnapshotSummary[] }> {
    return apiClient.get(`/api/backups/${id}/snapshots`);
  },

  async snapshot(id: string): Promise<Snapshot> {
    return apiClient.get(`/api/backups/snapshots/${id}`);
  },

  exportURL(id: string, format: 'json' | 'csv'): string {
    return apiClient.buildURL(`/api/backups/snapshots/${id}/export`, { format });
  },

  async restore(id: string, data: { mode: RestoreMode; service?: Service; name?: string }): Promise<RestoreResult> {
    return apiClient.post(`/api/backups/snapshots/${id}/restore`, data);
  },
};
