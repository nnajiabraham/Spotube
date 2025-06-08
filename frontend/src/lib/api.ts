// API client for making requests to the backend

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8090';

export class ApiError extends Error {
  status: number;
  
  constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

async function handleResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const error = await response.text().catch(() => 'Request failed');
    throw new ApiError(response.status, error);
  }
  
  // Handle empty responses
  const contentType = response.headers.get('content-type');
  if (!contentType || !contentType.includes('application/json')) {
    return {} as T;
  }
  
  return response.json();
}

export const api = {
  // Setup API
  getSetupStatus: () =>
    fetch(`${API_BASE_URL}/api/setup/status`)
      .then(response => handleResponse<{ required: boolean }>(response)),
  
  // Spotify API
  getSpotifyPlaylists: (params?: { limit?: number; offset?: number }) => {
    const searchParams = new URLSearchParams();
    if (params?.limit) searchParams.set('limit', params.limit.toString());
    if (params?.offset) searchParams.set('offset', params.offset.toString());
    
    const queryString = searchParams.toString();
    const url = `${API_BASE_URL}/api/spotify/playlists${queryString ? `?${queryString}` : ''}`;
    
    return fetch(url).then(response => 
      handleResponse<{
        items: Array<{
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
        }>;
        total: number;
        limit: number;
        offset: number;
        next: string;
      }>(response)
    );
  },
}; 