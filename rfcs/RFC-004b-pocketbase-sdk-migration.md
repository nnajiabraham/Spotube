# RFC-004b: PocketBase JS SDK Migration

**Status:** Done  
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

- [X] **M1** Install PocketBase JS SDK dependency
- [X] **M2** Create PocketBase client instance with TypeScript types
- [X] **M3** Update `api.getSetupStatus()` to use PocketBase SDK
- [X] **M4** Update `api.getSpotifyPlaylists()` to use PocketBase SDK
- [X] **M5** Verify all existing unit tests pass with MSW mocks
- [X] **M6** Test real app with Playwright MCP tool (Skipped)
- [X] **M7** Update any error handling to work with PocketBase error format
- [X] **M8** Document migration in Implementation Notes with examples

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

**M1 COMPLETED** - Installed PocketBase JS SDK dependency:
* Executed `npm install pocketbase --save` within the `frontend` directory.
* `package.json` and `package-lock.json` were updated with the new dependency (`pocketbase: ^0.21.3`).

**M2 COMPLETED** - Created PocketBase client instance:
* Created `frontend/src/lib/pocketbase.ts`.
* Added PocketBase client initialization pointing to `VITE_API_URL`.
* Defined and exported TypeScript interfaces for `SetupStatus`, `SpotifyPlaylist`, and `PlaylistsResponse` to be used across the application.

**M3 & M4 COMPLETED** - Updated API methods to use PocketBase SDK:
* Refactored `frontend/src/lib/api.ts` to use the new PocketBase client instance (`pb`).
* Replaced `fetch()` calls in `getSetupStatus` and `getSpotifyPlaylists` with `pb.send()`.
* The existing `ApiError` class is preserved for consistent error handling in the UI components.
* Updated `catch` blocks to handle errors of `unknown` type and cast them to access `status` and `message` properties, making the code more type-safe and compliant with linter rules.
* Removed the now-unused `handleResponse` helper function and `API_BASE_URL` constant.

**M5 COMPLETED** - Verified existing unit tests pass:
* Executed `npm run test:run` in the `frontend` directory.
* All 10 existing unit tests in 2 test suites (`setup-schema.test.ts` and `SpotifyConnectionCard.test.tsx`) passed successfully.
* This confirms that the migration to the PocketBase SDK in `api.ts` did not break the components that rely on it, as the MSW mocks correctly intercepted the API calls.

**M6 COMPLETED** - E2E testing was skipped as per the user's request due to failures in the test suite that are outside the scope of this RFC. The existing E2E test suite will be addressed in a future update.

**M7 COMPLETED** - Error handling updated for PocketBase SDK:
* The `catch` blocks within the `api.ts` methods were updated to handle `unknown` error types thrown by the PocketBase SDK.
* Errors are cast to a generic error shape (`{ status?: number, message?: string }`) and then re-thrown as the application's custom `ApiError`. This ensures that UI components continue to work with the existing error handling logic while being compatible with the errors from the new SDK.

**M8 COMPLETED** - Migration documented with implementation notes:
* The "Implementation Notes / Summary" section of this RFC has been updated sequentially after each step (M1-M7).
* The notes provide a detailed log of all changes, including commands run, files created/modified, and key decisions made during the migration. This document now serves as a comprehensive record of the implementation.

## Resources & References

* [PocketBase JavaScript SDK](https://github.com/pocketbase/js-sdk) - Official SDK documentation
* [PocketBase JS SDK API Docs](https://github.com/pocketbase/js-sdk#pocketbase-javascript-sdk) - Detailed API reference
* [MSW Documentation](https://mswjs.io/) - For maintaining test mocks
* RFC-004 Implementation - Current API client structure and usage patterns

---

*End of RFC-004b* 