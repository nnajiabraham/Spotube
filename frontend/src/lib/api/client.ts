// HTTP client for Spotube backend API communication

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8090';

export interface APIErrorResponse {
  error: {
    code: string;
    message: string;
  };
}

export class APIClient {
  private baseURL: string;

  constructor(baseURL = API_BASE_URL) {
    this.baseURL = baseURL;
  }

  private async fetch(endpoint: string, options: RequestInit = {}): Promise<Response> {
    const url = endpoint.startsWith('http') ? endpoint : `${this.baseURL}${endpoint}`;
    
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string> || {}),
    };

    const config: RequestInit = {
      ...options,
      headers,
      credentials: 'include',
    };

    const response = await fetch(url, config);

    if (!response.ok) {
      const errorData = await response.json().catch(() => null);
      if (errorData && errorData.error) {
        throw new APIClientError(errorData.error.code, errorData.error.message);
      }
      throw new APIClientError('http_error', `HTTP ${response.status}: ${response.statusText}`);
    }

    return response;
  }

  async get<T>(endpoint: string): Promise<T> {
    const response = await this.fetch(endpoint, { method: 'GET' });
    return response.json();
  }

  async post<T>(endpoint: string, data?: unknown): Promise<T> {
    const response = await this.fetch(endpoint, {
      method: 'POST',
      body: data ? JSON.stringify(data) : undefined,
    });
    
    if (response.status === 204) {
      return null as T;
    }
    
    return response.json();
  }

  async patch<T>(endpoint: string, data: unknown): Promise<T> {
    const response = await this.fetch(endpoint, {
      method: 'PATCH',
      body: JSON.stringify(data),
    });
    return response.json();
  }

  async delete(endpoint: string): Promise<void> {
    await this.fetch(endpoint, { method: 'DELETE' });
  }

  buildURL(endpoint: string, params: Record<string, string | number | boolean> = {}): string {
    const url = new URL(endpoint.startsWith('http') ? endpoint : `${this.baseURL}${endpoint}`);
    
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== null && value !== '') {
        url.searchParams.append(key, String(value));
      }
    });
    
    return url.toString();
  }
}

export class APIClientError extends Error {
  constructor(public code: string, message: string) {
    super(message);
    this.name = 'APIClientError';
  }
}

export const DateUtils = {
  toEpoch(date: Date | string): number {
    return Math.floor(new Date(date).getTime() / 1000);
  },

  fromEpoch(timestamp: number): Date {
    return new Date(timestamp * 1000);
  },

  format(timestamp: number | Date, options: Intl.DateTimeFormatOptions = {}): string {
    const date = typeof timestamp === 'number' ? this.fromEpoch(timestamp) : timestamp;
    return new Intl.DateTimeFormat('en-US', {
      year: 'numeric',
      month: 'short', 
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      ...options,
    }).format(date);
  },
};

export const apiClient = new APIClient();
