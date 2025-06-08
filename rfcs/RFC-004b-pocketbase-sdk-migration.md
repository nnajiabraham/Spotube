# RFC-004b: PocketBase JS SDK Migration

**Status:** Draft  
**Branch:** `rfc/004b-pocketbase-sdk-migration`  
**Related Issues:** _n/a_  
**Depends On:** RFC-004 (Spotify OAuth Integration)

---

## 1. Goal

Migrate the frontend from using a custom API client (`frontend/src/lib/api.ts`) to the official [PocketBase JavaScript SDK](https://github.com/pocketbase/js-sdk). This will provide better type safety, automatic token management, real-time subscriptions support, and alignment with PocketBase best practices.

## 2. Background & Context

RFC-004 implemented Spotify OAuth integration using a custom fetch-based API client. While functional, this approach:
- Requires manual token management
- Lacks built-in error handling for PocketBase-specific responses
- Doesn't support real-time subscriptions (needed for future features)
- Duplicates functionality already provided by the official SDK

The PocketBase JS SDK provides:
- Automatic auth token management with `authStore`
- Built-in error handling and typed responses
- Real-time subscriptions via SSE
- File upload/download helpers
- Request interceptors and hooks

Current implementation context from RFC-004:
- Custom API client at `frontend/src/lib/api.ts` with methods:
  - `getSetupStatus()` - checks if setup is required
  - `getSpotifyPlaylists()` - fetches user's Spotify playlists
- Components using the API client:
  - `SpotifyConnectionCard` - checks auth status
  - `SpotifyPlaylists` - displays playlist data
  - Dashboard route - handles OAuth callback
- MSW mocks configured for all API endpoints

## 3. Technical Design

### 3.1 Install PocketBase SDK
```bash
cd frontend
npm install pocketbase --save
```

### 3.2 Create PocketBase Client Instance
Create `frontend/src/lib/pocketbase.ts`:
```typescript
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
```

### 3.3 Migrate API Methods
Update API client to use PocketBase SDK while maintaining the same interface:

```typescript
// frontend/src/lib/api.ts
import { pb, SetupStatus, PlaylistsResponse } from './pocketbase';

export class ApiError extends Error {
  status: number;
  
  constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

export const api = {
  // Setup API
  getSetupStatus: async (): Promise<SetupStatus> => {
    try {
      const response = await pb.send('/api/setup/status', {
        method: 'GET',
      });
      return response as SetupStatus;
    } catch (error: any) {
      throw new ApiError(error.status || 500, error.message || 'Request failed');
    }
  },
  
  // Spotify API
  getSpotifyPlaylists: async (params?: { limit?: number; offset?: number }): Promise<PlaylistsResponse> => {
    try {
      const searchParams = new URLSearchParams();
      if (params?.limit) searchParams.set('limit', params.limit.toString());
      if (params?.offset) searchParams.set('offset', params.offset.toString());
      
      const response = await pb.send('/api/spotify/playlists', {
        method: 'GET',
        query: searchParams,
      });
      return response as PlaylistsResponse;
    } catch (error: any) {
      throw new ApiError(error.status || 500, error.message || 'Request failed');
    }
  },
};
```

### 3.4 Update MSW Handlers
The MSW handlers need minimal changes since we're maintaining the same API interface. However, we should ensure they handle PocketBase response format if needed.

### 3.5 Future Enhancements (Out of Scope)
These can be addressed in future RFCs:
- Migrate to PocketBase collections API (e.g., `pb.collection('playlists').getList()`)
- Use PocketBase auth store for token management
- Implement real-time subscriptions for sync status

## 4. Dependencies

- `pocketbase` (^0.21.0 or latest) - [NPM](https://www.npmjs.com/package/pocketbase)

## 5. Checklist

- [ ] **M1** Install PocketBase JS SDK dependency
- [ ] **M2** Create PocketBase client instance with TypeScript types
- [ ] **M3** Update `api.getSetupStatus()` to use PocketBase SDK
- [ ] **M4** Update `api.getSpotifyPlaylists()` to use PocketBase SDK
- [ ] **M5** Verify all existing unit tests pass with MSW mocks
- [ ] **M6** Test real app with Playwright MCP tool
- [ ] **M7** Update any error handling to work with PocketBase error format
- [ ] **M8** Document migration in Implementation Notes with examples

## 6. Definition of Done

* All API calls use PocketBase SDK instead of custom fetch
* Existing functionality remains unchanged
* All tests (unit and E2E) pass without modification
* MSW mocks continue to work correctly
* No regression in user experience

## Implementation Notes / Summary

* This RFC focuses on minimal migration - replacing fetch with PocketBase SDK
* Maintains existing API interface to minimize component changes
* Future RFCs can leverage more PocketBase features (collections API, realtime, etc.)
* MSW mocks remain unchanged since we're keeping the same API structure

## Resources & References

* [PocketBase JavaScript SDK](https://github.com/pocketbase/js-sdk) - Official SDK documentation
* [PocketBase JS SDK API Docs](https://github.com/pocketbase/js-sdk#pocketbase-javascript-sdk) - Detailed API reference
* [MSW Documentation](https://mswjs.io/) - For maintaining test mocks
* RFC-004 Implementation - Current API client structure and usage patterns

---

*End of RFC-004b* 