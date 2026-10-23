import { apiClient } from './client';

export type Service = 'spotify' | 'youtube';

/** How a candidate compared with the source track (matching.Score on the backend). */
export interface ScoreBreakdown {
  total: number;
  title: number;
  artist: number;
  artist_known: boolean;
  duration_diff_sec?: number;
  adjustments?: Array<{ reason: string; delta: number }>;
}

export interface SearchHit {
  id: string;
  title: string;
  artist?: string;
  channel?: string;
  album?: string;
  raw_title?: string;
  duration_ms?: number;
  image?: string;
  /** Set when the search was scored against a sync item's source track. */
  score?: number;
  breakdown?: ScoreBreakdown;
}

export interface SearchResponse {
  items: SearchHit[];
  cached: boolean;
  /** YouTube quota units this search spent (0 for Spotify and cached results). */
  quota_cost: number;
}

export interface TrackLink {
  id: string;
  /** Empty when the song is known not to be on Spotify. */
  spotify_id?: string;
  /** Empty when the song is known not to be on YouTube. */
  youtube_id?: string;
  title?: string;
  artist?: string;
  origin: 'auto' | 'manual';
  score?: number;
  created: number;
  updated: number;
}

export type TrackLinkFilter = '' | 'linked' | 'missing_spotify' | 'missing_youtube';

export interface TrackLinksResponse {
  items: TrackLink[];
  page: number;
  perPage: number;
  totalItems: number;
  totalPages: number;
}

/** A YouTube search costs 100 units plus 1 to fetch durations. */
export const YOUTUBE_SEARCH_COST = 101;

export const tracksAPI = {
  search(service: Service, q: string, itemId?: string): Promise<SearchResponse> {
    return apiClient.get(apiClient.buildURL('/api/tracks/search', { service, q, item_id: itemId ?? '' }));
  },

  /** Resolve a pasted link, URI or ID. Costs 1 YouTube unit; Spotify is free. */
  lookup(service: Service, ref: string, itemId?: string): Promise<SearchHit> {
    return apiClient.get(apiClient.buildURL('/api/tracks/lookup', { service, ref, item_id: itemId ?? '' }));
  },

  listLinks(options: { filter?: TrackLinkFilter; q?: string; page?: number; per_page?: number } = {}): Promise<TrackLinksResponse> {
    return apiClient.get(
      apiClient.buildURL('/api/track-links', {
        filter: options.filter ?? '',
        q: options.q ?? '',
        page: options.page ?? 1,
        per_page: options.per_page ?? 50,
      }),
    );
  },

  deleteLink(id: string): Promise<void> {
    return apiClient.delete(`/api/track-links/${id}`);
  },
};
