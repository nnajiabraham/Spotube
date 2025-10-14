// Custom HTTP client to replace PocketBase SDK
// Provides similar interface with typed methods for API communication

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8090';

export interface APIErrorResponse {
  error: {
    code: string;
    message: string;
  };
}

export class APIClient {
  private baseURL: string;
  private csrfToken: string | null = null;

  constructor(baseURL = API_BASE_URL) {
    this.baseURL = baseURL;
  }

  // Get CSRF token for state-changing requests
  private async getCSRFToken(): Promise<string> {
    if (this.csrfToken) {
      return this.csrfToken;
    }

    try {
      // Try to get from cookie first
      const cookieToken = this.getCSRFFromCookie();
      if (cookieToken) {
        this.csrfToken = cookieToken;
        return cookieToken;
      }

      // Fallback: fetch from API
      const response = await this.fetch('/api/csrf', {
        method: 'GET',
        credentials: 'include',
      });
      
      if (response.ok) {
        const data = await response.json();
        this.csrfToken = data.csrf || '';
        return this.csrfToken || '';
      }
    } catch (error) {
      console.warn('Failed to get CSRF token:', error);
    }

    return '';
  }

  private getCSRFFromCookie(): string | null {
    const match = document.cookie.match(/spotube_csrf=([^;]+)/);
    return match ? match[1] : null;
  }

  // Core fetch wrapper with error handling and CSRF
  private async fetch(endpoint: string, options: RequestInit = {}): Promise<Response> {
    const url = endpoint.startsWith('http') ? endpoint : `${this.baseURL}${endpoint}`;
    
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string> || {}),
    };

    // Add CSRF token for non-GET requests
    if (options.method && options.method !== 'GET') {
      const csrfToken = await this.getCSRFToken();
      if (csrfToken) {
        headers['X-CSRF-Token'] = csrfToken;
      }
    }

    const config: RequestInit = {
      ...options,
      headers,
      credentials: 'include', // Include cookies for session auth
    };

    const response = await fetch(url, config);

    // Handle error responses
    if (!response.ok) {
      const errorData = await response.json().catch(() => null);
      if (errorData && errorData.error) {
        throw new APIClientError(errorData.error.code, errorData.error.message);
      }
      throw new APIClientError('http_error', `HTTP ${response.status}: ${response.statusText}`);
    }

    return response;
  }

  // GET request
  async get<T>(endpoint: string): Promise<T> {
    const response = await this.fetch(endpoint, { method: 'GET' });
    return response.json();
  }

  // POST request
  async post<T>(endpoint: string, data?: unknown): Promise<T> {
    const response = await this.fetch(endpoint, {
      method: 'POST',
      body: data ? JSON.stringify(data) : undefined,
    });
    
    if (response.status === 204) {
      return null as T; // No content responses
    }
    
    return response.json();
  }

  // PATCH request
  async patch<T>(endpoint: string, data: unknown): Promise<T> {
    const response = await this.fetch(endpoint, {
      method: 'PATCH',
      body: JSON.stringify(data),
    });
    return response.json();
  }

  // DELETE request
  async delete(endpoint: string): Promise<void> {
    await this.fetch(endpoint, { method: 'DELETE' });
  }

  // URL helper for building query parameters
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

// Timestamp conversion utilities
export const DateUtils = {
  // Convert Date to epoch seconds for API requests
  toEpoch(date: Date | string): number {
    return Math.floor(new Date(date).getTime() / 1000);
  },

  // Convert epoch seconds to Date for frontend use
  fromEpoch(timestamp: number): Date {
    return new Date(timestamp * 1000);
  },

  // Format for display (local timezone)
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

// Global client instance
export const apiClient = new APIClient();
